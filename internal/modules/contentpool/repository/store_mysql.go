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

const sourceColumns = `id, team_id, game_id, platform, platform_content_id, title, description, cover_url, source_url, author_id, author_sec_uid, author_uid, author_home_url, author_name, source_type, strategy_id, crawl_task_id, like_count, favorite_count, view_count, comment_count, share_count, published_at, status, ignored_reason, audit_note, failure_reason, material_id, created_by, updated_by, audited_by, audited_at, created_at, updated_at`

const sourceViewColumns = `s.id, s.team_id, s.game_id, s.platform, s.platform_content_id, s.title, s.description, s.cover_url, s.source_url, s.author_id, s.author_sec_uid, s.author_uid, s.author_home_url, s.author_name, s.source_type, s.strategy_id, s.crawl_task_id, s.like_count, s.favorite_count, s.view_count, s.comment_count, s.share_count, s.published_at, s.status, s.ignored_reason, s.audit_note, s.failure_reason, s.material_id, s.created_by, s.updated_by, s.audited_by, s.audited_at, s.created_at, s.updated_at, COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(t.snapshot_json, '$.strategy_name')), ''), st.name, '') AS strategy_name, CASE WHEN t.id IS NULL THEN '' ELSE CONCAT(COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(t.snapshot_json, '$.strategy_name')), ''), st.name, '人工任务'), '_', DATE_FORMAT(t.created_at, '%Y%m%d%H%i%s')) END AS crawl_task_name, COALESCE(u.nickname, u.username, '') AS created_by_name, COALESCE(au.nickname, au.username, '') AS audited_by_name`

