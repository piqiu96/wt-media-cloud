package service

import (
	"encoding/hex"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/repository"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid work input")
var ErrNotFound = repository.ErrNotFound
var ErrConflict = repository.ErrConflict

type Filter = repository.Filter
type Summary = repository.Summary
type Repository interface {
	Get(uint64) (model.Work, error)
	List(Filter) ([]model.Work, int64, error)
	Summary(Filter, time.Time, time.Time) (Summary, error)
}
type Service struct{ Repo Repository }

func Wired() *Service { return &Service{Repo: repository.Store{DB: database.DB()}} }
func CanRead(a identity.PublicUser, v model.Work) bool {
	t := identity.TeamID(v.TeamID)
	return a.ID > 0 && a.Status == identity.UserStatusEnabled && a.CanAccess(identity.UserID(v.OperatorID), &t, identity.CategoryID(v.CategoryID))
}
func (s *Service) Get(a identity.PublicUser, id uint64) (model.Work, error) {
	v, err := s.Repo.Get(id)
	if err == nil && !CanRead(a, v) {
		return model.Work{}, ErrNotFound
	}
	return v, err
}
func validate(a identity.PublicUser, f Filter) error {
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled || a.Role != identity.RoleAdmin && a.Role != identity.RoleSeniorOperator && a.Role != identity.RoleOperator || a.Role == identity.RoleSeniorOperator && a.TeamID == nil {
		return ErrNotFound
	}
	if f.Page < 1 || f.PageSize < 1 || f.PageSize > 100 || f.CategoryID < 0 || utf8.RuneCountInString(f.Search) > 100 || f.Platform != "" && f.Platform != "baidu" && f.Platform != "bilibili" && f.Platform != "xiaohongshu" || f.PublishedFrom != nil && f.PublishedBefore != nil && !f.PublishedFrom.Before(*f.PublishedBefore) {
		return ErrInvalid
	}
	return nil
}
func (s *Service) List(a identity.PublicUser, f Filter) ([]model.Work, int64, error) {
	if err := validate(a, f); err != nil {
		return nil, 0, err
	}
	f.Actor = a
	return s.Repo.List(f)
}
func (s *Service) Summary(a identity.PublicUser, f Filter, now time.Time) (Summary, error) {
	if err := validate(a, f); err != nil {
		return Summary{}, err
	}
	f.Actor = a
	zone := time.FixedZone("Asia/Shanghai", 8*3600)
	local := now.In(zone)
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone).UTC()
	return s.Repo.Summary(f, from, from.Add(24*time.Hour))
}

// RecordPublication is internal-only. The publishing service has already
// verified exact original-attempt evidence before entering this transaction.
func RecordPublication(tx *gorm.DB, v model.Work) (model.Work, error) {
	hash, err := hex.DecodeString(v.VideoSHA256)
	if v.Platform != "baidu" || strings.TrimSpace(v.PlatformWorkID) == "" || len(v.PlatformWorkID) > 255 || v.WorkURL == "" || len(v.WorkURL) > 2048 || v.PlatformAccountUID == "" || v.MediaAccountID == 0 || v.OperatorID <= 0 || v.TeamID <= 0 || v.CategoryID <= 0 || v.PublicationID == 0 || v.AttemptID == 0 || v.CompositeOutputID <= 0 || strings.TrimSpace(v.Title) == "" || utf8.RuneCountInString(v.Title) > 1000 || v.CoverFileID <= 0 || err != nil || len(hash) != 32 || v.PublishedAt.IsZero() || v.FirstObservedAt.IsZero() || v.PublishedAt.After(v.FirstObservedAt) {
		return model.Work{}, ErrInvalid
	}
	if err := files.LockAvailable(tx, []int64{v.CoverFileID}); err != nil {
		return model.Work{}, err
	}
	f, err := files.ReadSavedReference(tx, v.CoverFileID)
	if err != nil {
		return model.Work{}, err
	}
	v.CoverResourcePath = "/api/v1/file-resources/" + f.ResourceToken
	return (repository.Store{DB: tx}).Record(v)
}
func CheckFileReferences(tx *gorm.DB, id int64) error {
	used, err := repository.FileReferenced(tx, id)
	if err != nil {
		return err
	}
	if used {
		return files.ErrInUse
	}
	return nil
}
