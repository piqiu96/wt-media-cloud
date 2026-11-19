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
		var v model.FailureResolution
		if json.Unmarshal([]byte(*a.ResolutionNote), &v) == nil && v.Valid() && v.AttemptID == a.ID && v.Conclusion == model.StatusFailed && v.Evidence.PlatformUID == a.Input.PlatformUID {
			return model.StatusFailed
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
	if !ev.SceneClosed || ev.PlatformUID != a.Input.PlatformUID || ev.BitProfileID != a.Input.BitProfileID || ev.VideoSHA256 != a.Input.VideoSHA256 || ev.CoverSHA256 != a.Input.CoverSHA256 {
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
func (s ExecutionStore) ResolveFailure(candidate model.Task, actor int64, in model.FailureResolution, now time.Time) (model.Task, error) {
	if !in.Valid() {
		return model.Task{}, ErrConflict
	}
	var out model.Task
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		v, e := taskSnapshot(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(v, candidate) || v.Revision != in.ExpectedRevision || v.Status != model.StatusUncertain || v.CurrentAttemptID == nil || *v.CurrentAttemptID != in.AttemptID {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		var a model.Attempt
		if e := tx.First(&a, in.AttemptID).Error; e != nil {
			return e
		}
		if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || a.ResolvedAt != nil || in.Evidence.PlatformUID != a.Input.PlatformUID || a.Status != model.AttemptStatusInterrupted {
			return ErrConflict
		}
		note := mustJSON(in)
		changes := map[string]any{"resolution_note": note}
		if in.Conclusion == model.StatusFailed {
			changes["resolved_at"] = now
			changes["resolved_by"] = actor
		}
		updates := map[string]any{"updated_at": now}
		if in.Conclusion == model.StatusFailed {
			updates["status"] = model.StatusFailed
			updates["failure_code"] = "VERIFIED_NOT_PUBLISHED"
			updates["failure_message"] = in.Evidence.Reason
		}
		if e := updateTask(tx, v, updates); e != nil {
			return e
		}
		if e := updateAttempt(tx, a, changes); e != nil {
			return e
		}
		if e := auditDetails(tx, actor, v.ID, "publication.resolve", map[string]any{"revision": v.Revision + 1, "verification": in}); e != nil {
			return e
		}
		return tx.First(&out, v.ID).Error
	})
	return out, e
}
func (s ExecutionStore) Retry(candidate model.Task, revision uint64, actor int64, now time.Time) (model.Task, error) {
	var out model.Task
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if _, e := s.lockedInput(tx, candidate); e != nil {
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
		if v.NotBefore != nil && v.NotBefore.After(now) {
			status = model.StatusPending
		}
		if e := updateTask(tx, v, map[string]any{"status": status, "current_attempt_id": nil, "authorized_at": now, "updated_at": now, "failure_code": nil, "failure_message": nil}); e != nil {
			return e
		}
		if e := audit(tx, actor, v.ID, "publication.retry", v.Revision+1); e != nil {
			return e
		}
		return tx.First(&out, v.ID).Error
	})
	return out, e
}
