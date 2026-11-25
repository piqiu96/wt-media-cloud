package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
)

type BatchCommand struct {
	Action           string
	ExpectedRevision uint64
	Reason           string
	RequestKey       string
	Digest           string
	ExpectedPauseID  uint64
	Tasks            []model.TaskRevision
}

func batchTasks(tx *gorm.DB, id uint64) ([]model.Task, error) {
	rows := []model.Task{}
	err := tx.Where("batch_id=?", id).Order("task_order,id").Find(&rows).Error
	return rows, err
}
func batchExceptions(tx *gorm.DB, rows []model.Task) ([]model.BatchException, map[string]string, error) {
	out := []model.BatchException{}
	basis := map[string]string{}
	for _, row := range rows {
		switch row.Status {
		case model.StatusFailed, model.StatusAwaitingHuman, model.StatusUncertain:
			code := ""
			if row.FailureCode != nil {
				code = *row.FailureCode
			}
			var a model.Attempt
			if row.CurrentAttemptID != nil {
				if err := tx.First(&a, *row.CurrentAttemptID).Error; err != nil {
					return nil, nil, err
				}
				if code == "" && a.ErrorCode != nil {
					code = *a.ErrorCode
				}
			}
			out = append(out, model.BatchException{TaskRevision: model.TaskRevision{TaskID: row.ID, Revision: row.Revision}, Status: row.Status, Code: code})
			raw := mustJSON([]any{row.Status, code, row.FailureMessage, a.ID, a.ResultHash, a.ResolutionNote})
			sum := sha256.Sum256([]byte(raw))
			basis[strconv.FormatUint(row.ID, 10)] = hex.EncodeToString(sum[:])
		}
	}
	return out, basis, nil
}

func projectBatchExecution(tx *gorm.DB, v model.Batch, rows []model.Task, now time.Time) (model.BatchExecution, error) {
	out := model.BatchExecution{
		Phase: v.ExecutionStatus, FirstStartedAt: v.StartedAt, GroupNo: v.ExecutionGroupNo, TaskIDs: []string{},
		GroupEndedAt: v.GroupEndedAt, NextGroupAt: v.NextGroupAt, Pause: v.Pause, StoppedAt: v.StoppedAt, EndedAt: v.EndedAt,
		StopReason: v.StopReason, FinishReason: v.FinishReason, TaskCounts: map[model.Status]int64{},
	}
	for _, target := range v.Targets {
		out.TargetCount += target.TargetCount
	}
	out.CreatedCount = int64(len(rows))
	for _, row := range rows {
		out.TaskCounts[row.Status]++
		if row.Status == model.StatusPublished && row.WorkID != nil {
			out.PublishedCount++
		}
		if row.ExecutionGroupNo != nil && v.ExecutionGroupNo != nil && *row.ExecutionGroupNo == *v.ExecutionGroupNo {
			out.TaskIDs = append(out.TaskIDs, strconv.FormatUint(row.ID, 10))
		}
	}
	if v.StartedAt != nil {
		shortfall := max(0, out.TargetCount-out.CreatedCount)
		out.CreationShortfall = &shortfall
	}
	out.TargetShortfall = max(0, out.TargetCount-out.PublishedCount)
	if v.ExecutionStatus == model.BatchCancelled {
		out.Phase = "ended"
	}
	if v.ExecutionStatus == model.BatchRunning {
		switch {
		case v.NextGroupAt != nil && now.Before(*v.NextGroupAt):
			out.Phase = "waiting_interval"
		case out.TaskCounts[model.StatusPending]+out.TaskCounts[model.StatusQueued]+out.TaskCounts[model.StatusRunning] == 0:
			out.Phase = "awaiting_processing"
		case out.TaskCounts[model.StatusRunning]+out.TaskCounts[model.StatusQueued] == 0:
			out.Phase = "waiting_resources"
		}
	}
	if v.Pause != nil && v.ExecutionStatus == model.BatchPaused {
		pause := *v.Pause
		var err error
		pause.Exceptions, _, err = batchExceptions(tx, rows)
		if err != nil {
			return out, err
		}
		out.Pause = &pause
	}
	return out, nil
}

func (s Store) enrichBatch(v model.Batch) (model.Batch, error) {
	if v.PlanID != nil {
		var p model.Plan
		if e := s.DB.First(&p, *v.PlanID).Error; e != nil {
			return v, e
		}
		v = model.ProjectPlanBatch(p, v)
	}
	rows, err := batchTasks(s.DB, v.ID)
	if err != nil {
		return v, err
	}
	v.Execution, err = projectBatchExecution(s.DB, v, rows, time.Now().UTC())
	return v, err
}

