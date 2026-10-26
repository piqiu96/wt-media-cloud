package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
)

const composeTaskColumns = `id, display_name, team_id, game_id, operator_id, trigger_type, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, cancel_requested, request_key, created_by, claimed_node_id, lease_expires_at, heartbeat_at, attempt_count, max_attempts, next_attempt_at, started_at, finished_at, stage, progress, error_code, error_message, output_object_key, composite_output_id, created_at, updated_at`

// ErrComposeTaskMaterialUsed reports that the same (strategy, material) pair
// already has a task row, whatever its status: a pair is produced once, and a
// second run is the retry action on that task, not another create.
var ErrComposeTaskMaterialUsed = errors.New("compose task material already used")

// composeTaskInFlight still defines "this task occupies the strategy's material
// right now". The material-state aggregate is its only reader: the create guard
// and the scheduler judge the pair by the existence of any task row instead.
const composeTaskInFlight = `status IN ('pending', 'running')`

type ComposeTaskFilter struct {
	TeamID     int64
	OperatorID int64
	// IncludeUnownedCreatedBy is the actor's own ID when the caller asks for
	// "mine": a historical row without an operator stays visible to the person
	// who created it, without pretending that person is the operator.
	IncludeUnownedCreatedBy int64
	GameIDs                 []string
	Status                  model.ComposeTaskStatus
	TriggerType             string
	StrategyID              int64
	CreatedFrom             time.Time
	CreatedBefore           time.Time
}

type ComposeTaskStore struct{}

func (ComposeTaskStore) CreateAudited(item model.ComposeTask, action string) (model.ComposeTask, error) {
	return createComposeTaskAudited(database.DB(), item, action, time.Now())
}

// createComposeTaskAudited writes the task row and its audit entry in one
// transaction. The snapshot columns ride on the insert — the caller has
// already merged and validated config, so nothing here recomputes it.
func createComposeTaskAudited(db *gorm.DB, item model.ComposeTask, action string, now time.Time) (model.ComposeTask, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		return composeTaskInsertWithAudit(tx, &item, action, item.CreatedBy, now)
	})
	if err != nil {
		return model.ComposeTask{}, err
	}
	stored, found, err := findComposeTask(db, item.ID)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if !found {
		return model.ComposeTask{}, fmt.Errorf("created compose task %d cannot be read", item.ID)
	}
	return stored, nil
}

