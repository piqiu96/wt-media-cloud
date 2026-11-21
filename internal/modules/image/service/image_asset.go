package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"gorm.io/gorm"
)

var ErrInvalid = errors.New("image asset invalid")
var ErrNotFound = repository.ErrNotFound
var ErrConflict = repository.ErrConflict

type Repository interface {
	ByFile(int64) (model.ImageAsset, error)
	Register(model.ImageAsset) (model.ImageAsset, error)
	Get(uint64) (model.ImageAsset, error)
	Update(model.ImageAsset, uint64) (model.ImageAsset, error)
	List(model.Scope, repository.Filter) ([]model.ImageAsset, int64, error)
	Delete(uint64, uint64, func(*gorm.DB, int64) error) error
}
type Service struct {
	Repo            Repository
	ReadFile        func(context.Context, identity.PublicUser, int64) (filemodel.File, error)
	CategoryAllowed func(identity.PublicUser, int64) (bool, error)
	TeamAllowed     func(identity.PublicUser, int64) (bool, error)
	ReferenceCheck  func(*gorm.DB, int64) error
	ListReference   func(identity.PublicUser, int64) ([]model.Reference, error)
}
type RegisterInput struct {
	FileID            int64    `json:"file_id,string"`
	TeamID            int64    `json:"team_id,string"`
	Name              string   `json:"name"`
	AllowAsCover      bool     `json:"allow_as_cover"`
	AllowAsBackground bool     `json:"allow_as_background"`
	CategoryIDs       []string `json:"category_ids"`
}
type UpdateInput struct {
	ExpectedRevision  uint64   `json:"expected_revision,string"`
	Name              string   `json:"name"`
	AllowAsCover      bool     `json:"allow_as_cover"`
	AllowAsBackground bool     `json:"allow_as_background"`
	CategoryIDs       []string `json:"category_ids"`
}
type UsageInput struct {
	ExpectedRevision uint64 `json:"expected_revision,string"`
	Enabled          *bool  `json:"enabled"`
}
type SelectInput struct {
	Usage      string `json:"usage"`
	CategoryID string `json:"category_id"`
	MinWidth   int    `json:"min_width"`
	MinHeight  int    `json:"min_height"`
	Aspect     string `json:"aspect"`
}

