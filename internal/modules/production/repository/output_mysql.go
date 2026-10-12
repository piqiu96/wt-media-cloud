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

// ComposeOutputFilter scopes the my-outputs list the same way the compose-task
// list is scoped: admins pass nothing, everyone else passes their team, games
// and (for operators) the created-by narrowing.
type ComposeOutputFilter struct {
	TeamID        int64
	GameIDs       []string
	CreatedBy     int64
	PublishStatus string
}

const composeOutputColumns = `id, team_id, game_id, material_id, material_usage_id, compose_task_id, compose_strategy_id, template_code, template_version, object_key, file_size_bytes, sha256, media_json, publish_status, created_by, created_at, updated_at`

// ComposeOutputStore reads the verified-output asset table and archives rows.
// Writes to this table belong to the worker's success transaction; the only
// actor-facing write is the archive flip.
type ComposeOutputStore struct{}

func (ComposeOutputStore) List(filter ComposeOutputFilter) ([]model.CompositeOutput, error) {
	return listComposeOutputs(database.DB(), filter)
}

func (ComposeOutputStore) Find(id int64) (model.CompositeOutput, bool, error) {
	return findComposeOutput(database.DB(), id)
}

func (ComposeOutputStore) Archive(id, actorID int64, now time.Time) (bool, error) {
	return archiveComposeOutput(database.DB(), id, actorID, now)
}

func listComposeOutputs(db *gorm.DB, filter ComposeOutputFilter) ([]model.CompositeOutput, error) {
	query := `SELECT ` + composeOutputColumns + ` FROM composite_outputs WHERE 1=1`
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
	if filter.CreatedBy > 0 {
		query += ` AND created_by = ?`
		args = append(args, filter.CreatedBy)
	}
	if filter.PublishStatus != "" {
		query += ` AND publish_status = ?`
		args = append(args, filter.PublishStatus)
	}
	query += ` ORDER BY id DESC`
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.CompositeOutput, 0)
	for rows.Next() {
		item, err := scanComposeOutput(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func findComposeOutput(db *gorm.DB, id int64) (model.CompositeOutput, bool, error) {
	if id <= 0 {
		return model.CompositeOutput{}, false, nil
	}
	item, err := scanComposeOutput(db.Raw(`SELECT `+composeOutputColumns+` FROM composite_outputs WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.CompositeOutput{}, false, nil
	}
	return item, err == nil, err
}

// archiveComposeOutput flips pending_publish to archived in one conditional
// write and audits it in the same transaction. Published rows and rows that
// are already archived write nothing — the caller names the refusal.
func archiveComposeOutput(db *gorm.DB, outputID, actorID int64, now time.Time) (bool, error) {
	ok := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE composite_outputs SET publish_status = 'archived', updated_at = ? WHERE id = ? AND publish_status = 'pending_publish'`, now, outputID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		ok = true
		summary, err := json.Marshal(map[string]any{"publish_status": "archived"})
		if err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, 'compose_output.archive', 'compose_output', ?, ?, ?)`, id.NewID("audit"), actorID, fmt.Sprint(outputID), summary, now).Error
	})
	if err != nil {
		return false, err
	}
	return ok, nil
}

type composeOutputScanner interface{ Scan(...any) error }

func scanComposeOutput(row composeOutputScanner) (model.CompositeOutput, error) {
	var item model.CompositeOutput
	var media []byte
	err := row.Scan(&item.ID, &item.TeamID, &item.GameID, &item.MaterialID, &item.MaterialUsageID, &item.TaskID, &item.StrategyID, &item.TemplateCode, &item.TemplateVersion, &item.ObjectKey, &item.FileSizeBytes, &item.SHA256, &media, &item.PublishStatus, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.CompositeOutput{}, err
	}
	item.Media = media
	return item, nil
}
