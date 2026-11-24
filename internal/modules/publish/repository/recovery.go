package repository

import (
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"time"
)

// Disposition refers to this original attempt, independent of the task's next
// attempt. It is never a permit to reopen the old scene or click again.
func safeDisposition(a model.Attempt) model.Status {
	if a.ResolvedAt != nil && a.ResolutionNote != nil {
		var v model.ResultResolution
		if json.Unmarshal([]byte(*a.ResolutionNote), &v) == nil && v.Valid() && v.AttemptID == a.ID && (v.Conclusion == model.StatusFailed || v.Conclusion == model.StatusReviewing || v.Conclusion == model.StatusPublished) && v.Evidence.PlatformUID == a.Input.PlatformUID {
			return v.Conclusion
		}
	}
	if a.Status != model.AttemptStatusCompleted || a.ResultHash == nil {
		return ""
	}
	r, e := model.ParseResult(a.Evidence)
	if e != nil {
		return ""
	}
	ev := r.Evidence
	if !ev.SceneClosed || ev.PlatformUID != a.Input.PlatformUID || ev.BitProfileID != a.Input.BitProfileID || ev.VideoSHA256 != a.Input.VideoSHA256 || ev.CoverSHA256 != a.Input.CoverSHA256 || ev.PortraitCoverSHA256 != a.Input.PortraitCoverSHA256 {
		return ""
	}
	if r.Status == model.StatusFailed && ev.NotSubmitted && a.SubmitStartedAt == nil {
		return model.StatusFailed
	}
	if r.Status == model.StatusReviewing && a.SubmitStartedAt != nil && r.PlatformWorkID != "" && ev.ReceiptID == r.PlatformWorkID {
		return model.StatusReviewing
	}
	return ""
}
func (s ExecutionStore) OriginalAttempt(task, attempt uint64, user int64) (model.OriginalAttemptState, error) {
	var a model.Attempt
	e := s.DB.Where("id=? AND publication_id=? AND executor_user_id=?", attempt, task, user).First(&a).Error
	if e != nil {
		return model.OriginalAttemptState{}, missing(e)
	}
	status := safeDisposition(a)
	return model.OriginalAttemptState{AttemptID: a.ID, ResultHash: a.ResultHash, SceneResolved: status != "", ResolvedStatus: status}, nil
}
func (s ExecutionStore) ResolveResult(candidate model.Task, actor int64, in model.ResultResolution, now time.Time) (model.Task, error) {
	if !in.Valid() {
		return model.Task{}, ErrConflict
	}
	var out model.Task
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, e := lockTaskBatch(tx, candidate)
		if e != nil {
			return e
		}
		v, e := taskSnapshot(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(v, candidate) || v.Revision != in.ExpectedRevision || (v.Status != model.StatusUncertain && !(v.Status == model.StatusReviewing && in.Conclusion == model.StatusPublished)) || v.CurrentAttemptID == nil || *v.CurrentAttemptID != in.AttemptID {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		var a model.Attempt
		if e := tx.First(&a, in.AttemptID).Error; e != nil {
			return e
		}
		if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || in.Evidence.PlatformUID != a.Input.PlatformUID || a.Input.PlatformUID != v.PlatformAccountUID || (v.Status == model.StatusUncertain && (a.ResolvedAt != nil || a.Status != model.AttemptStatusInterrupted)) || (v.Status == model.StatusReviewing && safeDisposition(a) != model.StatusReviewing) {
			return ErrConflict
		}
		if v.Platform != "baidu" {
			return ErrConflict
		}
		if v.PlatformSubmissionID != nil && *v.PlatformSubmissionID != "" && (in.Conclusion == model.StatusReviewing || in.Conclusion == model.StatusPublished) && *v.PlatformSubmissionID != in.PlatformSubmissionID {
			return ErrConflict
		}
		changes := map[string]any{"resolution_note": mustJSON(in)}
		updates := map[string]any{"updated_at": now}
		if in.Conclusion != model.StatusUncertain {
			changes["resolved_at"] = now
			changes["resolved_by"] = actor
			updates["status"] = in.Conclusion
			updates["failure_code"] = nil
			updates["failure_message"] = nil
		}
		if in.Conclusion == model.StatusFailed {
			updates["failure_code"] = "VERIFIED_NOT_PUBLISHED"
			updates["failure_message"] = in.Evidence.Reason
		}
		if in.Conclusion == model.StatusReviewing || in.Conclusion == model.StatusPublished {
			updates["platform_submission_id"] = in.PlatformSubmissionID
			if in.TargetURL != "" {
				updates["target_url"] = in.TargetURL
			}
			if v.AcceptedAt == nil {
				updates["accepted_at"] = now
			}
		}
		if in.Conclusion == model.StatusPublished {
			updates["next_check_at"] = nil
			updates["check_stopped_at"] = now
			updates["last_checked_at"] = now
			updates["last_check_message"] = in.Evidence.Reason
			updates["platform_work_id"] = in.PlatformWorkID
			updates["work_url"] = in.WorkURL
			updates["published_at"] = in.PublishedAt
		}
		if e := updateTask(tx, v, updates); e != nil {
			return e
		}
		if in.Conclusion == model.StatusPublished {
			if e := s.recordVerifiedWork(tx, v, a, in, now); e != nil {
				return e
			}
		}
		if e := updateAttempt(tx, a, changes); e != nil {
			return e
		}
		if in.Conclusion != model.StatusUncertain {
			if e := releasePublishUse(tx, a.ID, now); e != nil {
				return e
			}
		}
		if e := auditDetails(tx, actor, v.ID, "publication.resolve", map[string]any{"revision": v.Revision + 1, "verification": in}); e != nil {
			return e
		}
		if e = advanceBatch(tx, parent, now); e != nil {
			return e
		}
		return tx.First(&out, v.ID).Error
	})
	return out, e
}
func (s ExecutionStore) Retry(candidate model.Task, revision uint64, actor int64, now time.Time) (model.Task, error) {
	var out model.Task
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, e := lockTaskBatch(tx, candidate)
		if e != nil {
			return e
		}
		if parent != nil && (parent.ExecutionStatus != model.BatchRunning || stoppedParent(parent) ||
			parent.ExecutionGroupNo != nil && candidate.ExecutionGroupNo != nil && *parent.ExecutionGroupNo == *candidate.ExecutionGroupNo && parent.GroupEndedAt == nil) {
			return ErrParentBlocked
		}
		if s.RequireAdmission {
			if e = admissionOpen(tx); e != nil {
				return e
			}
		}
		if _, e := s.readInput(tx, candidate); e != nil {
			return e
		}
		v, e := taskSnapshot(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(v, candidate) || v.Revision != revision || v.Status != model.StatusFailed || v.CancelRequestedAt != nil || v.CurrentAttemptID == nil || v.AuthorizedAt == nil {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		var a model.Attempt
		if e := tx.First(&a, *v.CurrentAttemptID).Error; e != nil {
			return e
		}
		if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || safeDisposition(a) != model.StatusFailed {
			return ErrConflict
		}
		// Keep the original immutable result and occupancy. Only the next claim
		// creates a new attempt; the old attempt stays available for exact read-back.
		status := model.StatusQueued
		if parent != nil || v.NotBefore != nil && v.NotBefore.After(now) {
			status = model.StatusPending
		}
		if e := updateTask(tx, v, map[string]any{"status": status, "current_attempt_id": nil, "authorized_at": now, "updated_at": now, "failure_code": nil, "failure_message": nil, "execution_group_no": nil}); e != nil {
			return e
		}
		if e := audit(tx, actor, v.ID, "publication.retry", v.Revision+1); e != nil {
			return e
		}
		return tx.First(&out, v.ID).Error
	})
	return out, e
}
