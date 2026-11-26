package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wt-media/wt-media-cloud/internal/config"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	images "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

const maxSafeTarget = 9007199254740991

func (s *Service) batchLimits() config.PublicationBatchConfig { return s.BatchLimits.WithDefaults() }

type BatchFilter = repository.BatchFilter
type BatchConflictError = repository.BatchConflictError

type BatchInputError struct {
	Issue model.BatchIssue `json:"issue"`
	Cause error            `json:"-"`
}

func (e *BatchInputError) Error() string { return e.Issue.Message }
func (e *BatchInputError) Unwrap() error { return e.Cause }

type BatchRepository interface {
	BatchByRequest(int64, string) (model.Batch, error)
	GetBatch(uint64) (model.Batch, error)
	ListBatches(repository.BatchFilter) ([]model.Batch, int64, error)
	CreateBatch(model.Batch) (model.Batch, error)
	EditBatch(model.Batch, uint64) (model.Batch, error)
	ListBatchRecords(uint64, int, int) ([]model.BatchRecord, int64, error)
}

type BatchInput struct {
	Platform         string                      `json:"platform"`
	Targets          []model.BatchTarget         `json:"targets"`
	Config           model.BatchConfig           `json:"config"`
	ExecutionSetting model.BatchExecutionSetting `json:"execution_setting"`
}
type CreateBatchInput struct {
	BatchInput
	Name       string `json:"name"`
	RequestKey string `json:"request_key"`
}
type EditBatchInput struct {
	BatchInput
	Name             string `json:"name"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
}
type ValidateBatchInput struct {
	BatchInput
	BatchID          uint64 `json:"batch_id,string"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
}
type CopyBatchResult struct {
	BatchInput
	Name           string               `json:"name"`
	SourceID       uint64               `json:"source_id,string"`
	SourceRevision uint64               `json:"source_revision,string"`
	Readiness      model.BatchReadiness `json:"readiness"`
}
type BatchPage[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func validBatchPagination(page, size int) bool {
	return page > 0 && page-1 <= math.MaxInt/100 && size > 0 && size <= 100
}
func validBatchKey(key string) bool {
	return len(key) > 0 && len(key) <= 64 && strings.TrimSpace(key) == key
}
func validBatchName(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= 100
}
func canPrepareBatch(a identity.PublicUser) bool {
	return canExecutePublication(a) && a.TeamID != nil && *a.TeamID > 0
}
func canReadBatch(a identity.PublicUser, v model.Batch) bool {
	team := identity.TeamID(v.TeamID)
	if len(v.Targets) == 0 {
		return false
	}
	for _, target := range v.Targets {
		if !a.CanAccess(identity.UserID(v.OperatorID), &team, identity.CategoryID(target.CategoryID)) {
			return false
		}
	}
	return a.ID > 0
}
func ownsBatch(a identity.PublicUser, v model.Batch) bool {
	return canPrepareBatch(a) && int64(a.ID) == v.OperatorID && int64(*a.TeamID) == v.TeamID
}
func batchCopyAllowed(v model.Batch) bool {
	return v.ExecutionStatus == model.BatchEnded || v.ExecutionStatus == model.BatchCancelled
}
func projectBatch(a identity.PublicUser, v model.Batch) model.Batch {
	v.ActionBlockers = []model.BatchActionBlocker{}
	v.AllowedActions = []string{}
	if ownsBatch(a, v) {
		if v.ExecutionStatus == model.BatchNotStarted && v.StartedAt == nil {
			v.AllowedActions = append(v.AllowedActions, "edit", "start", "stop")
		}
		if v.ExecutionStatus == model.BatchRunning {
			v.AllowedActions = append(v.AllowedActions, "pause", "stop")
		}
		if v.ExecutionStatus == model.BatchPaused {
			v.AllowedActions = append(v.AllowedActions, "resume", "stop")
		}
		if (v.ExecutionStatus == model.BatchRunning || v.ExecutionStatus == model.BatchPaused) &&
			v.Execution.TaskCounts[model.StatusPending]+v.Execution.TaskCounts[model.StatusQueued]+v.Execution.TaskCounts[model.StatusRunning]+
				v.Execution.TaskCounts[model.StatusAwaitingHuman]+v.Execution.TaskCounts[model.StatusUncertain] == 0 {
			v.AllowedActions = append(v.AllowedActions, "finish")
		}
		if batchCopyAllowed(v) {
			v.AllowedActions = append(v.AllowedActions, "copy")
		}
	}
	if v.PlanID != nil {
		filtered := []string{}
		v.ActionBlockers = []model.BatchActionBlocker{}
		for _, act := range v.AllowedActions {
			blocked := false
			code := ""
			message := ""
			if v.PlanConfigurationFrozen && (act == "edit" || act == "start") {
				blocked = true
				code = "plan_configuration_frozen"
				message = "计划已启动，配置固定；请从计划当前位置处理"
			}
			if (v.PlanPermanentStopped || v.PlanPositionBlocked) && act != "copy" {
				blocked = true
				code = "plan_position_blocked"
				message = "所属计划已停止或该批次不在当前执行位置"
			}
			if act == "start" {
				blocked = true
				code = "plan_position_blocked"
				message = "所属计划批次需从计划统一启动，请到计划完成全批确认"
			}
			if blocked {
				v.ActionBlockers = append(v.ActionBlockers, model.BatchActionBlocker{Action: act, Code: code, Message: message})
			} else {
				filtered = append(filtered, act)
			}
		}
		v.AllowedActions = filtered
		if ownsBatch(a, v) && v.PlanTakeoverAvailable {
			v.AllowedActions = append(v.AllowedActions, "takeover")
		}
	}

	return v
}
func batchDetail(a identity.PublicUser, v model.Batch) model.BatchDetail {
	return model.BatchDetail{Batch: projectBatch(a, v), Config: v.Config, Readiness: v.Readiness}
}
func (s *Service) loadBatch(a identity.PublicUser, id uint64) (model.Batch, error) {
	if s.BatchRepo == nil {
		return model.Batch{}, errors.New("batch repository unavailable")
	}
	v, err := s.BatchRepo.GetBatch(id)
	if err == nil && !canReadBatch(a, v) {
		return model.Batch{}, ErrNotFound
	}
	return v, err
}
func (s *Service) GetBatch(a identity.PublicUser, id uint64) (model.BatchDetail, error) {
	v, err := s.loadBatch(a, id)
	return batchDetail(a, v), err
}
func (s *Service) ListBatches(a identity.PublicUser, f BatchFilter) (BatchPage[model.Batch], error) {
	var empty BatchPage[model.Batch]
	if err := validateFilter(a, Filter{Page: f.Page, PageSize: f.PageSize, Search: f.Search, Platform: f.Platform}); err != nil {
		return empty, err
	}
	if !validBatchPagination(f.Page, f.PageSize) ||
		f.RequestKey != "" && !validBatchKey(f.RequestKey) ||
		f.ConfigurationStatus != "" && f.ConfigurationStatus != model.BatchIncomplete && f.ConfigurationStatus != model.BatchReady ||
		f.ExecutionStatus != "" && !slices.Contains([]string{model.BatchNotStarted, model.BatchRunning, model.BatchPaused, model.BatchStopping, model.BatchEnded, model.BatchCancelled}, f.ExecutionStatus) {
		return empty, ErrInvalid
	}
	if s.BatchRepo == nil {
		return empty, errors.New("batch repository unavailable")
	}
	f.Actor = a
	rows, total, err := s.BatchRepo.ListBatches(f)
	for i := range rows {
		rows[i] = projectBatch(a, rows[i])
	}
	return BatchPage[model.Batch]{Items: rows, Total: total, Page: f.Page, PageSize: f.PageSize}, err
}
func (s *Service) BatchRecords(a identity.PublicUser, id uint64, page, size int) (BatchPage[model.BatchRecord], error) {
	var empty BatchPage[model.BatchRecord]
	if !validBatchPagination(page, size) {
		return empty, ErrInvalid
	}
	if _, err := s.loadBatch(a, id); err != nil {
		return empty, err
	}
	rows, total, err := s.BatchRepo.ListBatchRecords(id, page, size)
	return BatchPage[model.BatchRecord]{Items: rows, Total: total, Page: page, PageSize: size}, err
}

func (s *Service) batchTargets(a identity.PublicUser, platform string, targets []model.BatchTarget) (map[int64]int64, error) {
	if !canPrepareBatch(a) {
		return nil, ErrNotFound
	}
	if !s.platformEnabled(platform) {
		return nil, ErrClosed
	}
	if len(targets) == 0 || len(targets) > 100 {
		return nil, ErrInvalid
	}
	counts := make(map[int64]int64, len(targets))
	for _, target := range targets {
		_, duplicate := counts[target.CategoryID]
		if target.CategoryID <= 0 || target.TargetCount < 0 || target.TargetCount > maxSafeTarget || duplicate {
			return nil, ErrInvalid
		}
		if err := s.checkBatchCategory(a, target.CategoryID); err != nil {
			return nil, err
		}
		counts[target.CategoryID] = target.TargetCount
	}
	return counts, nil
}

func (s *Service) checkBatchSetting(in model.BatchExecutionSetting) error {
	limits := s.batchLimits()
	var field, message string
	switch {
	case in.ConcurrencyLimit < 1 || in.ConcurrencyLimit > limits.MaxConcurrency:
		field, message = "concurrency_limit", fmt.Sprintf("并发数必须在 1 到 %d 之间", limits.MaxConcurrency)
	case in.GroupIntervalSeconds < 0 || in.GroupIntervalSeconds > limits.MaxGroupIntervalSeconds:
		field, message = "group_interval_seconds", fmt.Sprintf("组间发布间隔必须在 0 到 %d 秒之间", limits.MaxGroupIntervalSeconds)
	case in.ExceptionTaskLimit == 0:
		field, message = "exception_task_limit", "异常任务上限必须为正整数"
	case in.BetweenGroupPolicy != "auto_continue" && in.BetweenGroupPolicy != "pause_on_exception":
		field, message = "between_group_policy", "请选择组间处理方式"
	default:
		return nil
	}
	return &BatchInputError{Issue: model.BatchIssue{Field: field, Code: "setting_invalid", Message: message}, Cause: ErrInvalid}
}
func validBatchCoverReference(ref *model.CoverRender, fileID int64) bool {
	if ref == nil {
		return true
	}
	hash, err := hex.DecodeString(ref.RenderInputHash)
	return fileID > 0 && ref.TemplateID > 0 && ref.PublishedRevision > 0 &&
		utf8.RuneCountInString(ref.Title) <= 2000 && strings.TrimSpace(ref.Title) != "" &&
		ref.BackgroundOutputID >= 0 && (ref.BackgroundOutputID == 0 || ref.BackgroundImageAssetID == 0) &&
		len(hash) == sha256.Size && err == nil && strings.ToLower(ref.RenderInputHash) == ref.RenderInputHash
}
func validBatchCandidate(row model.BatchCandidate) bool {
	c := row.Content
	if !validBatchKey(row.RowKey) || !validGenerationRevision(row.InputRevision, false) || row.FieldRevisions == nil ||
		len(row.FieldRevisions) > 16 || row.CategoryID <= 0 || row.MediaAccountID == 0 || row.CompositeOutputID <= 0 ||
		row.Order < 0 || row.Order > maxSafeTarget || c.CoverFileID < 0 || c.PortraitCoverFileID < 0 ||
		utf8.RuneCountInString(row.CoverTitle) > 2000 || utf8.RuneCountInString(c.Description) > 2000 || utf8.RuneCountInString(c.Title) > 80 || c.SpaceCoverFileID < 0 ||
		utf8.RuneCountInString(c.Topics) > 200 || utf8.RuneCountInString(c.Declaration) > 100 ||
		!validBatchCoverReference(c.CoverRender, c.CoverFileID) || !validBatchCoverReference(c.PortraitCoverRender, c.PortraitCoverFileID) || !validBatchCoverReference(c.SpaceCoverRender, c.SpaceCoverFileID) {
		return false
	}
	for field, rev := range row.FieldRevisions {
		if !validBatchKey(field) || !validGenerationRevision(rev, true) {
			return false
		}
	}
	return row.NotBefore == nil || row.NotBefore.Year() >= 1000 && row.NotBefore.Year() <= 9999
}
func shortfalls(targets []model.BatchTarget, rows []model.BatchCandidate) []model.BatchShortfall {
	counts := make(map[int64]int, len(targets))
	for _, row := range rows {
		counts[row.CategoryID]++
	}
	out := make([]model.BatchShortfall, 0, len(targets))
	for _, target := range targets {
		count := counts[target.CategoryID]
		out = append(out, model.BatchShortfall{CategoryID: target.CategoryID, TargetCount: target.TargetCount, CandidateCount: count, Shortfall: max(0, target.TargetCount-int64(count))})
	}
	return out
}

func batchIssue(row model.BatchCandidate, field, code, message string) model.BatchIssue {
	return model.BatchIssue{RowKey: row.RowKey, CategoryID: row.CategoryID, Field: field, Code: code, Message: message}
}
func batchRowError(row model.BatchCandidate, field, code, message string, cause error) error {
	return &BatchInputError{Issue: batchIssue(row, field, code, message), Cause: cause}
}
func outputMatches(o outputmodel.CompositeOutput, teamID, categoryID int64, platform string) bool {
	return int64(o.CategoryID) == categoryID && o.TeamID == teamID && slices.Contains(o.Platforms, platform)
}
func outputReady(o outputmodel.CompositeOutput) bool {
	return o.PublishStatus != outputmodel.OutputPublishArchived && o.Video.Status == "ready" && o.FileSizeBytes > 0 && len(o.SHA256) == 64
}
func batchResourceMissing(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, accounts.ErrNotFound) || errors.Is(err, accounts.ErrForbidden) ||
		errors.Is(err, production.ErrComposeOutputNotFound) || errors.Is(err, production.ErrComposeOutputForbidden) ||
		errors.Is(err, files.ErrNotFound) || errors.Is(err, files.ErrForbidden) || errors.Is(err, images.ErrNotFound)
}

