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

const strategyColumns = `id, team_id, game_id, operator_id, compose_template_id, compose_template_version, name, config_json, execution_mode, schedule_json, priority_rules_json, single_limit, daily_limit, next_run_at, status, created_by, updated_by, created_at, updated_at`

type StrategyFilter struct {
	TeamID     int64
	OperatorID int64
	GameIDs    []string
	ActiveOnly bool
	Search     string
	GameID     string
	TemplateID int64
	Status     string
}

type StrategyStore struct{}

func (StrategyStore) FindTemplate(templateID int64, version uint32) (model.ComposeTemplate, bool, error) {
	var item model.ComposeTemplate
	var schema []byte
	err := database.DB().Raw(`SELECT id, code, name, version, schema_json, executor, status FROM compose_templates WHERE id = ? AND version = ?`, templateID, version).
		Row().Scan(&item.ID, &item.Code, &item.Name, &item.Version, &schema, &item.ExecutorID, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ComposeTemplate{}, false, nil
	}
	if err != nil {
		return model.ComposeTemplate{}, false, err
	}
	if !json.Valid(schema) {
		return model.ComposeTemplate{}, false, fmt.Errorf("template %d has invalid schema_json", templateID)
	}
	item.Schema = append(json.RawMessage(nil), schema...)
	return item, true, nil
}

func (StrategyStore) CreateAudited(item model.ComposeStrategy, action string) (model.ComposeStrategy, error) {
	return createComposeStrategyAudited(database.DB(), item, action, time.Now())
}

func createComposeStrategyAudited(db *gorm.DB, item model.ComposeStrategy, action string, now time.Time) (model.ComposeStrategy, error) {
	item.CreatedAt, item.UpdatedAt = now, now
	if len(item.RulesJSON) == 0 {
		item.RulesJSON = json.RawMessage(`[]`)
	}
	if item.Schedule != nil {
		encoded, err := json.Marshal(item.Schedule)
		if err != nil {
			return model.ComposeStrategy{}, err
		}
		item.ScheduleJSON = encoded
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return appendStrategyAudit(tx, item, action, now)
	})
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	stored, found, err := findComposeStrategy(db, item.ID)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found {
		return model.ComposeStrategy{}, fmt.Errorf("created strategy %d cannot be read", item.ID)
	}
	return stored, nil
}

func (StrategyStore) UpdateAudited(item model.ComposeStrategy, action string) (model.ComposeStrategy, error) {
	return updateComposeStrategyAudited(database.DB(), item, action, time.Now())
}

func updateComposeStrategyAudited(db *gorm.DB, item model.ComposeStrategy, action string, now time.Time) (model.ComposeStrategy, error) {
	item.UpdatedAt = now
	var scheduleJSON []byte
	if item.Schedule != nil {
		var err error
		scheduleJSON, err = json.Marshal(item.Schedule)
		if err != nil {
			return model.ComposeStrategy{}, err
		}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_strategies SET name = ?, operator_id = ?, config_json = ?, execution_mode = ?, schedule_json = ?, priority_rules_json = ?, single_limit = ?, daily_limit = ?, next_run_at = ?, status = ?, updated_by = ?, updated_at = ? WHERE id = ? AND team_id = ? AND game_id = ?`, item.Name, item.OperatorID, item.Config, item.ExecutionMode, scheduleJSON, item.RulesJSON, item.SingleLimit, item.DailyLimit, item.NextRunAt, item.Status, item.UpdatedBy, now, item.ID, item.TeamID, item.GameID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		return appendStrategyAudit(tx, item, action, now)
	})
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	stored, found, err := findComposeStrategy(db, item.ID)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found {
		return model.ComposeStrategy{}, fmt.Errorf("updated strategy %d cannot be read", item.ID)
	}
	return stored, nil
}

func appendStrategyAudit(tx *gorm.DB, item model.ComposeStrategy, action string, now time.Time) error {
	summary, err := json.Marshal(map[string]any{"team_id": item.TeamID, "game_id": item.GameID, "operator_id": item.OperatorID, "status": item.Status, "execution_mode": item.ExecutionMode})
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, 'compose_strategy', ?, ?, ?)`, id.NewID("audit"), item.UpdatedBy, action, fmt.Sprint(item.ID), summary, now).Error
}