type batchResumeReceipt struct {
	RequestKey string                `json:"request_key"`
	Digest     string                `json:"digest"`
	Result     model.BatchStopResult `json:"result"`
}

func findBatchResume(tx *gorm.DB, v model.Batch, cmd BatchCommand) (*model.BatchStopResult, error) {
	var raw struct{ SummaryJSON string }
	err := tx.Table("audit_logs").Select("summary_json").Where("target_type='publication_batch' AND target_id=? AND action='publication_batch.resume' AND JSON_UNQUOTE(JSON_EXTRACT(summary_json,'$.request_key'))=?", strconv.FormatUint(v.ID, 10), cmd.RequestKey).Take(&raw).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var receipt batchResumeReceipt
	if err = json.Unmarshal([]byte(raw.SummaryJSON), &receipt); err != nil {
		return nil, err
	}
	if cmd.Digest != receipt.Digest {
		return nil, BatchConflict(v, ErrConflict)
	}
	return &receipt.Result, nil
}

func matchingTaskRevisions(expected, actual []model.TaskRevision) bool {
	if len(expected) != len(actual) {
		return false
	}
	revisions := make(map[uint64]uint64, len(actual))
	for _, row := range actual {
		revisions[row.TaskID] = row.Revision
	}
	for _, row := range expected {
		if row.Revision == 0 || revisions[row.TaskID] != row.Revision {
			return false
		}
		delete(revisions, row.TaskID)
	}
	return len(revisions) == 0
}

