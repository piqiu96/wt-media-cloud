// Package repository persists executor-neutral file-transfer tasks.
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("file transfer task not found")

// taskColumnList is the one column list every task read shares, in the order
// `scanTask` expects. A column added to the table has to be added here to be
// visible at all, which is the point: with a list per query, a column reaches
// the callers that remembered it and is silently missing from the ones that did
// not.
const taskColumnList = `id, team_id, asset_type, asset_id, asset_title, source_object_key, purpose, execution_scope, status, requested_by, assigned_node_id, claimed_by_node_id, dependency_task_id, total_bytes, transferred_bytes, speed_bytes_per_sec, eta_seconds, attempt_count, max_attempts, lease_expires_at, heartbeat_at, started_at, finished_at, cancel_requested_at, expected_sha256, file_name, error_code, error_message, integrity_sha256, integrity_bytes, created_at, updated_at`

type CreateTaskInput struct {
	ID               string
	TeamID           identity.TeamID
	AssetType        model.AssetType
	AssetID          int64
	AssetTitle       string
	SourceObjectKey  string
	Purpose          model.Purpose
	ExecutionScope   model.ExecutionScope
	RequestedBy      identity.UserID
	AssignedNodeID   string
	DependencyTaskID string
	DedupeKey        string
	TotalBytes       int64
	ExpectedSHA256   string
	MaxAttempts      int
}

// TaskFilter narrows a task listing. Every field is optional and is combined
// with AND; an empty filter is not scoped to anything, so callers that need a
// boundary must set one.
type TaskFilter struct {
	RequestedBy *identity.UserID
	AssetID     *int64
	Statuses    []model.Status
	Purposes    []model.Purpose
	Limit       int
}

type ProgressInput struct {
	TaskID           string
	NodeID           string
	TransferredBytes int64
	TotalBytes       int64
	SpeedBytesPerSec int64
	ETASeconds       int64
}

type CompletionInput struct {
	TaskID   string
	NodeID   string
	Bytes    int64
	SHA256   string
	FileName string
}

type FailureInput struct {
	TaskID       string
	NodeID       string
	Status       model.Status
	ErrorCode    string
	ErrorMessage string
}

func CreateTask(input CreateTaskInput, now time.Time) (model.Task, error) {
	return createTask(database.DB(), input, now)
}

// createTask writes the task as a durable instruction rather than a pointer into
// `materials`: the title, object key, total size and expected hash are copied in
// here because this module may not read the production tables, and because a
// later rename or re-preparation must not change the download a user was already
// promised.
//
// The insert is idempotent on `dedupe_key`, so a repeat click while the first
// transfer is still outstanding resolves to the same row rather than creating a
// second one. `ON DUPLICATE KEY UPDATE id = id` is a deliberate no-op: it turns
// the unique-key collision into a harmless statement, and it does not rewrite
// the instruction, because re-running a task is `RetryTask`'s job and doing it
// here would let a second click mid-transfer retarget a task an executor is
// already running.
//
// The row is then read back by `dedupe_key`, not by the id we tried to insert.
// On a collision the surviving row keeps its own id — the caller's freshly
// generated id was never written — so reading by id would report a missing task
// for a request that succeeded.
func createTask(db *gorm.DB, input CreateTaskInput, now time.Time) (model.Task, error) {
	if err := validateCreateInput(input); err != nil {
		return model.Task{}, err
	}
	if err := db.Exec(`INSERT INTO file_transfer_tasks (id, team_id, asset_type, asset_id, asset_title, source_object_key, purpose, execution_scope, status, requested_by, assigned_node_id, dependency_task_id, dedupe_key, total_bytes, expected_sha256, max_attempts, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?, 'pending', ?,?,?,?,?,?, ?,?,?) ON DUPLICATE KEY UPDATE id = id`, input.ID, input.TeamID, input.AssetType, input.AssetID, nullIfEmpty(input.AssetTitle), nullIfEmpty(input.SourceObjectKey), input.Purpose, input.ExecutionScope, input.RequestedBy, nullIfEmpty(input.AssignedNodeID), nullIfEmpty(input.DependencyTaskID), input.DedupeKey, input.TotalBytes, nullIfEmpty(input.ExpectedSHA256), input.MaxAttempts, now, now).Error; err != nil {
		return model.Task{}, err
	}
	return selectTask(db, "dedupe_key = ?", input.DedupeKey)
}

func GetTask(taskID string) (model.Task, error) {
	return getTask(database.DB(), taskID)
}

