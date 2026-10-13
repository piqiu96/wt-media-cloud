package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
)

const composeTaskColumns = `id, team_id, game_id, trigger_type, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, cancel_requested, request_key, created_by, claimed_node_id, lease_expires_at, heartbeat_at, attempt_count, max_attempts, started_at, finished_at, stage, progress, error_code, error_message, output_object_key, composite_output_id, created_at, updated_at`

type ComposeTaskFilter struct {
	TeamID        int64
	GameIDs       []string
	Status        model.ComposeTaskStatus
	StrategyID    int64
	CreatedBy     int64
	CreatedFrom   time.Time
	CreatedBefore time.Time
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
		"team_id": item.TeamID, "game_id": item.GameID, "status": item.Status,
		"material_id": item.MaterialID, "template_code": item.TemplateCode, "template_version": item.TemplateVersion,
	})
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, 'compose_task', ?, ?, ?)`, id.NewID("audit"), actorID, action, fmt.Sprint(item.ID), summary, now).Error
}

// composeTaskInsertWithAudit writes the task row and its audit entry on the
// given handle without managing the transaction, so callers that need the
// insert inside a larger transaction (CreateForUsage) share the same shape.
// It takes a pointer because GORM fills the generated ID back into the row.
func composeTaskInsertWithAudit(tx *gorm.DB, item *model.ComposeTask, action string, actorID int64, now time.Time) error {
	// GORM writes every column explicitly, so the column DEFAULT 3 would never
	// fire and a zero MaxAttempts would be persisted as "no retries allowed".
	if item.MaxAttempts <= 0 {
		item.MaxAttempts = 3
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
	if filter.StrategyID > 0 {
		query += ` AND compose_strategy_id = ?`
		args = append(args, filter.StrategyID)
	}
	if filter.CreatedBy > 0 {
		query += ` AND created_by = ?`
		args = append(args, filter.CreatedBy)
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
	var claimedNode, compositeOutput sql.NullInt64
	var leaseExpires, heartbeat, started, finished sql.NullTime
	err := row.Scan(&item.ID, &item.TeamID, &item.GameID, &item.TriggerType, &item.MaterialID, &item.MaterialUsageID, &item.StrategyID, &item.TemplateCode, &item.TemplateVersion, &snapshot, &item.Status, &item.CancelRequested, &item.RequestKey, &item.CreatedBy, &claimedNode, &leaseExpires, &heartbeat, &item.AttemptCount, &item.MaxAttempts, &started, &finished, &item.Stage, &item.Progress, &item.ErrorCode, &item.ErrorMessage, &item.OutputObjectKey, &compositeOutput, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.ComposeTask{}, err
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
