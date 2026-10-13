package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

// ErrComposeCommitLost means the success transaction found the task row no
// longer running under the caller — cancelled, reaped or finished elsewhere.
// The output insert rolls back with it, so no asset row survives without a
// success task behind it.
var ErrComposeCommitLost = errors.New("compose output commit lost the task row")

// errNoClaimableTask rolls the claim transaction back (nothing was written)
// and is translated to found=false at the boundary.
var errNoClaimableTask = errors.New("no claimable compose task")

// Worker lifecycle writes for compose tasks (engineering spec §5). Every
// mutation is a conditional UPDATE keyed by the current status and the
// claiming node, so a worker whose lease has been taken over — or a cancel
// racing the commit — can never move a row it no longer owns. Rows-affected 0
// means "not yours anymore"; the caller reloads and reacts, never retries
// blindly.

// ClaimNextComposeTask atomically claims the oldest claimable pending task:
// one row SELECT ... FOR UPDATE SKIP LOCKED inside a transaction, then a
// conditional UPDATE that flips it to running with the caller's node id and
// fresh lease. attempt_count is incremented here, on every claim, so retries
// are counted by how often work actually started, not by how often failures
// were recorded.
func (ComposeTaskStore) ClaimNextComposeTask(nodeID int64, lease time.Duration) (model.ComposeTask, bool, error) {
	return claimNextComposeTask(database.DB(), nodeID, lease, time.Now())
}

func claimNextComposeTask(db *gorm.DB, nodeID int64, lease time.Duration, now time.Time) (model.ComposeTask, bool, error) {
	var claimed model.ComposeTask
	found := false
	err := db.Transaction(func(tx *gorm.DB) error {
		var taskID int64
		err := tx.Raw(`SELECT id FROM compose_tasks WHERE status = 'pending' AND attempt_count < max_attempts ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Row().Scan(&taskID)
		if errors.Is(err, sql.ErrNoRows) {
			return errNoClaimableTask
		}
		if err != nil {
			return err
		}
		result := tx.Exec(`UPDATE compose_tasks SET status = 'running', claimed_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND status = 'pending'`, nodeID, now.Add(lease), now, now, now, taskID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errNoClaimableTask
		}
		item, foundOne, err := findComposeTask(tx, taskID)
		if err != nil || !foundOne {
			return err
		}
		// One attempt row per claim (M4-G). A manual retry already opened
		// the row as source=manual/pending — the duplicate-key branch only
		// stamps started_at and the running outcome, never the source.
		if err := tx.Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, started_at, created_at, updated_at) VALUES (?, ?, 'auto', 'running', ?, ?, ?) ON DUPLICATE KEY UPDATE started_at = VALUES(started_at), outcome = 'running', updated_at = VALUES(updated_at)`, item.ID, item.AttemptCount, now, now, now).Error; err != nil {
			return err
		}
		claimed, found = item, true
		return nil
	})
	if err != nil {
		if errors.Is(err, errNoClaimableTask) {
			return model.ComposeTask{}, false, nil
		}
		return model.ComposeTask{}, false, err
	}
	return claimed, found, nil
}

// RenewComposeLease extends the caller's own running lease; a worker that
// lost the row (reaped, cancelled, finished) gets false and must stop working
// on it.
func (ComposeTaskStore) RenewComposeLease(taskID, nodeID int64, lease time.Duration) (bool, error) {
	return renewComposeLease(database.DB(), taskID, nodeID, lease, time.Now())
}

