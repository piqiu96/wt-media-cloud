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
	"gorm.io/gorm"
)

const strategyColumns = `id, team_id, game_id, compose_template_id, compose_template_version, name, content_type, config_json, status, created_by, updated_by, created_at, updated_at`

type StrategyFilter struct {
	TeamID     int64
	GameIDs    []string
	ActiveOnly bool
}

type StrategyStore struct{}

func (StrategyStore) Create(item model.ComposeStrategy) (model.ComposeStrategy, error) {
	return createComposeStrategy(database.DB(), item, time.Now())
}

func createComposeStrategy(db *gorm.DB, item model.ComposeStrategy, now time.Time) (model.ComposeStrategy, error) {
	item.CreatedAt, item.UpdatedAt = now, now
	if err := db.Create(&item).Error; err != nil {
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
	query := `SELECT s.id, s.team_id, s.game_id, s.compose_template_id, s.compose_template_version, s.name, s.content_type, s.config_json, s.status, s.created_by, s.updated_by, s.created_at, s.updated_at FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version WHERE 1=1`
	args := make([]any, 0, len(filter.GameIDs)+2)
	if filter.TeamID > 0 {
		query += ` AND s.team_id = ?`
		args = append(args, filter.TeamID)
	}
	if len(filter.GameIDs) > 0 {
		query += ` AND s.game_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(filter.GameIDs)), ",") + `)`
		for _, id := range filter.GameIDs {
			args = append(args, id)
		}
	}
	if filter.ActiveOnly {
		query += ` AND s.status = ? AND t.status = 'active'`
		args = append(args, "active")
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
	err := row.Scan(&item.ID, &item.TeamID, &item.GameID, &item.TemplateID, &item.TemplateVersion, &item.Name, &item.ContentType, &config, &item.Status, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !json.Valid(config) {
		return model.ComposeStrategy{}, fmt.Errorf("strategy %d has invalid config_json", item.ID)
	}
	item.Config = append(json.RawMessage(nil), config...)
	return item, nil
}
