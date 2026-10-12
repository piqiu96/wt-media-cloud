package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

type StrategyStore struct{}

type StrategyFilter struct {
	TeamID          *identity.TeamID
	GameIDs         []string
	IncludeInactive bool
}

func (StrategyStore) CreateStrategy(row model.ComposeStrategy) (model.ComposeStrategy, error) {
	return createStrategy(database.DB(), row)
}
func (StrategyStore) UpdateStrategy(row model.ComposeStrategy, expected uint32) (model.ComposeStrategy, bool, error) {
	return updateStrategy(database.DB(), row, expected)
}
func (StrategyStore) FindStrategy(id int64) (model.ComposeStrategy, bool, error) {
	return findStrategy(database.DB(), id)
}
func (StrategyStore) ListStrategies(filter StrategyFilter) ([]model.ComposeStrategy, error) {
	return listStrategies(database.DB(), filter)
}
func (StrategyStore) ListStrategyVersions(id int64) ([]model.ComposeStrategyVersion, error) {
	return listStrategyVersions(database.DB(), id)
}

const strategyColumns = `id, team_id, game_id, name, content_type, template_key, template_version, parameters_json, allowed_overrides_json, status, current_version, created_by, updated_by, created_at, updated_at`

func scanStrategy(row *sql.Row) (model.ComposeStrategy, error) {
	var item model.ComposeStrategy
	var params, overrides []byte
	err := row.Scan(&item.ID, &item.TeamID, &item.GameID, &item.Name, &item.ContentType, &item.TemplateKey, &item.TemplateVersion, &params, &overrides, &item.Status, &item.CurrentVersion, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal(params, &item.Parameters); err != nil {
		return item, fmt.Errorf("decode strategy parameters: %w", err)
	}
	if err := json.Unmarshal(overrides, &item.AllowedOverrides); err != nil {
		return item, fmt.Errorf("decode strategy overrides: %w", err)
	}
	return item, nil
}

func findStrategy(db *gorm.DB, id int64) (model.ComposeStrategy, bool, error) {
	item, err := scanStrategy(db.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.ComposeStrategy{}, false, nil
	}
	return item, err == nil, err
}

func listStrategies(db *gorm.DB, filter StrategyFilter) ([]model.ComposeStrategy, error) {
	query := `SELECT ` + strategyColumns + ` FROM compose_strategies WHERE 1 = 1`
	args := []any{}
	if filter.TeamID != nil {
		if len(filter.GameIDs) == 0 {
			return []model.ComposeStrategy{}, nil
		}
		query += ` AND team_id = ? AND game_id IN (` + strings.TrimRight(strings.Repeat("?,", len(filter.GameIDs)), ",") + `)`
		args = append(args, *filter.TeamID)
		for _, game := range filter.GameIDs {
			args = append(args, game)
		}
	}
	if !filter.IncludeInactive {
		query += ` AND status = 'active'`
	}
	query += ` ORDER BY updated_at DESC, id DESC`
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.ComposeStrategy{}
	for rows.Next() {
		var item model.ComposeStrategy
		var params, overrides []byte
		if err := rows.Scan(&item.ID, &item.TeamID, &item.GameID, &item.Name, &item.ContentType, &item.TemplateKey, &item.TemplateVersion, &params, &overrides, &item.Status, &item.CurrentVersion, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(params, &item.Parameters); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(overrides, &item.AllowedOverrides); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func strategyJSON(row model.ComposeStrategy) ([]byte, []byte, error) {
	params, err := json.Marshal(row.Parameters)
	if err != nil {
		return nil, nil, err
	}
	overrides, err := json.Marshal(row.AllowedOverrides)
	return params, overrides, err
}

func insertVersion(tx *gorm.DB, row model.ComposeStrategy) error {
	snapshot, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO compose_strategy_versions (strategy_id, version, snapshot_json, created_by, created_at) VALUES (?, ?, ?, ?, ?)`, row.ID, row.CurrentVersion, snapshot, row.UpdatedBy, row.UpdatedAt).Error
}

func createStrategy(db *gorm.DB, row model.ComposeStrategy) (model.ComposeStrategy, error) {
	params, overrides, err := strategyJSON(row)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO compose_strategies (team_id, game_id, name, content_type, template_key, template_version, parameters_json, allowed_overrides_json, status, current_version, created_by, updated_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, row.TeamID, row.GameID, row.Name, row.ContentType, row.TemplateKey, row.TemplateVersion, params, overrides, row.Status, row.CurrentVersion, row.CreatedBy, row.UpdatedBy, row.CreatedAt, row.UpdatedAt).Error; err != nil {
			return err
		}
		if err := tx.Raw(`SELECT LAST_INSERT_ID()`).Scan(&row.ID).Error; err != nil {
			return err
		}
		return insertVersion(tx, row)
	})
	return row, err
}

func updateStrategy(db *gorm.DB, row model.ComposeStrategy, expected uint32) (model.ComposeStrategy, bool, error) {
	params, overrides, err := strategyJSON(row)
	if err != nil {
		return model.ComposeStrategy{}, false, err
	}
	row.CurrentVersion = expected + 1
	changed := false
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE compose_strategies SET name = ?, content_type = ?, template_key = ?, template_version = ?, parameters_json = ?, allowed_overrides_json = ?, status = ?, current_version = ?, updated_by = ?, updated_at = ? WHERE id = ? AND current_version = ? AND team_id = ? AND game_id = ?`, row.Name, row.ContentType, row.TemplateKey, row.TemplateVersion, params, overrides, row.Status, row.CurrentVersion, row.UpdatedBy, row.UpdatedAt, row.ID, expected, row.TeamID, row.GameID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		changed = true
		return insertVersion(tx, row)
	})
	if err != nil {
		return model.ComposeStrategy{}, false, err
	}
	return row, changed, nil
}

func listStrategyVersions(db *gorm.DB, id int64) ([]model.ComposeStrategyVersion, error) {
	rows, err := db.Raw(`SELECT strategy_id, version, snapshot_json, created_by, created_at FROM compose_strategy_versions WHERE strategy_id = ? ORDER BY version DESC`, id).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.ComposeStrategyVersion{}
	for rows.Next() {
		var item model.ComposeStrategyVersion
		var snapshot []byte
		if err := rows.Scan(&item.StrategyID, &item.Version, &snapshot, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(snapshot, &item.Snapshot); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
