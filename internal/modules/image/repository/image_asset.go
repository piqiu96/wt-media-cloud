package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("image asset not found")
var ErrConflict = errors.New("image asset revision or registration conflict")

type Filter struct {
	Search, Usage, CategoryID, CategoryMode, Aspect, Source string
	Enabled                                                 *bool
	Page, PageSize, MinWidth, MinHeight                     int
}
type Store struct{ DB *gorm.DB }

func (s Store) query() *gorm.DB {
	return s.DB.Table("image_asset AS a").Joins("JOIN operations_files AS f ON f.id=a.file_id").Where("f.uploaded_at IS NOT NULL AND f.deleted_at IS NULL")
}

const projection = "a.*, f.metadata_json, f.resource_token, f.size_bytes"

func (s Store) Get(id uint64) (model.ImageAsset, error) {
	var v model.ImageAsset
	r := s.query().Select(projection).Where("a.id=?", id).Limit(1).Scan(&v)
	if r.Error != nil {
		return v, r.Error
	}
	if r.RowsAffected == 0 {
		return v, ErrNotFound
	}
	return v, nil
}
func (s Store) ByFile(id int64) (model.ImageAsset, error) {
	var v model.ImageAsset
	r := s.query().Select(projection).Where("a.file_id=?", id).Limit(1).Scan(&v)
	if r.Error != nil {
		return v, r.Error
	}
	if r.RowsAffected == 0 {
		return v, ErrNotFound
	}
	return v, nil
}
func (s Store) Register(v model.ImageAsset) (model.ImageAsset, error) {
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := files.LockAvailable(tx, []int64{v.FileID}); e != nil {
			return e
		}
		return tx.Create(&v).Error
	})
	var duplicate *mysql.MySQLError
	if errors.As(e, &duplicate) && duplicate.Number == 1062 {
		return model.ImageAsset{}, ErrConflict
	}
	if e != nil {
		return model.ImageAsset{}, e
	}
	return s.Get(v.ID)
}
func (s Store) Update(v model.ImageAsset, revision uint64) (model.ImageAsset, error) {
	categories, e := json.Marshal(v.CategoryIDs)
	if e != nil {
		return v, e
	}
	r := s.DB.Model(&model.ImageAsset{}).Where("id=? AND revision=?", v.ID, revision).Updates(map[string]any{"name": v.Name, "allow_as_cover": v.AllowAsCover, "allow_as_background": v.AllowAsBackground, "category_ids_json": string(categories), "enabled": v.Enabled, "revision": gorm.Expr("revision+1"), "updated_by": v.UpdatedBy, "updated_at": v.UpdatedAt})
	if r.Error != nil {
		return v, r.Error
	}
	if r.RowsAffected != 1 {
		return v, ErrConflict
	}
	return s.Get(v.ID)
}
func (s Store) List(sc model.Scope, f Filter) ([]model.ImageAsset, int64, error) {
	q := s.query()
	if !sc.All {
		if sc.OwnerID > 0 {
			q = q.Where("a.created_by=?", sc.OwnerID)
		} else if sc.TeamID > 0 {
			q = q.Where("a.team_id=?", sc.TeamID)
		} else {
			q = q.Where("1=0")
		}
		predicates := []string{"JSON_LENGTH(a.category_ids_json)=0"}
		args := []any{}
		for _, id := range sc.CategoryIDs {
			predicates = append(predicates, "JSON_CONTAINS(a.category_ids_json,JSON_QUOTE(?))")
			args = append(args, id)
		}
		q = q.Where("("+strings.Join(predicates, " OR ")+")", args...)
	}
	if f.Search != "" {
		q = q.Where("a.name LIKE ?", "%"+f.Search+"%")
	}
	if f.Enabled != nil {
		q = q.Where("a.enabled=?", *f.Enabled)
	}
	if f.Usage == "cover" {
		q = q.Where("a.allow_as_cover=1")
	}
	if f.Usage == "background" {
		q = q.Where("a.allow_as_background=1")
	}
	if f.CategoryMode == "general" {
		q = q.Where("JSON_LENGTH(a.category_ids_json)=0")
	} else if f.CategoryID != "" {
		predicate := "JSON_CONTAINS(a.category_ids_json,JSON_QUOTE(?))"
		if f.CategoryMode != "exact" {
			predicate = "(" + predicate + " OR JSON_LENGTH(a.category_ids_json)=0)"
		}
		q = q.Where(predicate, f.CategoryID)
	}
	width := "CAST(JSON_UNQUOTE(JSON_EXTRACT(f.metadata_json,'$.width')) AS UNSIGNED)"
	height := "CAST(JSON_UNQUOTE(JSON_EXTRACT(f.metadata_json,'$.height')) AS UNSIGNED)"
	if f.MinWidth > 0 {
		q = q.Where(width+">=?", f.MinWidth)
	}
	if f.MinHeight > 0 {
		q = q.Where(height+">=?", f.MinHeight)
	}
	if f.Source != "" {
		q = q.Where("JSON_UNQUOTE(JSON_EXTRACT(f.metadata_json, '$.source'))=?", f.Source)
	}
	switch f.Aspect {
	case "landscape":
		q = q.Where(width + ">" + height)
	case "portrait":
		q = q.Where(width + "<" + height)
	case "square":
		q = q.Where(width + "=" + height)
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		return nil, 0, e
	}
	out := []model.ImageAsset{}
	e := q.Select(projection).Order("a.created_at DESC,a.id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Scan(&out).Error
	return out, total, e
}
func FileReferenced(db *gorm.DB, id int64) (bool, error) {
	var n int64
	e := db.Model(&model.ImageAsset{}).Where("file_id=?", id).Count(&n).Error
	return n > 0, e
}

// Delete removes a library registration; the independently managed source file remains.
// The file lock serializes this check with every business reference writer.
func (s Store) Delete(id, revision uint64, check func(*gorm.DB, int64) error) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		v, e := (Store{DB: tx}).Get(id)
		if e != nil {
			return e
		}
		if e = files.LockAvailable(tx, []int64{v.FileID}); e != nil {
			return e
		}
		// The initial registration read may precede a reference writer's
		// commit. Read committed gives all subsequent reference checks a
		// current view once the shared file lock has been acquired.
		v, e = (Store{DB: tx}).Get(id)
		if e != nil {
			return e
		}
		if v.Revision != revision {
			return ErrConflict
		}
		if check == nil {
			return errors.New("image reference checker is not configured")
		}
		if e = check(tx, v.FileID); e != nil {
			return e
		}
		r := tx.Where("id=? AND revision=?", id, revision).Delete(&model.ImageAsset{})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
