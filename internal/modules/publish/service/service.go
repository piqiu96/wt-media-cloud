package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/deepseek"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("publication input invalid")
	ErrNotFound = repository.ErrNotFound
	ErrConflict = repository.ErrConflict
	ErrOccupied = repository.ErrOccupied
	ErrClosed   = errors.New("publication platform not available")
)

type Filter = repository.Filter
type ConflictError = repository.ConflictError
type Repository interface {
	ByRequest(int64, string) (model.Task, error)
	Create(model.Task) (model.Task, error)
	Get(uint64) (model.Task, error)
	List(Filter) ([]model.Task, int64, error)
	Summary(Filter) (map[string]int64, error)
	Edit(model.Task, uint64) (model.Task, error)
}
type Service struct {
	BatchExecutionRepo BatchExecutionRepository
	BatchRepo          BatchRepository
	BatchCategory      func(identity.CategoryID) (identity.Category, bool, error)
	BatchAccounts      batchAccountPage
	BatchLimits        config.PublicationBatchConfig

	GenerateJSON func(context.Context, string, string) (string, error)
	OutputPage   func(context.Context, identity.PublicUser, production.ListComposeOutputsInput) (OutputPage, error)
	SavedFile    func(context.Context, int64) (filemodel.File, error)
	Repo         Repository
	Account      func(identity.PublicUser, string) (accounts.Account, error)
	Output       func(identity.PublicUser, int64) (production.ComposeOutputDetail, error)
	File         func(context.Context, identity.PublicUser, int64, int64) (filemodel.File, error)
}