func (s *Service) batchCandidateCovers(ctx context.Context, a identity.PublicUser, platform string, row model.BatchCandidate, retained map[int64]bool, save bool) ([]model.BatchIssue, model.BatchCoverResource, error) {
	var issues []model.BatchIssue
	resources := model.BatchCoverResource{RowKey: row.RowKey}
	mainWidth, mainHeight := 16, 9
	if platform == "bilibili" {
		mainWidth, mainHeight = 4, 3
	}
	secondaryID, secondaryRef, secondaryWidth, secondaryHeight, secondaryKey := secondaryCover(platform, row.Content)
	for _, cover := range []struct {
		id            int64
		ref           *model.CoverRender
		width, height int
		field         string
	}{
		{row.Content.CoverFileID, row.Content.CoverRender, mainWidth, mainHeight, "cover_file_id"},
		{secondaryID, secondaryRef, secondaryWidth, secondaryHeight, secondaryKey + "_file_id"},
	} {
		if cover.id == 0 {
			issues = append(issues, batchIssue(row, cover.field, "cover_missing", "请选择所需规格的封面"))
			continue
		}
		if s.File == nil {
			return nil, resources, errors.New("cover provider unavailable")
		}
		var file filemodel.File
		var err error
		if retained[cover.id] && s.SavedFile != nil {
			file, err = s.SavedFile(ctx, cover.id)
		} else {
			file, err = s.File(ctx, a, cover.id, row.CategoryID)
		}
		if err != nil {
			known := batchResourceMissing(err) || errors.Is(err, files.ErrInvalid) || errors.Is(err, images.ErrInvalid)
			if known && !save {
				issues = append(issues, batchIssue(row, cover.field, "cover_unavailable", "封面不可用或已超出访问范围"))
				continue
			}
			return nil, resources, err
		}
		if previewCover(file) || file.Type != "image" {
			if save {
				return nil, resources, batchRowError(row, cover.field, "cover_invalid", "封面必须选用正式图片，不能采用预览样例", ErrInvalid)
			}
			issues = append(issues, batchIssue(row, cover.field, "cover_invalid", "封面必须选用正式图片，不能采用预览样例"))
			continue
		}
		if file.ResourceToken != "" {
			if cover.field == "cover_file_id" {
				resources.CoverResourcePath = files.ResourcePath(file.ResourceToken)
			} else if platform == "bilibili" {
				resources.SpaceCoverResourcePath = files.ResourcePath(file.ResourceToken)
			} else {
				resources.PortraitCoverResourcePath = files.ResourcePath(file.ResourceToken)
			}
		}
		if len(file.SHA256) != 64 || verifyCoverSpec(file, cover.width, cover.height) != nil {
			issues = append(issues, batchIssue(row, cover.field, "cover_invalid", "封面必须是符合平台规格的正式图片"))
			continue
		}
		if _, err = verifyCoverRender(file, model.Content{CoverRender: cover.ref}, row.CompositeOutputID); err != nil ||
			cover.ref != nil && strings.TrimSpace(row.CoverTitle) != cover.ref.Title {
			issues = append(issues, batchIssue(row, cover.field, "cover_outdated", "封面与当前标题或成片不匹配，请重新生成或选择"))
		}
	}
	return issues, resources, nil
}

