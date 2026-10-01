// Package repository persists executor-neutral file-transfer tasks.
package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
const taskColumnList = `id, team_id, asset_type, asset_id, asset_title, game_name, published_at, source_object_key, purpose, execution_scope, status, requested_by, assigned_node_id, claimed_by_node_id, dependency_task_id, total_bytes, transferred_bytes, speed_bytes_per_sec, eta_seconds, attempt_count, max_attempts, lease_expires_at, heartbeat_at, started_at, finished_at, cancel_requested_at, expected_sha256, file_name, error_code, error_message, integrity_sha256, integrity_bytes, created_at, updated_at`

type CreateTaskInput struct {
	ID               string
	TeamID           identity.TeamID
	AssetType        model.AssetType
	AssetID          int64
	AssetTitle       string
	GameName         string
	PublishedAt      *time.Time
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
	RequestedBy   *identity.UserID
	AssetID       *int64
	Statuses      []model.Status
	Purposes      []model.Purpose
	FinishedAfter *time.Time
	Limit         int
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

// CreateUserDownloadInput is everything a user download needs that the caller
// can know. It is deliberately not a `CreateTaskInput`: the dedupe key and the
// purpose/scope pair are this function's to decide, and a caller that supplied
// them could supply them inconsistently.
type CreateUserDownloadInput struct {
	ID         string
	TeamID     identity.TeamID
	AssetID    int64
	AssetTitle string
	GameName   string
	// PublishedAt is when the material was published, copied onto the task like
	// GameName so the lease can carry it to the executor for naming. A material
	// without a publish time leaves it nil and the executor omits that segment.
	PublishedAt     *time.Time
	SourceObjectKey string
	RequestedBy     identity.UserID
	AssignedNodeID  string
	TotalBytes      int64
	ExpectedSHA256  string
	// DependencyTaskID names the Cloud preparation task whose object this download
	// will fetch, when there is one. With it set, the object key, size and hash may
	// be left empty: the download is a queued request whose facts do not exist yet,
	// and the task it names will write them and clear this field (see
	// `HandOverDependencies`). Until then the task is not leasable, so nothing can
	// promise a hash before there is one.
	//
	// It is empty for a download of an object that is already prepared, which is the
	// ordinary case after the first download of a material.
	DependencyTaskID string
	MaxAttempts      int
}

func CreateTask(input CreateTaskInput, now time.Time) (model.Task, error) {
	return createTask(database.DB(), input, now)
}

// CreateUserDownloadTask queues one user download, deciding for itself whether
// this click is a repeat of one already outstanding.
//
// The idempotency key carries a generation, and the generation is how many
// downloads of this material this user has already **finished**. Two clicks while
// the first transfer is still outstanding count the same generation, compute the
// same key, and the unique index collapses them into one task — which is what a
// double click means. A click after that transfer reached a terminal state counts
// one more and creates a new task, because "download it again" is a different
// request from "download it", and the first task's row is history that the second
// one must not overwrite.
//
// The count and the insert share a transaction. Read outside it, two clicks could
// count the same generation and still race to two different keys; inside it, the
// loser's insert collides and `createTask` reads back the survivor.
//
// A status filter that meant "outstanding" instead of "finished" would be wrong
// in the direction that is hard to see: a cancelled transfer would let the next
// click resurrect the same key, and the row it lands on already carries the
// cancelled executor's state.
func CreateUserDownloadTask(input CreateUserDownloadInput, now time.Time) (model.Task, error) {
	return createUserDownloadTask(database.DB(), input, now)
}

func createUserDownloadTask(db *gorm.DB, input CreateUserDownloadInput, now time.Time) (model.Task, error) {
	if input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 {
		return model.Task{}, fmt.Errorf("invalid user download input")
	}
	var task model.Task
	err := db.Transaction(func(tx *gorm.DB) error {
		var finished int64
		if err := tx.Raw(`SELECT COUNT(*) FROM file_transfer_tasks WHERE asset_type = ? AND asset_id = ? AND purpose = ? AND requested_by = ? AND status IN ('success', 'failed', 'cancelled')`, model.AssetMaterial, input.AssetID, model.PurposeUserDownload, input.RequestedBy).Row().Scan(&finished); err != nil {
			return err
		}
		var err error
		task, err = createTask(tx, CreateTaskInput{
			ID:               input.ID,
			TeamID:           input.TeamID,
			AssetType:        model.AssetMaterial,
			AssetID:          input.AssetID,
			AssetTitle:       input.AssetTitle,
			GameName:         input.GameName,
			PublishedAt:      input.PublishedAt,
			SourceObjectKey:  input.SourceObjectKey,
			Purpose:          model.PurposeUserDownload,
			ExecutionScope:   model.ExecutionLocalAgent,
			RequestedBy:      input.RequestedBy,
			AssignedNodeID:   input.AssignedNodeID,
			DependencyTaskID: input.DependencyTaskID,
			DedupeKey:        userDownloadDedupeKey(input.AssetID, input.RequestedBy, input.AssignedNodeID, finished+1),
			TotalBytes:       input.TotalBytes,
			ExpectedSHA256:   input.ExpectedSHA256,
			MaxAttempts:      input.MaxAttempts,
		}, now)
		return err
	})
	if err != nil {
		return model.Task{}, err
	}
	return task, nil
}

// userDownloadDedupeKey binds a download to the user, the material, **the node**
// and the generation.
//
// The node is in the key because the task is assigned to one: two devices are two
// destinations, and collapsing them would send a file the user asked for on the
// laptop only to the desktop. The generation is a counter rather than a timestamp
// so that the key is a pure function of durable state — a clock or a random value
// here would make the duplicate-click case depend on timing.
//
// It is hashed because `dedupe_key` is `CHAR(64)`: the readable form is longer
// than that as soon as a node id is a uuid, and MySQL would answer a silent
// truncation with a collision between two different downloads. The generation is
// part of the digest, so the key is still exactly as discriminating as the tuple
// — it is just no longer readable in a row dump.
func userDownloadDedupeKey(assetID int64, userID identity.UserID, nodeID string, generation int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("user_download|%d|%d|%s|%d", assetID, userID, strings.TrimSpace(nodeID), generation)))
	return hex.EncodeToString(sum[:])
}

