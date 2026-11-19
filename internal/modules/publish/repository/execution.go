package repository

import (
	"database/sql"
	"errors"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"strings"
	"time"
)

// ErrInputChanged is a business rejection; infrastructure errors remain errors.
var ErrInputChanged = fmt.Errorf("%w: execution input changed", ErrConflict)

type ExecutionStore struct {
	Store
	ReadInput func(*gorm.DB, model.Task) (model.ExecutionInput, error)
}

func (s ExecutionStore) readInput(tx *gorm.DB, v model.Task) (model.ExecutionInput, error) {
	if s.ReadInput == nil {
		return model.ExecutionInput{}, errors.New("publication input reader is unavailable")
	}
	return s.ReadInput(tx, v)
}

func currentTransaction(db *gorm.DB, work func(*gorm.DB) error) error {
	return db.Transaction(work, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}

func sameCandidate(v, old model.Task) bool {
	return v.ID == old.ID && v.Revision == old.Revision && v.OperatorID == old.OperatorID && v.PlatformAccountUID == old.PlatformAccountUID && v.MediaAccountID == old.MediaAccountID && v.CompositeOutputID == old.CompositeOutputID && v.CoverFileID == old.CoverFileID
}
func (s ExecutionStore) Start(candidate model.Task, revision uint64, actor int64, now time.Time) (model.Task, error) {
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		if _, e := s.readInput(tx, candidate); e != nil {
			return e
		}
		old, e := taskSnapshot(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(candidate, old) || old.Revision != revision || old.Status != model.StatusPending || old.AuthorizedAt != nil || old.CurrentAttemptID != nil || old.CancelRequestedAt != nil {
			return &ConflictError{TaskID: old.ID, Revision: old.Revision}
		}
		status := model.StatusQueued
		if old.NotBefore != nil && old.NotBefore.After(now) {
			status = model.StatusPending
		}
		if e := updateTask(tx, old, map[string]any{"authorized_at": now, "status": status, "updated_at": now}); e != nil {
			return e
		}
		return audit(tx, actor, old.ID, "publication.start", old.Revision+1)
	})
	if e != nil {
		return model.Task{}, e
	}
	return s.Get(candidate.ID)
}
func (s ExecutionStore) QueueDue(now time.Time) error {
	var candidates []model.Task
	if e := s.DB.Where("status=? AND authorized_at IS NOT NULL AND cancel_requested_at IS NULL AND (not_before IS NULL OR not_before<=?)", model.StatusPending, now).Order("id").Limit(100).Find(&candidates).Error; e != nil {
		return e
	}
	for _, candidate := range candidates {
		e := currentTransaction(s.DB, func(tx *gorm.DB) error {
			if _, e := s.readInput(tx, candidate); e != nil {
				return e
			}
			old, e := taskSnapshot(tx, candidate.ID)
			if e != nil {
				return e
			}
			if !sameCandidate(candidate, old) || old.Status != model.StatusPending || old.AuthorizedAt == nil || old.CancelRequestedAt != nil || old.NotBefore != nil && old.NotBefore.After(now) {
				return ErrConflict
			}
			if e := updateTask(tx, old, map[string]any{"status": model.StatusQueued, "updated_at": now}); e != nil {
				return e
			}
			return audit(tx, 0, old.ID, "publication.enqueue", old.Revision+1)
		})
		if e != nil && !errors.Is(e, ErrConflict) && !errors.Is(e, ErrNotFound) {
			return e
		}
	}
	return nil
}
func (s ExecutionStore) prior(user int64, key string) (*model.Claim, error) {
	var a model.Attempt
	e := s.DB.Where("executor_user_id=? AND claim_request_key=?", user, key).First(&a).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := s.Get(a.PublicationID)
	if e != nil {
		return nil, e
	}
	return &model.Claim{Task: v, Attempt: a, Replayed: true}, nil
}
func (s ExecutionStore) Claim(user int64, key string, now time.Time) (*model.Claim, error) {
	if prior, e := s.prior(user, key); e != nil || prior != nil {
		return prior, e
	}
	var candidates []model.Task
	if e := s.DB.Where("operator_id=? AND status=? AND authorized_at IS NOT NULL AND cancel_requested_at IS NULL AND (not_before IS NULL OR not_before<=?)", user, model.StatusQueued, now).Order("id").Limit(100).Find(&candidates).Error; e != nil {
		return nil, e
	}
	for _, candidate := range candidates {
		var claimed model.Claim
		e := pureTransaction(s.DB, func(tx *gorm.DB) error {
			input, e := s.readInput(tx, candidate)
			if e != nil {
				return e
			}
			v, e := taskSnapshot(tx, candidate.ID)
			if e != nil {
				return e
			}
			if !sameCandidate(candidate, v) || v.OperatorID != user || v.Status != model.StatusQueued || v.AuthorizedAt == nil || v.CurrentAttemptID != nil || v.CancelRequestedAt != nil || v.NotBefore != nil && v.NotBefore.After(now) {
				return ErrConflict
			}
			// The read only chooses a candidate slot. Generated unique indexes
			// arbitrate competing inserts, including different users/Profiles.
			var occupied []uint8
			if e := tx.Model(&model.Attempt{}).Where("executor_user_id=? AND active_admission_slot IS NOT NULL", user).Pluck("admission_slot", &occupied).Error; e != nil {
				return e
			}
			slot := uint8(1)
			for ; slot <= 3; slot++ {
				used := false
				for _, other := range occupied {
					used = used || other == slot
				}
				if !used {
					break
				}
			}
			if slot > 3 {
				return ErrConflict
			}
			a := model.Attempt{AdmissionSlot: slot, PublicationID: v.ID, AttemptNo: v.AttemptCount + 1, ExecutorUserID: user, ProfileID: input.ProfileID, ClaimRequestKey: key, Input: input, Status: model.AttemptStatusRunning, Stage: model.AttemptStageClaimed, StartedAt: now, HeartbeatAt: now, Steps: []byte(`[]`), Evidence: []byte(`{}`)}
			if e := tx.Create(&a).Error; e != nil {
				return e
			}
			if e := updateTask(tx, v, map[string]any{"status": model.StatusRunning, "current_attempt_id": a.ID, "attempt_count": a.AttemptNo, "updated_at": now}); e != nil {
				return e
			}
			if e := audit(tx, user, v.ID, "publication.claim", v.Revision+1); e != nil {
				return e
			}
			v.Status = model.StatusRunning
			v.CurrentAttemptID = &a.ID
			v.AttemptCount = a.AttemptNo
			v.Revision++
			v.UpdatedAt = now
			claimed = model.Claim{Task: v, Attempt: a}
			return nil
		})
		if e == nil {
			return &claimed, nil
		}
		var duplicate *driver.MySQLError
		if errors.As(e, &duplicate) && duplicate.Number == 1062 {
			if prior, e := s.prior(user, key); e != nil || prior != nil {
				return prior, e
			}
			// A task, Profile, UID or user-slot constraint rejected the claim. Nothing committed.
			continue
		}
		if errors.Is(e, ErrConflict) || errors.Is(e, ErrNotFound) {
			continue
		}
		return nil, e
	}
	return nil, nil
}