func (s *Service) checkBatch(ctx context.Context, a identity.PublicUser, in BatchInput, prior *model.Batch, save bool) (model.BatchReadiness, error) {
	checks := model.BatchReadiness{Issues: []model.BatchIssue{}, CoverResources: []model.BatchCoverResource{}, Shortfalls: shortfalls(in.Targets, in.Config.Candidates)}
	targets, err := s.batchTargets(a, in.Platform, in.Targets)
	if err != nil {
		return checks, err
	}
	if in.Config.Candidates == nil {
		return checks, ErrInvalid
	}
	if bound := s.batchLimits().MaxCandidates; len(in.Config.Candidates) > bound {
		return checks, &BatchInputError{Issue: model.BatchIssue{Field: "candidates", Code: "candidate_limit", Message: fmt.Sprintf("当前服务每次最多接受 %d 条候选", bound)}, Cause: ErrInvalid}
	}
	if err := s.checkBatchSetting(in.ExecutionSetting); err != nil {
		return checks, err
	}
	inventory, err := s.loadAllocation(ctx, a, in.Platform, targets, in.Config.BatchAllocation, save)
	if err != nil {
		return checks, err
	}
	accountUsage := map[uint64]int{}
	rows := map[string]bool{}
	outputs := map[int64]bool{}
	order := map[int]bool{}
	ids := make([]int64, 0, len(in.Config.Candidates))
	for _, row := range in.Config.Candidates {
		_, targetExists := targets[row.CategoryID]
		if !validBatchCandidate(row) || !validCandidatePlatform(in.Platform, row.Content) || !targetExists || rows[row.RowKey] || outputs[row.CompositeOutputID] || order[row.Order] {
			return checks, batchRowError(row, "", "candidate_invalid", "候选身份、顺序或数据结构无效", ErrInvalid)
		}
		rows[row.RowKey], outputs[row.CompositeOutputID], order[row.Order] = true, true, true
		ids = append(ids, row.CompositeOutputID)
		accountUsage[row.MediaAccountID]++
		if limit := inventory.limits[row.MediaAccountID]; len(inventory.limits) > 0 && (limit == 0 || accountUsage[row.MediaAccountID] > limit) {
			return checks, batchRowError(row, "media_account_id", "account_limit", "候选超出该账号在本批的上限，或账号不在所选名单中", ErrInvalid)
		}
	}
	for id := range inventory.unavailableOutputs {
		if !outputs[id] {
			checks.Issues = append(checks.Issues, model.BatchIssue{Field: "output_ids", Code: "output_unavailable", Message: "所选成片中存在不可用或已超出访问范围的条目"})
			break
		}
	}
	occupied, err := s.batchOccupied(in.Platform, ids)
	if err != nil {
		return checks, err
	}
	retained := map[string]model.BatchCandidate{}
	if prior != nil {
		for _, row := range prior.Config.Candidates {
			retained[row.RowKey] = row
		}
	}
	for _, row := range in.Config.Candidates {
		if err := ctx.Err(); err != nil {
			return checks, err
		}
		account, err := inventory.account(s, a, row.MediaAccountID)
		if err != nil {
			if !save && batchResourceMissing(err) {
				checks.Issues = append(checks.Issues, batchIssue(row, "media_account_id", "account_unavailable", "账号不可用或不在本人范围内"))
				continue
			}
			return checks, err
		}
		reason, err := accountQualification(a, account, in.Platform, row.CategoryID)
		if err != nil || reason != "" {
			if save {
				if err == nil {
					err = ErrInvalid
				}
				return checks, batchRowError(row, "media_account_id", "account_ineligible", "该账号不符合当前发布类目和资格", err)
			}
			checks.Issues = append(checks.Issues, batchIssue(row, "media_account_id", "account_ineligible", "该账号不符合当前发布类目和资格"))
		}
		output, err := inventory.output(s, a, row.CompositeOutputID)
		if err != nil {
			if !save && batchResourceMissing(err) {
				checks.Issues = append(checks.Issues, batchIssue(row, "composite_output_id", "output_unavailable", "成片不可用或已超出访问范围"))
				continue
			}
			return checks, err
		}
		if !outputMatches(output, int64(*a.TeamID), row.CategoryID, in.Platform) {
			if save {
				return checks, batchRowError(row, "composite_output_id", "output_mismatch", "成片与候选平台、类目或团队不一致", ErrInvalid)
			}
			checks.Issues = append(checks.Issues, batchIssue(row, "composite_output_id", "output_mismatch", "成片与候选平台、类目或团队不一致"))
		} else if occupied[row.CompositeOutputID] {
			checks.Issues = append(checks.Issues, batchIssue(row, "composite_output_id", "output_occupied", "此成片已被当前平台任务占用"))
		} else if !outputReady(output) {
			checks.Issues = append(checks.Issues, batchIssue(row, "composite_output_id", "output_unavailable", "成片状态不可发布"))
		}
		if in.Platform == "bilibili" {
			probe := row.Content
			probe.CoverFileID = 1
			probe.SpaceCoverFileID = 2
			probe.Confirmed = true
			if !contentValidFor(in.Platform, probe) {
				checks.Issues = append(checks.Issues, batchIssue(row, "bilibili", "content_invalid", "请补齐B站标题、分区、标签、声明及可见范围；转载须填写来源说明"))
			}
		} else {
			if strings.TrimSpace(row.Content.Description) == "" || publicationTextLength(row.Content.Description, row.Content.Topics) > 50 {
				checks.Issues = append(checks.Issues, batchIssue(row, "description", "description_invalid", "请填写文案，描述与话题合计不超过50字"))
			}
			if row.Content.Declaration != "无需声明" {
				checks.Issues = append(checks.Issues, batchIssue(row, "declaration", "declaration_required", "请确认创作声明"))
			}
		}
		if !row.Content.Confirmed {
			checks.Issues = append(checks.Issues, batchIssue(row, "confirmed", "confirmation_required", "请确认当前内容"))
		}
		savedFiles := map[int64]bool{}
		if old, found := retained[row.RowKey]; found && old.CategoryID == row.CategoryID {
			savedFiles[old.Content.CoverFileID] = true
			savedFiles[old.Content.PortraitCoverFileID] = true
			savedFiles[old.Content.SpaceCoverFileID] = true
		}
		issues, resources, err := s.batchCandidateCovers(ctx, a, in.Platform, row, savedFiles, save)
		if err != nil {
			return checks, err
		}
		checks.Issues = append(checks.Issues, issues...)
		if resources.CoverResourcePath != "" || resources.PortraitCoverResourcePath != "" || resources.SpaceCoverResourcePath != "" {
			checks.CoverResources = append(checks.CoverResources, resources)
		}
	}
	if len(in.Config.Candidates) == 0 {
		checks.Issues = append(checks.Issues, model.BatchIssue{Field: "candidates", Code: "candidate_missing", Message: "请添加至少一条发布候选"})
	}
	checks.Ready = len(checks.Issues) == 0
	for _, gap := range checks.Shortfalls {
		if int64(gap.CandidateCount) != gap.TargetCount {
			checks.Ready = false
			if int64(gap.CandidateCount) > gap.TargetCount {
				checks.Issues = append(checks.Issues, model.BatchIssue{CategoryID: gap.CategoryID, Code: "target_exceeded", Message: "候选数量超过本类目目标"})
			}
		}
	}
	return checks, nil
}

