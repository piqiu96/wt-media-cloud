package contentpool

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

const sourceColumns = `id, team_id, platform, platform_content_id, title, description, cover_url, source_url, author_id, author_name, source_type, published_at, status, ignored_reason, created_by, created_at, updated_at`

func scanSource(row interface{ Scan(...any) error }) (SourceContent, error) {
	var v SourceContent
	var status string
	var team, creator int64
	var desc, cover, url, authorID, authorName, sourceType, reason sql.NullString
	var published sql.NullTime
	err := row.Scan(&v.ID, &team, &v.Platform, &v.PlatformContentID, &v.Title, &desc, &cover, &url, &authorID, &authorName, &sourceType, &published, &status, &reason, &creator, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	v.TeamID = identity.TeamID(team)
	v.CreatedBy = identity.UserID(creator)
	v.Status = Status(status)
	v.Description = desc.String
	v.CoverURL = cover.String
	v.SourceURL = url.String
	v.AuthorID = authorID.String
	v.AuthorName = authorName.String
	v.SourceType = sourceType.String
	v.IgnoredReason = reason.String
	if published.Valid {
		v.PublishedAt = &published.Time
	}
	return v, nil
}

func (s *MySQLStore) CreateSource(v SourceContent, raw json.RawMessage) (SourceContent, error) {
	res, err := s.db.Exec(`INSERT INTO source_contents (team_id, platform, platform_content_id, title, description, cover_url, source_url, author_id, author_name, source_type, published_at, status, raw_json, created_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,'pending',?,?,?,?)`, v.TeamID, v.Platform, v.PlatformContentID, v.Title, nullIfEmpty(v.Description), nullIfEmpty(v.CoverURL), nullIfEmpty(v.SourceURL), nullIfEmpty(v.AuthorID), nullIfEmpty(v.AuthorName), v.SourceType, v.PublishedAt, raw, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return SourceContent{}, fmt.Errorf("%w: %v", ErrDuplicate, err)
		}
		return SourceContent{}, err
	}
	id, _ := res.LastInsertId()
	v.ID = id
	return v, nil
}
func (s *MySQLStore) ListSources(f Filter) ([]SourceContent, error) {
	q := `SELECT ` + sourceColumns + ` FROM source_contents`
	var cond []string
	var args []any
	if f.TeamID != nil {
		cond = append(cond, "team_id = ?")
		args = append(args, *f.TeamID)
	}
	if f.Platform != "" {
		cond = append(cond, "platform = ?")
		args = append(args, f.Platform)
	}
	if f.Status != "" {
		cond = append(cond, "status = ?")
		args = append(args, f.Status)
	}
	if f.SourceType != "" {
		cond = append(cond, "source_type = ?")
		args = append(args, f.SourceType)
	}
	if strings.TrimSpace(f.Search) != "" {
		like := "%" + strings.TrimSpace(f.Search) + "%"
		cond = append(cond, "(title LIKE ? OR platform_content_id LIKE ? OR author_name LIKE ? OR source_url LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if len(cond) > 0 {
		q += " WHERE " + strings.Join(cond, " AND ")
	}
	q += " ORDER BY created_at DESC, id DESC LIMIT 500"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SourceContent{}
	for rows.Next() {
		v, e := scanSource(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *MySQLStore) FindSource(id int64) (SourceContent, bool, error) {
	v, e := scanSource(s.db.QueryRow(`SELECT `+sourceColumns+` FROM source_contents WHERE id = ?`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return SourceContent{}, false, nil
	}
	return v, e == nil, e
}
func (s *MySQLStore) UpdateStatus(id int64, status Status, reason string) (SourceContent, error) {
	res, err := s.db.Exec(`UPDATE source_contents SET status = ?, ignored_reason = ?, updated_at = ? WHERE id = ?`, status, nullIfEmpty(reason), time.Now(), id)
	if err != nil {
		return SourceContent{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return SourceContent{}, ErrNotFound
	}
	v, ok, e := s.FindSource(id)
	if e != nil {
		return SourceContent{}, e
	}
	if !ok {
		return SourceContent{}, ErrNotFound
	}
	return v, nil
}
func (s *MySQLStore) Materialize(id int64, creator identity.UserID, now time.Time) (Material, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Material{}, err
	}
	defer tx.Rollback()
	v, e := scanSource(tx.QueryRow(`SELECT `+sourceColumns+` FROM source_contents WHERE id = ? FOR UPDATE`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return Material{}, ErrNotFound
	}
	if e != nil {
		return Material{}, e
	}
	var m Material
	var snapshot = []byte{}
	e = tx.QueryRow(`SELECT id, team_id, source_content_id, title, source_snapshot, created_by, created_at, updated_at FROM materials WHERE source_content_id = ?`, id).Scan(&m.ID, &m.TeamID, &m.SourceContentID, &m.Title, &snapshot, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt)
	if e == nil {
		_ = json.Unmarshal(snapshot, &m.SourceSnapshot)
		return m, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Material{}, e
	}
	snap, _ := json.Marshal(map[string]any{"platform": v.Platform, "platform_content_id": v.PlatformContentID, "title": v.Title, "author_id": v.AuthorID, "author_name": v.AuthorName, "source_url": v.SourceURL, "created_at": now})
	res, e := tx.Exec(`INSERT INTO materials (team_id, source_content_id, title, source_snapshot, created_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`, v.TeamID, v.ID, v.Title, snap, creator, now, now)
	if e != nil {
		return Material{}, e
	}
	m.ID, _ = res.LastInsertId()
	m.TeamID = v.TeamID
	m.SourceContentID = v.ID
	m.Title = v.Title
	m.CreatedBy = creator
	m.CreatedAt = now
	m.UpdatedAt = now
	_ = json.Unmarshal(snap, &m.SourceSnapshot)
	if _, e = tx.Exec(`UPDATE source_contents SET status = 'material_created', ignored_reason = NULL, updated_at = ? WHERE id = ?`, now, id); e != nil {
		return Material{}, e
	}
	if e = tx.Commit(); e != nil {
		return Material{}, e
	}
	return m, nil
}
func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