// CreateMaterialSourcePrepareInput is what queueing a Cloud preparation needs.
// The object key, size and hash are absent by construction: producing them is
// what the task is for.
type CreateMaterialSourcePrepareInput struct {
	ID          string
	TeamID      identity.TeamID
	AssetID     int64
	AssetTitle  string
	RequestedBy identity.UserID
	MaxAttempts int
}

// CreateMaterialSourcePrepareTask queues the Cloud task that fetches a material's
// source video into the object store.
//
// It is keyed on the **material**, not on the user who clicked, unlike a user
// download. A download is a destination — two devices are two files, so the node
// is part of its key — while a preparation is a property of the material: the
// object is content-addressed and identical for everyone, so a second user
// clicking while the first preparation is outstanding must attach to it rather
// than start a second download of the same video. That is the "no unbounded
// duplicate tasks for one business command" rule, and it is why this key carries
// no user id.
//
// The generation is the count of *finished* preparations, exactly as on the user
// download side: a click while one is outstanding counts the same generation and
// collapses onto the same row, whereas a click after a preparation failed counts
// one more and creates a new task — "prepare it again" is a different request
// from "prepare it", and the failed row is history that must not be overwritten.
// A preparation that is stuck `running` with an expired lease is reused as well,
// and `leaseable` makes it claimable again, so a worker that died mid-download
// needs no repair step.
func CreateMaterialSourcePrepareTask(input CreateMaterialSourcePrepareInput, now time.Time) (model.Task, error) {
	return createMaterialSourcePrepareTask(database.DB(), input, now)
}