func Wired() *Service {
	store := repository.Store{DB: database.DB(), LockFiles: files.LockAvailable}
	return &Service{BatchRepo: store, BatchExecutionRepo: ExecutionStore(), BatchCategory: identity.ResolveCategory, BatchAccounts: accounts.ListAccountPage,
		BatchLimits: config.Get().App.PublicationBatch, GenerateJSON: func(ctx context.Context, system, user string) (string, error) {
			return deepseek.Get().CompleteJSON(ctx, system, user)
		}, OutputPage: func(ctx context.Context, a identity.PublicUser, in production.ListComposeOutputsInput) (OutputPage, error) {
			page, err := production.ListComposeOutputPage(ctx, a, in)
			return OutputPage{Items: page.Items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, err
		}, SavedFile: func(ctx context.Context, id int64) (filemodel.File, error) {
			s, e := files.Wired()
			if e != nil {
				return filemodel.File{}, e
			}
			return s.ReadSavedReference(ctx, id)
		}, Repo: store, Account: accounts.GetAccount, Output: production.GetComposeOutput, File: readCoverFile,
	}
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

type CreateInput struct {
	RequestKey        string        `json:"request_key"`
	Platform          string        `json:"platform"`
	CategoryID        int64         `json:"category_id,string"`
	MediaAccountID    uint64        `json:"media_account_id,string"`
	CompositeOutputID int64         `json:"composite_output_id,string"`
	Content           model.Content `json:"content"`
	NotBefore         *time.Time    `json:"not_before"`
}
type EditInput struct {
	ExpectedRevision uint64        `json:"expected_revision,string"`
	Content          model.Content `json:"content"`
	NotBefore        *time.Time    `json:"not_before"`
}

func CanRead(a identity.PublicUser, v model.Task) bool {
	t := identity.TeamID(v.TeamID)
	return a.ID > 0 && a.CanAccess(identity.UserID(v.OperatorID), &t, identity.CategoryID(v.CategoryID))
}
func canExecutePublication(a identity.PublicUser) bool {
	return a.ID > 0 && a.Status == identity.UserStatusEnabled && (a.Role == identity.RoleOperator || a.Role == identity.RoleSeniorOperator)
}
func ownsPublication(a identity.PublicUser, v model.Task) bool {
	return canExecutePublication(a) && int64(a.ID) == v.OperatorID
}
func (s *Service) Get(a identity.PublicUser, id uint64) (model.Task, error) {
	v, e := s.Repo.Get(id)
	if e != nil {
		return v, e
	}
	if !CanRead(a, v) {
		return model.Task{}, ErrNotFound
	}
	rows := []model.Task{v}
	if e = s.decorateTaskParents(a, rows); e != nil {
		return model.Task{}, e
	}
	return rows[0], nil
}
func (s *Service) FindByRequest(a identity.PublicUser, key string) (model.Task, error) {
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled {
		return model.Task{}, ErrNotFound
	}
	if len(key) < 1 || len(key) > 64 || strings.TrimSpace(key) != key {
		return model.Task{}, ErrInvalid
	}
	task, err := s.Repo.ByRequest(int64(a.ID), key)
	if err == nil && !CanRead(a, task) {
		return model.Task{}, ErrNotFound
	}
	return task, err
}

func validateFilter(a identity.PublicUser, f Filter) error {
	if a.Role == identity.RoleSeniorOperator && a.TeamID == nil {
		return ErrNotFound
	}
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled || a.Role != identity.RoleAdmin && a.Role != identity.RoleSeniorOperator && a.Role != identity.RoleOperator {
		return ErrNotFound
	}
	if f.CategoryID < 0 || f.Platform != "" && f.Platform != "baidu" && f.Platform != "bilibili" && f.Platform != "xiaohongshu" || f.CreatedFrom != nil && f.CreatedBefore != nil && !f.CreatedFrom.Before(*f.CreatedBefore) || f.Page < 1 || f.PageSize < 1 || f.PageSize > 100 || utf8.RuneCountInString(f.Search) > 100 {
		return ErrInvalid
	}
	if f.Status != "" && !model.Status(f.Status).Valid() {
		return ErrInvalid
	}
	return nil
}
func (s *Service) List(a identity.PublicUser, f Filter) ([]model.Task, int64, error) {
	if err := validateFilter(a, f); err != nil {
		return nil, 0, err
	}
	f.Actor = a
	rows, total, err := s.Repo.List(f)
	if err != nil {
		return nil, 0, err
	}
	if err = s.decorateTaskParents(a, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) decorateTaskParents(a identity.PublicUser, tasks []model.Task) error {
	parents := map[uint64]model.Batch{}
	for i := range tasks {
		task := &tasks[i]
		if task.BatchID == nil {
			continue
		}
		parent, ok := parents[*task.BatchID]
		if !ok {
			if s.BatchRepo == nil {
				return errors.New("publication batch provider unavailable")
			}
			var err error
			parent, err = s.BatchRepo.GetBatch(*task.BatchID)
			if err != nil {
				return err
			}
			parents[parent.ID] = parent
		}
		decorateBatchTask(a, parent, task)
	}
	return nil
}

func (s *Service) Summary(a identity.PublicUser, f Filter) (map[string]int64, error) {
	if err := validateFilter(a, f); err != nil {
		return nil, err
	}
	f.Actor = a
	return s.Repo.Summary(f)
}

func contentValid(c model.Content) bool {
	return strings.TrimSpace(c.Description) != "" && publicationTextLength(c.Description, c.Topics) <= 50 && c.CoverFileID > 0 && c.PortraitCoverFileID > 0 && c.Declaration == "无需声明" && utf8.RuneCountInString(c.Topics) <= 200 && c.Confirmed
}
func (s *Service) Create(ctx context.Context, a identity.PublicUser, in CreateInput) (model.Task, error) {
	if !canExecutePublication(a) {
		return model.Task{}, ErrNotFound
	}
	if len(in.RequestKey) < 1 || len(in.RequestKey) > 64 || strings.TrimSpace(in.RequestKey) != in.RequestKey || in.CategoryID <= 0 || in.MediaAccountID == 0 || in.CompositeOutputID <= 0 || !contentValid(in.Content) {
		return model.Task{}, ErrInvalid
	}
	bytes, _ := json.Marshal(in)
	sum := sha256.Sum256(bytes)
	digest := hex.EncodeToString(sum[:])
	if prior, err := s.Repo.ByRequest(int64(a.ID), in.RequestKey); err == nil {
		if !CanRead(a, prior) {
			return model.Task{}, ErrNotFound
		}
		if prior.RequestHash != digest {
			return model.Task{}, ErrConflict
		}
		return prior, nil
	} else if !errors.Is(err, ErrNotFound) {
		return model.Task{}, err
	}
	v, err := s.prepareTask(ctx, a, in, false)
	if err != nil {
		return model.Task{}, err
	}
	v.RequestHash = digest
	return s.Repo.Create(v)
}
func (s *Service) prepareTask(ctx context.Context, a identity.PublicUser, in CreateInput, saved bool) (model.Task, error) {
	if s.Account == nil || s.Output == nil || s.File == nil {
		return model.Task{}, errors.New("publication resource provider unavailable")
	}
	if in.Platform != "baidu" {
		return model.Task{}, ErrClosed
	}
	account, err := s.Account(a, fmt.Sprint(in.MediaAccountID))
	if err != nil {
		return model.Task{}, err
	}
	reason, err := accountQualification(a, account, in.Platform, in.CategoryID)
	if err != nil {
		return model.Task{}, err
	}
	if reason != "" {
		return model.Task{}, ErrInvalid
	}
	output, err := s.Output(a, in.CompositeOutputID)
	if err != nil {
		return model.Task{}, err
	}
	o := output.Output
	if !outputMatches(o, int64(*account.TeamID), in.CategoryID, in.Platform) || !outputReady(o) {
		return model.Task{}, ErrInvalid
	}
	var cover filemodel.File
	if saved && s.SavedFile != nil {
		cover, err = s.SavedFile(ctx, in.Content.CoverFileID)
	} else {
		cover, err = s.File(ctx, a, in.Content.CoverFileID, in.CategoryID)
	}
	if err != nil {
		return model.Task{}, err
	}
	if cover.Type != "image" || cover.SizeBytes <= 0 || len(cover.SHA256) != 64 || previewCover(cover) {
		return model.Task{}, ErrInvalid
	}
	if err := verifyCoverSpec(cover, 16, 9); err != nil {
		return model.Task{}, err
	}
	portrait, portraitSnapshot, err := s.preparePortraitCover(ctx, a, in.Content, in.CategoryID, in.CompositeOutputID, saved)
	if err != nil {
		return model.Task{}, err
	}
	coverSnapshot, err := verifyCoverRender(cover, in.Content, in.CompositeOutputID)
	if err != nil {
		return model.Task{}, err
	}
	snapshot, _ := json.Marshal(map[string]any{"portrait_cover_render": portraitSnapshot, "portrait_cover_sha256": portrait.SHA256, "portrait_cover_name": portrait.Name, "portrait_cover_resource_path": "/api/v1/file-resources/" + portrait.ResourceToken, "cover_render": coverSnapshot, "account_name": account.PlatformName, "account_owner": account.UserName, "browser_profile_id": account.BrowserProfileID, "output_name": o.Name, "video_sha256": o.SHA256, "video_size": o.FileSizeBytes, "cover_sha256": cover.SHA256, "cover_name": cover.Name, "cover_resource_path": "/api/v1/file-resources/" + cover.ResourceToken})
	now := time.Now().UTC()
	v := model.Task{TeamID: int64(*account.TeamID), OperatorID: int64(account.UserID), Platform: in.Platform, CategoryID: in.CategoryID, MediaAccountID: in.MediaAccountID, CompositeOutputID: in.CompositeOutputID, PlatformAccountUID: account.PlatformAccountID, CoverFileID: cover.ID, ResourceSnapshot: snapshot, Content: in.Content, Status: model.StatusPending, NotBefore: in.NotBefore, Revision: 1, CreatedBy: int64(a.ID), RequestKey: in.RequestKey, CreatedAt: now, UpdatedAt: now}
	return v, nil
}
func (s *Service) Edit(ctx context.Context, a identity.PublicUser, id uint64, in EditInput) (model.Task, error) {
	v, err := s.Get(a, id)
	if err != nil {
		return v, err
	}
	if v.Status != model.StatusPending || v.CurrentAttemptID != nil || v.AuthorizedAt != nil || v.Revision != in.ExpectedRevision {
		return model.Task{}, &ConflictError{TaskID: v.ID, Revision: v.Revision}
	}
	if !contentValid(in.Content) {
		return model.Task{}, ErrInvalid
	}
	var cover filemodel.File
	if in.Content.CoverFileID == v.CoverFileID && s.SavedFile != nil {
		cover, err = s.SavedFile(ctx, v.CoverFileID)
	} else {
		cover, err = s.File(ctx, a, in.Content.CoverFileID, v.CategoryID)
	}
	if err != nil {
		return model.Task{}, err
	}
	if cover.Type != "image" || cover.SizeBytes <= 0 || len(cover.SHA256) != 64 || previewCover(cover) {
		return model.Task{}, ErrInvalid
	}
	if err := verifyCoverSpec(cover, 16, 9); err != nil {
		return model.Task{}, err
	}
	portrait, portraitSnapshot, err := s.preparePortraitCover(ctx, a, in.Content, v.CategoryID, v.CompositeOutputID, in.Content.PortraitCoverFileID == v.Content.PortraitCoverFileID)
	if err != nil {
		return model.Task{}, err
	}
	coverSnapshot, err := verifyCoverRender(cover, in.Content, v.CompositeOutputID)
	if err != nil {
		return model.Task{}, err
	}
	var snapshot map[string]any
	if err := json.Unmarshal(v.ResourceSnapshot, &snapshot); err != nil {
		return model.Task{}, err
	}
	snapshot["portrait_cover_render"] = portraitSnapshot
	snapshot["portrait_cover_sha256"] = portrait.SHA256
	snapshot["portrait_cover_name"] = portrait.Name
	snapshot["portrait_cover_resource_path"] = "/api/v1/file-resources/" + portrait.ResourceToken
	snapshot["cover_render"] = coverSnapshot
	snapshot["cover_sha256"] = cover.SHA256
	snapshot["cover_name"] = cover.Name
	snapshot["cover_resource_path"] = "/api/v1/file-resources/" + cover.ResourceToken
	v.ResourceSnapshot, _ = json.Marshal(snapshot)
	v.Content = in.Content
	v.CoverFileID = cover.ID
	v.NotBefore = in.NotBefore
	v.AuditActor = int64(a.ID)
	return s.Repo.Edit(v, in.ExpectedRevision)
}

func ListFileReference(a identity.PublicUser, id int64) ([]repository.FileReference, error) {
	return repository.ListFileReference(database.DB(), a, id)
}

// A renderer preview is a validation sample and must never become task content.
func previewCover(f filemodel.File) bool {
	var metadata filemodel.ImageMetadata
	return len(f.MetadataJSON) > 0 && json.Unmarshal(f.MetadataJSON, &metadata) == nil && metadata.Preview
}
