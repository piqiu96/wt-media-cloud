package repository

import (
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"reflect"
	"time"
)

func currentAttempt(tx *gorm.DB, task uint64, user int64, attempt uint64) (model.Task, model.Attempt, error) {
	v, e := taskSnapshot(tx, task)
	if e != nil {
		return v, model.Attempt{}, e
	}
	if v.OperatorID != user || v.CurrentAttemptID == nil || *v.CurrentAttemptID != attempt {
		return v, model.Attempt{}, ErrNotFound
	}
	var a model.Attempt
	e = tx.Session(&gorm.Session{}).Where("id=? AND publication_id=? AND executor_user_id=?", attempt, task, user).First(&a).Error
	return v, a, missing(e)
}
func feedback(v model.Task, a model.Attempt) model.ExecutionFeedback {
	reason := ""
	if a.ErrorCode != nil && *a.ErrorCode == model.ExecutionInputChanged {
		reason = *a.ErrorCode
	}
	return model.ExecutionFeedback{SceneParked: a.Status == model.AttemptStatusAwaitingHuman && a.ReliablyParkedAt != nil && a.AutomaticSlot == nil, StopReason: reason, TaskStatus: v.Status, AttemptStatus: a.Status, AttemptID: a.ID, Seq: a.ReportSeq, StopRequested: reason != "" || v.CancelRequestedAt != nil || v.AuthorizedAt == nil || v.Status != model.StatusRunning && v.Status != model.StatusAwaitingHuman, ResultHash: a.ResultHash}
}

type reportStep struct {
	Seq    uint64                `json:"seq"`
	Report model.ExecutionReport `json:"report"`
	At     time.Time             `json:"at"`
}

