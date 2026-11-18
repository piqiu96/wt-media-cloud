package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	profiles "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"gorm.io/gorm"
)

type ExecutionRepository interface {
	Get(uint64) (model.Task, error)
	Start(model.Task, uint64, int64, time.Time) (model.Task, error)
	Claim(int64, string, time.Time) (*model.Claim, error)
	Attempts(uint64) ([]model.Attempt, error)
	Heartbeat(uint64, int64, uint64, uint64, time.Time) (model.ExecutionFeedback, error)
	Progress(model.Task, int64, model.ExecutionReport, bool, time.Time) (model.ExecutionFeedback, error)
	Complete(model.Task, int64, model.Completion, time.Time) (model.ExecutionFeedback, error)
}
type ExecutionService struct {
	Repo    ExecutionRepository
	Enabled bool
	Grant   func(context.Context, model.Claim) (FileGrants, error)
}
type FileGrant struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type FileGrants struct {
	Video FileGrant `json:"video"`
	Cover FileGrant `json:"cover"`
}
type GrantedClaim struct {
	model.Claim
	Files *FileGrants `json:"files,omitempty"`
}

func ExecutionStore() repository.ExecutionStore {
	return NewExecutionStore(database.DB())
}
func NewExecutionStore(db *gorm.DB) repository.ExecutionStore {
	return repository.ExecutionStore{Store: repository.Store{DB: db, LockFiles: files.LockAvailable}, LockInput: lockExecutionInput, LockScene: func(tx *gorm.DB, profile uint64, owner int64) error {
		return profiles.LockPublicationScene(tx, strconv.FormatUint(profile, 10), owner)
	}}
}
func WiredExecution() *ExecutionService {
	return &ExecutionService{Repo: ExecutionStore(), Enabled: config.Get().App.PublishingBaiduExecution, Grant: grantExecutionFiles}
}
func lockExecutionInput(tx *gorm.DB, v model.Task) (model.ExecutionInput, error) {
	var snap struct {
		Profile   string `json:"browser_profile_id"`
		VideoSHA  string `json:"video_sha256"`
		VideoSize int64  `json:"video_size"`
		CoverSHA  string `json:"cover_sha256"`
	}
	var out model.ExecutionInput
	if json.Unmarshal(v.ResourceSnapshot, &snap) != nil || v.Platform != "baidu" || !contentValid(v.Content) {
		return out, ErrConflict
	}
	id, e := strconv.ParseUint(snap.Profile, 10, 64)
	if e != nil || id == 0 {
		return out, ErrConflict
	}
	p, e := profiles.LockPublicationProfile(tx, snap.Profile, v.OperatorID)
	if errors.Is(e, profiles.ErrProfileNotFound) || errors.Is(e, profiles.ErrProfileInactive) {
		return out, ErrConflict
	}
	if e != nil {
		return out, e
	}
	f, e := files.LockSavedPublicationFile(tx, v.CoverFileID)
	if errors.Is(e, files.ErrNotFound) {
		return out, ErrConflict
	}
	if e != nil {
		return out, e
	}
	eligible, e := identity.CheckPublicationScope(tx, v.OperatorID, v.TeamID, v.CategoryID)
	if e != nil {
		return out, e
	}
	if !eligible {
		return out, ErrConflict
	}
	eligible, e = accounts.CheckPublicationBinding(tx, accounts.PublicationBinding{ID: v.MediaAccountID, Owner: v.OperatorID, Team: v.TeamID, Category: v.CategoryID, Platform: v.Platform, UID: v.PlatformAccountUID, Profile: snap.Profile})
	if e != nil {
		return out, e
	}
	if !eligible {
		return out, ErrConflict
	}
	o, found, e := production.LockSavedPublicationOutput(tx, v.CompositeOutputID)
	if e != nil {
		return out, e
	}
	supported := false
	for _, platform := range o.Platforms {
		if platform == v.Platform {
			supported = true
		}
	}
	if !found || !supported || o.TeamID != v.TeamID || int64(o.CategoryID) != v.CategoryID || o.PublishStatus == "archived" || o.SHA256 != snap.VideoSHA || o.FileSizeBytes != snap.VideoSize || f.Type != "image" || f.SHA256 != snap.CoverSHA || f.SizeBytes <= 0 || len(o.SHA256) != 64 || len(f.SHA256) != 64 || o.FileSizeBytes <= 0 {
		return out, ErrConflict
	}
	return model.ExecutionInput{ProfileID: id, BitProfileID: p.BitProfileID, BitMainUserID: p.BitMainUserID, PlatformUID: v.PlatformAccountUID, Content: v.Content, VideoSHA256: o.SHA256, VideoSize: o.FileSizeBytes, CoverSHA256: f.SHA256, CoverSize: f.SizeBytes}, nil
}
func grantExecutionFiles(ctx context.Context, c model.Claim) (FileGrants, error) {
	var out FileGrants
	i := c.Attempt.Input
	video, e := production.SavedPublicationOutputURL(ctx, c.Task.CompositeOutputID, i.VideoSHA256, i.VideoSize)
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
	return FileGrants{Video: FileGrant{URL: video, SHA256: i.VideoSHA256, Size: i.VideoSize}, Cover: FileGrant{URL: cover.URL, SHA256: i.CoverSHA256, Size: i.CoverSize}}, nil
}
func (s *ExecutionService) readable(a identity.PublicUser, id uint64) (model.Task, error) {
	v, e := s.Repo.Get(id)
	if e == nil && !CanRead(a, v) {
		return model.Task{}, ErrNotFound
	}
	return v, e
}
func (s *ExecutionService) Start(a identity.PublicUser, id, revision uint64) (model.Task, error) {
	v, e := s.readable(a, id)
	if e != nil {
		return v, e
	}
	if !s.Enabled || v.Platform != "baidu" {
		return model.Task{}, ErrClosed
	}
	if revision == 0 {
		return model.Task{}, ErrInvalid
	}
	return s.Repo.Start(v, revision, int64(a.ID), time.Now().UTC())
}
func (s *ExecutionService) Attempts(a identity.PublicUser, id uint64) ([]model.Attempt, error) {
	if _, e := s.readable(a, id); e != nil {
		return nil, e
	}
	return s.Repo.Attempts(id)
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
	if !s.Enabled {
		return nil, ErrClosed
	}
	c, e := s.Repo.Claim(user, key, time.Now().UTC())
	if e != nil || c == nil {
		return nil, e
	}
	out := &GrantedClaim{Claim: *c}
	// Replaying finished or lost work never signs new input. The old attempt is
	// returned so the Agent can reconcile its own durable record without acting.
	if c.Task.Status != "running" || c.Attempt.Status != "running" || c.Attempt.Stage != "claimed" {
		return out, nil
	}
	grants, e := s.Grant(ctx, *c)
	if e != nil {
		return nil, e
	}
	out.Files = &grants
	return out, nil
}
func (s *ExecutionService) Heartbeat(user int64, id, attempt, seq uint64) (model.ExecutionFeedback, error) {
	if user <= 0 || attempt == 0 || seq == 0 {
		return model.ExecutionFeedback{}, ErrInvalid
	}
	return s.Repo.Heartbeat(id, user, attempt, seq, time.Now().UTC())
}
func (s *ExecutionService) Progress(user int64, id uint64, in model.ExecutionReport, current bool) (model.ExecutionFeedback, error) {
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
	// Disabling execution prevents new external actions, but not result reporting.
	return s.Repo.Progress(v, user, in, current && s.Enabled, time.Now().UTC())
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

func CheckProfileExecution(tx *gorm.DB, id string) (bool, error) {
	e := repository.ProfileExecutionBusy(tx, id)
	if errors.Is(e, repository.ErrConflict) {
		return true, nil
	}
	return false, e
}