func renewComposeLease(db *gorm.DB, taskID, nodeID int64, lease time.Duration, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE compose_tasks SET lease_expires_at = ?, heartbeat_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running'`, now.Add(lease), now, now, taskID, nodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ReportComposeProgress writes the stage projection for a task the caller
// still owns. Progress values are stage boundaries decided by the service —
// the repository does not compute them.
func (ComposeTaskStore) ReportComposeProgress(taskID, nodeID int64, stage string, progress int) (bool, error) {
	return reportComposeProgress(database.DB(), taskID, nodeID, stage, progress, time.Now())
}

func reportComposeProgress(db *gorm.DB, taskID, nodeID int64, stage string, progress int, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE compose_tasks SET stage = ?, progress = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running'`, stage, progress, now, taskID, nodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// RequestComposeCancel flags a running task for the worker's controlled stop
// and audits the request (change.md: audit on cancel). A pending task never
// reaches here — the service cancels it directly with CancelAudited.
func (ComposeTaskStore) RequestComposeCancel(taskID, actorID int64, action string) (bool, error) {
	return requestComposeCancel(database.DB(), taskID, actorID, action, time.Now())
}

func requestComposeCancel(db *gorm.DB, taskID, actorID int64, action string, now time.Time) (bool, error) {
	requested := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_tasks SET cancel_requested = 1, updated_at = ? WHERE id = ? AND status = 'running' AND cancel_requested = 0`, now, taskID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		item, found, err := findComposeTask(tx, taskID)
		if err != nil || !found {
			return err
		}
		requested = true
		return appendComposeTaskAudit(tx, item, action, actorID, now)
	})
	if err != nil {
		return false, err
	}
	return requested, nil
}

// ConfirmComposeCancel is the worker's side of the running-cancel protocol:
// the row becomes cancelled only once the caller has stopped the process and
// disposed temp files, and only if a cancel was actually requested.
func (ComposeTaskStore) ConfirmComposeCancel(taskID, nodeID int64) (bool, error) {
	return confirmComposeCancel(database.DB(), taskID, nodeID, time.Now())
}

func confirmComposeCancel(db *gorm.DB, taskID, nodeID int64, now time.Time) (bool, error) {
	confirmed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_tasks SET status = 'cancelled', finished_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running' AND cancel_requested = 1`, now, now, taskID, nodeID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		confirmed = true
		return closeRunningComposeTaskAttempt(tx, taskID, "cancelled", "", "", now)
	})
	if err != nil {
		return false, err
	}
	return confirmed, nil
}

// CommitComposeOutput persists the verified output and the success flip in
// one transaction (spec §6.3): the output row exists only if the task row is
// still running and un-cancelled under the claiming node, and the task is
// success only if the output row was written.
func (ComposeTaskStore) CommitComposeOutput(out model.CompositeOutput, outputKey string, nodeID int64) (model.ComposeOutputCommit, error) {
	return commitComposeOutput(database.DB(), out, outputKey, nodeID, time.Now())
}

func commitComposeOutput(db *gorm.DB, out model.CompositeOutput, outputKey string, nodeID int64, now time.Time) (model.ComposeOutputCommit, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		out.CreatedAt, out.UpdatedAt = now, now
		if err := tx.Create(&out).Error; err != nil {
			return err
		}
		result := tx.Exec(`UPDATE compose_tasks SET status = 'success', output_object_key = ?, composite_output_id = ?, finished_at = ?, progress = 100, cancel_requested = 0, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running' AND cancel_requested = 0`, outputKey, out.ID, now, now, out.TaskID, nodeID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrComposeCommitLost
		}
		return closeRunningComposeTaskAttempt(tx, out.TaskID, "success", "", "", now)
	})
	if err != nil {
		return model.ComposeOutputCommit{}, err
	}
	return model.ComposeOutputCommit{OutputID: out.ID, TaskID: out.TaskID}, nil
}

// ReleaseComposeForRetry requeues a transiently failed attempt. attempt_count
// stays where the claim left it; the next claim re-reads the attempt cap.
func (ComposeTaskStore) ReleaseComposeForRetry(taskID, nodeID int64, errorCode, message string) (bool, error) {
	return releaseComposeForRetry(database.DB(), taskID, nodeID, errorCode, message, time.Now())
}