// Retry only pure database races, before any external action or grant signing.
// A user-slot collision must re-read availability in a fresh transaction.
func pureTransaction(db *gorm.DB, work func(*gorm.DB) error) error {
	for attempt := 0; ; attempt++ {
		e := currentTransaction(db, work)
		var deadlock *driver.MySQLError
		retry := errors.As(e, &deadlock) && (deadlock.Number == 1213 || deadlock.Number == 1062 && strings.Contains(deadlock.Message, "uq_attempt_user_slot"))
		if !retry || attempt >= 2 {
			return e
		}
	}
}
func (s ExecutionStore) Attempt(task uint64) ([]model.Attempt, error) {
	v := []model.Attempt{}
	e := s.DB.Where("publication_id=?", task).Order("attempt_no").Find(&v).Error
	return v, e
}
func (s ExecutionStore) InterruptStale(now time.Time) error {
	var candidates []model.Attempt
	if e := s.DB.Where("status IN ? AND heartbeat_at<?", []model.AttemptStatus{model.AttemptStatusRunning, model.AttemptStatusAwaitingHuman}, now.Add(-90*time.Second)).Order("id").Limit(100).Find(&candidates).Error; e != nil {
		return e
	}
	for _, candidate := range candidates {
		e := currentTransaction(s.DB, func(tx *gorm.DB) error {
			v, e := taskSnapshot(tx, candidate.PublicationID)
			if e != nil {
				return e
			}
			var a model.Attempt
			if e := tx.First(&a, candidate.ID).Error; e != nil {
				return e
			}
			if v.CurrentAttemptID == nil || *v.CurrentAttemptID != a.ID || a.ResolvedAt != nil || a.Status != model.AttemptStatusRunning && a.Status != model.AttemptStatusAwaitingHuman || !a.HeartbeatAt.Before(now.Add(-90*time.Second)) {
				return nil
			}
			if e := updateTask(tx, v, map[string]any{"status": model.StatusUncertain, "updated_at": now}); e != nil {
				return e
			}
			if e := updateAttempt(tx, a, map[string]any{"status": model.AttemptStatusInterrupted, "stage": model.AttemptStageConnectionLost, "finished_at": now}); e != nil {
				return e
			}
			return audit(tx, 0, v.ID, "publication.connection_lost", v.Revision+1)
		})
		if e != nil && !errors.Is(e, ErrConflict) {
			return e
		}
	}
	return nil
}

// ProfileReferenced includes pending tasks and completed historical attempts.
func ProfileReferenced(db *gorm.DB, profile string) (bool, error) {
	var n int64
	if e := db.Model(&model.Attempt{}).Where("browser_profile_id=?", profile).Count(&n).Error; e != nil {
		return false, e
	}
	if n > 0 {
		return true, nil
	}
	e := db.Model(&model.Task{}).Where("browser_profile_id=?", profile).Count(&n).Error
	return n > 0, e
}

// ProfileExecutionBusy observes occupancy; unique constraints enforce admission.
func ProfileExecutionBusy(tx *gorm.DB, profile string) error {
	var n int64
	if e := tx.Model(&model.Attempt{}).Where("browser_profile_id=? AND (status IN ? OR (status=? AND resolved_at IS NULL))", profile, []model.AttemptStatus{model.AttemptStatusRunning, model.AttemptStatusAwaitingHuman}, model.AttemptStatusInterrupted).Count(&n).Error; e != nil {
		return e
	}
	if n > 0 {
		return ErrConflict
	}
	return nil
}