func appendComposeTaskAudit(tx *gorm.DB, item model.ComposeTask, action string, actorID int64, now time.Time) error {
	summary, err := json.Marshal(map[string]any{
		"team_id": item.TeamID, "game_id": item.GameID, "operator_id": item.OperatorID, "status": item.Status,
		"material_id": item.MaterialID, "template_code": item.TemplateCode, "template_version": item.TemplateVersion,
	})
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, 'compose_task', ?, ?, ?)`, id.NewID("audit"), actorID, action, fmt.Sprint(item.ID), summary, now).Error
}

// composeAutoRetryQuota is the automatic retry budget a compose task starts
// with, mirroring the column DEFAULT 3 in migration 053. A manual retry rebases
// max_attempts to attempt_count + this quota, so the operator gets the same
// fault-tolerance budget again without renumbering the attempt history.
const composeAutoRetryQuota = 3

// composeTaskInsertWithAudit writes the task row and its audit entry on the
// given handle without managing the transaction, so callers that need the
// insert inside a larger transaction (CreateForUsage) share the same shape.
// It takes a pointer because GORM fills the generated ID back into the row.
func composeTaskInsertWithAudit(tx *gorm.DB, item *model.ComposeTask, action string, actorID int64, now time.Time) error {
	// GORM writes every column explicitly, so the column DEFAULT 3 would never
	// fire and a zero MaxAttempts would be persisted as "no retries allowed".
	if item.MaxAttempts <= 0 {
		item.MaxAttempts = composeAutoRetryQuota
	}
	item.CreatedAt, item.UpdatedAt = now, now
	if err := tx.Create(item).Error; err != nil {
		return err
	}
	return appendComposeTaskAudit(tx, *item, action, actorID, now)
}

func (ComposeTaskStore) Find(id int64) (model.ComposeTask, bool, error) {
	return findComposeTask(database.DB(), id)
}

func (ComposeTaskStore) FindByRequestKey(key string) (model.ComposeTask, bool, error) {
	return findComposeTaskByKey(database.DB(), key)
}

func findComposeTaskByKey(db *gorm.DB, key string) (model.ComposeTask, bool, error) {
	if key == "" {
		return model.ComposeTask{}, false, nil
	}
	item, err := scanComposeTask(db.Raw(`SELECT `+composeTaskColumns+` FROM compose_tasks WHERE request_key = ?`, key).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.ComposeTask{}, false, nil
	}
	return item, err == nil, err
}

// CreateForUsage checks the actor's active usage and writes the task in one
// transaction, so a usage removed mid-command cannot slip a task past §5.4.5's
// "no new tasks on a removed relation" rule. found=false means nothing was
// written; the caller names it as a per-item failure.
// ListAttempts reads one task's per-attempt execution records, oldest first
// (M4-G). Visibility is the task's own: the service resolves and scopes the
// task before calling this.
func (ComposeTaskStore) ListAttempts(taskID int64) ([]model.ComposeTaskAttempt, error) {
	return listComposeTaskAttempts(database.DB(), taskID)
}

func listComposeTaskAttempts(db *gorm.DB, taskID int64) ([]model.ComposeTaskAttempt, error) {
	rows, err := db.Raw(`SELECT id, task_id, attempt, source, started_at, finished_at, outcome, stage, error_code, error_message, steps_json, created_at, updated_at FROM compose_task_attempts WHERE task_id = ? ORDER BY attempt ASC`, taskID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ComposeTaskAttempt, 0)
	for rows.Next() {
		var item model.ComposeTaskAttempt
		var started, finished sql.NullTime
		var steps []byte
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Attempt, &item.Source, &started, &finished, &item.Outcome, &item.Stage, &item.ErrorCode, &item.ErrorMessage, &steps, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if started.Valid {
			item.StartedAt = &started.Time
		}
		if finished.Valid {
			item.FinishedAt = &finished.Time
		}
		item.Steps, err = model.DecodeComposeAttemptSteps(steps)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (ComposeTaskStore) CreateForUsage(item model.ComposeTask, action string) (model.ComposeTask, bool, error) {
	return createForUsage(database.DB(), item, action, time.Now())
}

func createForUsage(db *gorm.DB, item model.ComposeTask, action string, now time.Time) (model.ComposeTask, bool, error) {
	written := false
	err := db.Transaction(func(tx *gorm.DB) error {
		usage, found, err := findActiveUsage(tx, item.CreatedBy, item.MaterialID)
		if err != nil || !found {
			return err
		}
		item.MaterialUsageID = usage.ID
		if err := lockComposeTaskMaterial(tx, item.StrategyID, item.MaterialID); err != nil {
			return err
		}
		if err := composeTaskInsertWithAudit(tx, &item, action, item.CreatedBy, now); err != nil {
			return err
		}
		written = true
		return nil
	})
	if err != nil {
		return model.ComposeTask{}, false, err
	}
	if !written {
		return model.ComposeTask{}, false, nil
	}
	stored, found, err := findComposeTask(db, item.ID)
	if err != nil {
		return model.ComposeTask{}, false, err
	}
	if !found {
		return model.ComposeTask{}, false, fmt.Errorf("created compose task %d cannot be read", item.ID)
	}
	return stored, true, nil
}

// lockComposeTaskMaterial refuses a create when the strategy already has any
// task row for this material, finished or not: (策略,素材) is produced once, and
// running it again is the retry action on the existing task. That is the same
// rule the scheduler's candidate check applies, so a pair the drawer still
// offers is exactly a pair with no task row at all.
//
// The strategy row is locked first: the task read matches one row at most and
// usually none, so two concurrent creates for two different materials of one
// strategy would otherwise gap-lock each other's insert position. That read is
// a locking read because findActiveUsage already pinned this transaction's
// snapshot, and a task created since then is invisible to a plain read.
func lockComposeTaskMaterial(tx *gorm.DB, strategyID, materialID int64) error {
	var locked int64
	if err := tx.Raw(`SELECT id FROM compose_strategies WHERE id = ? FOR UPDATE`, strategyID).Row().Scan(&locked); err != nil {
		return err
	}
	var usedID int64
	err := tx.Raw(`SELECT id FROM compose_tasks WHERE compose_strategy_id = ? AND material_id = ? LIMIT 1 FOR UPDATE`, strategyID, materialID).Row().Scan(&usedID)
	switch {
	case err == nil:
		return ErrComposeTaskMaterialUsed
	case errors.Is(err, sql.ErrNoRows):
		return nil
	default:
		return err
	}
}

// MaterialStates answers "what has this strategy done with each material" in
// one query. The four status counts are what the drawer reads: any of them
// above zero means the pair is already used and read-only, and a missing row
// (all four zero) is the only shape a create accepts. Every requested id gets a
// row, ascending: a missing row means the pair never ran, and the caller must
// not have to tell that apart from a strategy that came back short.
// materialIDs are expected deduplicated.
func (ComposeTaskStore) MaterialStates(strategyID int64, materialIDs []int64) ([]model.ComposeMaterialState, error) {
	return composeMaterialStates(database.DB(), strategyID, materialIDs)
}

func composeMaterialStates(db *gorm.DB, strategyID int64, materialIDs []int64) ([]model.ComposeMaterialState, error) {
	states := make([]model.ComposeMaterialState, 0, len(materialIDs))
	if strategyID <= 0 || len(materialIDs) == 0 {
		return states, nil
	}
	ids := slices.Clone(materialIDs)
	slices.Sort(ids)
	query := `SELECT material_id, COALESCE(SUM(status = 'success'), 0), COALESCE(SUM(` + composeTaskInFlight + `), 0), COALESCE(SUM(status = 'failed'), 0), COALESCE(SUM(status = 'cancelled'), 0), MAX(created_at) FROM compose_tasks WHERE compose_strategy_id = ? AND material_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + `) GROUP BY material_id`
	args := make([]any, 0, len(ids)+1)
	args = append(args, strategyID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[int64]model.ComposeMaterialState, len(ids))
	for rows.Next() {
		var item model.ComposeMaterialState
		var last sql.NullTime
		if err := rows.Scan(&item.MaterialID, &item.SuccessCount, &item.ActiveCount, &item.FailedCount, &item.CancelledCount, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			at := last.Time
			item.LastTaskAt = &at
		}
		seen[item.MaterialID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if item, ok := seen[id]; ok {
			states = append(states, item)
			continue
		}
		states = append(states, model.ComposeMaterialState{MaterialID: id})
	}
	return states, nil
}

