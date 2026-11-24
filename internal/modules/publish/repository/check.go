package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"reflect"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
)

const publicCheckLifetime = 24 * time.Hour

func publicCheckDelay(age time.Duration) time.Duration {
	low, high := 4*time.Minute, 6*time.Minute
	if age >= 3*time.Hour {
		low, high = 25*time.Minute, 35*time.Minute
	} else if age >= 30*time.Minute {
		low, high = 10*time.Minute, 15*time.Minute
	}
	return low + time.Duration(rand.Int64N(int64((high-low)/time.Second)+1))*time.Second
}
func missingCheckDelay(count uint32) time.Duration {
	low, high := 45*time.Second, 75*time.Second
	if count >= 2 {
		low, high = 150*time.Second, 210*time.Second
	}
	return low + time.Duration(rand.Int64N(int64((high-low)/time.Second)+1))*time.Second
}
func (s ExecutionStore) SchedulePublicCheck(now time.Time) error {
	base := s.DB.Model(&model.Task{}).Where("status=? AND platform=? AND check_stopped_at IS NULL AND accepted_at IS NOT NULL", model.StatusReviewing, "baidu")
	if err := base.Session(&gorm.Session{}).Where("accepted_at<=?", now.Add(-publicCheckLifetime)).UpdateColumns(map[string]any{"check_stopped_at": now, "next_check_at": nil, "last_check_message": "已超过24小时自动核验期限，请人工核实或补录结果"}).Error; err != nil {
		return err
	}
	return s.DB.Model(&model.Task{}).Where("status=? AND platform=? AND current_attempt_id IS NOT NULL AND platform_submission_id IS NOT NULL AND platform_submission_id<>'' AND next_check_at IS NULL AND check_stopped_at IS NULL AND accepted_at>?", model.StatusReviewing, "baidu", now.Add(-publicCheckLifetime)).UpdateColumn("next_check_at", now).Error
}

func (s ExecutionStore) ClaimPublicCheck(user int64, now time.Time) (*model.PublicCheckClaim, error) {
	if s.RequireAdmission {
		return nil, ErrAgentUpgrade
	}
	if user <= 0 {
		return nil, ErrNotFound
	}
	var candidates []model.Task
	if err := s.DB.Where("operator_id=? AND status=? AND platform=? AND next_check_at<=? AND check_stopped_at IS NULL AND accepted_at>?", user, model.StatusReviewing, "baidu", now, now.Add(-publicCheckLifetime)).Order("next_check_at,id").Limit(20).Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		var claim *model.PublicCheckClaim
		err := currentTransaction(s.DB, func(tx *gorm.DB) error {
			v, err := taskSnapshot(tx, candidate.ID)
			if err != nil {
				return err
			}
			if v.OperatorID != user || v.Status != model.StatusReviewing || v.CurrentAttemptID == nil || v.PlatformSubmissionID == nil || v.NextCheckAt == nil || v.NextCheckAt.After(now) || v.CheckStoppedAt != nil || v.AcceptedAt == nil || !now.Before(v.AcceptedAt.Add(publicCheckLifetime)) {
				return nil
			}
			_, a, err := currentAttempt(tx, v.ID, user, *v.CurrentAttemptID)
			if err != nil {
				return err
			}
			next := now.Add(publicCheckDelay(now.Sub(*v.AcceptedAt)))
			if deadline := v.AcceptedAt.Add(publicCheckLifetime); next.After(deadline) {
				next = deadline
			}
			// Schedule only another read. This is never a publishing attempt or permission.
			result := tx.Model(&model.Task{}).Where("id=? AND revision=? AND status=? AND check_generation=? AND next_check_at<=? AND check_stopped_at IS NULL", v.ID, v.Revision, model.StatusReviewing, v.CheckGeneration, now).UpdateColumns(map[string]any{"check_generation": v.CheckGeneration + 1, "next_check_at": next})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return nil
			}
			if safeDisposition(a) != model.StatusReviewing {
				return nil
			}
			input, err := s.readInput(tx, v)
			if errors.Is(err, ErrInputChanged) {
				return nil
			}
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(input, a.Input) {
				return nil
			}
			claim = &model.PublicCheckClaim{BrowserFallbackAllowed: v.CheckBrowserUsedAt == nil, Task: v, Attempt: a, Generation: v.CheckGeneration + 1, ExpectedRevision: v.Revision}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if claim != nil {
			return claim, nil
		}
	}
	return nil, nil
}

