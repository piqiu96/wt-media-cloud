package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	images "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

type BatchExecutionRepository interface {
	StartBatch(uint64, int64, model.BatchStartInput, string, []model.Task, time.Time) (model.BatchStartResult, error)
	ControlBatch(uint64, int64, repository.BatchCommand, time.Time) (model.BatchStopResult, error)
	BatchTaskPage(uint64, repository.BatchTaskFilter) ([]model.Task, int64, error)
	ValidatePreparedTask(model.Task) error
}
type BatchTaskFilter = repository.BatchTaskFilter

type BatchPreviewInput struct {
	ExpectedRevision uint64   `json:"expected_revision,string"`
	RowKeys          []string `json:"row_keys"`
}
type BatchPreviewRow struct {
	RowKey string             `json:"row_key"`
	Ready  bool               `json:"ready"`
	Issues []model.BatchIssue `json:"issues"`
}
type BatchExecutionPreview struct {
	BatchID    uint64                    `json:"batch_id,string"`
	Revision   uint64                    `json:"revision,string"`
	CanStart   bool                      `json:"can_start"`
	Rows       []BatchPreviewRow         `json:"rows"`
	Shortfalls []model.CreationShortfall `json:"shortfalls"`
}
type BatchReasonInput struct {
	ExpectedRevision uint64 `json:"expected_revision,string"`
	Reason           string `json:"reason"`
}
type BatchResumeInput struct {
	RequestKey             string               `json:"request_key"`
	ExpectedRevision       uint64               `json:"expected_revision,string"`
	ExpectedPauseID        uint64               `json:"expected_pause_id,string"`
	AcknowledgedExceptions []model.TaskRevision `json:"acknowledged_exceptions"`
}
type BatchFinishInput struct {
	BatchReasonInput
	ExpectedFailedTasks []model.TaskRevision `json:"expected_failed_tasks"`
}

func batchCommandHash(value any) string {
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func startHash(in model.BatchStartInput) string {
	in.IncludedRowKeys = slices.Clone(in.IncludedRowKeys)
	in.ExcludedRowKeys = slices.Clone(in.ExcludedRowKeys)
	in.ExpectedShortfalls = slices.Clone(in.ExpectedShortfalls)
	slices.Sort(in.IncludedRowKeys)
	slices.Sort(in.ExcludedRowKeys)
	slices.SortFunc(in.ExpectedShortfalls, func(a, b model.ExpectedShortfall) int {
		if a.CategoryID < b.CategoryID {
			return -1
		}
		if a.CategoryID > b.CategoryID {
			return 1
		}
		return 0
	})
	return batchCommandHash(in)
}

func (s *Service) batchOwner(a identity.PublicUser, id uint64) (model.Batch, error) {
	b, err := s.loadBatch(a, id)
	if err != nil {
		return b, err
	}
	if !ownsBatch(a, b) {
		return model.Batch{}, ErrNotFound
	}
	if s.BatchExecutionRepo == nil {
		return model.Batch{}, errors.New("publication batch execution provider unavailable")
	}
	return b, nil
}

func (s *Service) prepareBatchExecution(ctx context.Context, a identity.PublicUser, b model.Batch, rows []model.BatchCandidate, gaps []model.CreationShortfall) (BatchExecutionPreview, []model.Task, error) {
	result := BatchExecutionPreview{BatchID: b.ID, Revision: b.Revision, CanStart: true, Rows: make([]BatchPreviewRow, 0, len(rows)), Shortfalls: gaps}
	tasks := make([]model.Task, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.CompositeOutputID)
	}
	occupied, err := s.batchOccupied(b.Platform, ids)
	if err != nil {
		return result, nil, err
	}
	for _, gap := range gaps {
		if gap.CreatedCount > gap.TargetCount {
			result.CanStart = false
		}
	}
	for _, row := range rows {
		if err = ctx.Err(); err != nil {
			return result, nil, err
		}
		preview := BatchPreviewRow{RowKey: row.RowKey, Issues: []model.BatchIssue{}}
		switch {
		case occupied[row.CompositeOutputID]:
			preview.Issues = append(preview.Issues, batchIssue(row, "composite_output_id", "output_occupied", "成片已被当前平台的任务占用"))
		case !contentValid(row.Content):
			preview.Issues = append(preview.Issues, batchIssue(row, "content", "content_incomplete", "请完成并确认文案、声明和全部封面"))
		default:
			title := strings.TrimSpace(row.CoverTitle)
			if (row.Content.CoverRender != nil && row.Content.CoverRender.Title != title) ||
				(row.Content.PortraitCoverRender != nil && row.Content.PortraitCoverRender.Title != title) {
				preview.Issues = append(preview.Issues, batchIssue(row, "cover_title", "cover_outdated", "当前封面短标题与已采用封面不匹配"))
				break
			}
			input := CreateInput{Platform: b.Platform, CategoryID: row.CategoryID, MediaAccountID: row.MediaAccountID, CompositeOutputID: row.CompositeOutputID, Content: row.Content, NotBefore: row.NotBefore}
			task, e := s.prepareTask(ctx, a, input, true)
			if e == nil {
				task.RowKey = &row.RowKey
				e = s.BatchExecutionRepo.ValidatePreparedTask(task)
			}
			if e == nil {
				tasks = append(tasks, task)
			} else if batchResourceMissing(e) || errors.Is(e, ErrInvalid) || errors.Is(e, ErrClosed) || errors.Is(e, repository.ErrInputChanged) || errors.Is(e, files.ErrInvalid) || errors.Is(e, images.ErrInvalid) {
				preview.Issues = append(preview.Issues, batchIssue(row, "resources", "execution_input_unavailable", "请检查本人账号、原浏览器环境、成片和封面"))
			} else {
				return result, nil, e
			}
		}
		preview.Ready = len(preview.Issues) == 0
		result.CanStart = result.CanStart && preview.Ready
		result.Rows = append(result.Rows, preview)
	}
	return result, tasks, nil
}