func (s ExecutionStore) ControlBatch(id uint64, actor int64, cmd BatchCommand, now time.Time) (model.BatchStopResult, error) {
	var out model.BatchStopResult
	now = now.UTC().Truncate(time.Microsecond)
	err := currentTransaction(s.DB, func(tx *gorm.DB) error {
		v, err := lockBatch(tx, id)
		if err != nil {
			return err
		}
		if v.OperatorID != actor {
			return ErrNotFound
		}
		if cmd.Action == "resume" {
			prior, e := findBatchResume(tx, v, cmd)
			if e != nil {
				return e
			}
			if prior != nil {
				out = *prior
				return nil
			}
		}
		if v.Revision != cmd.ExpectedRevision {
			return BatchConflict(v, ErrConflict)
		}
		rows, err := batchTasks(tx, id)
		if err != nil {
			return err
		}
		patch := map[string]any{"revision": v.Revision + 1, "updated_at": now}
		switch cmd.Action {
		case "pause":
			if v.ExecutionStatus != model.BatchRunning || stoppedParent(&v) {
				return BatchConflict(v, ErrParentBlocked)
			}
			exceptions, _, e := batchExceptions(tx, rows)
			if e != nil {
				return e
			}
			setBatchPause(&v, patch, "manual", cmd.Reason, exceptions, now)
		case "resume":
			if v.ExecutionStatus != model.BatchPaused || v.Pause == nil || stoppedParent(&v) {
				return BatchConflict(v, ErrParentBlocked)
			}
			exceptions, basis, e := batchExceptions(tx, rows)
			if e != nil {
				return e
			}
			expected := make([]model.TaskRevision, 0, len(exceptions))
			for _, item := range exceptions {
				expected = append(expected, item.TaskRevision)
			}
			if v.Pause.ID != cmd.ExpectedPauseID || !matchingTaskRevisions(cmd.Tasks, expected) {
				v.Pause.Exceptions = exceptions
				return BatchConflict(v, ErrPauseChanged)
			}
			v.ExecutionStatus, v.Pause, v.ExceptionAck = model.BatchRunning, nil, basis
			patch["pause_json"], patch["exception_ack_json"] = nil, mustJSON(basis)
		case "stop":
			if v.PlanPermanentStopped || v.StoppedAt != nil || v.ExecutionStatus == model.BatchStopping || v.ExecutionStatus == model.BatchCancelled || v.ExecutionStatus == model.BatchEnded {
				return BatchConflict(v, ErrParentBlocked)
			}
			v.StoppedAt, v.StopReason = &now, cmd.Reason
			patch["stopped_at"], patch["stop_reason"] = now, cmd.Reason
			for i, row := range rows {
				if row.CurrentAttemptID == nil && (row.Status == model.StatusPending || row.Status == model.StatusQueued) {
					if err = updateTask(tx, row, map[string]any{"status": model.StatusCancelled, "cancel_requested_at": now, "updated_at": now}); err != nil {
						return err
					}
					rows[i].Status = model.StatusCancelled
					out.CancelledUnclaimedCount++
				} else if row.CurrentAttemptID != nil && (row.Status == model.StatusRunning || row.Status == model.StatusAwaitingHuman) {
					// Stop is an instruction to close safely, never evidence of closure.
					if row.CancelRequestedAt == nil {
						if err = updateTask(tx, row, map[string]any{"cancel_requested_at": now, "updated_at": now}); err != nil {
							return err
						}
					}
				}
				if pendingSettlement(rows[i].Status) {
					out.InflightCount++
				}
			}
			v.ExecutionStatus = model.BatchStopping
			if out.InflightCount == 0 {
				v.ExecutionStatus, v.EndedAt = model.BatchCancelled, &now
				patch["ended_at"] = now
			}
			v.Pause = nil
			patch["pause_json"] = nil
		case "finish":
			if v.StartedAt == nil || stoppedParent(&v) {
				return BatchConflict(v, ErrParentBlocked)
			}
			failed := []model.TaskRevision{}
			for _, row := range rows {
				if pendingSettlement(row.Status) || row.Status == model.StatusPending || row.Status == model.StatusQueued {
					return BatchConflict(v, ErrUnresolved)
				}
				if row.Status == model.StatusFailed {
					failed = append(failed, model.TaskRevision{TaskID: row.ID, Revision: row.Revision})
				}
			}
			if !matchingTaskRevisions(cmd.Tasks, failed) {
				return BatchConflict(v, ErrUnresolved)
			}
			v.ExecutionStatus, v.EndedAt, v.FinishReason, v.Pause = model.BatchEnded, &now, cmd.Reason, nil
			patch["ended_at"], patch["finish_reason"], patch["pause_json"] = now, cmd.Reason, nil
		default:
			return ErrConflict
		}
		v.Revision++
		patch["execution_status"] = v.ExecutionStatus
		if err = tx.Model(&model.Batch{}).Where("id=?", v.ID).Updates(patch).Error; err != nil {
			return err
		}
		if cmd.Action == "resume" {
			if err = advanceBatch(tx, &v, now); err != nil {
				return err
			}
		}
		status, err := projectBatchExecution(tx, v, rows, now)
		if err != nil {
			return err
		}
		out.BatchControlResult = model.BatchControlResult{BatchID: v.ID, Revision: v.Revision, ExecutionStatus: v.ExecutionStatus, Execution: status}
		var details any = map[string]any{"reason": cmd.Reason, "revision": v.Revision, "result": out}
		if cmd.Action == "resume" {
			details = batchResumeReceipt{RequestKey: cmd.RequestKey, Digest: cmd.Digest, Result: out}
		}
		return auditBatchDetails(tx, v, actor, "publication_batch."+cmd.Action, details)
	})
	if err != nil {
		return model.BatchStopResult{}, err
	}
	return out, nil
}

func setBatchPause(v *model.Batch, patch map[string]any, kind, reason string, exceptions []model.BatchException, now time.Time) {
	v.PauseID++
	v.ExecutionStatus = model.BatchPaused
	v.Pause = &model.BatchPause{ID: v.PauseID, Kind: kind, Reason: reason, PausedAt: now, Exceptions: exceptions}
	patch["pause_id"] = v.PauseID
	patch["pause_json"] = mustJSON(v.Pause)
}

func pendingSettlement(status model.Status) bool {
	return status == model.StatusRunning || status == model.StatusAwaitingHuman || status == model.StatusUncertain
}

func groupRowFinished(tx *gorm.DB, t model.Task) (bool, error) {
	switch t.Status {
	case model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusUncertain:
		return false, nil
	case model.StatusAwaitingHuman:
		if t.CurrentAttemptID == nil {
			return false, nil
		}
		var a model.Attempt
		err := tx.First(&a, *t.CurrentAttemptID).Error
		return a.ReliablyParkedAt != nil && a.AutomaticSlot == nil, err
	default:
		return true, nil
	}
}