func (ComposeTaskStore) CancelAudited(item model.ComposeTask, actorID int64, action string) (bool, error) {
	return cancelComposeTaskAudited(database.DB(), item, actorID, action, time.Now())
}

func findComposeTask(db *gorm.DB, id int64) (model.ComposeTask, bool, error) {
	if id <= 0 {
		return model.ComposeTask{}, false, nil
	}
	item, err := scanComposeTask(db.Raw(`SELECT `+composeTaskColumns+` FROM compose_tasks WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.ComposeTask{}, false, nil
	}
	return item, err == nil, err
}

func (ComposeTaskStore) List(filter ComposeTaskFilter) ([]model.ComposeTask, error) {
	return listComposeTasks(database.DB(), filter)
}

func listComposeTasks(db *gorm.DB, filter ComposeTaskFilter) ([]model.ComposeTask, error) {
	query := `SELECT ` + composeTaskColumns + ` FROM compose_tasks WHERE 1=1`
	args := make([]any, 0, len(filter.GameIDs)+4)
	if filter.TeamID > 0 {
		query += ` AND team_id = ?`
		args = append(args, filter.TeamID)
	}
	if len(filter.GameIDs) > 0 {
		query += ` AND game_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(filter.GameIDs)), ",") + `)`
		for _, id := range filter.GameIDs {
			args = append(args, id)
		}
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.TriggerType != "" {
		query += ` AND trigger_type = ?`
		args = append(args, filter.TriggerType)
	}
	if filter.StrategyID > 0 {
		query += ` AND compose_strategy_id = ?`
		args = append(args, filter.StrategyID)
	}
	if filter.OperatorID > 0 {
		if filter.IncludeUnownedCreatedBy == filter.OperatorID {
			query += ` AND (operator_id = ? OR (operator_id IS NULL AND created_by = ?))`
			args = append(args, filter.OperatorID, filter.IncludeUnownedCreatedBy)
		} else {
			query += ` AND operator_id = ?`
			args = append(args, filter.OperatorID)
		}
	}
	if !filter.CreatedFrom.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, filter.CreatedFrom)
	}
	if !filter.CreatedBefore.IsZero() {
		query += ` AND created_at < ?`
		args = append(args, filter.CreatedBefore)
	}
	query += ` ORDER BY id DESC LIMIT 200`
	rows, err := db.Raw(query, args...).Rows()
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