func releaseComposeForRetry(db *gorm.DB, taskID, nodeID int64, errorCode, message string, now time.Time) (bool, error) {
	return requeueRunningComposeTask(db, taskID, nodeID, false, errorCode, message, now)
}

// FailComposeTask records the terminal failure of an attempt the caller owns.
func (ComposeTaskStore) FailComposeTask(taskID, nodeID int64, errorCode, message string) (bool, error) {
	return failComposeTask(database.DB(), taskID, nodeID, errorCode, message, time.Now())
}

func failComposeTask(db *gorm.DB, taskID, nodeID int64, errorCode, message string, now time.Time) (bool, error) {
	failed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_tasks SET status = 'failed', error_code = ?, error_message = ?, finished_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running'`, errorCode, message, now, now, taskID, nodeID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		failed = true
		return closeRunningComposeTaskAttempt(tx, taskID, "failed", errorCode, message, now)
	})
	if err != nil {
		return false, err
	}
	return failed, nil
}

// RequeueExpiredComposeLease takes a dead worker's row back: the UPDATE only
// lands when the lease has already lapsed under the previous node, so a live
// worker's row — or one already taken over — cannot be touched by the scan.
// attempt_count is left alone; the next claim re-reads the cap.
func (ComposeTaskStore) RequeueExpiredComposeLease(taskID, previousNodeID int64, errorCode, message string) (bool, error) {
	return requeueExpiredComposeLease(database.DB(), taskID, previousNodeID, errorCode, message, time.Now())
}

func requeueExpiredComposeLease(db *gorm.DB, taskID, previousNodeID int64, errorCode, message string, now time.Time) (bool, error) {
	return requeueRunningComposeTask(db, taskID, previousNodeID, true, errorCode, message, now)
}

// FailExpiredComposeLease terminalizes a dead worker's row whose attempt
// budget is spent, under the same lapsed-lease guard as the requeue.
func (ComposeTaskStore) FailExpiredComposeLease(taskID, previousNodeID int64, errorCode, message string) (bool, error) {
	return failExpiredComposeLease(database.DB(), taskID, previousNodeID, errorCode, message, time.Now())
}

func failExpiredComposeLease(db *gorm.DB, taskID, previousNodeID int64, errorCode, message string, now time.Time) (bool, error) {
	failed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_tasks SET status = 'failed', error_code = ?, error_message = ?, finished_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running' AND lease_expires_at < ?`, errorCode, message, now, now, taskID, previousNodeID, now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		failed = true
		return closeRunningComposeTaskAttempt(tx, taskID, "failed", errorCode, message, now)
	})
	if err != nil {
		return false, err
	}
	return failed, nil
}

