package repository

import (
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"time"
)

// Cancel ends unclaimed tasks immediately. For a running original attempt it
// only fences submission; Complete needs affirmative safe-close evidence before
// projecting cancelled. Missing submit time alone never proves a safe scene.
func (s ExecutionStore) Cancel(candidate model.Task, in model.Cancellation, actor int64, now time.Time) (model.Task, error) {
	var out model.Task
	err := currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, err := lockTaskBatch(tx, candidate)
		if err != nil {
			return err
		}
		if stoppedParent(parent) {
			return ErrParentBlocked
		}
		v, err := taskSnapshot(tx, candidate.ID)
		if err != nil {
			return err
		}
		if !in.Valid() || !sameCandidate(v, candidate) || v.Revision != in.ExpectedRevision || v.CancelRequestedAt != nil {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		status := model.StatusCancelled
		basis := map[string]any{"status": v.Status, "unclaimed": v.CurrentAttemptID == nil}
		if v.CurrentAttemptID == nil {
			if v.Status != model.StatusPending && v.Status != model.StatusQueued {
				return ErrConflict
			}
		} else {
			if v.Status != model.StatusRunning {
				return ErrConflict
			}
			var a model.Attempt
			if err := tx.First(&a, *v.CurrentAttemptID).Error; err != nil {
				return err
			}
			if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || a.Status != model.AttemptStatusRunning || a.SubmitStartedAt != nil || a.ResultHash != nil || a.ResolvedAt != nil {
				return ErrConflict
			}
			status = model.StatusRunning
			basis["attempt_id"] = fmt.Sprint(a.ID)
			basis["submission_permission_issued"] = false
		}
		if err := updateTask(tx, v, map[string]any{"status": status, "cancel_requested_at": now, "updated_at": now}); err != nil {
			return err
		}
		if err := auditDetails(tx, actor, v.ID, "publication.cancel", map[string]any{"revision": v.Revision + 1, "reason": in.Reason, "basis": basis}); err != nil {
			return err
		}
		if err = advanceBatch(tx, parent, now); err != nil {
			return err
		}
		return tx.First(&out, v.ID).Error
	})
	return out, err
}

func (s ExecutionStore) Abandon(candidate model.Task, actor int64, in model.Abandonment, now time.Time) (model.Task, error) {
	if !in.Valid() {
		return model.Task{}, ErrConflict
	}
	var out model.Task
	err := currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, err := lockTaskBatch(tx, candidate)
		if err != nil {
			return err
		}
		v, err := taskSnapshot(tx, candidate.ID)
		if err != nil {
			return err
		}
		if !sameCandidate(v, candidate) || v.Revision != in.ExpectedRevision || v.Status != model.StatusFailed || v.CurrentAttemptID == nil || *v.CurrentAttemptID != in.AttemptID || in.Evidence.PlatformUID != v.PlatformAccountUID {
			return &ConflictError{TaskID: v.ID, Revision: v.Revision}
		}
		var a model.Attempt
		if err := tx.First(&a, in.AttemptID).Error; err != nil {
			return err
		}
		if a.PublicationID != v.ID || a.ExecutorUserID != v.OperatorID || a.Input.PlatformUID != in.Evidence.PlatformUID || safeDisposition(a) != model.StatusFailed {
			return ErrConflict
		}
		if err := updateTask(tx, v, map[string]any{"status": model.StatusAbandoned, "updated_at": now}); err != nil {
			return err
		}
		if err := auditDetails(tx, actor, v.ID, "publication.abandon", map[string]any{"revision": v.Revision + 1, "verification": in}); err != nil {
			return err
		}
		if err = advanceBatch(tx, parent, now); err != nil {
			return err
		}
		return tx.First(&out, v.ID).Error
	})
	return out, err
}
