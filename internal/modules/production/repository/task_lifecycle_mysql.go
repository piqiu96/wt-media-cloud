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
	result := db.Exec(`UPDATE compose_tasks SET status = 'cancelled', finished_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running' AND cancel_requested = 1`, now, now, taskID, nodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
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
		return nil
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
	result := db.Exec(`UPDATE compose_tasks SET status = 'pending', claimed_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, stage = '', error_code = ?, error_message = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running'`, errorCode, message, now, taskID, nodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// FailComposeTask records the terminal failure of an attempt the caller owns.
func (ComposeTaskStore) FailComposeTask(taskID, nodeID int64, errorCode, message string) (bool, error) {
	return failComposeTask(database.DB(), taskID, nodeID, errorCode, message, time.Now())
}

func failComposeTask(db *gorm.DB, taskID, nodeID int64, errorCode, message string, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE compose_tasks SET status = 'failed', error_code = ?, error_message = ?, finished_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running'`, errorCode, message, now, now, taskID, nodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// RequeueExpiredComposeLease takes a dead worker's row back: the UPDATE only
// lands when the lease has already lapsed under the previous node, so a live
// worker's row — or one already taken over — cannot be touched by the scan.
// attempt_count is left alone; the next claim re-reads the cap.
func (ComposeTaskStore) RequeueExpiredComposeLease(taskID, previousNodeID int64, errorCode, message string) (bool, error) {
	return requeueExpiredComposeLease(database.DB(), taskID, previousNodeID, errorCode, message, time.Now())
}

func requeueExpiredComposeLease(db *gorm.DB, taskID, previousNodeID int64, errorCode, message string, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE compose_tasks SET status = 'pending', claimed_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, stage = '', error_code = ?, error_message = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running' AND lease_expires_at < ?`, errorCode, message, now, taskID, previousNodeID, now)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// FailExpiredComposeLease terminalizes a dead worker's row whose attempt
// budget is spent, under the same lapsed-lease guard as the requeue.
func (ComposeTaskStore) FailExpiredComposeLease(taskID, previousNodeID int64, errorCode, message string) (bool, error) {
	return failExpiredComposeLease(database.DB(), taskID, previousNodeID, errorCode, message, time.Now())
}

func failExpiredComposeLease(db *gorm.DB, taskID, previousNodeID int64, errorCode, message string, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE compose_tasks SET status = 'failed', error_code = ?, error_message = ?, finished_at = ?, updated_at = ? WHERE id = ? AND claimed_node_id = ? AND status = 'running' AND lease_expires_at < ?`, errorCode, message, now, now, taskID, previousNodeID, now)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
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