// cancelComposeTaskAudited flips a pending task to cancelled and records the
// cancel in one transaction — change.md requires the audit trail on both the
// create and the cancel. M4-C3 scope: pending only, so one conditional UPDATE
// is the whole operation — a running task has a process behind it and waits
// for M4-C5. Rows-affected 0 (already claimed, finished, or gone) writes
// nothing and returns false; the caller reloads to name the outcome.
func cancelComposeTaskAudited(db *gorm.DB, item model.ComposeTask, actorID int64, action string, now time.Time) (bool, error) {
	ok := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_tasks SET status = 'cancelled', finished_at = ?, updated_at = ? WHERE id = ? AND status = 'pending'`, now, now, item.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		ok = true
		// A manual retry opens its attempt row before any claim; cancelling
		// the still-pending task closes it so no authorized attempt is left
		// open (M4-G).
		if err := tx.Exec(`UPDATE compose_task_attempts SET outcome = 'cancelled', finished_at = ?, updated_at = ? WHERE task_id = ? AND outcome = 'pending' AND finished_at IS NULL`, now, now, item.ID).Error; err != nil {
			return err
		}
		item.Status = model.ComposeTaskCancelled
		return appendComposeTaskAudit(tx, item, action, actorID, now)
	})
	if err != nil {
		return false, err
	}
	return ok, nil
}

type composeTaskScanner interface{ Scan(...any) error }

func scanComposeTask(row composeTaskScanner) (model.ComposeTask, error) {
	var item model.ComposeTask
	var snapshot []byte
	var operatorID, claimedNode, compositeOutput sql.NullInt64
	var leaseExpires, heartbeat, nextAttempt, started, finished sql.NullTime
	err := row.Scan(&item.ID, &item.DisplayName, &item.TeamID, &item.GameID, &operatorID, &item.TriggerType, &item.MaterialID, &item.MaterialUsageID, &item.StrategyID, &item.TemplateCode, &item.TemplateVersion, &snapshot, &item.Status, &item.CancelRequested, &item.RequestKey, &item.CreatedBy, &claimedNode, &leaseExpires, &heartbeat, &item.AttemptCount, &item.MaxAttempts, &nextAttempt, &started, &finished, &item.Stage, &item.Progress, &item.ErrorCode, &item.ErrorMessage, &item.OutputObjectKey, &compositeOutput, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if operatorID.Valid {
		value := operatorID.Int64
		item.OperatorID = &value
	}
	if !json.Valid(snapshot) {
		return model.ComposeTask{}, fmt.Errorf("compose task %d has invalid config_snapshot_json", item.ID)
	}
	item.ConfigSnapshot = append(json.RawMessage(nil), snapshot...)
	if claimedNode.Valid {
		v := claimedNode.Int64
		item.ClaimedNodeID = &v
	}
	if leaseExpires.Valid {
		v := leaseExpires.Time
		item.LeaseExpiresAt = &v
	}
	if heartbeat.Valid {
		v := heartbeat.Time
		item.HeartbeatAt = &v
	}
	if nextAttempt.Valid {
		v := nextAttempt.Time
		item.NextAttemptAt = &v
	}
	if started.Valid {
		v := started.Time
		item.StartedAt = &v
	}
	if finished.Valid {
		v := finished.Time
		item.FinishedAt = &v
	}
	if compositeOutput.Valid {
		v := compositeOutput.Int64
		item.CompositeOutput = &v
	}
	return item, nil
}

// findActiveUsage reads the caller's active relation for one material inside
// the caller's transaction, so a usage removed mid-command cannot slip a task
// past §5.4.5's "no new tasks on a removed relation" rule.
func findActiveUsage(db *gorm.DB, userID, materialID int64) (model.MaterialUsage, bool, error) {
	if userID <= 0 || materialID <= 0 {
		return model.MaterialUsage{}, false, nil
	}
	usage := &model.MaterialUsage{}
	err := scanUsage(db.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id = ? AND material_id = ? AND status = 'active'`, userID, materialID).Row(), usage)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MaterialUsage{}, false, nil
	}
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	return *usage, true, nil
}
