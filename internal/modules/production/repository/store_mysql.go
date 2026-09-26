// Package repository persists the M4 production facts with explicit SQL.
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
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("production record not found")

type CreateUsageInput struct {
	TeamID     identity.TeamID
	MaterialID int64
	UserID     identity.UserID
}

type MaterialFilter struct {
	TeamID  *identity.TeamID
	GameIDs []string
	Search  string
	Limit   int
}

const materialProjectionColumns = `m.id, m.team_id, s.game_id, m.source_content_id, m.title, s.source_url, s.platform, s.author_name, s.published_at, m.video_status, m.source_object_key, m.video_size_bytes, m.video_sha256, m.video_media_json, m.video_error, m.video_prepared_at, m.created_at, m.updated_at`

// FindMaterial returns the material readiness projection only within the
// caller's team boundary. Game-range filtering is applied by the Service.
func FindMaterial(id int64, teamID identity.TeamID) (model.Material, bool, error) {
	return findMaterial(database.DB(), id, teamID)
}

func findMaterial(db *gorm.DB, id int64, teamID identity.TeamID) (model.Material, bool, error) {
	if id <= 0 || teamID <= 0 {
		return model.Material{}, false, fmt.Errorf("invalid material lookup")
	}
	var material model.Material
	err := scanMaterial(db.Raw(`SELECT `+materialProjectionColumns+` FROM materials m JOIN source_contents s ON s.id = m.source_content_id WHERE m.id = ? AND m.team_id = ?`, id, teamID).Row(), &material)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Material{}, false, nil
	}
	return material, err == nil, err
}

// FindMaterialUnscoped is for the production Service, which applies the
// actor's team and game scope before returning the material to a caller.
func FindMaterialUnscoped(id int64) (model.Material, bool, error) {
	return findMaterialUnscoped(database.DB(), id)
}

func findMaterialUnscoped(db *gorm.DB, id int64) (model.Material, bool, error) {
	if id <= 0 {
		return model.Material{}, false, fmt.Errorf("invalid material lookup")
	}
	var material model.Material
	err := scanMaterial(db.Raw(`SELECT `+materialProjectionColumns+` FROM materials m JOIN source_contents s ON s.id = m.source_content_id WHERE m.id = ?`, id).Row(), &material)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Material{}, false, nil
	}
	return material, err == nil, err
}

func ListMaterials(filter MaterialFilter) ([]model.Material, error) {
	return listMaterials(database.DB(), filter)
}