func (s *Service) PreviewBatchExecution(ctx context.Context, a identity.PublicUser, id uint64, in BatchPreviewInput) (BatchExecutionPreview, error) {
	var empty BatchExecutionPreview
	if in.ExpectedRevision == 0 || len(in.RowKeys) == 0 || len(in.RowKeys) > s.batchLimits().MaxCandidates {
		return empty, ErrInvalid
	}
	b, err := s.batchOwner(a, id)
	if err != nil {
		return empty, err
	}
	if b.Revision != in.ExpectedRevision {
		return empty, repository.BatchConflict(b, ErrConflict)
	}
	if b.ExecutionStatus != model.BatchNotStarted || b.StartedAt != nil {
		return empty, repository.BatchConflict(b, repository.ErrParentBlocked)
	}
	rows, gaps, err := model.BatchPreviewSelection(b, in.RowKeys)
	if err != nil {
		return empty, ErrInvalid
	}
	preview, _, err := s.prepareBatchExecution(ctx, a, b, rows, gaps)
	return preview, err
}

func (s *Service) StartBatch(ctx context.Context, a identity.PublicUser, id uint64, in model.BatchStartInput) (model.BatchStartResult, error) {
	var empty model.BatchStartResult
	if !validBatchKey(in.RequestKey) || in.ExpectedRevision == 0 || len(in.IncludedRowKeys) == 0 || len(in.IncludedRowKeys)+len(in.ExcludedRowKeys) > s.batchLimits().MaxCandidates ||
		in.ExcludedRowKeys == nil || in.ExpectedShortfalls == nil {
		return empty, ErrInvalid
	}
	b, err := s.batchOwner(a, id)
	if err != nil {
		return empty, err
	}
	digest := startHash(in)
	prior, err := repository.ReplayBatchStart(b, in.RequestKey, digest)
	if err != nil {
		return empty, err
	}
	if prior != nil {
		return *prior, nil
	}
	if b.Revision != in.ExpectedRevision {
		return empty, repository.BatchConflict(b, ErrConflict)
	}
	if b.ExecutionStatus != model.BatchNotStarted || b.StartedAt != nil {
		return empty, repository.BatchConflict(b, repository.ErrParentBlocked)
	}
	rows, gaps, err := model.BatchStartSelection(b, in)
	if err != nil {
		conflict := repository.BatchConflict(b, repository.ErrShortfallChanged)
		conflict.Shortfalls = gaps
		return empty, conflict
	}
	preview, tasks, err := s.prepareBatchExecution(ctx, a, b, rows, gaps)
	if err != nil {
		return empty, err
	}
	if !preview.CanStart {
		conflict := repository.BatchConflict(b, ErrConflict)
		conflict.Shortfalls = gaps
		for _, row := range preview.Rows {
			conflict.Issues = append(conflict.Issues, row.Issues...)
		}
		return empty, conflict
	}
	return s.BatchExecutionRepo.StartBatch(id, int64(a.ID), in, digest, tasks, time.Now().UTC())
}

func validReason(reason string) bool {
	return strings.TrimSpace(reason) != "" && utf8.RuneCountInString(reason) <= 200
}