func CreateSource(v model.SourceContent, raw json.RawMessage) (model.SourceContent, error) {
	return createSource(database.DB(), v, raw)
}
func createSource(db *gorm.DB, v model.SourceContent, raw json.RawMessage) (model.SourceContent, error) {
	result := db.Exec(`INSERT INTO source_contents (team_id, game_id, platform, platform_content_id, title, description, cover_url, source_url, author_id, author_sec_uid, author_uid, author_home_url, author_name, source_type, strategy_id, crawl_task_id, like_count, favorite_count, view_count, comment_count, share_count, published_at, status, raw_json, audit_note, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'pending',?,?,?,?,?)`,
		v.TeamID, nullableStringPtr(v.GameID), v.Platform, v.PlatformContentID, v.Title, nullIfEmpty(v.Description), nullIfEmpty(v.CoverURL), nullIfEmpty(v.SourceURL), nullIfEmpty(v.AuthorID), nullIfEmpty(v.AuthorSecUID), nullIfEmpty(v.AuthorUID), nullIfEmpty(v.AuthorHomeURL), nullIfEmpty(v.AuthorName), v.SourceType, nullableID(v.StrategyID), nullableID(v.CrawlTaskID), v.LikeCount, v.FavoriteCount, v.ViewCount, v.CommentCount, v.ShareCount, v.PublishedAt, raw, nullIfEmpty(v.AuditNote), v.CreatedBy, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if result.Error != nil {
		var mysqlError *mysql.MySQLError
		if errors.As(result.Error, &mysqlError) && mysqlError.Number == 1062 {
			return model.SourceContent{}, fmt.Errorf("%w: %v", errDuplicate, result.Error)
		}
		return model.SourceContent{}, result.Error
	}
	if err := db.Raw("SELECT LAST_INSERT_ID()").Row().Scan(&v.ID); err != nil {
		return model.SourceContent{}, err
	}
	return v, nil
}

func ListSources(filter Filter) ([]model.SourceContentView, error) {
	return listSources(database.DB(), filter)
}
func listSources(db *gorm.DB, filter Filter) ([]model.SourceContentView, error) {
	query := `SELECT ` + sourceViewColumns + ` FROM source_contents s LEFT JOIN discovery_strategies st ON st.id = s.strategy_id LEFT JOIN crawl_tasks t ON t.id = s.crawl_task_id LEFT JOIN users u ON u.id = s.created_by LEFT JOIN users au ON au.id = s.audited_by`
	conditions, args := sourceViewConditions(filter)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY s.id DESC LIMIT 500"
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SourceContentView{}
	for rows.Next() {
		item, err := scanSourceView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func FindSource(id int64) (model.SourceContentView, bool, error) {
	return findSource(database.DB(), id)
}
func findSource(db *gorm.DB, id int64) (model.SourceContentView, bool, error) {
	item, err := scanSourceView(db.Raw(`SELECT `+sourceViewColumns+` FROM source_contents s LEFT JOIN discovery_strategies st ON st.id = s.strategy_id LEFT JOIN crawl_tasks t ON t.id = s.crawl_task_id LEFT JOIN users u ON u.id = s.created_by LEFT JOIN users au ON au.id = s.audited_by WHERE s.id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.SourceContentView{}, false, nil
	}
	return item, err == nil, err
}

func sourceViewConditions(filter Filter) ([]string, []any) {
	conditions := []string{}
	args := []any{}
	if filter.TeamID != nil {
		conditions = append(conditions, "s.team_id = ?")
		args = append(args, *filter.TeamID)
	}
	if filter.Platform != "" {
		conditions = append(conditions, "s.platform = ?")
		args = append(args, filter.Platform)
	}
	if filter.Status != "" {
		conditions = append(conditions, "s.status = ?")
		args = append(args, filter.Status)
	}
	if filter.SourceType != "" {
		conditions = append(conditions, "s.source_type = ?")
		args = append(args, filter.SourceType)
	}
	if filter.StrategyID != nil {
		conditions = append(conditions, "s.strategy_id = ?")
		args = append(args, *filter.StrategyID)
	}
	if filter.CrawlTaskID != nil {
		conditions = append(conditions, "s.crawl_task_id = ?")
		args = append(args, *filter.CrawlTaskID)
	}
	if filter.MaterialID != nil {
		conditions = append(conditions, "s.material_id = ?")
		args = append(args, *filter.MaterialID)
	}
	if strings.TrimSpace(filter.Search) != "" {
		like := "%" + strings.TrimSpace(filter.Search) + "%"
		conditions = append(conditions, "(s.title LIKE ? OR s.platform_content_id LIKE ? OR s.author_name LIKE ? OR s.source_url LIKE ?)")
		args = append(args, like, like, like, like)
	}
	return conditions, args
}
func UpdateStatus(id int64, status model.Status, reason string, auditNote string, actorID sharedidentity.UserID, now time.Time) (model.SourceContent, error) {
	return updateStatus(database.DB(), id, status, reason, auditNote, actorID, now)
}
func updateStatus(db *gorm.DB, id int64, status model.Status, reason string, auditNote string, actorID sharedidentity.UserID, now time.Time) (model.SourceContent, error) {
	result := db.Exec(`UPDATE source_contents SET status = ?, ignored_reason = ?, audit_note = ?, updated_by = ?, audited_by = ?, audited_at = ?, updated_at = ? WHERE id = ?`, status, nullIfEmpty(reason), nullIfEmpty(auditNote), actorID, actorID, now, now, id)
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
	return item.SourceContent, nil
}
func RecordMaterialFailure(id int64, reason string) (model.SourceContent, error) {
	return recordMaterialFailure(database.DB(), id, reason)
}
func recordMaterialFailure(db *gorm.DB, id int64, reason string) (model.SourceContent, error) {
	result := db.Exec(`UPDATE source_contents SET failure_reason = ?, updated_at = ? WHERE id = ?`, truncateString(reason, 500), time.Now(), id)
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
	return item.SourceContent, nil
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
			return tx.Exec(`UPDATE source_contents SET status = 'material_created', ignored_reason = NULL, failure_reason = NULL, material_id = ?, updated_at = ? WHERE id = ?`, out.ID, now, id).Error
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
		return tx.Exec(`UPDATE source_contents SET status = 'material_created', ignored_reason = NULL, failure_reason = NULL, material_id = ?, updated_at = ? WHERE id = ?`, out.ID, now, id).Error
	})
	return out, err
}

type scannable interface{ Scan(...any) error }

func scanSource(row scannable) (model.SourceContent, error) {
	var item model.SourceContent
	var team, creator int64
	var likeCount, favoriteCount, viewCount, commentCount, shareCount int64
	var status string
	var strategyID, taskID, materialID, updatedBy, auditedBy sql.NullInt64
	var auditedAt sql.NullTime
	var gameID sql.NullString
	var description, cover, url, authorID, authorSecUID, authorUID, authorHomeURL, authorName, sourceType, reason, auditNote, failureReason sql.NullString
	var published sql.NullTime
	err := row.Scan(&item.ID, &team, &gameID, &item.Platform, &item.PlatformContentID, &item.Title, &description, &cover, &url, &authorID, &authorSecUID, &authorUID, &authorHomeURL, &authorName, &sourceType, &strategyID, &taskID, &likeCount, &favoriteCount, &viewCount, &commentCount, &shareCount, &published, &status, &reason, &auditNote, &failureReason, &materialID, &creator, &updatedBy, &auditedBy, &auditedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.TeamID = sharedidentity.TeamID(team)
	item.CreatedBy = sharedidentity.UserID(creator)
	item.Status = model.Status(status)
	item.Description, item.CoverURL, item.SourceURL, item.AuthorID, item.AuthorSecUID, item.AuthorUID, item.AuthorHomeURL, item.AuthorName, item.SourceType, item.IgnoredReason, item.AuditNote, item.FailureReason = description.String, cover.String, url.String, authorID.String, authorSecUID.String, authorUID.String, authorHomeURL.String, authorName.String, sourceType.String, reason.String, auditNote.String, failureReason.String
	item.LikeCount, item.FavoriteCount, item.ViewCount, item.CommentCount, item.ShareCount = likeCount, favoriteCount, viewCount, commentCount, shareCount
	if strategyID.Valid {
		value := strategyID.Int64
		item.StrategyID = &value
	}
	if taskID.Valid {
		value := taskID.Int64
		item.CrawlTaskID = &value
	}
	if materialID.Valid {
		value := materialID.Int64
		item.MaterialID = &value
	}
	if gameID.Valid {
		value := gameID.String
		item.GameID = &value
	}
	if published.Valid {
		item.PublishedAt = &published.Time
	}
	if updatedBy.Valid {
		value := sharedidentity.UserID(updatedBy.Int64)
		item.UpdatedBy = &value
	}
	if auditedBy.Valid {
		value := sharedidentity.UserID(auditedBy.Int64)
		item.AuditedBy = &value
	}
	if auditedAt.Valid {
		item.AuditedAt = &auditedAt.Time
	}
	return item, nil
}
func scanSourceView(row scannable) (model.SourceContentView, error) {
	var item model.SourceContentView
	var strategyName, crawlTaskName, createdByName, auditedByName sql.NullString
	base, err := scanSourceWithExtras(row, &strategyName, &crawlTaskName, &createdByName, &auditedByName)
	if err != nil {
		return item, err
	}
	item.SourceContent, item.StrategyName, item.CrawlTaskName, item.CreatedByName, item.AuditedByName = base, strategyName.String, crawlTaskName.String, createdByName.String, auditedByName.String
	return item, nil
}
func scanSourceWithExtras(row scannable, strategyName *sql.NullString, crawlTaskName *sql.NullString, createdByName *sql.NullString, auditedByName *sql.NullString) (model.SourceContent, error) {
	var item model.SourceContent
	var team, creator int64
	var likeCount, favoriteCount, viewCount, commentCount, shareCount int64
	var status string
	var strategyID, taskID, materialID, updatedBy, auditedBy sql.NullInt64
	var auditedAt sql.NullTime
	var gameID sql.NullString
	var description, cover, url, authorID, authorSecUID, authorUID, authorHomeURL, authorName, sourceType, reason, auditNote, failureReason sql.NullString
	var published sql.NullTime
	err := row.Scan(&item.ID, &team, &gameID, &item.Platform, &item.PlatformContentID, &item.Title, &description, &cover, &url, &authorID, &authorSecUID, &authorUID, &authorHomeURL, &authorName, &sourceType, &strategyID, &taskID, &likeCount, &favoriteCount, &viewCount, &commentCount, &shareCount, &published, &status, &reason, &auditNote, &failureReason, &materialID, &creator, &updatedBy, &auditedBy, &auditedAt, &item.CreatedAt, &item.UpdatedAt, strategyName, crawlTaskName, createdByName, auditedByName)
	if err != nil {
		return item, err
	}
	item.TeamID = sharedidentity.TeamID(team)
	item.CreatedBy = sharedidentity.UserID(creator)
	item.Status = model.Status(status)
	item.Description, item.CoverURL, item.SourceURL, item.AuthorID, item.AuthorSecUID, item.AuthorUID, item.AuthorHomeURL, item.AuthorName, item.SourceType, item.IgnoredReason, item.AuditNote, item.FailureReason = description.String, cover.String, url.String, authorID.String, authorSecUID.String, authorUID.String, authorHomeURL.String, authorName.String, sourceType.String, reason.String, auditNote.String, failureReason.String
	item.LikeCount, item.FavoriteCount, item.ViewCount, item.CommentCount, item.ShareCount = likeCount, favoriteCount, viewCount, commentCount, shareCount
	if strategyID.Valid {
		value := strategyID.Int64
		item.StrategyID = &value
	}
	if taskID.Valid {
		value := taskID.Int64
		item.CrawlTaskID = &value
	}
	if materialID.Valid {
		value := materialID.Int64
		item.MaterialID = &value
	}
	if gameID.Valid {
		value := gameID.String
		item.GameID = &value
	}
	if published.Valid {
		item.PublishedAt = &published.Time
	}
	if updatedBy.Valid {
		value := sharedidentity.UserID(updatedBy.Int64)
		item.UpdatedBy = &value
	}
	if auditedBy.Valid {
		value := sharedidentity.UserID(auditedBy.Int64)
		item.AuditedBy = &value
	}
	if auditedAt.Valid {
		item.AuditedAt = &auditedAt.Time
	}
	return item, nil
}

func nullableUserID(value *sharedidentity.UserID) any {
	if value == nil {
		return nil
	}
	return *value
}
func nullableID(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
func nullableStringPtr(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}
func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
func truncateString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

var errDuplicate = errors.New("content pool item already exists")
var errNotFound = errors.New("content pool item was not found")
