package repository

import (
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func currentAttempt(tx *gorm.DB, task uint64, user int64, attempt uint64) (model.Task, model.Attempt, error) {
	v, e := lockedTask(tx, task)
	if e != nil {
		return v, model.Attempt{}, e
	}
	if v.OperatorID != user || v.CurrentAttemptID == nil || *v.CurrentAttemptID != attempt {
		return v, model.Attempt{}, ErrNotFound
	}
	var a model.Attempt
	e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND publication_id=? AND executor_user_id=?", attempt, task, user).First(&a).Error
	return v, a, missing(e)
}
func feedback(v model.Task, a model.Attempt) model.ExecutionFeedback {
	return model.ExecutionFeedback{TaskStatus: v.Status, AttemptStatus: a.Status, AttemptID: a.ID, Seq: a.ReportSeq, StopRequested: v.CancelRequestedAt != nil || v.AuthorizedAt == nil || v.Status != "running" && v.Status != "awaiting_human", ResultHash: a.ResultHash}
}
func (s ExecutionStore) Heartbeat(task uint64, user int64, attempt, seq uint64, now time.Time) (model.ExecutionFeedback, error) {
	var out model.ExecutionFeedback
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		v, a, e := currentAttempt(tx, task, user, attempt)
		if e != nil {
			return e
		}
		if seq == 0 || seq < a.ReportSeq || a.ResolvedAt != nil {
			return ErrConflict
		}
		if a.Status == "running" || a.Status == "awaiting_human" {
			if e := tx.Model(&a).Updates(map[string]any{"heartbeat_at": now, "report_seq": seq}).Error; e != nil {
				return e
			}
			a.ReportSeq = seq
		}
		out = feedback(v, a)
		return nil
	})
	return out, e
}

type progressStep struct {
	Seq    uint64                `json:"seq"`
	Report model.ExecutionReport `json:"report"`
	At     time.Time             `json:"at"`
}