func listMaterials(db *gorm.DB, filter MaterialFilter) ([]model.Material, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, len(filter.GameIDs)+4)
	if filter.TeamID != nil {
		conditions = append(conditions, "m.team_id = ?")
		args = append(args, *filter.TeamID)
	}
	if len(filter.GameIDs) > 0 {
		placeholders := make([]string, 0, len(filter.GameIDs))
		for _, gameID := range filter.GameIDs {
			gameID = strings.TrimSpace(gameID)
			if gameID == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, gameID)
		}
		if len(placeholders) > 0 {
			conditions = append(conditions, "s.game_id IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		conditions = append(conditions, "(m.title LIKE ? OR s.platform_content_id LIKE ? OR s.author_name LIKE ?)")
		args = append(args, like, like, like)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	query := `SELECT ` + materialProjectionColumns + ` FROM materials m JOIN source_contents s ON s.id = m.source_content_id`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY m.id DESC LIMIT %d", limit)
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.Material, 0)
	for rows.Next() {
		var item model.Material
		if err := scanMaterial(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateOrRestoreUsage atomically creates the sole usage row or restores its
// historical row. The unique key prevents concurrent clicks from creating two.
//
// The `created` answer is what tells a caller whether it should say 201 or 200,
// and it is read out of the statement rather than out of the row: an insert
// touches one row, a restore changes one, and a click on an already-active
// relation changes nothing at all. See the statement for why "changes nothing"
// is a real outcome here and not a coincidence.
func CreateOrRestoreUsage(input CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	return createOrRestoreUsage(database.DB(), input, now)
}

func createOrRestoreUsage(db *gorm.DB, input CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	if input.TeamID <= 0 || input.MaterialID <= 0 || input.UserID <= 0 {
		return model.MaterialUsage{}, false, fmt.Errorf("invalid material usage input")
	}
	var usage model.MaterialUsage
	created := false
	err := db.Transaction(func(tx *gorm.DB) error {
		// Every assignment that reads `status` is written **before** the assignment
		// that sets it, because MySQL evaluates these left to right and a later one
		// sees the new value. Reading `status` after it had been set to 'active'
		// would make both `IF(status = 'removed', ...)` branches take the
		// already-restored path: a restored row would keep its stale `removed_at`,
		// and no row would ever look unchanged.
		//
		// `updated_at` is conditional for the same reason it exists: if it were
		// `VALUES(updated_at)` then every click would count as a change,
		// `RowsAffected` would be 2 even when the relation was already active, and
		// a repeat click would answer 201 forever. `id = LAST_INSERT_ID(id)` is
		// how the read below finds the row in all three cases, including the one
		// that touched nothing.
		result := tx.Exec(`INSERT INTO material_usages (team_id, material_id, user_id, status, created_at, updated_at) VALUES (?,?,?,'active',?,?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), removed_at = IF(status = 'removed', NULL, removed_at), updated_at = IF(status = 'removed', VALUES(updated_at), updated_at), status = IF(status = 'removed', 'active', status)`, input.TeamID, input.MaterialID, input.UserID, now, now)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected != 0
		return scanUsage(tx.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = LAST_INSERT_ID()`).Row(), &usage)
	})
	return usage, created, err
}

// RemoveUsage preserves the historical usage row and its downstream task
// references; only its active relationship is removed.
func RemoveUsage(teamID identity.TeamID, materialID int64, userID identity.UserID, now time.Time) (bool, error) {
	return removeUsage(database.DB(), teamID, materialID, userID, now)
}

func removeUsage(db *gorm.DB, teamID identity.TeamID, materialID int64, userID identity.UserID, now time.Time) (bool, error) {
	if teamID <= 0 || materialID <= 0 || userID <= 0 {
		return false, fmt.Errorf("invalid material usage removal")
	}
	result := db.Exec(`UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE team_id = ? AND material_id = ? AND user_id = ? AND status = 'active'`, now, now, teamID, materialID, userID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func ListActiveUsages(userID identity.UserID) ([]model.MaterialUsage, error) {
	return listActiveUsages(database.DB(), userID)
}

func listActiveUsages(db *gorm.DB, userID identity.UserID) ([]model.MaterialUsage, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid material usage user")
	}
	rows, err := db.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id = ? AND status = 'active' ORDER BY updated_at DESC, id DESC`, userID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.MaterialUsage, 0)
	for rows.Next() {
		var usage model.MaterialUsage
		if err := scanUsage(rows, &usage); err != nil {
			return nil, err
		}
		items = append(items, usage)
	}
	return items, rows.Err()
}

func FindUsageForUser(usageID int64, userID identity.UserID) (model.MaterialUsage, bool, error) {
	return findUsageForUser(database.DB(), usageID, userID)
}

func findUsageForUser(db *gorm.DB, usageID int64, userID identity.UserID) (model.MaterialUsage, bool, error) {
	if usageID <= 0 || userID <= 0 {
		return model.MaterialUsage{}, false, fmt.Errorf("invalid material usage lookup")
	}
	var usage model.MaterialUsage
	err := scanUsage(db.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = ? AND user_id = ?`, usageID, userID).Row(), &usage)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MaterialUsage{}, false, nil
	}
	return usage, err == nil, err
}

func RemoveUsageByID(usageID int64, userID identity.UserID, now time.Time) (bool, error) {
	return removeUsageByID(database.DB(), usageID, userID, now)
}

func removeUsageByID(db *gorm.DB, usageID int64, userID identity.UserID, now time.Time) (bool, error) {
	if usageID <= 0 || userID <= 0 {
		return false, fmt.Errorf("invalid material usage removal")
	}
	result := db.Exec(`UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'active'`, now, now, usageID, userID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

type rowScanner interface{ Scan(...any) error }

func scanUsage(row rowScanner, usage *model.MaterialUsage) error {
	var teamID, userID int64
	var status string
	var removedAt sql.NullTime
	if err := row.Scan(&usage.ID, &teamID, &usage.MaterialID, &userID, &status, &removedAt, &usage.CreatedAt, &usage.UpdatedAt); err != nil {
		return err
	}
	usage.TeamID = identity.TeamID(teamID)
	usage.UserID = identity.UserID(userID)
	usage.Status = model.MaterialUsageStatus(status)
	if removedAt.Valid {
		usage.RemovedAt = &removedAt.Time
	}
	return nil
}

func scanMaterial(row rowScanner, material *model.Material) error {
	var teamID int64
	var gameID, sourceURL, authorName, objectKey, sha256, videoError sql.NullString
	var publishedAt, preparedAt sql.NullTime
	var size sql.NullInt64
	var status string
	var mediaJSON []byte
	if err := row.Scan(&material.ID, &teamID, &gameID, &material.SourceContentID, &material.Title, &sourceURL, &material.Platform, &authorName, &publishedAt, &status, &objectKey, &size, &sha256, &mediaJSON, &videoError, &preparedAt, &material.CreatedAt, &material.UpdatedAt); err != nil {
		return err
	}
	material.TeamID = identity.TeamID(teamID)
	material.SourceURL = sourceURL.String
	material.AuthorName = authorName.String
	material.VideoStatus = model.VideoStatus(status)
	material.SourceObjectKey = objectKey.String
	material.VideoSHA256 = sha256.String
	material.VideoError = videoError.String
	if gameID.Valid {
		value := gameID.String
		material.GameID = &value
	}
	if publishedAt.Valid {
		material.PublishedAt = &publishedAt.Time
	}
	if size.Valid {
		value := size.Int64
		material.VideoSizeBytes = &value
	}
	if preparedAt.Valid {
		material.VideoPreparedAt = &preparedAt.Time
	}
	if len(mediaJSON) > 0 {
		if err := json.Unmarshal(mediaJSON, &material.VideoMedia); err != nil {
			return fmt.Errorf("decode material media summary: %w", err)
		}
	}
	return nil
}