func (s *Service) BatchReasonCommand(a identity.PublicUser, id uint64, action string, in BatchReasonInput) (model.BatchStopResult, error) {
	if in.ExpectedRevision == 0 || !validReason(in.Reason) {
		return model.BatchStopResult{}, ErrInvalid
	}
	if _, err := s.batchOwner(a, id); err != nil {
		return model.BatchStopResult{}, err
	}
	return s.BatchExecutionRepo.ControlBatch(id, int64(a.ID), repository.BatchCommand{Action: action, ExpectedRevision: in.ExpectedRevision, Reason: strings.TrimSpace(in.Reason)}, time.Now().UTC())
}
func (s *Service) ResumeBatch(a identity.PublicUser, id uint64, in BatchResumeInput) (model.BatchControlResult, error) {
	if !validBatchKey(in.RequestKey) || in.ExpectedRevision == 0 || in.ExpectedPauseID == 0 || in.AcknowledgedExceptions == nil {
		return model.BatchControlResult{}, ErrInvalid
	}
	if _, err := s.batchOwner(a, id); err != nil {
		return model.BatchControlResult{}, err
	}
	in.AcknowledgedExceptions = slices.Clone(in.AcknowledgedExceptions)
	slices.SortFunc(in.AcknowledgedExceptions, func(a, b model.TaskRevision) int {
		if a.TaskID < b.TaskID {
			return -1
		}
		if a.TaskID > b.TaskID {
			return 1
		}
		return 0
	})
	out, err := s.BatchExecutionRepo.ControlBatch(id, int64(a.ID), repository.BatchCommand{Action: "resume", ExpectedRevision: in.ExpectedRevision,
		ExpectedPauseID: in.ExpectedPauseID, RequestKey: in.RequestKey, Digest: batchCommandHash(in), Tasks: in.AcknowledgedExceptions}, time.Now().UTC())
	return out.BatchControlResult, err
}
func (s *Service) FinishBatch(a identity.PublicUser, id uint64, in BatchFinishInput) (model.BatchControlResult, error) {
	if in.ExpectedRevision == 0 || !validReason(in.Reason) || in.ExpectedFailedTasks == nil {
		return model.BatchControlResult{}, ErrInvalid
	}
	if _, err := s.batchOwner(a, id); err != nil {
		return model.BatchControlResult{}, err
	}
	out, err := s.BatchExecutionRepo.ControlBatch(id, int64(a.ID), repository.BatchCommand{Action: "finish", ExpectedRevision: in.ExpectedRevision,
		Reason: strings.TrimSpace(in.Reason), Tasks: in.ExpectedFailedTasks}, time.Now().UTC())
	return out.BatchControlResult, err
}
func (s *Service) BatchTasks(a identity.PublicUser, id uint64, f BatchTaskFilter) (BatchPage[model.Task], error) {
	var empty BatchPage[model.Task]
	if !validBatchPagination(f.Page, f.PageSize) || f.Status != "" && !model.Status(f.Status).Valid() || utf8.RuneCountInString(f.Search) > 100 {
		return empty, ErrInvalid
	}
	b, err := s.loadBatch(a, id)
	if err != nil {
		return empty, err
	}
	if s.BatchExecutionRepo == nil {
		return empty, errors.New("publication batch execution provider unavailable")
	}
	rows, total, err := s.BatchExecutionRepo.BatchTaskPage(id, f)
	for i := range rows {
		decorateBatchTask(a, b, &rows[i])
	}
	return BatchPage[model.Task]{Items: rows, Total: total, Page: f.Page, PageSize: f.PageSize}, err
}

func decorateBatchTask(a identity.PublicUser, b model.Batch, v *model.Task) {
	actions := []string{}
	v.AllowedActions = &actions
	owner := ownsBatch(a, b)
	stopped := b.StoppedAt != nil || b.ExecutionStatus == model.BatchEnded || b.ExecutionStatus == model.BatchCancelled || b.ExecutionStatus == model.BatchStopping
	if stopped {
		v.CanCancel = false
	}
	if v.CanCancel && !stopped && (v.Status != model.StatusRunning || owner) {
		*v.AllowedActions = append(*v.AllowedActions, "cancel")
	}
	switch v.Status {
	case model.StatusPending:
		if v.AuthorizedAt == nil && v.CurrentAttemptID == nil && !stopped {
			*v.AllowedActions = append(*v.AllowedActions, "edit")
		}
	case model.StatusFailed:
		if owner && !stopped && b.ExecutionStatus == model.BatchRunning &&
			(v.ExecutionGroupNo == nil || b.ExecutionGroupNo == nil || *v.ExecutionGroupNo != *b.ExecutionGroupNo || b.GroupEndedAt != nil) {
			*v.AllowedActions = append(*v.AllowedActions, "retry")
		}
	case model.StatusAwaitingHuman:
		if owner && !stopped && b.ExecutionStatus == model.BatchRunning {
			*v.AllowedActions = append(*v.AllowedActions, "resume")
		}
	case model.StatusUncertain:
		if owner {
			*v.AllowedActions = append(*v.AllowedActions, "resolve")
		}
	case model.StatusReviewing:
		*v.AllowedActions = append(*v.AllowedActions, "resolve")
	}
	if v.Status == model.StatusPending || v.Status == model.StatusQueued {
		switch {
		case stopped:
			v.ExecutionBlock = &model.TaskExecutionBlock{Code: "batch_stopped", Message: "批次已停止派发"}
		case b.ExecutionStatus == model.BatchPaused:
			v.ExecutionBlock = &model.TaskExecutionBlock{Code: "batch_paused", Message: "批次已暂停"}
		case v.ExecutionGroupNo == nil:
			v.ExecutionBlock = &model.TaskExecutionBlock{Code: "waiting_group", Message: "等待安排执行组"}
		}
	}
}