func (StrategyStore) Find(id int64) (model.ComposeStrategy, bool, error) {
	return findComposeStrategy(database.DB(), id)
}

func findComposeStrategy(db *gorm.DB, id int64) (model.ComposeStrategy, bool, error) {
	if id <= 0 {
		return model.ComposeStrategy{}, false, nil
	}
	item, err := scanComposeStrategy(db.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.ComposeStrategy{}, false, nil
	}
	return item, err == nil, err
}

func (StrategyStore) List(filter StrategyFilter) ([]model.ComposeStrategy, error) {
	return listComposeStrategies(database.DB(), filter)
}

func listComposeStrategies(db *gorm.DB, filter StrategyFilter) ([]model.ComposeStrategy, error) {
	query := `SELECT s.id, s.team_id, s.game_id, s.operator_id, s.compose_template_id, s.compose_template_version, s.name, s.config_json, s.execution_mode, s.schedule_json, s.priority_rules_json, s.single_limit, s.daily_limit, s.next_run_at, s.status, s.created_by, s.updated_by, s.created_at, s.updated_at FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version WHERE 1=1`
	args := make([]any, 0, len(filter.GameIDs)+3)
	if filter.TeamID > 0 {
		query += ` AND s.team_id = ?`
		args = append(args, filter.TeamID)
	}
	if filter.OperatorID > 0 {
		query += ` AND s.operator_id = ?`
		args = append(args, filter.OperatorID)
	}
	if len(filter.GameIDs) > 0 {
		query += ` AND s.game_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(filter.GameIDs)), ",") + `)`
		for _, id := range filter.GameIDs {
			args = append(args, id)
		}
	}
	if filter.ActiveOnly {
		query += ` AND s.status = ? AND t.status = 'active'`
		args = append(args, model.ComposeStrategyEnabled)
	}
	if filter.Search != "" {
		query += ` AND (s.name LIKE ? ESCAPE '!' OR CAST(s.id AS CHAR) = ?)`
		args = append(args, likeContains(filter.Search), filter.Search)
	}
	if filter.GameID != "" {
		query += ` AND s.game_id = ?`
		args = append(args, filter.GameID)
	}
	if filter.TemplateID > 0 {
		query += ` AND s.compose_template_id = ?`
		args = append(args, filter.TemplateID)
	}
	if filter.Status != "" {
		query += ` AND s.status = ?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY s.id DESC LIMIT 200`
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ComposeStrategy, 0)
	for rows.Next() {
		item, err := scanComposeStrategy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type strategyScanner interface{ Scan(...any) error }

func scanComposeStrategy(row strategyScanner) (model.ComposeStrategy, error) {
	var item model.ComposeStrategy
	var config []byte
	var schedule []byte
	var rules []byte
	var nextRun sql.NullTime
	var operatorID sql.NullInt64
	err := row.Scan(&item.ID, &item.TeamID, &item.GameID, &operatorID, &item.TemplateID, &item.TemplateVersion, &item.Name, &config, &item.ExecutionMode, &schedule, &rules, &item.SingleLimit, &item.DailyLimit, &nextRun, &item.Status, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if operatorID.Valid {
		value := operatorID.Int64
		item.OperatorID = &value
	}
	if !json.Valid(config) {
		return model.ComposeStrategy{}, fmt.Errorf("strategy %d has invalid config_json", item.ID)
	}
	item.Config = append(json.RawMessage(nil), config...)
	if err := json.Unmarshal(rules, &item.PriorityRules); err != nil {
		return model.ComposeStrategy{}, fmt.Errorf("strategy %d has invalid priority_rules_json: %w", item.ID, err)
	}
	item.RulesJSON = append(json.RawMessage(nil), rules...)
	if schedule != nil {
		var parsed model.ComposeSchedule
		if err := json.Unmarshal(schedule, &parsed); err != nil {
			return model.ComposeStrategy{}, fmt.Errorf("strategy %d has invalid schedule_json: %w", item.ID, err)
		}
		item.Schedule = &parsed
		item.ScheduleJSON = append(json.RawMessage(nil), schedule...)
	}
	if nextRun.Valid {
		timeValue := nextRun.Time.UTC()
		item.NextRunAt = &timeValue
	}
	return item, nil
}