func verified(a model.Attempt, r model.ExecutionReport) bool {
	return r.PlatformUID == a.Input.PlatformUID && r.BitProfileID == a.Input.BitProfileID && r.VideoSHA256 == a.Input.VideoSHA256 && r.CoverSHA256 == a.Input.CoverSHA256
}
func (s ExecutionStore) Progress(candidate model.Task, user int64, r model.ExecutionReport, currentSession bool, now time.Time) (model.ExecutionFeedback, error) {
	var out model.ExecutionFeedback
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if !currentSession && r.Stage != "awaiting_human" {
			return ErrConflict
		}
		var currentInput *model.ExecutionInput
		if r.Stage == "ready" || r.Stage == "submitting" || r.Stage == "preparing" {
			input, e := s.lockedInput(tx, candidate)
			currentInput = &input
			if e != nil {
				return e
			}
		}
		v, a, e := currentAttempt(tx, candidate.ID, user, r.AttemptID)
		if e != nil {
			return e
		}
		if currentInput != nil && *currentInput != a.Input {
			return ErrConflict
		}
		if a.ResolvedAt != nil || a.Status != "running" && a.Status != "awaiting_human" || r.Seq == 0 || r.Seq < a.ReportSeq {
			return ErrConflict
		}
		var steps []progressStep
		if e := json.Unmarshal(a.Steps, &steps); e != nil {
			return e
		}
		if r.Seq == a.ReportSeq {
			if len(steps) == 0 || steps[len(steps)-1].Report != r {
				return ErrConflict
			}
			out = feedback(v, a)
			return nil // An old permission is never returned again.
		}
		switch r.Stage {
		case "preparing":
			if a.Stage != "claimed" && a.Stage != "preparing" {
				return ErrConflict
			}
		case "ready":
			if a.Stage != "claimed" && a.Stage != "preparing" && a.Stage != "ready" || !verified(a, r) {
				return ErrConflict
			}
		case "submitting":
			if a.Stage != "ready" || !verified(a, r) {
				return ErrConflict
			}
		case "awaiting_human":
		default:
			return ErrConflict
		}
		allowed := r.Stage == "submitting" && v.Status == "running" && v.AuthorizedAt != nil && v.CancelRequestedAt == nil && (v.NotBefore == nil || !v.NotBefore.After(now))
		steps = append(steps, progressStep{Seq: r.Seq, Report: r, At: now})
		a.Stage = r.Stage
		a.ReportSeq = r.Seq
		a.HeartbeatAt = now
		updates := map[string]any{"stage": r.Stage, "report_seq": r.Seq, "heartbeat_at": now, "steps_json": mustJSON(steps)}
		if allowed {
			a.SubmitStartedAt = &now
			updates["submit_started_at"] = now
		}
		if r.Stage == "awaiting_human" && v.Status == "running" {
			a.Status = "awaiting_human"
			v.Status = "awaiting_human"
			v.Revision++
			updates["status"] = a.Status
			if e := tx.Model(&v).Updates(map[string]any{"status": v.Status, "revision": v.Revision, "updated_at": now}).Error; e != nil {
				return e
			}
			if e := audit(tx, user, v.ID, "publication.awaiting_human", v.Revision); e != nil {
				return e
			}
		}
		if e := tx.Model(&a).Updates(updates).Error; e != nil {
			return e
		}
		out = feedback(v, a)
		out.AllowSubmit = allowed
		return nil
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
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		v, a, e := currentAttempt(tx, candidate.ID, user, r.AttemptID)
		if e != nil {
			return e
		}
		if a.ResultHash != nil {
			if *a.ResultHash != r.ResultHash || a.ReportSeq != r.Seq {
				return ErrConflict
			}
			out = feedback(v, a)
			return nil
		}
		if r.Seq == 0 || r.Seq <= a.ReportSeq || a.ResolvedAt != nil || a.Status == "completed" {
			return ErrConflict
		}
		status := result.Status
		attemptStatus := "completed"
		switch status {
		case "reviewing":
			ev := result.Evidence
			if a.SubmitStartedAt == nil || result.PlatformWorkID == "" || ev.ReceiptID != result.PlatformWorkID || !ev.SceneClosed || ev.PlatformUID != a.Input.PlatformUID || ev.BitProfileID != a.Input.BitProfileID || ev.VideoSHA256 != a.Input.VideoSHA256 || ev.CoverSHA256 != a.Input.CoverSHA256 {
				return ErrConflict
			}
		case "failed":
			ev := result.Evidence
			if a.SubmitStartedAt != nil || !ev.NotSubmitted || !ev.SceneClosed || result.ErrorCode == "" || ev.PlatformUID != a.Input.PlatformUID || ev.BitProfileID != a.Input.BitProfileID || ev.VideoSHA256 != a.Input.VideoSHA256 || ev.CoverSHA256 != a.Input.CoverSHA256 {
				return ErrConflict
			}
		case "uncertain":
			attemptStatus = "interrupted"
		default:
			return ErrConflict
		}
		// Keep the complete original observation on the immutable attempt; task
		// links may later be corrected by an explicit verification.
		evidence := r.Result
		updates := map[string]any{"status": attemptStatus, "stage": "finished", "report_seq": r.Seq, "heartbeat_at": now, "finished_at": now, "result_status": status, "result_hash": r.ResultHash, "evidence_json": string(evidence), "error_code": optional(result.ErrorCode), "error_message": optional(result.ErrorMessage)}
		if e := tx.Model(&a).Updates(updates).Error; e != nil {
			return e
		}
		taskUpdates := map[string]any{"status": status, "revision": v.Revision + 1, "updated_at": now, "platform_work_id": optional(result.PlatformWorkID), "work_url": optional(result.WorkURL), "failure_code": optional(result.ErrorCode), "failure_message": optional(result.ErrorMessage)}
		if status == "reviewing" {
			taskUpdates["accepted_at"] = now
		}
		if e := tx.Model(&v).Updates(taskUpdates).Error; e != nil {
			return e
		}
		if e := audit(tx, user, v.ID, "publication.result", v.Revision+1); e != nil {
			return e
		}
		v.Status = status
		v.Revision++
		a.Status = attemptStatus
		a.ReportSeq = r.Seq
		a.ResultHash = &r.ResultHash
		out = feedback(v, a)
		return nil
	})
	return out, e
}
func optional(v string) any {
	if v == "" {
		return nil
	}
	return v
}