// Called with the current locked parent after a business task transition.
// Keepalive deliberately uses its own path and never advances this fence.
func advanceBatch(tx *gorm.DB, b *model.Batch, now time.Time) error {
	if b == nil || b.StartedAt == nil || (b.ExecutionStatus != model.BatchRunning && b.ExecutionStatus != model.BatchPaused && b.ExecutionStatus != model.BatchStopping) {
		return nil
	}
	rows, err := batchTasks(tx, b.ID)
	if err != nil {
		return err
	}
	patch := map[string]any{}
	event := ""
	if b.ExecutionStatus == model.BatchStopping {
		if slices.ContainsFunc(rows, func(row model.Task) bool { return pendingSettlement(row.Status) }) {
			return nil
		}
		b.ExecutionStatus, b.EndedAt = model.BatchCancelled, &now
		patch["ended_at"] = now
		event = "publication_batch.stop_settled"
	} else {
		groupEnd := false
		if b.ExecutionGroupNo != nil && b.GroupEndedAt == nil {
			groupEnd = true
			for _, row := range rows {
				if row.ExecutionGroupNo == nil || *row.ExecutionGroupNo != *b.ExecutionGroupNo {
					continue
				}
				finished, e := groupRowFinished(tx, row)
				if e != nil {
					return e
				}
				if !finished {
					groupEnd = false
					break
				}
			}
			if groupEnd {
				next := now.Add(time.Duration(b.ExecutionSetting.GroupIntervalSeconds) * time.Second)
				b.GroupEndedAt, b.NextGroupAt = &now, &next
				patch["group_ended_at"], patch["next_group_at"] = now, next
				event = "publication_batch.group_ended"
			}
		}
		if b.ExecutionStatus == model.BatchRunning {
			exceptions, basis, e := batchExceptions(tx, rows)
			if e != nil {
				return e
			}
			changed, groupChanged := false, false
			for _, item := range exceptions {
				key := strconv.FormatUint(item.TaskID, 10)
				if b.ExceptionAck[key] == basis[key] {
					continue
				}
				changed = true
				if b.ExecutionGroupNo != nil && b.GroupEndedAt != nil {
					for _, row := range rows {
						if row.ID == item.TaskID && row.ExecutionGroupNo != nil && *row.ExecutionGroupNo == *b.ExecutionGroupNo {
							groupChanged = true
							break
						}
					}
				}
			}
			if changed && len(exceptions) >= int(b.ExecutionSetting.ExceptionTaskLimit) {
				setBatchPause(b, patch, "exception_limit", "达到异常任务上限", exceptions, now)
				event = "publication_batch.pause"
			} else if groupChanged && b.ExecutionSetting.BetweenGroupPolicy == "pause_on_exception" {
				setBatchPause(b, patch, "group_exception", "本组存在异常任务", exceptions, now)
				event = "publication_batch.pause"
			} else if !slices.ContainsFunc(rows, func(row model.Task) bool {
				return pendingSettlement(row.Status) || row.Status == model.StatusPending || row.Status == model.StatusQueued || row.Status == model.StatusFailed
			}) {
				b.ExecutionStatus, b.EndedAt = model.BatchEnded, &now
				patch["ended_at"] = now
				event = "publication_batch.ended"
			}
		}
	}
	if len(patch) == 0 {
		return nil
	}
	b.Revision++
	patch["revision"], patch["updated_at"], patch["execution_status"] = b.Revision, now, b.ExecutionStatus
	if err = tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(patch).Error; err != nil {
		return err
	}
	return auditBatchDetails(tx, *b, 0, event, map[string]any{"revision": b.Revision, "execution_group_no": b.ExecutionGroupNo, "pause": b.Pause})
}

type BatchTaskFilter struct {
	Status, Search string
	AccountID      uint64
	Page, PageSize int
}

func (s Store) BatchTaskPage(id uint64, f BatchTaskFilter) ([]model.Task, int64, error) {
	q := s.DB.Model(&model.Task{}).Where("batch_id=?", id)
	if f.Status != "" {
		q = q.Where("status=?", f.Status)
	}
	if f.AccountID != 0 {
		q = q.Where("media_account_id=?", f.AccountID)
	}
	if f.Search != "" {
		text := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(f.Search)
		q = q.Where("(CAST(id AS CHAR)=? OR JSON_UNQUOTE(JSON_EXTRACT(content_json,'$.description')) LIKE ? ESCAPE '!')", f.Search, "%"+text+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Task{}
	err := q.Select(cancellationProjection).Order("task_order,id").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
