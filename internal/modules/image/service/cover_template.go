package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"gorm.io/gorm"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type TemplateService struct {
	Repo    repository.TemplateStore
	Assets  *Service
	Artwork *ArtworkService
}

func WiredTemplate() *TemplateService {
	return &TemplateService{Repo: repository.TemplateStore{DB: database.DB()}, Assets: Wired(), Artwork: WiredArtwork()}
}

type CreateTemplateInput struct {
	RequestKey    string   `json:"request_key"`
	TeamID        int64    `json:"team_id,string"`
	Name          string   `json:"name"`
	CanvasWidth   int      `json:"canvas_width"`
	CanvasHeight  int      `json:"canvas_height"`
	CategoryIDs   []string `json:"category_ids"`
	PlatformCodes []string `json:"platform_codes"`
}
type DraftInput struct {
	ExpectedRevision uint64          `json:"expected_revision,string"`
	Document         render.Document `json:"document"`
}
type TemplateCommand struct {
	ExpectedRevision       uint64 `json:"expected_revision,string"`
	BackgroundImageAssetID uint64 `json:"background_image_asset_id,string"`
}

func canReadTemplate(a identity.PublicUser, v model.CoverTemplate) bool {
	return canReadImageAsset(a, model.ImageAsset{TeamID: v.TeamID, CreatedBy: v.CreatedBy, CategoryIDs: v.CategoryIDs})
}
func (s *TemplateService) decorate(v model.CoverTemplate) (model.CoverTemplate, error) {
	if v.PublishedThumbnailFileID != nil {
		f, e := files.ReadSavedReference(s.Repo.DB, *v.PublishedThumbnailFileID)
		if e != nil {
			return v, e
		}
		v.ThumbnailResourcePath = files.ResourcePath(f.ResourceToken)
	}
	return v, nil
}
func (s *TemplateService) GetCoverTemplate(a identity.PublicUser, id uint64) (model.CoverTemplate, error) {
	if _, e := imageAssetScope(a); e != nil {
		return model.CoverTemplate{}, e
	}
	v, e := s.Repo.Get(id)
	if e != nil {
		return v, e
	}
	if !canReadTemplate(a, v) {
		return model.CoverTemplate{}, ErrNotFound
	}
	return s.decorate(v)
}
func (s *TemplateService) ListCoverTemplate(a identity.PublicUser, f repository.TemplateFilter) ([]model.CoverTemplate, int64, error) {
	sc, e := imageAssetScope(a)
	if e != nil {
		return nil, 0, e
	}
	if e = validateFilter(repository.Filter{Search: f.Search, Page: f.Page, PageSize: f.PageSize}); e != nil {
		return nil, 0, e
	}
	if f.Scope != "" && f.Scope != "general" {
		return nil, 0, ErrInvalid
	}
	if f.ContentStatus != "" && f.ContentStatus != "draft" && f.ContentStatus != "published" || f.Width < 0 || f.Height < 0 {
		return nil, 0, ErrInvalid
	}
	if f.CategoryID != "" && !canUseCategory(a, f.CategoryID) {
		return nil, 0, ErrInvalid
	}
	rows, total, e := s.Repo.List(sc, f)
	if e != nil {
		return nil, 0, e
	}
	for i := range rows {
		rows[i], e = s.decorate(rows[i])
		if e != nil {
			return nil, 0, e
		}
	}
	return rows, total, nil
}
func templateHash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (s *TemplateService) create(ctx context.Context, a identity.PublicUser, in CreateTemplateInput, document *render.Document, source uint64) (model.CoverTemplate, error) {
	if _, e := imageAssetScope(a); e != nil {
		return model.CoverTemplate{}, e
	}
	name, categories, e := s.Assets.validateImageAssetAttributes(a, in.Name, true, false, in.CategoryIDs)
	if e != nil {
		return model.CoverTemplate{}, e
	}
	if utf8.RuneCountInString(name) > 128 {
		return model.CoverTemplate{}, ErrInvalid
	}
	if len(in.RequestKey) < 8 || len(in.RequestKey) > 64 || strings.TrimSpace(in.RequestKey) != in.RequestKey || in.PlatformCodes == nil || len(in.PlatformCodes) > 3 {
		return model.CoverTemplate{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, p := range in.PlatformCodes {
		if p != "baidu" && p != "bilibili" && p != "xiaohongshu" || seen[p] {
			return model.CoverTemplate{}, ErrInvalid
		}
		seen[p] = true
	}
	sort.Strings(in.PlatformCodes)
	team := int64(0)
	if a.TeamID != nil {
		team = int64(*a.TeamID)
	}
	if a.Role == identity.RoleAdmin {
		team = in.TeamID
		allowed := false
		if s.Assets.TeamAllowed != nil {
			allowed, e = s.Assets.TeamAllowed(a, team)
		}
		if e != nil {
			return model.CoverTemplate{}, e
		}
		if !allowed {
			return model.CoverTemplate{}, ErrInvalid
		}
	} else if in.TeamID != 0 && in.TeamID != team {
		return model.CoverTemplate{}, ErrInvalid
	}
	if team <= 0 {
		return model.CoverTemplate{}, ErrInvalid
	}
	supported := (in.CanvasWidth == 1080 && in.CanvasHeight == 1440) || (in.CanvasWidth == 1200 && in.CanvasHeight == 900) || (in.CanvasWidth == 1920 && in.CanvasHeight == 1080) || (in.CanvasWidth == 1080 && in.CanvasHeight == 1920) || (in.CanvasWidth == 1080 && in.CanvasHeight == 1080)
	if !supported {
		return model.CoverTemplate{}, ErrInvalid
	}
	in.Name, in.CategoryIDs, in.TeamID = name, categories, team
	hash := templateHash(struct {
		Input  CreateTemplateInput
		Source uint64
	}{in, source})

	doc := render.DefaultDocument(in.CanvasWidth, in.CanvasHeight)
	if document != nil {
		doc = *document
	}
	if e = render.ValidateDraft(doc); e != nil {
		return model.CoverTemplate{}, fmt.Errorf("%w: %s", ErrInvalid, e)
	}
	bindings, e := s.bindDocument(a, doc)
	if e != nil {
		return model.CoverTemplate{}, e
	}
	data, _ := json.Marshal(doc)
	now := time.Now().UTC()
	scope := "general"
	if len(categories) > 0 {
		scope = "category"
	}
	v := model.CoverTemplate{TeamID: team, Name: name, CanvasWidth: in.CanvasWidth, CanvasHeight: in.CanvasHeight, Scope: scope, CategoryIDs: categories, PlatformCodes: in.PlatformCodes, DraftConfig: data, Revision: 1, RequestKey: in.RequestKey, RequestHash: hash, CreatedBy: int64(a.ID), UpdatedBy: int64(a.ID), CreatedAt: now, UpdatedAt: now}
	v, e = s.Repo.Create(v, bindings)
	if errors.Is(e, ErrConflict) {
		old, getErr := s.Repo.ByRequest(int64(a.ID), in.RequestKey)
		if getErr == nil && old.RequestHash == hash {
			return s.decorate(old)
		}
	}
	return v, e
}
func (s *TemplateService) CreateCoverTemplate(ctx context.Context, a identity.PublicUser, in CreateTemplateInput) (model.CoverTemplate, error) {
	if v, found, e := s.replayCoverTemplate(a, in, 0); found || e != nil {
		return v, e
	}
	return s.create(ctx, a, in, nil, 0)
}
func (s *TemplateService) bindDocument(a identity.PublicUser, d render.Document) ([]repository.AssetBinding, error) {
	if d.BackgroundMode != "fixed" {
		if d.ImageAssetID != 0 || d.Image != "" {
			return nil, ErrInvalid
		}
		return nil, nil
	}
	if d.ImageAssetID == 0 {
		return nil, nil
	} // unfinished fixed-background draft is allowed
	asset, e := s.Assets.GetImageAsset(a, d.ImageAssetID)
	if e != nil {
		return nil, e
	}
	if !asset.AllowAsBackground {
		return nil, ErrInvalid
	}
	if d.Image != "" && d.Image != asset.ResourcePath {
		return nil, ErrInvalid
	}
	return []repository.AssetBinding{{ID: asset.ID, FileID: asset.FileID}}, nil
}
func (s *TemplateService) SaveCoverTemplateDraft(a identity.PublicUser, id uint64, in DraftInput) (model.CoverTemplate, error) {
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		return v, e
	}
	if in.ExpectedRevision == 0 || v.Revision != in.ExpectedRevision {
		return v, ErrConflict
	}
	if in.Document.Width != v.CanvasWidth || in.Document.Height != v.CanvasHeight {
		return v, ErrInvalid
	}
	if e = render.ValidateDraft(in.Document); e != nil {
		return v, fmt.Errorf("%w: %s", ErrInvalid, e)
	}
	bindings, e := s.bindDocument(a, in.Document)
	if e != nil {
		return v, e
	}
	// A disabled image may remain in an existing draft, but cannot be newly selected.
	oldID := uint64(0)
	var old render.Document
	if json.Unmarshal(v.DraftConfig, &old) == nil {
		oldID = old.ImageAssetID
	} else if json.Unmarshal(v.PublishedConfig, &old) == nil {
		oldID = old.ImageAssetID
	}
	if in.Document.ImageAssetID > 0 && in.Document.ImageAssetID != oldID {
		asset, e := s.Assets.GetImageAsset(a, in.Document.ImageAssetID)
		if e != nil {
			return v, e
		}
		if !asset.Enabled {
			return v, ErrInvalid
		}
	}
	v.DraftConfig, _ = json.Marshal(in.Document)
	if repository.SameJSON(v.DraftConfig, v.PublishedConfig) {
		v.DraftConfig = nil
	}
	v.UpdatedBy = int64(a.ID)
	v.UpdatedAt = time.Now().UTC()
	v, e = s.Repo.SaveDraft(v, in.ExpectedRevision, bindings)
	if e != nil {
		return v, e
	}
	return s.decorate(v)
}
func (s *TemplateService) SetCoverTemplateUsage(a identity.PublicUser, id uint64, in UsageInput) (model.CoverTemplate, error) {
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		return v, e
	}
	if in.Enabled == nil || in.ExpectedRevision == 0 {
		return v, ErrInvalid
	}
	if v.Revision != in.ExpectedRevision || v.PublishedRevision == 0 {
		return v, ErrConflict
	}
	v, e = s.Repo.SetUsage(id, in.ExpectedRevision, int64(a.ID), *in.Enabled)
	if e != nil {
		return v, e
	}
	return s.decorate(v)
}
func (s *TemplateService) DeleteCoverTemplate(a identity.PublicUser, id, revision uint64) error {
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		return e
	}
	if revision == 0 {
		return ErrInvalid
	}
	if v.PublishedRevision > 0 {
		return ErrConflict
	}
	return s.Repo.Delete(id, revision)
}
func (s *TemplateService) CopyCoverTemplate(ctx context.Context, a identity.PublicUser, id uint64, in CreateTemplateInput) (model.CoverTemplate, error) {
	if v, found, e := s.replayCoverTemplate(a, in, id); found || e != nil {
		return v, e
	}
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		return v, e
	}
	data := v.DraftConfig
	if len(data) == 0 {
		data = v.PublishedConfig
	}
	var d render.Document
	if json.Unmarshal(data, &d) != nil {
		return v, ErrInvalid
	}
	sx, sy := float64(in.CanvasWidth)/float64(d.Width), float64(in.CanvasHeight)/float64(d.Height)
	uniform := math.Min(sx, sy)
	d.Width, d.Height = in.CanvasWidth, in.CanvasHeight
	d.Safe = math.Round(d.Safe * uniform)
	for i := range d.Layers {
		l := &d.Layers[i]
		l.X = math.Round(l.X * sx)
		l.Y = math.Round(l.Y * sy)
		l.W = math.Round(l.W * sx)
		l.H = math.Round(l.H * sy)
		if l.Kind == "text" {
			l.FontSize = math.Max(8, math.Min(600, math.Round(l.FontSize*uniform)))
			l.MinFont = math.Max(8, math.Min(l.FontSize, math.Round(l.MinFont*uniform)))
			l.Stroke = math.Min(20, l.Stroke*uniform)
			l.Spacing = math.Max(-100, math.Min(100, l.Spacing*uniform))
		}
	}
	return s.create(ctx, a, in, &d, id)
}
func (s *TemplateService) PublishCoverTemplate(ctx context.Context, a identity.PublicUser, id uint64, in TemplateCommand) (model.CoverTemplate, []render.Issue, error) {
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		return v, nil, e
	}
	if v.Revision != in.ExpectedRevision || len(v.DraftConfig) == 0 {
		return v, nil, ErrConflict
	}
	var d render.Document
	if json.Unmarshal(v.DraftConfig, &d) != nil {
		return v, nil, ErrInvalid
	}
	_, issues, e := s.Artwork.RenderDocument(ctx, a, d, render.ShortSample, in.BackgroundImageAssetID, true, ArtworkInput{TemplateID: id, PublishedRevision: v.PublishedRevision + 1})
	if e != nil {
		return v, issues, e
	}
	artwork, longIssues, e := s.Artwork.RenderDocument(ctx, a, d, render.LongSample, in.BackgroundImageAssetID, true, ArtworkInput{TemplateID: id, PublishedRevision: v.PublishedRevision + 1})
	if e != nil {
		return v, issues, e
	}
	issues = append(issues, longIssues...)
	bindings, e := s.bindDocument(a, d)
	if e != nil {
		return v, issues, e
	}
	v.PublishedConfig = v.DraftConfig
	v.PublishedThumbnailFileID = &artwork.File.ID
	v.UpdatedBy = int64(a.ID)
	v.UpdatedAt = time.Now().UTC()
	v, e = s.Repo.Publish(v, in.ExpectedRevision, bindings)
	if e != nil {
		return v, issues, e
	}
	v, e = s.decorate(v)
	return v, issues, e
}
func CheckTemplateFileReferences(tx *gorm.DB, id int64) error {
	used, e := repository.TemplateFileReferenced(tx, id)
	if e != nil {
		return e
	}
	if used {
		return files.ErrInUse
	}
	return nil
}
func ListTemplateFileReference(a identity.PublicUser, id int64) ([]model.Reference, error) {
	sc, e := imageAssetScope(a)
	if e != nil {
		return nil, e
	}
	return repository.ListTemplateFileReference(database.DB(), sc, id)
}