func verified(a model.Attempt, r model.ExecutionReport) bool {
	return r.PlatformUID == a.Input.PlatformUID && r.BitProfileID == a.Input.BitProfileID && r.VideoSHA256 == a.Input.VideoSHA256 && r.CoverSHA256 == a.Input.CoverSHA256 && r.PortraitCoverSHA256 == a.Input.PortraitCoverSHA256
}
func (s ExecutionStore) Report(candidate model.Task, user int64, r model.ExecutionReport, currentSession bool, now time.Time) (model.ExecutionFeedback, error) {
	var out model.ExecutionFeedback
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, e := lockTaskBatch(tx, candidate)
		if e != nil {
			return e
		}
		if r.Stage != "" {
			if e = lockPublishUse(tx, r.AttemptID); e != nil {
				return e
			}
		}
		if r.SceneParked && (r.Stage != model.AttemptStageAwaitingHuman || !currentSession) {
			return ErrConflict
		}
		if r.Stage == "" {
			// No stage means keepalive only. It never revalidates resources,
			// appends history, changes task revision, or grants submission.
			if r.PlatformUID != "" || r.BitProfileID != "" || r.VideoSHA256 != "" || r.CoverSHA256 != "" || r.PortraitCoverSHA256 != "" {
				return ErrConflict
			}
			v, a, e := currentAttempt(tx, candidate.ID, user, r.AttemptID)
			if e != nil {
				return e
			}
			if r.Seq == 0 || r.Seq < a.ReportSeq || a.ResolvedAt != nil {
				return ErrConflict
			}
			if a.Status == model.AttemptStatusRunning || a.Status == model.AttemptStatusAwaitingHuman {
				if e := updateAttempt(tx, a, map[string]any{"heartbeat_at": now, "report_seq": r.Seq}); e != nil {
					return e
				}
				a.ReportSeq = r.Seq
			}
			out = parentFeedback(v, a, parent)
			return nil
		}
		if !currentSession && r.Stage != model.AttemptStageAwaitingHuman {
			return ErrConflict
		}
		v, a, e := currentAttempt(tx, candidate.ID, user, r.AttemptID)
		if e != nil {
			return e
		}
		if a.ResolvedAt != nil || a.Status != model.AttemptStatusRunning && a.Status != model.AttemptStatusAwaitingHuman || r.Seq == 0 || r.Seq < a.ReportSeq {
			return ErrConflict
		}
		var steps []reportStep
		if e := json.Unmarshal(a.Steps, &steps); e != nil {
			return e
		}
		if r.Seq == a.ReportSeq {
			if len(steps) == 0 || steps[len(steps)-1].Report != r {
				return ErrConflict
			}
			out = parentFeedback(v, a, parent)
			return nil // An old permission is never returned again.
		}
		switch r.Stage {
		case model.AttemptStagePreparing:
			if a.Stage != model.AttemptStageClaimed && a.Stage != model.AttemptStagePreparing {
				return ErrConflict
			}
		case model.AttemptStageReady:
			if a.Stage != model.AttemptStageClaimed && a.Stage != model.AttemptStagePreparing && a.Stage != model.AttemptStageReady || !verified(a, r) {
				return ErrConflict
			}
		case model.AttemptStageSubmitting:
			if a.Stage != model.AttemptStageReady || !verified(a, r) {
				return ErrConflict
			}
		case model.AttemptStageAwaitingHuman:
			if a.Stage != model.AttemptStageClaimed && a.Stage != model.AttemptStagePreparing && a.Stage != model.AttemptStageReady && a.Stage != model.AttemptStageAwaitingHuman {
				return ErrConflict
			}
			if a.SubmitStartedAt != nil {
				return ErrConflict
			}
		default:
			return ErrConflict
		}
		if r.Stage == model.AttemptStageReady || r.Stage == model.AttemptStageSubmitting || r.Stage == model.AttemptStagePreparing {
			input, err := s.readInput(tx, v)
			if err != nil && !errors.Is(err, ErrInputChanged) {
				return err
			}
			if errors.Is(err, ErrInputChanged) || !reflect.DeepEqual(input, a.Input) || (a.ErrorCode != nil && *a.ErrorCode == model.ExecutionInputChanged) {
				reason := model.ExecutionInputChanged
				// Preserve the stage and submission barrier. Only the executor
				// can prove no submission and a safely closed browser scene.
				if err := updateTask(tx, v, map[string]any{"updated_at": now}); err != nil {
					return err
				}
				if err := updateAttempt(tx, a, map[string]any{"error_code": reason, "report_seq": r.Seq, "heartbeat_at": now}); err != nil {
					return err
				}
				a.ErrorCode = &reason
				a.ReportSeq = r.Seq
				out = parentFeedback(v, a, parent)
				return nil
			}
		}
		if r.Stage == model.AttemptStageSubmitting && s.RequireAdmission && !stoppedParent(parent) {
			if !a.AdmissionV2 {
				out = parentFeedback(v, a, parent)
				out.StopRequested, out.StopReason = true, "PUBLICATION_AGENT_UPGRADE_REQUIRED"
				return nil
			}
			if e = admissionOpen(tx); e != nil {
				if !errors.Is(e, ErrAdmissionClosed) {
					return e
				}
				out = parentFeedback(v, a, parent)
				out.StopRequested, out.StopReason = true, "PUBLICATION_ADMISSION_CLOSED"
				return nil
			}
		}
		allowed := r.Stage == model.AttemptStageSubmitting && !stoppedParent(parent) && v.Status == model.StatusRunning && v.AuthorizedAt != nil && v.CancelRequestedAt == nil && (v.NotBefore == nil || !v.NotBefore.After(now))
		previousStage := a.Stage
		steps = append(steps, reportStep{Seq: r.Seq, Report: r, At: now})
		a.Stage = r.Stage
		a.ReportSeq = r.Seq
		a.HeartbeatAt = now
		updates := map[string]any{"stage": r.Stage, "report_seq": r.Seq, "heartbeat_at": now, "steps_json": mustJSON(steps)}
		if allowed {
			a.SubmitStartedAt = &now
			updates["submit_started_at"] = now
		}
		taskUpdates := map[string]any{"updated_at": now}
		awaitingHuman := r.Stage == model.AttemptStageAwaitingHuman && v.Status == model.StatusRunning
		if awaitingHuman {
			updates["paused_stage"] = previousStage
			updates["last_resume_revision"] = nil
			a.Status = model.AttemptStatusAwaitingHuman
			v.Status = model.StatusAwaitingHuman
			updates["status"] = a.Status
			taskUpdates["status"] = v.Status
		}
		if r.SceneParked && a.ReliablyParkedAt == nil {
			if a.PausedStage == nil && !awaitingHuman {
				return ErrConflict
			}
			updates["automatic_slot"], updates["reliably_parked_at"] = nil, now
			a.AutomaticSlot, a.ReliablyParkedAt = nil, &now
		}

		// Bind permission to the task authorization/version as well as the
		// report version, so concurrent cancellation cannot be overwritten.
		if e := updateTask(tx, v, taskUpdates); e != nil {
			return e
		}
		if e := updateAttempt(tx, a, updates); e != nil {
			return e
		}
		v.Revision++
		if awaitingHuman {
			if e := audit(tx, user, v.ID, "publication.awaiting_human", v.Revision); e != nil {
				return e
			}
		}
		out = parentFeedback(v, a, parent)
		out.AllowSubmit = allowed
		return advanceBatch(tx, parent, now)
	})
	return out, e
}
func (s ExecutionStore) Complete(candidate model.Task, user int64, r model.Completion, now time.Time) (model.ExecutionFeedback, error) {
	var out model.ExecutionFeedback
	result, e := model.ParseResult(r.Result)
	if e != nil {
		return out, ErrConflict
	}
	hash, e := model.CompletionHash(r.Seq, r.Result)
	if e != nil || hash != r.ResultHash {
		return out, ErrConflict
	}
	e = currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, e := lockTaskBatch(tx, candidate)
		if e != nil {
			return e
		}
		if e = lockPublishUse(tx, r.AttemptID); e != nil {
			return e
		}
		v, a, e := currentAttempt(tx, candidate.ID, user, r.AttemptID)
		if e != nil {
			return e
		}
		if a.ResultHash != nil {
			if *a.ResultHash != r.ResultHash || a.ReportSeq != r.Seq {
				return ErrConflict
			}
			out = parentFeedback(v, a, parent)
			return nil
		}
		if r.Seq == 0 || r.Seq <= a.ReportSeq || a.ResolvedAt != nil || a.Status == model.AttemptStatusCompleted {
			return ErrConflict
		}
		status := result.Status
		attemptStatus := model.AttemptStatusCompleted
		switch status {
		case model.StatusReviewing:
			ev := result.Evidence
			if a.SubmitStartedAt == nil || result.PlatformWorkID == "" || ev.ReceiptID != result.PlatformWorkID || !ev.SceneClosed || ev.PlatformUID != a.Input.PlatformUID || ev.BitProfileID != a.Input.BitProfileID || ev.VideoSHA256 != a.Input.VideoSHA256 || ev.CoverSHA256 != a.Input.CoverSHA256 || ev.PortraitCoverSHA256 != a.Input.PortraitCoverSHA256 {
				return ErrConflict
			}
		case model.StatusFailed:
			ev := result.Evidence
			if a.SubmitStartedAt != nil || !ev.NotSubmitted || !ev.SceneClosed || result.ErrorCode == "" || ev.PlatformUID != a.Input.PlatformUID || ev.BitProfileID != a.Input.BitProfileID || ev.VideoSHA256 != a.Input.VideoSHA256 || ev.CoverSHA256 != a.Input.CoverSHA256 || ev.PortraitCoverSHA256 != a.Input.PortraitCoverSHA256 {
				return ErrConflict
			}
		case model.StatusUncertain:
			attemptStatus = model.AttemptStatusInterrupted
		default:
			return ErrConflict
		}
		// Keep the complete original observation on the immutable attempt; task
		// links may later be corrected by an explicit verification.
		evidence := r.Result
		updates := map[string]any{"status": attemptStatus, "stage": model.AttemptStageFinished, "report_seq": r.Seq, "heartbeat_at": now, "finished_at": now, "result_status": status, "result_hash": r.ResultHash, "evidence_json": string(evidence), "error_code": optional(result.ErrorCode), "error_message": optional(result.ErrorMessage)}
		taskUpdates := map[string]any{"status": status, "updated_at": now, "platform_submission_id": optional(result.PlatformWorkID), "target_url": optional(result.WorkURL), "failure_code": optional(result.ErrorCode), "failure_message": optional(result.ErrorMessage)}
		taskStatus := status
		if status == model.StatusFailed && v.CancelRequestedAt != nil {
			// The failed branch above has verified the original, closed,
			// unsubmitted scene. Keep that observation on the immutable attempt.
			taskStatus = model.StatusCancelled
			taskUpdates["status"] = taskStatus
			taskUpdates["failure_code"] = nil
			taskUpdates["failure_message"] = nil
		}
		if status == model.StatusReviewing {
			taskUpdates["accepted_at"] = now
		}
		if e := updateTask(tx, v, taskUpdates); e != nil {
			return e
		}
		if e := updateAttempt(tx, a, updates); e != nil {
			return e
		}
		if e := audit(tx, user, v.ID, "publication.result", v.Revision+1); e != nil {
			return e
		}
		if attemptStatus == model.AttemptStatusCompleted {
			if e = releasePublishUse(tx, a.ID, now); e != nil {
				return e
			}
		}
		v.Status = taskStatus
		v.Revision++
		a.Status = attemptStatus
		a.ReportSeq = r.Seq
		a.ResultHash = &r.ResultHash
		out = parentFeedback(v, a, parent)
		return advanceBatch(tx, parent, now)
	})
	return out, e
}

func parentFeedback(v model.Task, a model.Attempt, b *model.Batch) model.ExecutionFeedback {
	out := feedback(v, a)
	if stoppedParent(b) {
		out.StopRequested = true
		if out.StopReason == "" {
			out.StopReason = "PUBLICATION_BATCH_STOPPED"
		}
	}
	return out
}

func optional(v string) any {
	if v == "" {
		return nil
	}
	return v
}
