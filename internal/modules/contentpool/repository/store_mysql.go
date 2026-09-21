package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

const sourceColumns = `id, team_id, platform, platform_content_id, title, description, cover_url, source_url, author_id, author_name, source_type, published_at, status, ignored_reason, created_by, created_at, updated_at`

func CreateSource(v model.SourceContent, raw json.RawMessage) (model.SourceContent, error) {
	return createSource(database.DB(), v, raw)
}
func createSource(db *gorm.DB, v model.SourceContent, raw json.RawMessage) (model.SourceContent, error) {
	result := db.Exec(`INSERT INTO source_contents (team_id, platform, platform_content_id, title, description, cover_url, source_url, author_id, author_name, source_type, published_at, status, raw_json, created_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,'pending',?,?,?,?)`, v.TeamID, v.Platform, v.PlatformContentID, v.Title, nullIfEmpty(v.Description), nullIfEmpty(v.CoverURL), nullIfEmpty(v.SourceURL), nullIfEmpty(v.AuthorID), nullIfEmpty(v.AuthorName), v.SourceType, v.PublishedAt, raw, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if result.Error != nil {
		var mysqlError *mysql.MySQLError
		if errors.As(result.Error, &mysqlError) && mysqlError.Number == 1062 {
			return model.SourceContent{}, fmt.Errorf("%w: %v", errDuplicate, result.Error)
		}
		return model.SourceContent{}, result.Error
	}
	v.ID = result.RowsAffected // MySQL insert IDs are retrieved explicitly to preserve SQL semantics.
	if err := db.Raw("SELECT LAST_INSERT_ID()").Row().Scan(&v.ID); err != nil {
		return model.SourceContent{}, err
	}
	return v, nil
}

func ListSources(filter Filter) ([]model.SourceContent, error) {
	return listSources(database.DB(), filter)
}
func listSources(db *gorm.DB, filter Filter) ([]model.SourceContent, error) {
	query := `SELECT ` + sourceColumns + ` FROM source_contents`
	conditions := []string{}
	args := []any{}
	if filter.TeamID != nil {
		conditions = append(conditions, "team_id = ?")
		args = append(args, *filter.TeamID)
	}
	if filter.Platform != "" {
		conditions = append(conditions, "platform = ?")
		args = append(args, filter.Platform)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.SourceType != "" {
		conditions = append(conditions, "source_type = ?")
		args = append(args, filter.SourceType)
	}
	if strings.TrimSpace(filter.Search) != "" {
		like := "%" + strings.TrimSpace(filter.Search) + "%"
		conditions = append(conditions, "(title LIKE ? OR platform_content_id LIKE ? OR author_name LIKE ? OR source_url LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT 500"
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SourceContent{}
	for rows.Next() {
		item, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func FindSource(id int64) (model.SourceContent, bool, error) { return findSource(database.DB(), id) }
func findSource(db *gorm.DB, id int64) (model.SourceContent, bool, error) {
	item, err := scanSource(db.Raw(`SELECT `+sourceColumns+` FROM source_contents WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.SourceContent{}, false, nil
	}
	return item, err == nil, err
}
func UpdateStatus(id int64, status model.Status, reason string) (model.SourceContent, error) {
	return updateStatus(database.DB(), id, status, reason)
}
func updateStatus(db *gorm.DB, id int64, status model.Status, reason string) (model.SourceContent, error) {
	result := db.Exec(`UPDATE source_contents SET status = ?, ignored_reason = ?, updated_at = ? WHERE id = ?`, status, nullIfEmpty(reason), time.Now(), id)
	if result.Error != nil {
		return model.SourceContent{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.SourceContent{}, errNotFound
	}
	item, ok, err := findSource(db, id)
	if err != nil {
		return model.SourceContent{}, err
	}
	if !ok {
		return model.SourceContent{}, errNotFound
	}
	return item, nil
}
func Materialize(id int64, creator int64, now time.Time) (model.Material, error) {
	return materialize(database.DB(), id, creator, now)
}
func materialize(db *gorm.DB, id int64, creator int64, now time.Time) (model.Material, error) {
	var out model.Material
	err := db.Transaction(func(tx *gorm.DB) error {
		source, err := scanSource(tx.Raw(`SELECT `+sourceColumns+` FROM source_contents WHERE id = ? FOR UPDATE`, id).Row())
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		var snapshot []byte
		err = tx.Raw(`SELECT id, team_id, source_content_id, title, source_snapshot, created_by, created_at, updated_at FROM materials WHERE source_content_id = ?`, id).Row().Scan(&out.ID, &out.TeamID, &out.SourceContentID, &out.Title, &snapshot, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
		if err == nil {
			_ = json.Unmarshal(snapshot, &out.SourceSnapshot)
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		snapshot, _ = json.Marshal(map[string]any{"platform": source.Platform, "platform_content_id": source.PlatformContentID, "title": source.Title, "author_id": source.AuthorID, "author_name": source.AuthorName, "source_url": source.SourceURL, "created_at": now})
		result := tx.Exec(`INSERT INTO materials (team_id, source_content_id, title, source_snapshot, created_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`, source.TeamID, source.ID, source.Title, snapshot, creator, now, now)
		if result.Error != nil {
			return result.Error
		}
		if err := tx.Raw("SELECT LAST_INSERT_ID()").Row().Scan(&out.ID); err != nil {
			return err
		}
		out.TeamID, out.SourceContentID, out.Title, out.CreatedBy, out.CreatedAt, out.UpdatedAt = source.TeamID, source.ID, source.Title, sharedidentity.UserID(creator), now, now
		_ = json.Unmarshal(snapshot, &out.SourceSnapshot)
		return tx.Exec(`UPDATE source_contents SET status = 'material_created', ignored_reason = NULL, updated_at = ? WHERE id = ?`, now, id).Error
	})
	return out, err
}

type scannable interface{ Scan(...any) error }

func scanSource(row scannable) (model.SourceContent, error) {
	var item model.SourceContent
	var team, creator int64
	var status string
	var description, cover, url, authorID, authorName, sourceType, reason sql.NullString
	var published sql.NullTime
	err := row.Scan(&item.ID, &team, &item.Platform, &item.PlatformContentID, &item.Title, &description, &cover, &url, &authorID, &authorName, &sourceType, &published, &status, &reason, &creator, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.TeamID = sharedidentity.TeamID(team)
	item.CreatedBy = sharedidentity.UserID(creator)
	item.Status = model.Status(status)
	item.Description, item.CoverURL, item.SourceURL, item.AuthorID, item.AuthorName, item.SourceType, item.IgnoredReason = description.String, cover.String, url.String, authorID.String, authorName.String, sourceType.String, reason.String
	if published.Valid {
		item.PublishedAt = &published.Time
	}
	return item, nil
}
func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

var errDuplicate = errors.New("content pool item already exists")
var errNotFound = errors.New("content pool item was not found")