func (s ExecutionStore) ObservePublication(candidate model.Task, user int64, r model.PublicCheckReport, now time.Time) (model.PublicCheckAcknowledgement, error) {
	var ack model.PublicCheckAcknowledgement
	if !r.Valid() {
		return ack, ErrConflict
	}
	raw, err := json.Marshal(r.Observation)
	if err != nil {
		return ack, err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	err = currentTransaction(s.DB, func(tx *gorm.DB) error {
		parent, err := lockTaskBatch(tx, candidate)
		if err != nil {
			return err
		}
		var use model.ProfileUse
		if r.ReservationID != 0 {
			use, err = lockedCheckUse(tx, r.ReservationID, user)
			if err != nil {
				return err
			}
			if use.PublicationID != candidate.ID || use.AttemptID == nil || *use.AttemptID != r.Observation.AttemptID ||
				use.CheckGeneration == nil || *use.CheckGeneration != r.Generation || use.CheckRevision == nil || *use.CheckRevision != r.Observation.ExpectedRevision {
				return ErrProfileUse
			}
		} else if s.RequireAdmission {
			return ErrAgentUpgrade
		}
		v, err := taskSnapshot(tx, candidate.ID)
		if err != nil {
			return err
		}
		if v.OperatorID != user {
			return ErrNotFound
		}
		ack = model.PublicCheckAcknowledgement{ReservationID: r.ReservationID, Released: r.ReservationID != 0, PublicationID: v.ID, AttemptID: r.Observation.AttemptID, Generation: r.Generation, ObservationHash: hash}
		var prior model.PublicationObservation
		err = tx.Where("publication_id=? AND generation=?", v.ID, r.Generation).First(&prior).Error
		if err == nil {
			if prior.ObserverID != user || prior.AttemptID != r.Observation.AttemptID || prior.ObservationHash != hash {
				return ErrConflict
			}
			if r.ReservationID != 0 && (use.ReleasedAt == nil || use.ObservationHash == nil || *use.ObservationHash != hash || use.Disposition == nil || *use.Disposition != r.ProfileDisposition) {
				return ErrProfileUse
			}
			return nil // Confirm the recorded fact without changing a newer task.
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if r.ReservationID != 0 && use.ReleasedAt != nil {
			return ErrProfileUse
		}
		in := r.Observation
		if v.CheckStoppedAt != nil || v.AcceptedAt == nil || !now.Before(v.AcceptedAt.Add(publicCheckLifetime)) || v.Status != model.StatusReviewing || v.Revision != in.ExpectedRevision || v.CheckGeneration != r.Generation || v.PlatformSubmissionID == nil || v.CurrentAttemptID == nil || *v.CurrentAttemptID != in.AttemptID {
			return ErrConflict
		}
		_, a, err := currentAttempt(tx, v.ID, user, in.AttemptID)
		if err != nil {
			return err
		}
		if safeDisposition(a) != model.StatusReviewing || a.Input.PlatformUID != in.Evidence.PlatformUID || v.PlatformAccountUID != in.Evidence.PlatformUID {
			return ErrConflict
		}
		input, err := s.readInput(tx, v)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(input, a.Input) {
			return ErrInputChanged
		}
		if in.Conclusion != model.StatusUncertain && in.PlatformSubmissionID != *v.PlatformSubmissionID {
			return ErrConflict
		}
		updates := map[string]any{"last_checked_at": now, "last_check_message": in.Evidence.Reason, "updated_at": now}
		if r.BrowserUsed {
			updates["check_browser_used_at"] = now
		}
		switch r.LookupResult {
		case "not_found":
			count := v.CheckNotFoundCount + 1
			updates["check_not_found_count"] = count
			if count >= 3 {
				updates["check_stopped_at"] = now
				updates["next_check_at"] = nil
				updates["last_check_message"] = "连续3次未找到原投稿，已停止自动核验，请人工核实或补录结果"
			} else {
				next := now.Add(missingCheckDelay(count))
				if deadline := v.AcceptedAt.Add(publicCheckLifetime); next.After(deadline) {
					next = deadline
				}
				updates["next_check_at"] = next
			}
		case "found":
			updates["check_not_found_count"] = 0
		case "blocked":
			updates["last_check_message"] = in.Evidence.Reason + "；自动核验已停止，请人工核实或补录"
			updates["check_stopped_at"] = now
			updates["next_check_at"] = nil
		}
		if in.Conclusion == model.StatusPublished {
			updates["check_stopped_at"] = now
			updates["next_check_at"] = nil
		}

		if in.TargetURL != "" {
			if v.TargetURL != nil && *v.TargetURL != "" && *v.TargetURL != in.TargetURL {
				return ErrConflict
			}
			updates["target_url"] = in.TargetURL
		}
		if in.Conclusion == model.StatusPublished {
			updates["status"] = model.StatusPublished
			updates["platform_work_id"] = in.PlatformWorkID
			updates["work_url"] = in.WorkURL
			updates["published_at"] = in.PublishedAt
			updates["failure_code"] = nil
			updates["failure_message"] = nil
		}
		if err = updateTask(tx.Where("check_generation=? AND check_stopped_at IS NULL", r.Generation), v, updates); err != nil {
			return err
		}
		if in.Conclusion == model.StatusPublished {
			if err = s.recordVerifiedWork(tx, v, a, in, now); err != nil {
				return err
			}
		}
		observation := model.PublicationObservation{PublicationID: v.ID, AttemptID: a.ID, ObserverID: user, Generation: r.Generation, ExpectedRevision: in.ExpectedRevision, ObservationHash: hash, Observation: raw, ObservedAt: now}
		if err = tx.Create(&observation).Error; err != nil {
			return err
		}
		if r.ReservationID != 0 {
			if err = releaseCheckUse(tx, &use, r.ProfileDisposition, &hash, now); err != nil {
				return err
			}
		}
		if err = auditDetails(tx, user, v.ID, "publication.observed", map[string]any{"revision": v.Revision + 1, "generation": r.Generation, "observation_hash": hash, "conclusion": in.Conclusion}); err != nil {
			return err
		}
		return advanceBatch(tx, parent, now)
	})
	return ack, err
}

func (s ExecutionStore) recordVerifiedWork(tx *gorm.DB, v model.Task, a model.Attempt, in model.ResultResolution, now time.Time) error {
	if s.RecordWork == nil {
		return errors.New("public work recorder is unavailable")
	}
	id, err := s.RecordWork(tx, v, a, in, now)
	if err != nil {
		return err
	}
	result := tx.Model(&model.Task{}).Where("id=? AND revision=?", v.ID, v.Revision+1).Update("work_id", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
