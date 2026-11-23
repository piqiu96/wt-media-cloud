package service

import (
	"context"
	"encoding/json"
	"errors"
	accountmodel "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	profilemodel "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	productionmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	profiles "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	workmodel "github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	work "github.com/wt-media/wt-media-cloud/internal/modules/work/service"
	"gorm.io/gorm"
)

type ExecutionRepository interface {
	Get(uint64) (model.Task, error)
	Start(model.Task, uint64, int64, time.Time) (model.Task, error)
	Claim(int64, string, time.Time) (*model.Claim, error)
	Attempt(uint64) ([]model.Attempt, error)
	Report(model.Task, int64, model.ExecutionReport, bool, time.Time) (model.ExecutionFeedback, error)
	Complete(model.Task, int64, model.Completion, time.Time) (model.ExecutionFeedback, error)
}
type ExecutionService struct {
	Repo  ExecutionRepository
	Grant func(context.Context, model.Claim) (FileGrants, error)
}
type FileGrant struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type FileGrants struct {
	PortraitCover FileGrant `json:"portrait_cover"`
	Video         FileGrant `json:"video"`
	Cover         FileGrant `json:"cover"`
}
type GrantedClaim struct {
	model.Claim
	Files *FileGrants `json:"files,omitempty"`
}

func ExecutionStore() repository.ExecutionStore {
	return NewExecutionStore(database.DB())
}
func NewExecutionStore(db *gorm.DB) repository.ExecutionStore {
	return repository.ExecutionStore{Store: repository.Store{DB: db, LockFiles: files.LockAvailable}, ReadInput: readExecutionInput, RecordWork: func(tx *gorm.DB, v model.Task, a model.Attempt, in model.ResultResolution, now time.Time) (uint64, error) {
		var snapshot struct {
			AccountName string `json:"account_name"`
		}
		if err := json.Unmarshal(v.ResourceSnapshot, &snapshot); err != nil {
			return 0, err
		}
		w, err := work.RecordPublication(tx, workmodel.Work{AccountName: snapshot.AccountName, Platform: v.Platform, PlatformWorkID: in.PlatformWorkID, WorkURL: in.WorkURL, PlatformAccountUID: a.Input.PlatformUID, MediaAccountID: v.MediaAccountID, OperatorID: v.OperatorID, TeamID: v.TeamID, CategoryID: v.CategoryID, PublicationID: v.ID, AttemptID: a.ID, CompositeOutputID: v.CompositeOutputID, Title: a.Input.Content.Description, CoverFileID: a.Input.Content.CoverFileID, VideoSHA256: a.Input.VideoSHA256, PublishedAt: in.PublishedAt.UTC(), FirstObservedAt: now.UTC()})
		if errors.Is(err, work.ErrConflict) || errors.Is(err, work.ErrInvalid) {
			err = repository.ErrConflict
		}
		return w.ID, err
	}}
}
func WiredExecution() *ExecutionService {
	return &ExecutionService{Repo: ExecutionStore(), Grant: grantExecutionFiles}
}
func readExecutionInput(tx *gorm.DB, v model.Task) (model.ExecutionInput, error) {
	var snap struct {
		Profile          string `json:"browser_profile_id"`
		VideoSHA         string `json:"video_sha256"`
		VideoSize        int64  `json:"video_size"`
		CoverSHA         string `json:"cover_sha256"`
		PortraitCoverSHA string `json:"portrait_cover_sha256"`
	}
	var out model.ExecutionInput
	if json.Unmarshal(v.ResourceSnapshot, &snap) != nil || v.Platform != "baidu" || !contentValid(v.Content) {
		return out, repository.ErrInputChanged
	}
	id, e := strconv.ParseUint(snap.Profile, 10, 64)
	if e != nil || id == 0 {
		return out, repository.ErrInputChanged
	}
	p, found, e := profiles.GetProfileInScope(snap.Profile, tx)
	if e == nil && (!found || int64(p.UserID) != v.OperatorID || !p.Enabled || p.LocalState != profilemodel.ProfileActive || p.ReservedProxyID != "" || p.CookieOperationID != "") {
		return out, repository.ErrInputChanged
	}
	if e != nil {
		return out, e
	}
	f, e := files.ReadSavedReference(tx, v.CoverFileID)
	if errors.Is(e, files.ErrNotFound) {
		return out, repository.ErrInputChanged
	}
	if e != nil {
		return out, e
	}
	owner, found, e := identity.ResolveUserIn(tx, identity.UserID(v.OperatorID))
	if e != nil {
		return out, e
	}
	if !found || !canExecutePublication(owner) || owner.TeamID == nil || int64(*owner.TeamID) != v.TeamID || !slices.Contains(owner.CategoryIDs, identity.CategoryID(v.CategoryID)) {
		return out, repository.ErrInputChanged
	}
	category, found, e := identity.ResolveCategoryIn(tx, identity.CategoryID(v.CategoryID))
	if e != nil {
		return out, e
	}
	if !found || !category.Enabled {
		return out, repository.ErrInputChanged
	}
	account, e := accounts.GetOwnedAccountRecord(owner, strconv.FormatUint(v.MediaAccountID, 10), tx)
	if errors.Is(e, accounts.ErrNotFound) || errors.Is(e, accounts.ErrForbidden) {
		return out, repository.ErrInputChanged
	}
	if e != nil {
		return out, e
	}
	if account.TeamID == nil || int64(*account.TeamID) != v.TeamID || string(account.Platform) != v.Platform || account.PlatformAccountID != v.PlatformAccountUID || account.BrowserProfileID != snap.Profile || !account.Enabled || account.IdentificationStatus != accountmodel.IdentificationIdentified || account.PlatformLoginStatus != accountmodel.PlatformLoginNormal || !slices.Contains(account.CategoryIDs, identity.CategoryID(v.CategoryID)) {
		return out, repository.ErrInputChanged
	}
	o, found, e := production.ReadSavedOutput(tx, v.CompositeOutputID)
	if e != nil {
		return out, e
	}
	supported := false
	for _, platform := range o.Platforms {
		if platform == v.Platform {
			supported = true
		}
	}
	if !found || !supported || o.TeamID != v.TeamID || int64(o.CategoryID) != v.CategoryID || o.PublishStatus == productionmodel.OutputPublishArchived || o.SHA256 != snap.VideoSHA || o.FileSizeBytes != snap.VideoSize || f.Type != "image" || f.SHA256 != snap.CoverSHA || f.SizeBytes <= 0 || len(o.SHA256) != 64 || len(f.SHA256) != 64 || o.FileSizeBytes <= 0 {
		return out, repository.ErrInputChanged
	}
	portrait, e := files.ReadSavedReference(tx, v.Content.PortraitCoverFileID)
	if e != nil {
		return out, e
	}
	if portrait.SHA256 != snap.PortraitCoverSHA || len(portrait.SHA256) != 64 || verifyCoverSpec(portrait, 3, 4) != nil || verifyCoverSpec(f, 16, 9) != nil {
		return out, repository.ErrInputChanged
	}
	return model.ExecutionInput{PortraitCoverSHA256: portrait.SHA256, PortraitCoverSize: portrait.SizeBytes, ProfileID: id, BitProfileID: p.BitProfileID, BitMainUserID: p.BitMainUserID, PlatformUID: v.PlatformAccountUID, Content: v.Content, VideoSHA256: o.SHA256, VideoSize: o.FileSizeBytes, CoverSHA256: f.SHA256, CoverSize: f.SizeBytes}, nil
}
func grantExecutionFiles(ctx context.Context, c model.Claim) (FileGrants, error) {
	var out FileGrants
	i := c.Attempt.Input
	video, e := production.SavedOutputURL(ctx, c.Task.CompositeOutputID, i.VideoSHA256, i.VideoSize)
	if e != nil {
		return out, e
	}
	fsvc, e := files.Wired()
	if e != nil {
		return out, e
	}
	f, e := fsvc.ReadSavedReference(ctx, c.Task.CoverFileID)
	if e != nil {
		return out, e
	}
	if f.SHA256 != i.CoverSHA256 || f.SizeBytes != i.CoverSize {
		return out, ErrConflict
	}
	cover, e := fsvc.Resource(ctx, f.ResourceToken, true)
	if e != nil {
		return out, e
	}
	portrait, e := fsvc.ReadSavedReference(ctx, i.Content.PortraitCoverFileID)
	if e != nil {
		return out, e
	}
	if portrait.SHA256 != i.PortraitCoverSHA256 || portrait.SizeBytes != i.PortraitCoverSize {
		return out, ErrConflict
	}
	portraitResource, e := fsvc.Resource(ctx, portrait.ResourceToken, true)
	if e != nil {
		return out, e
	}
	return FileGrants{PortraitCover: FileGrant{URL: portraitResource.URL, SHA256: i.PortraitCoverSHA256, Size: i.PortraitCoverSize}, Video: FileGrant{URL: video, SHA256: i.VideoSHA256, Size: i.VideoSize}, Cover: FileGrant{URL: cover.URL, SHA256: i.CoverSHA256, Size: i.CoverSize}}, nil
}
func (s *ExecutionService) readable(a identity.PublicUser, id uint64) (model.Task, error) {
	v, e := s.Repo.Get(id)
	if e == nil && !CanRead(a, v) {
		return model.Task{}, ErrNotFound
	}
	return v, e
}
func (s *ExecutionService) Start(a identity.PublicUser, id, revision uint64) (model.Task, error) {
	if !canExecutePublication(a) {
		return model.Task{}, ErrNotFound
	}
	v, e := s.readable(a, id)
	if e != nil {
		return v, e
	}
	if !ownsPublication(a, v) {
		return model.Task{}, ErrNotFound
	}
	if v.Platform != "baidu" {
		return model.Task{}, ErrClosed
	}
	if revision == 0 {
		return model.Task{}, ErrInvalid
	}
	return s.Repo.Start(v, revision, int64(a.ID), time.Now().UTC())
}
func (s *ExecutionService) Attempt(a identity.PublicUser, id uint64) ([]model.Attempt, error) {
	if _, e := s.readable(a, id); e != nil {
		return nil, e
	}
	return s.Repo.Attempt(id)
}
func validKey(key string) bool {
	if len(key) < 1 || len(key) > 64 {
		return false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_:.", c)) {
			return false
		}
	}
	return true
}
func (s *ExecutionService) Claim(ctx context.Context, user int64, key string) (*GrantedClaim, error) {
	if user <= 0 || !validKey(key) {
		return nil, ErrInvalid
	}
	c, e := s.Repo.Claim(user, key, time.Now().UTC())
	if e != nil || c == nil {
		return nil, e
	}
	out := &GrantedClaim{Claim: *c}
	// Replaying finished or lost work never signs new input. The old attempt is
	// returned so the Agent can reconcile its own durable record without acting.
	if c.Task.Status != model.StatusRunning || c.Attempt.Status != model.AttemptStatusRunning || c.Attempt.Stage != model.AttemptStageClaimed {
		return out, nil
	}
	grants, e := s.Grant(ctx, *c)
	if e != nil {
		return nil, e
	}
	out.Files = &grants
	return out, nil
}
func (s *ExecutionService) Report(user int64, id uint64, in model.ExecutionReport, current bool) (model.ExecutionFeedback, error) {
	if user <= 0 || in.AttemptID == 0 || in.Seq == 0 {
		return model.ExecutionFeedback{}, ErrInvalid
	}
	v, e := s.Repo.Get(id)
	if e != nil {
		return model.ExecutionFeedback{}, e
	}
	if v.OperatorID != user {
		return model.ExecutionFeedback{}, ErrNotFound
	}
	return s.Repo.Report(v, user, in, current, time.Now().UTC())
}
func safePublicURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Host != "baijiahao.baidu.com" || u.Path != "/s" {
		return false
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil || len(q) != 1 || len(q["id"]) != 1 {
		return false
	}
	id := q.Get("id")
	if id == "" {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func (s *ExecutionService) Complete(user int64, id uint64, in model.Completion) (model.ExecutionFeedback, error) {
	if user <= 0 || in.AttemptID == 0 || in.Seq == 0 || len(in.ResultHash) != 64 {
		return model.ExecutionFeedback{}, ErrInvalid
	}
	result, e := model.ParseResult(in.Result)
	if e != nil || !safePublicURL(result.WorkURL) || len(result.ErrorCode) > 64 || len(result.ErrorMessage) > 1000 || len(result.PlatformWorkID) > 128 {
		return model.ExecutionFeedback{}, ErrInvalid
	}
	v, e := s.Repo.Get(id)
	if e != nil {
		return model.ExecutionFeedback{}, e
	}
	return s.Repo.Complete(v, user, in, time.Now().UTC())
}

// CheckProfileReferences protects saved task inputs and immutable attempt history
// from destructive Profile deletion; it does not block ordinary resource edits.
func CheckProfileReferences(id string) (bool, error) {
	return repository.ProfileReferenced(database.DB(), id)
}