func (s *Service) ValidateBatch(ctx context.Context, a identity.PublicUser, in ValidateBatchInput) (model.BatchReadiness, error) {
	if (in.BatchID == 0) != (in.ExpectedRevision == 0) {
		return model.BatchReadiness{}, ErrInvalid
	}
	if in.BatchID == 0 {
		return s.checkBatch(ctx, a, in.BatchInput, nil, false)
	}
	current, err := s.editableBatch(a, in.BatchID, in.ExpectedRevision)
	if err != nil {
		return model.BatchReadiness{}, err
	}
	if current.Platform != in.Platform {
		return model.BatchReadiness{}, ErrInvalid
	}
	reconcileBatchConfirmations(current.Config.Candidates, in.Config.Candidates)
	return s.checkBatch(ctx, a, in.BatchInput, &current, false)
}

func withBatchConfig(v model.Batch, name string, in BatchInput, readiness model.BatchReadiness) model.Batch {
	v.Name = strings.TrimSpace(name)
	v.Platform = in.Platform
	v.Targets = in.Targets
	v.Config = in.Config
	v.Config.Candidates = slices.Clone(in.Config.Candidates)
	slices.SortFunc(v.Config.Candidates, func(a, b model.BatchCandidate) int { return a.Order - b.Order })
	v.Readiness = readiness
	v.CandidatesCount = len(in.Config.Candidates)
	v.ExecutionSetting = in.ExecutionSetting
	v.ConfigurationStatus = model.BatchIncomplete
	if readiness.Ready {
		v.ConfigurationStatus = model.BatchReady
	}
	return v
}

