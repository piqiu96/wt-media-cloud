package repository

import (
	"reflect"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
)

func (s ExecutionStore) humanScene(tx *gorm.DB, candidate model.Task, user int64, in model.HumanContinuation, replay bool) (model.Task, model.Attempt, error) {
	v, a, err := currentAttempt(tx, candidate.ID, user, in.AttemptID)
	if err != nil {
		return v, a, err
	}
	if !in.Valid() || v.AuthorizedAt == nil || v.CancelRequestedAt != nil || a.SubmitStartedAt != nil || a.ResultHash != nil || a.ResolvedAt != nil || a.ErrorCode != nil || a.PausedStage == nil {
		return v, a, ErrConflict
	}
	switch *a.PausedStage {
	case model.AttemptStageClaimed, model.AttemptStagePreparing, model.AttemptStageReady:
	default:
		return v, a, ErrConflict
	}
	waiting := v.Status == model.StatusAwaitingHuman && a.Status == model.AttemptStatusAwaitingHuman && a.Stage == model.AttemptStageAwaitingHuman && v.Revision == in.ExpectedRevision
	confirmed := replay && v.Status == model.StatusRunning && a.Status == model.AttemptStatusRunning && a.LastResumeRevision != nil && *a.LastResumeRevision == in.ExpectedRevision && v.Revision == in.ExpectedRevision+1 && a.Stage == *a.PausedStage
	if !waiting && !confirmed {
		return v, a, ErrConflict
	}
	input, err := s.readInput(tx, v)
	if err != nil {
		return v, a, err
	}
	if !reflect.DeepEqual(input, a.Input) {
		return v, a, ErrInputChanged
	}
	return v, a, nil
}

// The caller must also hold the still-live local scene. Cloud never opens one.
func (s ExecutionStore) GetHumanScene(candidate model.Task, user int64, in model.HumanContinuation) (model.ExecutionFeedback, error) {
	var out model.ExecutionFeedback
	err := currentTransaction(s.DB, func(tx *gorm.DB) error {
		b, err := lockTaskBatch(tx, candidate)
		if err != nil {
			return err
		}
		if stoppedParent(b) {
			return ErrParentBlocked
		}
		v, a, err := s.humanScene(tx, candidate, user, in, false)
		if err != nil {
			return err
		}
		out = feedback(v, a)
		return nil
	})
	return out, err
}
func (s ExecutionStore) Resume(candidate model.Task, user int64, in model.HumanContinuation, now time.Time) (model.ExecutionFeedback, error) {
	if s.RequireAdmission {
		return model.ExecutionFeedback{}, ErrAgentUpgrade
	}
	return s.ResumeWithCapacity(candidate, user, in, model.ExecutionCapacity{AutomaticSlots: 3, WindowSlots: 3}, now)
}
func (s ExecutionStore) ResumeWithCapacity(candidate model.Task, user int64, in model.HumanContinuation, agent model.ExecutionCapacity, now time.Time) (model.ExecutionFeedback, error) {
	var out model.ExecutionFeedback
	if !agent.Valid() {
		return out, ErrAgentUpgrade
	}
	limit := s.localCapacity(agent)
	err := pureTransaction(s.DB, func(tx *gorm.DB) error {
		parent, err := lockTaskBatch(tx, candidate)
		if err != nil {
			return err
		}
		if stoppedParent(parent) || parent != nil && parent.ExecutionStatus != model.BatchRunning {
			return ErrParentBlocked
		}
		if err = admissionOpen(tx); err != nil {
			return err
		}
		if err = lockPublishUse(tx, in.AttemptID); err != nil {
			return err
		}
		v, a, err := s.humanScene(tx, candidate, user, in, true)
		if err != nil {
			return err
		}
		if v.Status == model.StatusRunning {
			out = feedback(v, a)
			return nil
		}
		slot := a.AutomaticSlot
		if slot == nil {
			available, e := batchAutomaticAvailable(tx, parent)
			if e != nil {
				return e
			}
			if !available {
				return ErrCapacityWait
			}
			free, e := freeAutomaticSlot(tx, user, limit.AutomaticSlots)
			if e != nil {
				return e
			}
			if free == 0 {
				return ErrCapacityWait
			}
			slot = &free
		}

		if err = updateTask(tx, v, map[string]any{"status": model.StatusRunning, "updated_at": now}); err != nil {
			return err
		}
		if err = updateAttempt(tx, a, map[string]any{"status": model.AttemptStatusRunning, "stage": *a.PausedStage, "last_resume_revision": in.ExpectedRevision, "heartbeat_at": now, "automatic_slot": slot, "reliably_parked_at": nil, "admission_v2": true}); err != nil {
			return err
		}
		if err = audit(tx, user, v.ID, "publication.resume", v.Revision+1); err != nil {
			return err
		}
		v.Status = model.StatusRunning
		a.Status = model.AttemptStatusRunning
		a.Stage = *a.PausedStage
		a.AutomaticSlot, a.ReliablyParkedAt = slot, nil
		out = feedback(v, a)
		return nil
	})
	return out, err
}