func Wired() *Service {
	return &Service{Repo: repository.Store{DB: database.DB()}, ReferenceCheck: referenceCheck, ListReference: listReference, ReadFile: func(ctx context.Context, a identity.PublicUser, id int64) (filemodel.File, error) {
		s, e := files.Wired()
		if e != nil {
			return filemodel.File{}, e
		}
		f, e := s.Read(a, id)
		if e != nil {
			return f, e
		}
		return s.ResolveToken(ctx, f.ResourceToken)
	}, CategoryAllowed: func(a identity.PublicUser, id int64) (bool, error) {
		c, ok, e := identity.ResolveCategory(identity.CategoryID(id))
		return ok && c.Enabled, e
	}, TeamAllowed: func(a identity.PublicUser, id int64) (bool, error) {
		teams, e := identity.ListTeams(a.ID, "")
		if e != nil {
			return false, e
		}
		for _, team := range teams {
			if int64(team.ID) == id {
				return true, nil
			}
		}
		return false, nil
	}}
}
func imageAssetScope(a identity.PublicUser) (model.Scope, error) {
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled {
		return model.Scope{}, ErrNotFound
	}
	sc := model.Scope{CategoryIDs: []string{}}
	for _, id := range a.CategoryIDs {
		sc.CategoryIDs = append(sc.CategoryIDs, strconv.FormatUint(uint64(id), 10))
	}
	switch a.Role {
	case identity.RoleAdmin:
		sc.All = true
	case identity.RoleOperator:
		sc.OwnerID = int64(a.ID)
	case identity.RoleSeniorOperator:
		if a.TeamID == nil {
			return sc, ErrNotFound
		}
		sc.TeamID = int64(*a.TeamID)
	default:
		return sc, ErrNotFound
	}
	return sc, nil
}
func canReadImageAsset(a identity.PublicUser, v model.ImageAsset) bool {
	team := identity.TeamID(v.TeamID)
	if a.ID <= 0 || !a.CanAccessOwnedResource(identity.UserID(v.CreatedBy), &team) {
		return false
	}
	if a.Role == identity.RoleAdmin || len(v.CategoryIDs) == 0 {
		return true
	}
	for _, id := range v.CategoryIDs {
		for _, allowed := range a.CategoryIDs {
			if id == strconv.FormatUint(uint64(allowed), 10) {
				return true
			}
		}
	}
	return false
}
func canUseCategory(a identity.PublicUser, id string) bool {
	if a.Role == identity.RoleAdmin {
		return true
	}
	for _, allowed := range a.CategoryIDs {
		if id == strconv.FormatUint(uint64(allowed), 10) {
			return true
		}
	}
	return false
}
func (s *Service) validateImageAssetAttributes(a identity.PublicUser, name string, cover, background bool, categories []string) (string, []string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 160 || !cover && !background || categories == nil || len(categories) > 100 {
		return "", nil, ErrInvalid
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", nil, ErrInvalid
		}
	}
	ids := append([]string{}, categories...)
	seen := map[int64]bool{}
	for i, id := range ids {
		n, e := strconv.ParseInt(id, 10, 64)
		if e != nil || n <= 0 || seen[n] || !canUseCategory(a, strconv.FormatInt(n, 10)) {
			return "", nil, ErrInvalid
		}
		allowed := false
		if s.CategoryAllowed != nil {
			allowed, e = s.CategoryAllowed(a, n)
			if e != nil {
				return "", nil, e
			}
		}
		if !allowed {
			return "", nil, ErrInvalid
		}
		seen[n] = true
		ids[i] = strconv.FormatInt(n, 10)
	}
	sort.Strings(ids)
	return name, ids, nil
}
func withImageMetadata(v model.ImageAsset) (model.ImageAsset, error) {
	var metadata filemodel.ImageMetadata
	if e := json.Unmarshal(v.MetadataJSON, &metadata); e != nil || metadata.Width <= 0 || metadata.Height <= 0 {
		return v, ErrInvalid
	}
	if metadata.Preview {
		return v, ErrInvalid
	}
	v.Width, v.Height, v.Format = metadata.Width, metadata.Height, metadata.Format
	if v.ResourceToken != "" {
		v.ResourcePath = files.ResourcePath(v.ResourceToken)
	}
	return v, nil
}
func (s *Service) RegisterImageAsset(ctx context.Context, a identity.PublicUser, in RegisterInput) (model.ImageAsset, error) {
	if _, e := imageAssetScope(a); e != nil {
		return model.ImageAsset{}, e
	}
	name, categories, e := s.validateImageAssetAttributes(a, in.Name, in.AllowAsCover, in.AllowAsBackground, in.CategoryIDs)
	if e != nil {
		return model.ImageAsset{}, e
	}
	if in.FileID <= 0 {
		return model.ImageAsset{}, ErrInvalid
	}
	f, e := s.ReadFile(ctx, a, in.FileID)
	if e != nil {
		return model.ImageAsset{}, e
	}
	var team *identity.TeamID
	if f.TeamID != nil {
		t := identity.TeamID(*f.TeamID)
		team = &t
	}
	if !a.CanAccessOwnedResource(identity.UserID(f.OwnerID), team) {
		return model.ImageAsset{}, ErrNotFound
	}
	if f.ID != in.FileID || f.Type != "image" || f.UploadedAt == nil || f.DeletedAt != nil || f.SizeBytes <= 0 {
		return model.ImageAsset{}, ErrInvalid
	}
	teamID := int64(0)
	if f.TeamID != nil {
		teamID = *f.TeamID
	}
	teamAllowed := false
	if teamID <= 0 && a.Role == identity.RoleAdmin && s.TeamAllowed != nil {
		teamAllowed, e = s.TeamAllowed(a, in.TeamID)
		if e != nil {
			return model.ImageAsset{}, e
		}
	}
	if teamID <= 0 && teamAllowed {
		teamID = in.TeamID
	}
	if teamID <= 0 || in.TeamID != 0 && in.TeamID != teamID {
		return model.ImageAsset{}, ErrInvalid
	}
	now := time.Now().UTC()
	v := model.ImageAsset{FileID: f.ID, TeamID: teamID, Name: name, AllowAsCover: in.AllowAsCover, AllowAsBackground: in.AllowAsBackground, CategoryIDs: categories, Enabled: true, Revision: 1, CreatedBy: int64(a.ID), UpdatedBy: int64(a.ID), CreatedAt: now, UpdatedAt: now, MetadataJSON: f.MetadataJSON, ResourceToken: f.ResourceToken, SizeBytes: f.SizeBytes}
	if v, e = withImageMetadata(v); e != nil {
		return v, e
	}
	existing, e := s.Repo.ByFile(f.ID)
	if e == nil {
		return registrationReplay(a, existing, v)
	}
	if !errors.Is(e, ErrNotFound) {
		return v, e
	}
	saved, e := s.Repo.Register(v)
	if errors.Is(e, ErrConflict) {
		existing, getErr := s.Repo.ByFile(f.ID)
		if getErr != nil {
			return v, getErr
		}
		return registrationReplay(a, existing, v)
	}
	if e != nil {
		return v, e
	}
	return withImageMetadata(saved)
}
func registrationReplay(a identity.PublicUser, existing, want model.ImageAsset) (model.ImageAsset, error) {
	if !canReadImageAsset(a, existing) {
		return model.ImageAsset{}, ErrNotFound
	}
	if existing.TeamID != want.TeamID || existing.Name != want.Name || existing.AllowAsCover != want.AllowAsCover || existing.AllowAsBackground != want.AllowAsBackground || !reflect.DeepEqual(existing.CategoryIDs, want.CategoryIDs) {
		return model.ImageAsset{}, ErrConflict
	}
	return withImageMetadata(existing)
}
func (s *Service) GetImageAsset(a identity.PublicUser, id uint64) (model.ImageAsset, error) {
	v, e := s.Repo.Get(id)
	if e != nil {
		return v, e
	}
	if !canReadImageAsset(a, v) {
		return model.ImageAsset{}, ErrNotFound
	}
	return withImageMetadata(v)
}
func (s *Service) UpdateImageAsset(a identity.PublicUser, id uint64, in UpdateInput) (model.ImageAsset, error) {
	v, e := s.GetImageAsset(a, id)
	if e != nil {
		return v, e
	}
	if in.ExpectedRevision == 0 || v.Revision != in.ExpectedRevision {
		return v, ErrConflict
	}
	name, categories, e := s.validateImageAssetAttributes(a, in.Name, in.AllowAsCover, in.AllowAsBackground, in.CategoryIDs)
	if e != nil {
		return v, e
	}
	v.Name, v.CategoryIDs, v.AllowAsCover, v.AllowAsBackground = name, categories, in.AllowAsCover, in.AllowAsBackground
	v.UpdatedBy = int64(a.ID)
	v.UpdatedAt = time.Now().UTC()
	v, e = s.Repo.Update(v, in.ExpectedRevision)
	if e != nil {
		return v, e
	}
	return withImageMetadata(v)
}
func (s *Service) SetImageAssetUsage(a identity.PublicUser, id uint64, in UsageInput) (model.ImageAsset, error) {
	v, e := s.GetImageAsset(a, id)
	if e != nil {
		return v, e
	}
	if in.ExpectedRevision == 0 || v.Revision != in.ExpectedRevision {
		return v, ErrConflict
	}
	if in.Enabled == nil {
		return v, ErrInvalid
	}
	v.Enabled = *in.Enabled
	v.UpdatedBy = int64(a.ID)
	v.UpdatedAt = time.Now().UTC()
	v, e = s.Repo.Update(v, in.ExpectedRevision)
	if e != nil {
		return v, e
	}
	return withImageMetadata(v)
}
func validateFilter(f repository.Filter) error {
	if f.Source != "" && f.Source != "upload" && f.Source != "render" {
		return ErrInvalid
	}
	if f.Page < 1 || f.PageSize < 1 || f.PageSize > 100 || f.Page > int(^uint(0)>>1)/f.PageSize || utf8.RuneCountInString(f.Search) > 100 || f.MinWidth < 0 || f.MinHeight < 0 {
		return ErrInvalid
	}
	if f.Usage != "" && f.Usage != "cover" && f.Usage != "background" {
		return ErrInvalid
	}
	if f.Aspect != "" && f.Aspect != "landscape" && f.Aspect != "portrait" && f.Aspect != "square" {
		return ErrInvalid
	}
	return nil
}
func (s *Service) ListImageAsset(a identity.PublicUser, f repository.Filter) ([]model.ImageAsset, int64, error) {
	sc, e := imageAssetScope(a)
	if e != nil {
		return nil, 0, e
	}
	if e = validateFilter(f); e != nil {
		return nil, 0, e
	}
	if f.CategoryMode != "" {
		return nil, 0, ErrInvalid
	}
	if f.CategoryID != "" {
		id, e := strconv.ParseInt(f.CategoryID, 10, 64)
		if e != nil || id <= 0 || strconv.FormatInt(id, 10) != f.CategoryID || !canUseCategory(a, f.CategoryID) {
			return nil, 0, ErrInvalid
		}
	}
	rows, total, e := s.Repo.List(sc, f)
	if e != nil {
		return nil, 0, e
	}
	for i := range rows {
		rows[i], e = withImageMetadata(rows[i])
		if e != nil {
			return nil, 0, e
		}
	}
	return rows, total, nil
}
func (s *Service) SelectImageAsset(a identity.PublicUser, in SelectInput) (*model.ImageAsset, error) {
	sc, e := imageAssetScope(a)
	if e != nil {
		return nil, e
	}
	id, e := strconv.ParseInt(in.CategoryID, 10, 64)
	if e != nil || id <= 0 || strconv.FormatInt(id, 10) != in.CategoryID || !canUseCategory(a, in.CategoryID) || in.Usage == "" {
		return nil, ErrInvalid
	}
	allowed := false
	if s.CategoryAllowed != nil {
		allowed, e = s.CategoryAllowed(a, id)
		if e != nil {
			return nil, e
		}
	}
	if !allowed {
		return nil, ErrInvalid
	}
	enabled := true
	f := repository.Filter{Page: 1, PageSize: 1, CategoryID: in.CategoryID, CategoryMode: "exact", Usage: in.Usage, Enabled: &enabled, MinWidth: in.MinWidth, MinHeight: in.MinHeight, Aspect: in.Aspect}
	if e = validateFilter(f); e != nil {
		return nil, e
	}
	for _, mode := range []string{"exact", "general"} {
		f.CategoryMode = mode
		f.Page = 1
		rows, total, e := s.Repo.List(sc, f)
		if e != nil {
			return nil, e
		}
		if total == 0 {
			continue
		}
		if total > 1 {
			index, e := rand.Int(rand.Reader, big.NewInt(total))
			if e != nil {
				return nil, e
			}
			f.Page = int(index.Int64()) + 1
			rows, _, e = s.Repo.List(sc, f)
			if e != nil {
				return nil, e
			}
		}
		if len(rows) == 0 {
			return nil, nil
		}
		v, e := withImageMetadata(rows[0])
		if e != nil {
			return nil, e
		}
		return &v, nil
	}
	return nil, nil
}
func CheckFileReferences(tx *gorm.DB, id int64) error {
	used, e := repository.FileReferenced(tx, id)
	if e != nil {
		return e
	}
	if used {
		return files.ErrInUse
	}
	if e := CheckTemplateFileReferences(tx, id); e != nil {
		return e
	}
	return CheckArtworkFileReferences(tx, id)
}
