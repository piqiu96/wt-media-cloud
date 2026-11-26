package repository

import (
	"errors"
	"github.com/go-sql-driver/mysql"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"gorm.io/gorm"
	"strings"
	"time"
)

var ErrNotFound = errors.New("work not found")
var ErrConflict = errors.New("public work identity conflicts")

type Filter struct {
	Actor                          identity.PublicUser
	Page, PageSize                 int
	Search, Platform               string
	CategoryID                     int64
	MediaAccountID                 uint64
	PublishedFrom, PublishedBefore *time.Time
}
type Summary struct {
	All   int64 `json:"all"`
	Today int64 `json:"today"`
}
type Store struct{ DB *gorm.DB }

// Insert once, then read under the same caller transaction. A duplicate cannot
// replace the original source, title or timestamps.
func (s Store) Record(v model.Work) (model.Work, error) {
	err := s.DB.Create(&v).Error
	if err == nil {
		return v, nil
	}
	var duplicate *mysql.MySQLError
	if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
		return model.Work{}, err
	}
	var prior model.Work
	if err = s.DB.Where("platform=? AND platform_work_id=?", v.Platform, v.PlatformWorkID).First(&prior).Error; err != nil {
		return prior, err
	}
	if prior.PlatformAccountUID != v.PlatformAccountUID || prior.MediaAccountID != v.MediaAccountID || prior.OperatorID != v.OperatorID || prior.TeamID != v.TeamID || prior.CategoryID != v.CategoryID || prior.CompositeOutputID != v.CompositeOutputID || prior.VideoSHA256 != v.VideoSHA256 || prior.WorkURL != v.WorkURL || !prior.PublishedAt.Equal(v.PublishedAt) {
		return model.Work{}, ErrConflict
	}
	return prior, nil
}
func (s Store) query(f Filter) *gorm.DB {
	q := s.DB.Model(&model.Work{})
	a := f.Actor
	if a.Role != identity.RoleAdmin {
		if a.Role == identity.RoleSeniorOperator && a.TeamID != nil {
			q = q.Where("team_id=?", *a.TeamID)
		} else {
			q = q.Where("operator_id=?", a.ID)
		}
		q = q.Where("category_id IN ?", a.CategoryIDs)
	}
	if f.Platform != "" {
		q = q.Where("platform=?", f.Platform)
	}
	if f.CategoryID > 0 {
		q = q.Where("category_id=?", f.CategoryID)
	}
	if f.MediaAccountID > 0 {
		q = q.Where("media_account_id=?", f.MediaAccountID)
	}
	if f.Search != "" {
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(f.Search)
		q = q.Where("CAST(id AS CHAR)=? OR title LIKE ? ESCAPE '!'", f.Search, "%"+escaped+"%")
	}
	if f.PublishedFrom != nil {
		q = q.Where("published_at>=?", *f.PublishedFrom)
	}
	if f.PublishedBefore != nil {
		q = q.Where("published_at<?", *f.PublishedBefore)
	}
	return q
}
func (s Store) Get(id uint64) (model.Work, error) {
	var v model.Work
	err := s.DB.Preload("Metrics").First(&v, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrNotFound
	}
	return v, err
}
func (s Store) List(f Filter) ([]model.Work, int64, error) {
	q := s.query(f)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	out := []model.Work{}
	err := q.Order("published_at DESC,id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Preload("Metrics").Find(&out).Error
	return out, total, err
}
func (s Store) Summary(f Filter, from, before time.Time) (Summary, error) {
	var v Summary
	q := s.query(f)
	err := q.Select("COUNT(*) AS all_count,COALESCE(SUM(published_at>=? AND published_at<?),0) AS today", from, before).Row().Scan(&v.All, &v.Today)
	return v, err
}
func FileReferenced(tx *gorm.DB, id int64) (bool, error) {
	var n int64
	err := tx.Model(&model.Work{}).Where("cover_file_id=?", id).Count(&n).Error
	return n > 0, err
}
