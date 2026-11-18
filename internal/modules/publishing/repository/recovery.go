package repository

import (
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Disposition refers to this original attempt, independent of the task's next
// attempt. It is never a permit to reopen the old scene or click again.
func safeDisposition(a model.Attempt) string {
	if a.ResolvedAt != nil && a.ResolutionNote != nil {
		var v model.FailureResolution
		if json.Unmarshal([]byte(*a.ResolutionNote), &v) == nil && v.Valid() && v.AttemptID == a.ID && v.Conclusion == "failed" && v.Evidence.PlatformUID == a.Input.PlatformUID {
			return "failed"
		}
	}
	if a.Status != "completed" || a.ResultHash == nil {
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
	if r.Status == "failed" && ev.NotSubmitted && a.SubmitStartedAt == nil {
		return "failed"
	}
	if r.Status == "reviewing" && a.SubmitStartedAt != nil && r.PlatformWorkID != "" && ev.ReceiptID == r.PlatformWorkID {
		return "reviewing"
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
	var prior model.Attempt
	if e := s.DB.Where("id=? AND publication_id=?", in.AttemptID, candidate.ID).First(&prior).Error; e != nil {
		return model.Task{}, missing(e)
	}
	var out model.Task
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if s.LockScene == nil {
			return errors.New("publication scene locking unavailable")
		}
		if e := s.LockScene(tx, prior.ProfileID, candidate.OperatorID); e != nil {
			return e
		}
		v, e := lockedTask(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(v, candidate) || v.Revision != in.ExpectedRevision || v.Status != "uncertain" || v.CurrentAttemptID == nil || *v.CurrentAttemptID != in.AttemptID {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		var a model.Attempt
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, in.AttemptID).Error; e != nil {
			return e
		}
		if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || a.ResolvedAt != nil || in.Evidence.PlatformUID != a.Input.PlatformUID || a.Status != "interrupted" {
			return ErrConflict
		}
		note := mustJSON(in)
		changes := map[string]any{"resolution_note": note}
		if in.Conclusion == "failed" {
			changes["resolved_at"] = now
			changes["resolved_by"] = actor
		}
		if e := tx.Model(&a).Updates(changes).Error; e != nil {
			return e
		}
		updates := map[string]any{"revision": v.Revision + 1, "updated_at": now}
		if in.Conclusion == "failed" {
			updates["status"] = "failed"
			updates["failure_code"] = "VERIFIED_NOT_PUBLISHED"
			updates["failure_message"] = in.Evidence.Reason
		}
		if e := tx.Model(&v).Updates(updates).Error; e != nil {
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
		v, e := lockedTask(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(v, candidate) || v.Revision != revision || v.Status != "failed" || v.CancelRequestedAt != nil || v.CurrentAttemptID == nil || v.AuthorizedAt == nil {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		var a model.Attempt
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, *v.CurrentAttemptID).Error; e != nil {
			return e
		}
		if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || safeDisposition(a) != "failed" {
			return ErrConflict
		}
		// Keep the original immutable result and occupancy. Only the next claim
		// creates a new attempt; the old attempt stays available for exact read-back.
		status := "queued"
		if v.NotBefore != nil && v.NotBefore.After(now) {
			status = "pending"
		}
		if e := tx.Model(&v).Updates(map[string]any{"status": status, "current_attempt_id": nil, "authorized_at": now, "revision": v.Revision + 1, "updated_at": now, "failure_code": nil, "failure_message": nil}).Error; e != nil {
			return e
		}
		if e := audit(tx, actor, v.ID, "publication.retry", v.Revision+1); e != nil {
			return e
		}
		return tx.First(&out, v.ID).Error
	})
	return out, e
}