func createMaterialSourcePrepareTask(db *gorm.DB, input CreateMaterialSourcePrepareInput, now time.Time) (model.Task, error) {
	if input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 {
		return model.Task{}, fmt.Errorf("invalid material source preparation input")
	}
	var task model.Task
	err := db.Transaction(func(tx *gorm.DB) error {
		var finished int64
		if err := tx.Raw(`SELECT COUNT(*) FROM file_transfer_tasks WHERE asset_type = ? AND asset_id = ? AND purpose = ? AND status IN ('success', 'failed', 'cancelled')`, model.AssetMaterial, input.AssetID, model.PurposeComposeInputPrepare).Row().Scan(&finished); err != nil {
			return err
		}
		var err error
		task, err = createTask(tx, CreateTaskInput{
			ID:             input.ID,
			TeamID:         input.TeamID,
			AssetType:      model.AssetMaterial,
			AssetID:        input.AssetID,
			AssetTitle:     input.AssetTitle,
			Purpose:        model.PurposeComposeInputPrepare,
			ExecutionScope: model.ExecutionCloud,
			RequestedBy:    input.RequestedBy,
			DedupeKey:      materialSourcePrepareDedupeKey(input.AssetID, finished+1),
			MaxAttempts:    input.MaxAttempts,
		}, now)
		return err
	})
	if err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func materialSourcePrepareDedupeKey(assetID int64, generation int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("material_source_prepare|%d|%d", assetID, generation)))
	return hex.EncodeToString(sum[:])
}

// DependencyFacts are the verified facts a preparation hands to the downloads
// waiting on it.
type DependencyFacts struct {
	SourceObjectKey string
	TotalBytes      int64
	ExpectedSHA256  string
}

// HandOverDependencies gives a finished preparation's facts to every download
// waiting on it and releases them, in one statement.
//
// The facts and the release are one statement because they are one transition:
// a row that was told its facts without being released would never be claimed,
// and a row released without facts would be claimed into a lease with no object
// to grant. Writing them together makes that unrepresentable rather than
// unlikely, and `leaseable` refuses the row until the pointer is cleared.
//
// Every waiter is updated, not one: several users may have clicked the same
// material while the single preparation for it ran, and all of their downloads
// are satisfied by the same object.
func HandOverDependencies(prepareTaskID string, facts DependencyFacts, now time.Time) (int64, error) {
	return handOverDependencies(database.DB(), prepareTaskID, facts, now)
}

