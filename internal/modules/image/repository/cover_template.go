package repository

import (
	"encoding/json"
	"errors"
	mysql "github.com/go-sql-driver/mysql"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

type TemplateStore struct{ DB *gorm.DB }
type TemplateFilter struct {
	Search, ContentStatus, CategoryID, Platform, Scope string
	Enabled                                            *bool
	Width, Height, Page, PageSize                      int
}
type AssetBinding struct {
	ID     uint64
	FileID int64
}

func (s TemplateStore) Get(id uint64) (model.CoverTemplate, error) {
	var v model.CoverTemplate
	e := s.DB.First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	return v, e
}
func (s TemplateStore) ByRequest(actor int64, key string) (model.CoverTemplate, error) {
	var v model.CoverTemplate
	e := s.DB.Where("created_by=? AND request_key=?", actor, key).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	return v, e
}
func lockTemplateFile(tx *gorm.DB, bindings []AssetBinding, extra []int64) error {
	ids := append([]int64{}, extra...)
	for _, b := range bindings {
		ids = append(ids, b.FileID)
	}
	if e := files.LockAvailable(tx, ids); e != nil {
		return e
	}
	for _, b := range bindings {
		var n int64
		if e := tx.Model(&model.ImageAsset{}).Where("id=? AND file_id=?", b.ID, b.FileID).Count(&n).Error; e != nil {
			return e
		}
		if n != 1 {
			return ErrConflict
		}
	}
	return nil
}
func (s TemplateStore) Create(v model.CoverTemplate, bindings []AssetBinding) (model.CoverTemplate, error) {
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := lockTemplateFile(tx, bindings, nil); e != nil {
			return e
		}
		return tx.Create(&v).Error
	})
	var duplicate *mysql.MySQLError
	if errors.As(e, &duplicate) && duplicate.Number == 1062 {
		return v, ErrConflict
	}
	return v, e
}
func (s TemplateStore) SaveDraft(v model.CoverTemplate, revision uint64, bindings []AssetBinding) (model.CoverTemplate, error) {
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := lockTemplateFile(tx, bindings, nil); e != nil {
			return e
		}
		var draft any
		if len(v.DraftConfig) > 0 {
			draft = string(v.DraftConfig)
		}
		r := tx.Model(&model.CoverTemplate{}).Where("id=? AND revision=?", v.ID, revision).Updates(map[string]any{"draft_config_json": draft, "revision": gorm.Expr("revision+1"), "updated_by": v.UpdatedBy, "updated_at": v.UpdatedAt})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	})
	if e != nil {
		return v, e
	}
	return s.Get(v.ID)
}
func (s TemplateStore) Publish(v model.CoverTemplate, revision uint64, bindings []AssetBinding) (model.CoverTemplate, error) {
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		extra := []int64{}
		if v.PublishedThumbnailFileID != nil {
			extra = append(extra, *v.PublishedThumbnailFileID)
		}
		if e := lockTemplateFile(tx, bindings, extra); e != nil {
			return e
		}
		r := tx.Model(&model.CoverTemplate{}).Where("id=? AND revision=? AND draft_config_json IS NOT NULL", v.ID, revision).Updates(map[string]any{"draft_config_json": nil, "published_config_json": string(v.PublishedConfig), "published_revision": gorm.Expr("published_revision+1"), "published_thumbnail_file_id": v.PublishedThumbnailFileID, "revision": gorm.Expr("revision+1"), "updated_by": v.UpdatedBy, "updated_at": v.UpdatedAt})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	})
	if e != nil {
		return v, e
	}
	return s.Get(v.ID)
}
func (s TemplateStore) SetUsage(id, revision uint64, actor int64, enabled bool) (model.CoverTemplate, error) {
	r := s.DB.Model(&model.CoverTemplate{}).Where("id=? AND revision=? AND published_revision>0", id, revision).Updates(map[string]any{"enabled": enabled, "revision": gorm.Expr("revision+1"), "updated_by": actor, "updated_at": time.Now().UTC()})
	if r.Error != nil {
		return model.CoverTemplate{}, r.Error
	}
	if r.RowsAffected != 1 {
		return model.CoverTemplate{}, ErrConflict
	}
	return s.Get(id)
}
func (s TemplateStore) Delete(id, revision uint64) error {
	r := s.DB.Where("id=? AND revision=? AND published_revision=0", id, revision).Delete(&model.CoverTemplate{})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
func (s TemplateStore) List(sc model.Scope, f TemplateFilter) ([]model.CoverTemplate, int64, error) {
	q := s.DB.Model(&model.CoverTemplate{})
	if !sc.All {
		if sc.OwnerID > 0 {
			q = q.Where("created_by=?", sc.OwnerID)
		} else {
			q = q.Where("team_id=?", sc.TeamID)
		}
		p := []string{"JSON_LENGTH(category_ids_json)=0"}
		args := []any{}
		for _, id := range sc.CategoryIDs {
			p = append(p, "JSON_CONTAINS(category_ids_json,JSON_QUOTE(?))")
			args = append(args, id)
		}
		q = q.Where("("+strings.Join(p, " OR ")+")", args...)
	}
	if f.Scope == "general" {
		q = q.Where("JSON_LENGTH(category_ids_json)=0")
	}
	if f.Search != "" {
		q = q.Where("name LIKE ?", "%"+f.Search+"%")
	}
	if f.Enabled != nil {
		q = q.Where("enabled=?", *f.Enabled)
	}
	if f.ContentStatus == "draft" {
		q = q.Where("draft_config_json IS NOT NULL")
	}
	if f.ContentStatus == "published" {
		q = q.Where("published_revision>0")
	}
	if f.CategoryID != "" {
		q = q.Where("JSON_CONTAINS(category_ids_json,JSON_QUOTE(?)) OR JSON_LENGTH(category_ids_json)=0", f.CategoryID)
	}
	if f.Platform != "" {
		q = q.Where("JSON_CONTAINS(platform_codes_json,JSON_QUOTE(?)) OR JSON_LENGTH(platform_codes_json)=0", f.Platform)
	}
	if f.Width > 0 {
		q = q.Where("canvas_width=?", f.Width)
	}
	if f.Height > 0 {
		q = q.Where("canvas_height=?", f.Height)
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		return nil, 0, e
	}
	out := []model.CoverTemplate{}
	e := q.Order("created_at DESC,id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&out).Error
	return out, total, e
}
func TemplateFileReferenced(tx *gorm.DB, fileID int64) (bool, error) {
	var n int64
	e := tx.Model(&model.CoverTemplate{}).Where("published_thumbnail_file_id=?", fileID).Count(&n).Error
	if e != nil || n > 0 {
		return n > 0, e
	}
	var id uint64
	e = tx.Model(&model.ImageAsset{}).Where("file_id=?", fileID).Select("id").Scan(&id).Error
	if e != nil || id == 0 {
		return false, e
	}
	idText := strconv.FormatUint(id, 10)
	e = tx.Model(&model.CoverTemplate{}).Where("JSON_UNQUOTE(JSON_EXTRACT(draft_config_json,'$.imageAssetId'))=? OR JSON_UNQUOTE(JSON_EXTRACT(published_config_json,'$.imageAssetId'))=?", idText, idText).Count(&n).Error
	return n > 0, e
}
func ListTemplateFileReference(db *gorm.DB, sc model.Scope, fileID int64) ([]model.Reference, error) {
	var id uint64
	if e := db.Model(&model.ImageAsset{}).Where("file_id=?", fileID).Select("id").Scan(&id).Error; e != nil {
		return nil, e
	}
	q := db.Model(&model.CoverTemplate{}).Where("published_thumbnail_file_id=?", fileID)
	if id > 0 {
		v := strconv.FormatUint(id, 10)
		q = db.Model(&model.CoverTemplate{}).Where("published_thumbnail_file_id=? OR JSON_UNQUOTE(JSON_EXTRACT(draft_config_json,'$.imageAssetId'))=? OR JSON_UNQUOTE(JSON_EXTRACT(published_config_json,'$.imageAssetId'))=?", fileID, v, v)
	}
	if !sc.All {
		if sc.OwnerID > 0 {
			q = q.Where("created_by=?", sc.OwnerID)
		} else {
			q = q.Where("team_id=?", sc.TeamID)
		}
		predicates := []string{"JSON_LENGTH(category_ids_json)=0"}
		args := []any{}
		for _, id := range sc.CategoryIDs {
			predicates = append(predicates, "JSON_CONTAINS(category_ids_json,JSON_QUOTE(?))")
			args = append(args, id)
		}
		q = q.Where("("+strings.Join(predicates, " OR ")+")", args...)
	}
	var ids []uint64
	if e := q.Pluck("id", &ids).Error; e != nil {
		return nil, e
	}
	out := []model.Reference{}
	for _, id := range ids {
		out = append(out, model.Reference{Kind: "cover_template", ID: strconv.FormatUint(id, 10)})
	}
	return out, nil
}
func SameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	canonicalX, _ := json.Marshal(x)
	canonicalY, _ := json.Marshal(y)
	return string(canonicalX) == string(canonicalY)
}