// Recover an already saved result before consulting mutable dependencies.
// New requests still pass every current category, team and source check.
func (s *TemplateService) replayCoverTemplate(a identity.PublicUser, in CreateTemplateInput, source uint64) (model.CoverTemplate, bool, error) {
	if _, e := imageAssetScope(a); e != nil {
		return model.CoverTemplate{}, false, e
	}
	if len(in.RequestKey) < 8 || len(in.RequestKey) > 64 || strings.TrimSpace(in.RequestKey) != in.RequestKey || in.CategoryIDs == nil || in.PlatformCodes == nil {
		return model.CoverTemplate{}, false, ErrInvalid
	}
	in.Name = strings.TrimSpace(in.Name)
	in.CategoryIDs = append([]string{}, in.CategoryIDs...)
	for i, id := range in.CategoryIDs {
		n, e := strconv.ParseInt(id, 10, 64)
		if e != nil || n <= 0 {
			return model.CoverTemplate{}, false, ErrInvalid
		}
		in.CategoryIDs[i] = strconv.FormatInt(n, 10)
	}
	sort.Strings(in.CategoryIDs)
	in.PlatformCodes = append([]string{}, in.PlatformCodes...)
	sort.Strings(in.PlatformCodes)
	if a.Role != identity.RoleAdmin {
		if a.TeamID == nil {
			return model.CoverTemplate{}, false, ErrInvalid
		}
		team := int64(*a.TeamID)
		if in.TeamID != 0 && in.TeamID != team {
			return model.CoverTemplate{}, false, ErrInvalid
		}
		in.TeamID = team
	}
	v, e := s.Repo.ByRequest(int64(a.ID), in.RequestKey)
	if errors.Is(e, ErrNotFound) {
		return model.CoverTemplate{}, false, nil
	}
	if e != nil {
		return model.CoverTemplate{}, false, e
	}
	if !canReadTemplate(a, v) {
		return model.CoverTemplate{}, true, ErrNotFound
	}
	hash := templateHash(struct {
		Input  CreateTemplateInput
		Source uint64
	}{in, source})
	if v.RequestHash != hash {
		return model.CoverTemplate{}, true, ErrConflict
	}
	v, e = s.decorate(v)
	return v, true, e
}
