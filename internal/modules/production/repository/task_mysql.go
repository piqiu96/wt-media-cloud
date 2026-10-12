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

const composeTaskColumns = `id, team_id, game_id, trigger_type, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, request_key, created_by, claimed_node_id, lease_expires_at, heartbeat_at, attempt_count, max_attempts, started_at, finished_at, stage, progress, error_code, error_message, output_object_key, composite_output_id, created_at, updated_at`

type ComposeTaskFilter struct {
	TeamID     int64
	GameIDs    []string
	Status     model.ComposeTaskStatus
	StrategyID int64
	CreatedBy  int64
}

type ComposeTaskStore struct{}

func (ComposeTaskStore) CreateAudited(item model.ComposeTask, action string) (model.ComposeTask, error) {
	return createComposeTaskAudited(database.DB(), item, action, time.Now())
}

// createComposeTaskAudited writes the task row and its audit entry in one
// transaction. The snapshot columns ride on the insert — the caller has
// already merged and validated config, so nothing here recomputes it.
func createComposeTaskAudited(db *gorm.DB, item model.ComposeTask, action string, now time.Time) (model.ComposeTask, error) {
	item.CreatedAt, item.UpdatedAt = now, now
	// GORM writes every column explicitly, so the column DEFAULT 3 would never
	// fire and a zero MaxAttempts would be persisted as "no retries allowed".
	if item.MaxAttempts <= 0 {
		item.MaxAttempts = 3
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return appendComposeTaskAudit(tx, item, action, now)
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

func appendComposeTaskAudit(tx *gorm.DB, item model.ComposeTask, action string, now time.Time) error {
	summary, err := json.Marshal(map[string]any{
		"team_id": item.TeamID, "game_id": item.GameID, "status": item.Status,
		"material_id": item.MaterialID, "template_code": item.TemplateCode, "template_version": item.TemplateVersion,
	})
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, 'compose_task', ?, ?, ?)`, id.NewID("audit"), item.CreatedBy, action, fmt.Sprint(item.ID), summary, now).Error
}

func (ComposeTaskStore) Find(id int64) (model.ComposeTask, bool, error) {
	return findComposeTask(database.DB(), id)
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

// cancelPendingComposeTask flips a task that has not been claimed to
// cancelled. M4-C3 scope: pending only, so one conditional UPDATE is the whole
// operation — a running task has a process behind it and waits for M4-C5.
// Rows-affected 0 means "not pending (or gone)"; the caller reloads to name it.
func cancelPendingComposeTask(db *gorm.DB, id int64, now time.Time) (bool, error) {
	result := db.Exec(`UPDATE compose_tasks SET status = 'cancelled', finished_at = ?, updated_at = ? WHERE id = ? AND status = 'pending'`, now, now, id)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

type composeTaskScanner interface{ Scan(...any) error }

func scanComposeTask(row composeTaskScanner) (model.ComposeTask, error) {
	var item model.ComposeTask
	var snapshot []byte
	var claimedNode, compositeOutput sql.NullInt64
	var leaseExpires, heartbeat, started, finished sql.NullTime
	err := row.Scan(&item.ID, &item.TeamID, &item.GameID, &item.TriggerType, &item.MaterialID, &item.MaterialUsageID, &item.StrategyID, &item.TemplateCode, &item.TemplateVersion, &snapshot, &item.Status, &item.RequestKey, &item.CreatedBy, &claimedNode, &leaseExpires, &heartbeat, &item.AttemptCount, &item.MaxAttempts, &started, &finished, &item.Stage, &item.Progress, &item.ErrorCode, &item.ErrorMessage, &item.OutputObjectKey, &compositeOutput, &item.CreatedAt, &item.UpdatedAt)
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