func getTask(db *gorm.DB, taskID string) (model.Task, error) {
	if strings.TrimSpace(taskID) == "" {
		return model.Task{}, ErrNotFound
	}
	return selectTask(db, "id = ?", taskID)
}

func ListTasks(filter TaskFilter) ([]model.Task, error) {
	return listTasks(database.DB(), filter)
}

func listTasks(db *gorm.DB, filter TaskFilter) ([]model.Task, error) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 8)
	if filter.RequestedBy != nil {
		conditions = append(conditions, "requested_by = ?")
		args = append(args, *filter.RequestedBy)
	}
	if filter.AssetID != nil {
		conditions = append(conditions, "asset_id = ?")
		args = append(args, *filter.AssetID)
	}
	if statuses := nonEmptyStatuses(filter.Statuses); len(statuses) > 0 {
		conditions = append(conditions, "status IN ("+placeholders(len(statuses))+")")
		for _, status := range statuses {
			args = append(args, status)
		}
	}
	if purposes := nonEmptyPurposes(filter.Purposes); len(purposes) > 0 {
		conditions = append(conditions, "purpose IN ("+placeholders(len(purposes))+")")
		for _, purpose := range purposes {
			args = append(args, purpose)
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	// created_at alone is not a total order: two tasks created in the same
	// microsecond would come back in an arbitrary order, and a client polling
	// this list would see rows swap places. id breaks the tie deterministically.
	query := "SELECT " + taskColumnList + " FROM file_transfer_tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d", limit)
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]model.Task, 0)
	for rows.Next() {
		var task model.Task
		if err := scanTask(rows, &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// leaseable is the single statement of "this task can be leased right now". It is
// a shared const rather than a phrase repeated per query because it is a rule, not
// an implementation detail: three claims and the candidate read below all have to
// agree on it, and a predicate that drifts in one of them produces a task that one
// path leases and another refuses to continue — the exact stuck-`running` state the
// two clauses below exist to prevent.
//
// `cancel_requested_at IS NULL`. Without it, cancelling a running task and then
// losing its executor (the node dies, the lease expires) makes the task claimable
// again — and every predicate the new owner would use to report progress,
// heartbeat, or complete also requires `cancel_requested_at IS NULL`. So the new
// owner could take the lease and then be refused by all three, and the task would
// stay `running` forever with nothing able to finish it. A task that has been asked
// to stop is not leasable at all; the reconcilers move it to its terminal state.
//
// `attempt_count < max_attempts`. `attempt_count` is incremented by a claim, so
// without the bound a task whose lease keeps expiring is retried forever and
// `max_attempts` would only ever constrain an explicit user retry. The bound is why
// `ReconcileExhaustedTasks` exists: a task that has used all its attempts and lost
// its executor has no claimable owner left, and must be terminated by someone.
const leaseable = `cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))`

func ClaimLocalTask(taskID, nodeID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	return claimLocalTask(database.DB(), taskID, nodeID, now, lease)
}

// claimLocalTask leases one task the caller named, to the node it was assigned.
func claimLocalTask(db *gorm.DB, taskID, nodeID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(nodeID) == "" || lease <= 0 {
		return model.Task{}, false, fmt.Errorf("invalid local transfer claim")
	}
	leaseUntil := now.Add(lease)
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'local_agent' AND assigned_node_id = ? AND `+leaseable, nodeID, leaseUntil, now, now, now, taskID, nodeID, now)
	if result.Error != nil {
		return model.Task{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return model.Task{}, false, nil
	}
	task, err := getTask(db, taskID)
	return task, err == nil, err
}

// NextLocalTask reports the oldest task waiting for one local node, without
// leasing it.
//
// It exists because a lease carries a download grant. Minting the grant after the
// lease would leave a task recorded as `running` with no executor holding anything
// whenever minting failed — and with `attempt_count` already incremented, so a
// storage outage would spend the user's retry budget and strand the row until the
// lease expired. Reading the candidate first costs one query, and one wasted
// presign when the claim below loses its race (a local HMAC computation), and in
// exchange a storage failure touches nothing at all.
//
// The caller must still lease through `ClaimLocalTask`, which re-asserts this same
// predicate: this function only says what is *probably* available.
func NextLocalTask(nodeID string, now time.Time) (model.Task, bool, error) {
	return nextLocalTask(database.DB(), nodeID, now)
}

func nextLocalTask(db *gorm.DB, nodeID string, now time.Time) (model.Task, bool, error) {
	if strings.TrimSpace(nodeID) == "" {
		return model.Task{}, false, fmt.Errorf("invalid local transfer lookup")
	}
	// The order is `created_at, id` rather than `created_at` alone: two tasks
	// created in the same microsecond would otherwise be picked in an arbitrary
	// order, and a node polling this could be handed a different one each time.
	var taskID string
	err := db.Raw(`SELECT id FROM file_transfer_tasks WHERE execution_scope = 'local_agent' AND assigned_node_id = ? AND `+leaseable+` ORDER BY created_at ASC, id ASC LIMIT 1`, nodeID, now).Row().Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, false, nil
	}
	if err != nil {
		return model.Task{}, false, err
	}
	task, err := getTask(db, taskID)
	if err != nil {
		return model.Task{}, false, err
	}
	return task, true, nil
}

// ClaimCloudTask leases the oldest Cloud-scope task that is waiting or whose
// lease has expired, and reports whether it found one.
//
// A Cloud worker has no task id to offer — nothing told it which task to run, it
// is polling — so this takes no task id and picks the queue head instead.
//
// The pick and the lease are two statements rather than one `UPDATE ... ORDER BY
// ... LIMIT 1`, because the caller needs to know *which* task it got and MySQL's
// multi-row UPDATE cannot report the row it chose. Safety comes from the second
// statement re-asserting the whole predicate: if another worker took the
// candidate in between, this update matches nothing and the caller is told there
// was no task, then polls again. Losing a race must not look like a failure.
func ClaimCloudTask(workerID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	return claimCloudTask(database.DB(), workerID, now, lease)
}

func claimCloudTask(db *gorm.DB, workerID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	if strings.TrimSpace(workerID) == "" || lease <= 0 {
		return model.Task{}, false, fmt.Errorf("invalid cloud transfer claim")
	}
	var taskID string
	err := db.Raw(`SELECT id FROM file_transfer_tasks WHERE execution_scope = 'cloud' AND `+leaseable+` ORDER BY created_at ASC, id ASC LIMIT 1`, now).Row().Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, false, nil
	}
	if err != nil {
		return model.Task{}, false, err
	}
	leaseUntil := now.Add(lease)
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'cloud' AND `+leaseable, workerID, leaseUntil, now, now, now, taskID, now)
	if result.Error != nil {
		return model.Task{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return model.Task{}, false, nil
	}
	task, err := getTask(db, taskID)
	return task, err == nil, err
}

func ReportProgress(input ProgressInput, now time.Time) (bool, error) {
	return reportProgress(database.DB(), input, now)
}

func reportProgress(db *gorm.DB, input ProgressInput, now time.Time) (bool, error) {
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.NodeID) == "" || input.TransferredBytes < 0 || input.TotalBytes < 0 || input.SpeedBytesPerSec < 0 || input.ETASeconds < 0 {
		return false, fmt.Errorf("invalid transfer progress")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, speed_bytes_per_sec = ?, eta_seconds = ?, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND cancel_requested_at IS NULL AND (lease_expires_at IS NULL OR lease_expires_at > ?) AND transferred_bytes <= ?`, input.TransferredBytes, input.TotalBytes, input.SpeedBytesPerSec, input.ETASeconds, now, now, input.TaskID, input.NodeID, now, input.TransferredBytes)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// HeartbeatTask renews only the current executor's unexpired lease. A stale or
// cancelled executor cannot revive its task through heartbeat traffic.
func HeartbeatTask(taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	return heartbeatTask(database.DB(), taskID, nodeID, now, lease)
}

func heartbeatTask(db *gorm.DB, taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(nodeID) == "" || lease <= 0 {
		return false, fmt.Errorf("invalid transfer heartbeat")
	}
	leaseUntil := now.Add(lease)
	result := db.Exec(`UPDATE file_transfer_tasks SET heartbeat_at = ?, lease_expires_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND cancel_requested_at IS NULL AND (lease_expires_at IS NULL OR lease_expires_at > ?)`, now, leaseUntil, now, taskID, nodeID, now)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func CompleteTask(input CompletionInput, now time.Time) (bool, error) {
	return completeTask(database.DB(), input, now)
}

// completeTask records the terminal success of a transfer, and it is the only
// write that can produce `success`.
//
// The completion has to agree with what the task declared, so both expectations
// are asserted in the predicate rather than trusted from the caller: the byte
// count must match `total_bytes` when one is known, and the hash must match
// `expected_sha256` when one was recorded. The service refuses a mismatch with
// the frozen `transfer_integrity_failed` first; this predicate is the same rule
// stated where the row can enforce it, so a race that slipped past the service
// still cannot mark unverified bytes as a success.
func completeTask(db *gorm.DB, input CompletionInput, now time.Time) (bool, error) {
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.NodeID) == "" || input.Bytes < 0 || len(strings.TrimSpace(input.SHA256)) != 64 {
		return false, fmt.Errorf("invalid transfer completion")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'success', transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, integrity_sha256 = ?, integrity_bytes = ?, file_name = COALESCE(?, file_name), finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND cancel_requested_at IS NULL AND (total_bytes = 0 OR total_bytes = ?) AND (expected_sha256 IS NULL OR expected_sha256 = ?)`, input.Bytes, input.Bytes, input.SHA256, input.Bytes, nullIfEmpty(input.FileName), now, now, now, input.TaskID, input.NodeID, input.Bytes, input.SHA256)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// CancelTask asks for a transfer to stop, from the user who requested it.
func CancelTask(taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (bool, error) {
	return cancelTask(database.DB(), taskID, teamID, requestedBy, now)
}

// cancelTask encodes a two-phase cancellation, and the CASE arms are the two
// phases:
//
//   - `pending` has no executor, so there is nobody to ask: the row becomes
//     `cancelled` outright and the terminal fields are written here.
//   - `running` has an executor mid-transfer. Writing `cancelled` here would
//     race bytes still arriving from an executor that has not been told, so only
//     `cancel_requested_at` is set. Every `reportProgress`/`heartbeatTask`/
//     `completeTask` predicate already carries `cancel_requested_at IS NULL`,
//     so the claimant's next call fails and the claimant itself writes the
//     terminal `cancelled` through `failTask`.
//
// That split is deliberate, and it is why the terminal write for a running task
// is absent here: the executor owns the transition out of `running`, and moving
// it to the requester would mark a task cancelled while it was still being
// written to disk. If the executor never comes back to claim that transition,
// `ReconcileCancelledTasks` finishes it.
//
// Scope is asserted in the WHERE clause rather than checked separately, so a
// wrong team or a different user's id is simply an unmatched row — there is no
// permission check a caller could forget to run.
func cancelTask(db *gorm.DB, taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (bool, error) {
	if strings.TrimSpace(taskID) == "" || teamID <= 0 || requestedBy <= 0 {
		return false, fmt.Errorf("invalid transfer cancellation")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = CASE WHEN status = 'pending' THEN 'cancelled' ELSE status END, cancel_requested_at = CASE WHEN status = 'running' THEN ? ELSE cancel_requested_at END, finished_at = CASE WHEN status = 'pending' THEN ? ELSE finished_at END, lease_expires_at = CASE WHEN status = 'pending' THEN NULL ELSE lease_expires_at END, error_code = CASE WHEN status = 'pending' THEN 'cancelled_by_user' ELSE error_code END, error_message = CASE WHEN status = 'pending' THEN 'cancelled by user' ELSE error_message END, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status IN ('pending', 'running')`, now, now, now, taskID, teamID, requestedBy)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// RetryTask requeues a failed transfer for the user who asked for it, within its
// bounded attempt count, and returns the requeued task.
//
// `attempt_count` is not reset: it is the count of attempts this task has
// already consumed, and clearing it would make `max_attempts` unbounded through
// repeated retries — the exact thing a bound is for. The task becomes `pending`
// again with the executor state cleared, because a previous executor's lease,
// heartbeat and byte counts describe bytes that are no longer known to be on
// disk.
func RetryTask(taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (model.Task, bool, error) {
	return retryTask(database.DB(), taskID, teamID, requestedBy, now)
}

func retryTask(db *gorm.DB, taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (model.Task, bool, error) {
	if strings.TrimSpace(taskID) == "" || teamID <= 0 || requestedBy <= 0 {
		return model.Task{}, false, fmt.Errorf("invalid transfer retry")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'pending', claimed_by_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, started_at = NULL, finished_at = NULL, cancel_requested_at = NULL, transferred_bytes = 0, speed_bytes_per_sec = 0, eta_seconds = NULL, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status = 'failed' AND attempt_count < max_attempts`, now, taskID, teamID, requestedBy)
	if result.Error != nil {
		return model.Task{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return model.Task{}, false, nil
	}
	task, err := getTask(db, taskID)
	return task, err == nil, err
}

// ReconcileCancelledTasks finishes the cancellation of running tasks whose
// executor never came back to confirm it.
//
// Cancelling a running transfer only records the request, because the executor
// owns the transition out of `running` (see `cancelTask`). An executor that
// stopped reporting — the machine was closed, the process was killed — will
// never make that transition, and since a task with a cancellation request is
// not leasable, nothing else can either. This is the caller that ends it, and it
// waits for the lease to expire first so a live executor's own report wins.
func ReconcileCancelledTasks(now time.Time) (int64, error) {
	return reconcileCancelledTasks(database.DB(), now)
}

func reconcileCancelledTasks(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'cancelled', finished_at = ?, lease_expires_at = NULL, error_code = 'cancelled_by_user', error_message = 'cancelled by user', updated_at = ? WHERE status = 'running' AND cancel_requested_at IS NOT NULL AND (lease_expires_at IS NULL OR lease_expires_at <= ?)`, now, now, now)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ReconcileExhaustedTasks fails running tasks that have used every attempt and
// lost their executor.
//
// The bound in the claim predicate (`attempt_count < max_attempts`) is what
// makes this necessary: without it a task whose lease keeps expiring is retried
// forever; with it, the last expiry leaves a task that no one may claim. The
// outcome is a failure, not a cancellation — nobody asked for it to stop, the
// executor stopped reporting — so it is recorded as `failed` with its own error
// code rather than being folded into the cancellation path.
func ReconcileExhaustedTasks(now time.Time) (int64, error) {
	return reconcileExhaustedTasks(database.DB(), now)
}

func reconcileExhaustedTasks(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'failed', error_code = 'lease_expired', error_message = 'transfer lease expired without a reporting executor', finished_at = ?, lease_expires_at = NULL, updated_at = ? WHERE status = 'running' AND cancel_requested_at IS NULL AND attempt_count >= max_attempts AND (lease_expires_at IS NULL OR lease_expires_at <= ?)`, now, now, now)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// FailTask records a failed or cancelled terminal outcome only from the node
// that currently owns the running task lease.
func FailTask(input FailureInput, now time.Time) (bool, error) {
	return failTask(database.DB(), input, now)
}

// failTask is deliberately the one terminal write with no
// `cancel_requested_at IS NULL` predicate, unlike `reportProgress`,
// `heartbeatTask` and `completeTask`.
//
// The asymmetry is the mechanism, not an oversight. A cancellation request only
// refuses the executor's *continuation* calls; the executor itself still has to
// write the terminal `cancelled` row, because it is the only party that knows
// whether it has stopped writing bytes. If this predicate carried the same
// clause, a cancelled running transfer would have no path to a terminal state at
// all — every writer would be refused and the row would sit in `running` until a
// reconciliation timeout that the design does not otherwise need.
func failTask(db *gorm.DB, input FailureInput, now time.Time) (bool, error) {
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.NodeID) == "" || (input.Status != model.StatusFailed && input.Status != model.StatusCancelled) || strings.TrimSpace(input.ErrorCode) == "" || len(input.ErrorCode) > 128 || len(input.ErrorMessage) > 500 {
		return false, fmt.Errorf("invalid transfer terminal failure")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = ?, error_code = ?, error_message = ?, finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ?`, input.Status, input.ErrorCode, input.ErrorMessage, now, now, now, input.TaskID, input.NodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func selectTask(db *gorm.DB, where string, args ...any) (model.Task, error) {
	var task model.Task
	err := scanTask(db.Raw(`SELECT `+taskColumnList+` FROM file_transfer_tasks WHERE `+where, args...).Row(), &task)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, ErrNotFound
	}
	return task, err
}

func validateCreateInput(input CreateTaskInput) error {
	if strings.TrimSpace(input.ID) == "" || input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 || strings.TrimSpace(input.DedupeKey) == "" || input.MaxAttempts <= 0 {
		return fmt.Errorf("invalid file transfer task")
	}
	if input.AssetType != model.AssetMaterial {
		return fmt.Errorf("unsupported transfer asset type")
	}
	if (input.Purpose == model.PurposeComposeInputPrepare && input.ExecutionScope != model.ExecutionCloud) || (input.Purpose == model.PurposeUserDownload && input.ExecutionScope != model.ExecutionLocalAgent) {
		return fmt.Errorf("invalid transfer purpose and execution scope")
	}
	if input.Purpose != model.PurposeComposeInputPrepare && input.Purpose != model.PurposeUserDownload {
		return fmt.Errorf("unsupported transfer purpose")
	}
	if strings.TrimSpace(input.AssetTitle) == "" {
		return fmt.Errorf("transfer task requires the asset title the executor names its file with")
	}
	if input.TotalBytes < 0 {
		return fmt.Errorf("invalid transfer total size")
	}
	if input.ExecutionScope == model.ExecutionLocalAgent && strings.TrimSpace(input.AssignedNodeID) == "" {
		return fmt.Errorf("local transfer requires assigned node")
	}
	// A local transfer downloads an already-verified object, so the lease can
	// promise a hash to check against and a size to check completeness with. The
	// Cloud prepare task is the one that *produces* those facts, so it may not
	// have them yet: it learns the size and hash while downloading.
	if input.Purpose == model.PurposeUserDownload {
		if strings.TrimSpace(input.SourceObjectKey) == "" {
			return fmt.Errorf("local transfer requires the object to download")
		}
		if input.TotalBytes <= 0 {
			return fmt.Errorf("local transfer requires the declared total size")
		}
		if !isSHA256(input.ExpectedSHA256) {
			return fmt.Errorf("local transfer requires the expected sha256")
		}
	}
	return nil
}

func isSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9', ch >= 'a' && ch <= 'f', ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}

func placeholders(count int) string { return strings.TrimRight(strings.Repeat("?, ", count), ", ") }

func nonEmptyStatuses(statuses []model.Status) []model.Status {
	kept := make([]model.Status, 0, len(statuses))
	for _, status := range statuses {
		if strings.TrimSpace(string(status)) != "" {
			kept = append(kept, status)
		}
	}
	return kept
}

func nonEmptyPurposes(purposes []model.Purpose) []model.Purpose {
	kept := make([]model.Purpose, 0, len(purposes))
	for _, purpose := range purposes {
		if strings.TrimSpace(string(purpose)) != "" {
			kept = append(kept, purpose)
		}
	}
	return kept
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

type rowScanner interface{ Scan(...any) error }

func scanTask(row rowScanner, task *model.Task) error {
	var teamID, requestedBy int64
	var assetType, purpose, scope, status string
	var assetTitle, objectKey, fileName, assignedNode, claimedNode, dependency, errorCode, errorMessage, sha256, expectedSHA256 sql.NullString
	var eta, integrityBytes sql.NullInt64
	var lease, heartbeat, started, finished, cancelRequested sql.NullTime
	if err := row.Scan(&task.ID, &teamID, &assetType, &task.AssetID, &assetTitle, &objectKey, &purpose, &scope, &status, &requestedBy, &assignedNode, &claimedNode, &dependency, &task.TotalBytes, &task.TransferredBytes, &task.SpeedBytesPerSec, &eta, &task.AttemptCount, &task.MaxAttempts, &lease, &heartbeat, &started, &finished, &cancelRequested, &expectedSHA256, &fileName, &errorCode, &errorMessage, &sha256, &integrityBytes, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return err
	}
	task.TeamID = identity.TeamID(teamID)
	task.AssetType = model.AssetType(assetType)
	task.AssetTitle = assetTitle.String
	task.SourceObjectKey = objectKey.String
	task.Purpose = model.Purpose(purpose)
	task.ExecutionScope = model.ExecutionScope(scope)
	task.Status = model.Status(status)
	task.RequestedBy = identity.UserID(requestedBy)
	task.AssignedNodeID = assignedNode.String
	task.ClaimedByNodeID = claimedNode.String
	task.DependencyTaskID = dependency.String
	task.ExpectedSHA256 = expectedSHA256.String
	task.FileName = fileName.String
	task.ErrorCode = errorCode.String
	task.ErrorMessage = errorMessage.String
	task.IntegritySHA256 = sha256.String
	if eta.Valid {
		value := eta.Int64
		task.ETASeconds = &value
	}
	if integrityBytes.Valid {
		value := integrityBytes.Int64
		task.IntegrityBytes = &value
	}
	if lease.Valid {
		task.LeaseExpiresAt = &lease.Time
	}
	if heartbeat.Valid {
		task.HeartbeatAt = &heartbeat.Time
	}
	if started.Valid {
		task.StartedAt = &started.Time
	}
	if finished.Valid {
		task.FinishedAt = &finished.Time
	}
	if cancelRequested.Valid {
		task.CancelRequestedAt = &cancelRequested.Time
	}
	return nil
}