// requeueRunningComposeTask sends one owned running attempt back to pending
// (transient failure or reaper takeover). It captures the task's stage and
// attempt number before the update clears them, closes that attempt row as
// failed, and requeues the task — all in one transaction, so a reader never
// sees a pending task whose last attempt is still open. A lapsed lease is
// part of the guard when the caller is the reaper.
func requeueRunningComposeTask(db *gorm.DB, taskID, nodeID int64, lapsed bool, errorCode, message string, now time.Time) (bool, error) {
	requeued := false
	err := db.Transaction(func(tx *gorm.DB) error {
		selectSQL := `SELECT stage, attempt_count FROM compose_tasks WHERE id = ? AND claimed_node_id = ? AND status = 'running'`
		args := []any{taskID, nodeID}
		if lapsed {
			selectSQL += ` AND lease_expires_at < ?`
			args = append(args, now)
		}
		var stage string
		var attempt int64
		err := tx.Raw(selectSQL, args...).Row().Scan(&stage, &attempt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		updateSQL := `UPDATE compose_tasks SET status = 'pending', claimed_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, stage = '', error_code = ?, error_message = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running'`
		updateArgs := []any{errorCode, message, now, taskID, nodeID}
		if lapsed {
			updateSQL += ` AND lease_expires_at < ?`
			updateArgs = append(updateArgs, now)
		}
		result := tx.Exec(updateSQL, updateArgs...)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		requeued = true
		return closeComposeTaskAttemptByNumber(tx, taskID, attempt, "failed", errorCode, message, stage, now)
	})
	if err != nil {
		return false, err
	}
	return requeued, nil
}

// RetryFailed requeues one failed task for another attempt (M4-G, PRD
// §5.8.5): the creation-time snapshot never changes, attempt_count keeps its
// history, max_attempts grows by one, and the manual attempt row is opened
// here so the audit trail shows who authorized the run. The guarded UPDATE
// refuses anything but a failed row whose error code a senior operator may
// retry — the service maps the refusal to compose_task_not_retryable.
func (ComposeTaskStore) RetryFailed(item model.ComposeTask, actorID int64, action string) (bool, error) {
	return retryFailedComposeTask(database.DB(), item.ID, actorID, action, time.Now())
}

func retryFailedComposeTask(db *gorm.DB, taskID, actorID int64, action string, now time.Time) (bool, error) {
	retried := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_tasks SET status = 'pending', claimed_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, cancel_requested = 0, stage = '', progress = 0, started_at = NULL, finished_at = NULL, max_attempts = max_attempts + 1, updated_at = ? WHERE id = ? AND status = 'failed' AND error_code IN ('storage_error', 'task_timeout', 'execution_failed')`, now, taskID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		item, found, err := findComposeTask(tx, taskID)
		if err != nil || !found {
			return err
		}
		if err := tx.Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, created_at, updated_at) VALUES (?, ?, 'manual', 'pending', ?, ?)`, item.ID, item.AttemptCount+1, now, now).Error; err != nil {
			return err
		}
		retried = true
		return appendComposeTaskAudit(tx, item, action, actorID, now)
	})
	if err != nil {
		return false, err
	}
	return retried, nil
}

// closeRunningComposeTaskAttempt closes the current attempt of a task whose
// stage survives the transition (terminal failure, success, worker-confirmed
// cancel) — the JOIN copies the stage the attempt actually reached.
func closeRunningComposeTaskAttempt(tx *gorm.DB, taskID int64, outcome, errorCode, message string, now time.Time) error {
	return tx.Exec(`UPDATE compose_task_attempts a JOIN compose_tasks t ON t.id = a.task_id SET a.outcome = ?, a.error_code = ?, a.error_message = ?, a.finished_at = ?, a.stage = t.stage, a.updated_at = ? WHERE a.task_id = ? AND a.attempt = t.attempt_count AND a.outcome = 'running'`, outcome, errorCode, message, now, now, taskID).Error
}

func closeComposeTaskAttemptByNumber(tx *gorm.DB, taskID, attempt int64, outcome, errorCode, message, stage string, now time.Time) error {
	return tx.Exec(`UPDATE compose_task_attempts SET outcome = ?, error_code = ?, error_message = ?, finished_at = ?, stage = ?, updated_at = ? WHERE task_id = ? AND attempt = ? AND outcome IN ('pending', 'running')`, outcome, errorCode, message, now, stage, now, taskID, attempt).Error
}

// FindExpiredComposeLeases lists running tasks whose lease has lapsed — the
// recovery scan's only input (spec §5.3). It reads; it never mutates.
func (ComposeTaskStore) FindExpiredComposeLeases() ([]model.ComposeTask, error) {
	return findExpiredComposeLeases(database.DB(), time.Now())
}

func findExpiredComposeLeases(db *gorm.DB, now time.Time) ([]model.ComposeTask, error) {
	rows, err := db.Raw(`SELECT `+composeTaskColumns+` FROM compose_tasks WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < ? ORDER BY id`, now).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ComposeTask, 0)
	for rows.Next() {
		item, err := scanComposeTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