func (s *Service) CreateBatch(ctx context.Context, a identity.PublicUser, in CreateBatchInput) (model.BatchDetail, error) {
	var empty model.BatchDetail
	if !canPrepareBatch(a) {
		return empty, ErrNotFound
	}
	if !validBatchKey(in.RequestKey) || !validBatchName(in.Name) {
		return empty, ErrInvalid
	}
	if s.BatchRepo == nil {
		return empty, errors.New("batch repository unavailable")
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return empty, ErrInvalid
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	existing, err := s.BatchRepo.BatchByRequest(int64(a.ID), in.RequestKey)
	if err == nil {
		if !canReadBatch(a, existing) {
			return empty, ErrNotFound
		}
		if existing.RequestHash != digest {
			return empty, &BatchConflictError{BatchID: existing.ID, Revision: existing.Revision}
		}
		return batchDetail(a, existing), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return empty, err
	}
	readiness, err := s.checkBatch(ctx, a, in.BatchInput, nil, true)
	if err != nil {
		return empty, err
	}
	v := model.Batch{TeamID: int64(*a.TeamID), OperatorID: int64(a.ID), CreatedBy: int64(a.ID), RequestKey: in.RequestKey, RequestHash: digest, Revision: 1, ExecutionStatus: model.BatchNotStarted}
	v, err = s.BatchRepo.CreateBatch(withBatchConfig(v, in.Name, in.BatchInput, readiness))
	if err != nil {
		return empty, err
	}
	if !canReadBatch(a, v) {
		return empty, ErrNotFound
	}
	return batchDetail(a, v), nil
}

func reconcileBatchConfirmations(old []model.BatchCandidate, rows []model.BatchCandidate) {
	previous := make(map[string]model.BatchCandidate, len(old))
	for _, row := range old {
		previous[row.RowKey] = row
	}
	for i, row := range rows {
		prior, ok := previous[row.RowKey]
		if !ok || !prior.Content.Confirmed || !row.Content.Confirmed {
			continue
		}
		before, after := prior.Content, row.Content
		before.Confirmed, after.Confirmed = false, false
		if prior.BatchPair != row.BatchPair || prior.CoverTitle != row.CoverTitle || !reflect.DeepEqual(before, after) {
			// Confirmation has its own field revision because changing an
			// input or another field does not confirm the new content.
			before, _ := strconv.ParseUint(prior.FieldRevisions["confirmed"], 10, 64)
			after, _ := strconv.ParseUint(row.FieldRevisions["confirmed"], 10, 64)
			if after <= before {
				rows[i].Content.Confirmed = false
			}
		}
	}
}

func (s *Service) editableBatch(a identity.PublicUser, id, revision uint64) (model.Batch, error) {
	v, err := s.loadBatch(a, id)
	if err != nil {
		return model.Batch{}, err
	}
	if !ownsBatch(a, v) {
		return model.Batch{}, ErrNotFound
	}
	if v.Revision != revision || v.StartedAt != nil || v.ExecutionStatus != model.BatchNotStarted {
		return model.Batch{}, &BatchConflictError{BatchID: v.ID, Revision: v.Revision}
	}
	return v, nil
}

func (s *Service) EditBatch(ctx context.Context, a identity.PublicUser, id uint64, in EditBatchInput) (model.BatchDetail, error) {
	var empty model.BatchDetail
	if in.ExpectedRevision == 0 || !validBatchName(in.Name) {
		return empty, ErrInvalid
	}
	v, err := s.editableBatch(a, id, in.ExpectedRevision)
	if err != nil {
		return empty, err
	}
	if v.Platform != in.Platform {
		return empty, ErrInvalid
	}
	reconcileBatchConfirmations(v.Config.Candidates, in.Config.Candidates)
	readiness, err := s.checkBatch(ctx, a, in.BatchInput, &v, true)
	if err != nil {
		return empty, err
	}
	updated, err := s.BatchRepo.EditBatch(withBatchConfig(v, in.Name, in.BatchInput, readiness), in.ExpectedRevision)
	return batchDetail(a, updated), err
}

func (s *Service) CopyBatch(ctx context.Context, a identity.PublicUser, id, revision uint64) (CopyBatchResult, error) {
	var out CopyBatchResult
	v, err := s.loadBatch(a, id)
	if err != nil {
		return out, err
	}
	if !ownsBatch(a, v) {
		return out, ErrNotFound
	}
	if v.Revision != revision || !batchCopyAllowed(v) {
		return out, &BatchConflictError{BatchID: v.ID, Revision: v.Revision}
	}
	config := model.BatchConfig{BatchAllocation: v.Config.BatchAllocation, Candidates: make([]model.BatchCandidate, 0, len(v.Config.Candidates))}
	for _, row := range v.Config.Candidates {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		config.Candidates = append(config.Candidates, model.BatchCandidate{
			BatchPair: row.BatchPair, RowKey: row.RowKey, InputRevision: "1", FieldRevisions: map[string]string{}, Order: row.Order, CoverTitle: row.CoverTitle,
			Content: copyCandidateContent(row.Content),
		})
	}
	in := BatchInput{Platform: v.Platform, Targets: v.Targets, Config: config, ExecutionSetting: v.ExecutionSetting}
	readiness, err := s.checkBatch(ctx, a, in, nil, false)
	if err != nil {
		return out, err
	}
	return CopyBatchResult{BatchInput: in, Name: v.Name, SourceID: v.ID, SourceRevision: v.Revision, Readiness: readiness}, nil
}

// Copies text/options only. Authorization, confirmation and generated cover products
// never survive a copy, including the platform-specific secondary cover.
func copyCandidateContent(c model.Content) model.Content {
	out := model.Content{Description: c.Description, Topics: c.Topics, Declaration: c.Declaration, Title: c.Title}
	if c.Bilibili != nil {
		b := *c.Bilibili
		b.Tags = append([]string(nil), b.Tags...)
		out.Bilibili = &b
	}
	return out
}

func validCandidatePlatform(platform string, c model.Content) bool {
	if platform == "baidu" {
		return c.Bilibili == nil && c.Title == "" && c.SpaceCoverFileID == 0 && c.SpaceCoverRender == nil && utf8.RuneCountInString(c.Description) <= 1000
	}
	if platform != "bilibili" || c.PortraitCoverFileID != 0 || c.PortraitCoverRender != nil || c.Topics != "" || c.Declaration != "" {
		return false
	}
	if c.Bilibili == nil {
		return true
	}
	b := c.Bilibili
	if b.PartitionID < 0 || utf8.RuneCountInString(b.PartitionName) > 100 || utf8.RuneCountInString(b.Statement) > 100 || platformTextLength("bilibili", b.Source) > 200 || b.Visibility != "" && b.Visibility != "public" && b.Visibility != "private" || len(b.Tags) > 10 {
		return false
	}
	seen := map[string]bool{}
	for _, tag := range b.Tags {
		if tag == "" || strings.TrimSpace(tag) != tag || strings.ContainsAny(tag, ",，\r\n") || platformTextLength("bilibili", tag) > 20 || seen[tag] {
			return false
		}
		seen[tag] = true
	}
	return true
}