func handOverDependencies(db *gorm.DB, prepareTaskID string, facts DependencyFacts, now time.Time) (int64, error) {
	if strings.TrimSpace(prepareTaskID) == "" || strings.TrimSpace(facts.SourceObjectKey) == "" || facts.TotalBytes <= 0 || !isSHA256(facts.ExpectedSHA256) {
		return 0, fmt.Errorf("invalid dependency hand-over")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET source_object_key = ?, total_bytes = ?, expected_sha256 = ?, dependency_task_id = NULL, updated_at = ? WHERE dependency_task_id = ? AND status = 'pending'`, facts.SourceObjectKey, facts.TotalBytes, strings.ToLower(strings.TrimSpace(facts.ExpectedSHA256)), now, prepareTaskID)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// FailDependents ends every download waiting on a preparation that ended without
// producing one.
//
// Without this the waiters would sit `pending` forever: their dependency is
// terminal, so nothing will ever hand them facts, and they are not leasable, so
// nothing will ever pick them up. They inherit the preparation's error rather
// than a generic one because the cause is the same event, and the user's retry is
// the same retry.
func FailDependents(prepareTaskID, errorCode, errorMessage string, now time.Time) (int64, error) {
	return failDependents(database.DB(), prepareTaskID, errorCode, errorMessage, now)
}

func failDependents(db *gorm.DB, prepareTaskID, errorCode, errorMessage string, now time.Time) (int64, error) {
	if strings.TrimSpace(prepareTaskID) == "" || strings.TrimSpace(errorCode) == "" || len(errorCode) > 128 || len(errorMessage) > 500 {
		return 0, fmt.Errorf("invalid dependency failure")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'failed', error_code = ?, error_message = ?, finished_at = ?, lease_expires_at = NULL, updated_at = ? WHERE dependency_task_id = ? AND status = 'pending'`, errorCode, errorMessage, now, now, prepareTaskID)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// failDependentsOfTerminalTasks is the safety net under the two targeted call
// sites above.
//
// A preparation can also end somewhere that has no waiters in hand:
// `reconcileCancelledTasks` finishes a cancellation the executor abandoned, and
// `reconcileExhaustedTasks` fails a task whose attempts ran out — both are bulk
// statements that never learn which tasks they moved. Without this, the downloads
// waiting on those rows would be stuck in exactly the way `FailDependents`
// exists to prevent, and the cause would be a path nobody was looking at.
//
// It is a join rather than a subquery on purpose: MySQL refuses a subquery that
// selects from the table being updated, and the obvious correction — a derived
// table — hides the same shape behind a materialisation.
func failDependentsOfTerminalTasks(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Exec(`UPDATE file_transfer_tasks AS dependent JOIN file_transfer_tasks AS dependency ON dependency.id = dependent.dependency_task_id SET dependent.status = 'failed', dependent.error_code = 'dependency_failed', dependent.error_message = 'the preparation this download waited for did not finish', dependent.finished_at = ?, dependent.lease_expires_at = NULL, dependent.updated_at = ? WHERE dependent.status = 'pending' AND dependency.status IN ('failed', 'cancelled')`, now, now)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
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
	if err := db.Exec(`INSERT INTO file_transfer_tasks (id, team_id, asset_type, asset_id, asset_title, game_name, published_at, source_object_key, purpose, execution_scope, status, requested_by, assigned_node_id, dependency_task_id, dedupe_key, total_bytes, expected_sha256, max_attempts, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?, 'pending', ?,?,?,?,?,?, ?,?,?) ON DUPLICATE KEY UPDATE id = id`, input.ID, input.TeamID, input.AssetType, input.AssetID, nullIfEmpty(input.AssetTitle), nullIfEmpty(input.GameName), input.PublishedAt, nullIfEmpty(input.SourceObjectKey), input.Purpose, input.ExecutionScope, input.RequestedBy, nullIfEmpty(input.AssignedNodeID), nullIfEmpty(input.DependencyTaskID), input.DedupeKey, input.TotalBytes, nullIfEmpty(input.ExpectedSHA256), input.MaxAttempts, now, now).Error; err != nil {
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
	if filter.FinishedAfter != nil {
		conditions = append(conditions, "finished_at >= ?")
		args = append(args, *filter.FinishedAfter)
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

// LatestUserDownloadStatuses answers, for each material, the status of this
// user's *latest* user_download task on it. It is the derived "download
// lifecycle" fact behind My Materials — the production module calls it after
// listing usages, never to scan the whole task table.
//
// The return value is the raw task status keyed by asset_id, so the mapping to a
// display state stays a production-domain decision and this store is not asked
// to learn what a badge means. `compose_input_prepare` rows are deliberately
// excluded: cloud preparation surfaces through `materials.video_status`, not
// through a per-user download state.
func LatestUserDownloadStatuses(userID identity.UserID, materialIDs []int64) (map[int64]string, error) {
	return latestUserDownloadStatuses(database.DB(), userID, materialIDs)
}

func latestUserDownloadStatuses(db *gorm.DB, userID identity.UserID, materialIDs []int64) (map[int64]string, error) {
	statuses := make(map[int64]string, len(materialIDs))
	if len(materialIDs) == 0 {
		return statuses, nil
	}
	// created_at alone is not a total order (see listTasks), so id breaks the
	// tie; the first row reached for an asset_id is its latest task.
	query := "SELECT asset_id, status FROM file_transfer_tasks" +
		" WHERE requested_by = ? AND asset_type = ? AND purpose = ? AND asset_id IN (" +
		placeholders(len(materialIDs)) + ")" +
		" ORDER BY created_at DESC, id DESC"
	args := make([]any, 0, 2+len(materialIDs))
	args = append(args, userID, model.AssetMaterial, model.PurposeUserDownload)
	for _, id := range materialIDs {
		args = append(args, id)
	}
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[int64]struct{}, len(materialIDs))
	for rows.Next() {
		var assetID int64
		var status string
		if err := rows.Scan(&assetID, &status); err != nil {
			return nil, err
		}
		if _, ok := seen[assetID]; ok {
			continue
		}
		seen[assetID] = struct{}{}
		statuses[assetID] = status
	}
	return statuses, rows.Err()
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
//
// `(execution_scope = 'cloud' OR dependency_task_id IS NULL)`. A local task that is
// still waiting for another task may not be claimed, because the facts its lease
// would promise — the object to grant, the size and the hash to verify against —
// are written *by* the task it waits for, and they are not known when the user
// clicks. Leasing it earlier would mint a grant for an object that does not exist
// yet. The clause names the Cloud scope rather than relying on prepare tasks never
// carrying a dependency: a prepare task that did would be silently unclaimable, and
// the symptom would be a queue head that never moves.
const leaseable = `cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?)) AND (execution_scope = 'cloud' OR dependency_task_id IS NULL)`

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

// cancelTask encodes a two-phase cancellation, and one statement per phase
// encodes them:
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
// The two phases are two statements rather than one statement carrying a
// `CASE WHEN status = 'pending'` per column, because MySQL evaluates a SET list
// from left to right and a later assignment reads the value an earlier one just
// wrote. With `status` assigned first, every following CASE tested the *new*
// status, fell through to its ELSE, and left the pending arm's terminal fields
// unwritten: the row read `cancelled` with `finished_at` NULL, `error_code`
// NULL and its lease still set. The missing `error_code` is the visible half --
// `dto.Task` exposes it and not `finished_at`, so the download centre had no way
// to say why the row ended, and a cancelled row could not be told apart from one
// cancelled with a reason. The rest is terminal-state integrity: every other
// route into `cancelled`, `ReconcileCancelledTasks` included, writes a finish
// time, and only this one left a cancelled row reading as never finished while
// holding a lease. Nothing in the suite can see any of it, because every test
// here asserts statement text through sqlmock rather than what MySQL does with
// the statement. Each phase below writes every field of its own transition, so
// neither depends on the order its assignments happen to be evaluated in.
//
// That split is deliberate, and it is why the terminal write for a running task
// is absent here: the executor owns the transition out of `running`, and moving
// it to the requester would mark a task cancelled while it was still being
// written to disk. If the executor never comes back to claim that transition,
// `ReconcileCancelledTasks` finishes it — writing the same terminal fields this
// function writes for a pending row.
//
// Scope is asserted in the WHERE clause rather than checked separately, so a
// wrong team or a different user's id is simply an unmatched row — there is no
// permission check a caller could forget to run. A row that is already terminal
// matches neither statement, which reports the same thing a caller's own state
// check would have.
func cancelTask(db *gorm.DB, taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (bool, error) {
	if strings.TrimSpace(taskID) == "" || teamID <= 0 || requestedBy <= 0 {
		return false, fmt.Errorf("invalid transfer cancellation")
	}
	pending := db.Exec(`UPDATE file_transfer_tasks SET status = 'cancelled', finished_at = ?, lease_expires_at = NULL, error_code = 'cancelled_by_user', error_message = 'cancelled by user', updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status = 'pending'`, now, now, taskID, teamID, requestedBy)
	if pending.Error != nil {
		return false, pending.Error
	}
	if pending.RowsAffected == 1 {
		return true, nil
	}
	running := db.Exec(`UPDATE file_transfer_tasks SET cancel_requested_at = ?, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status = 'running'`, now, now, taskID, teamID, requestedBy)
	if running.Error != nil {
		return false, running.Error
	}
	return running.RowsAffected == 1, nil
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
//
// A task still pointing at its preparation is refused, because requeueing it
// would write back the state it is already in. `leaseable` admits a local task
// only while `dependency_task_id IS NULL`, and the pointer is cleared by the
// hand-over alone, so a failed task that still has one is waiting on a
// preparation that ended without producing anything: the row would be `pending`,
// un-leasable, and never handed over or failed again, since the dependency is
// terminal too. The user's click would land on the database and change nothing
// they could see. Retrying a download whose preparation delivered is unaffected —
// the pointer is NULL by then — and the remedy for the refused rows is a new
// download, which queues a new preparation.
func RetryTask(taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (model.Task, bool, error) {
	return retryTask(database.DB(), taskID, teamID, requestedBy, now)
}

func retryTask(db *gorm.DB, taskID string, teamID identity.TeamID, requestedBy identity.UserID, now time.Time) (model.Task, bool, error) {
	if strings.TrimSpace(taskID) == "" || teamID <= 0 || requestedBy <= 0 {
		return model.Task{}, false, fmt.Errorf("invalid transfer retry")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'pending', claimed_by_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, started_at = NULL, finished_at = NULL, cancel_requested_at = NULL, transferred_bytes = 0, speed_bytes_per_sec = 0, eta_seconds = NULL, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status = 'failed' AND dependency_task_id IS NULL AND attempt_count < max_attempts`, now, taskID, teamID, requestedBy)
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
//
// The returned count is every row this pass took out of a state nothing else
// could leave: the cancelled tasks, and the downloads released by them.
func ReconcileCancelledTasks(now time.Time) (int64, error) {
	return reconcileCancelledTasks(database.DB(), now)
}

func reconcileCancelledTasks(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'cancelled', finished_at = ?, lease_expires_at = NULL, error_code = 'cancelled_by_user', error_message = 'cancelled by user', updated_at = ? WHERE status = 'running' AND cancel_requested_at IS NOT NULL AND (lease_expires_at IS NULL OR lease_expires_at <= ?)`, now, now, now)
	if result.Error != nil {
		return 0, result.Error
	}
	released, err := failDependentsOfTerminalTasks(db, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected + released, nil
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
//
// The returned count is every row this pass took out of a state nothing else
// could leave: the exhausted tasks, and the downloads released by them.
func ReconcileExhaustedTasks(now time.Time) (int64, error) {
	return reconcileExhaustedTasks(database.DB(), now)
}

func reconcileExhaustedTasks(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'failed', error_code = 'lease_expired', error_message = 'transfer lease expired without a reporting executor', finished_at = ?, lease_expires_at = NULL, updated_at = ? WHERE status = 'running' AND cancel_requested_at IS NULL AND attempt_count >= max_attempts AND (lease_expires_at IS NULL OR lease_expires_at <= ?)`, now, now, now)
	if result.Error != nil {
		return 0, result.Error
	}
	released, err := failDependentsOfTerminalTasks(db, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected + released, nil
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
	// A Cloud preparation produces the object; a local transfer downloads one that
	// is already verified. So the local side is the one that must have the object
	// key, size and hash — with one exception, the download that waits for a
	// preparation to produce them.
	//
	// That exception is safe because it is not a hole in the rule but the same rule
	// stated later: a local task with a dependency is not leasable, so the lease
	// that promises a hash cannot be minted before `HandOverDependencies` has
	// written one. The check is skipped rather than weakened — a local task with no
	// dependency still has to carry all three facts.
	if input.Purpose == model.PurposeComposeInputPrepare && strings.TrimSpace(input.DependencyTaskID) != "" {
		// A preparation is the producer of the dependency, never its consumer: one
		// that waited on another task could not be claimed, and the queue would stop
		// moving with nothing to show for it.
		return fmt.Errorf("a Cloud preparation may not depend on another task")
	}
	if input.Purpose == model.PurposeUserDownload && strings.TrimSpace(input.DependencyTaskID) == "" {
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
	var assetTitle, gameName, objectKey, fileName, assignedNode, claimedNode, dependency, errorCode, errorMessage, sha256, expectedSHA256 sql.NullString
	var eta, integrityBytes sql.NullInt64
	var lease, heartbeat, started, finished, cancelRequested, publishedAt sql.NullTime
	if err := row.Scan(&task.ID, &teamID, &assetType, &task.AssetID, &assetTitle, &gameName, &publishedAt, &objectKey, &purpose, &scope, &status, &requestedBy, &assignedNode, &claimedNode, &dependency, &task.TotalBytes, &task.TransferredBytes, &task.SpeedBytesPerSec, &eta, &task.AttemptCount, &task.MaxAttempts, &lease, &heartbeat, &started, &finished, &cancelRequested, &expectedSHA256, &fileName, &errorCode, &errorMessage, &sha256, &integrityBytes, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return err
	}
	task.TeamID = identity.TeamID(teamID)
	task.AssetType = model.AssetType(assetType)
	task.AssetTitle = assetTitle.String
	task.GameName = gameName.String
	task.SourceObjectKey = objectKey.String
	if publishedAt.Valid {
		task.PublishedAt = &publishedAt.Time
	}
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
