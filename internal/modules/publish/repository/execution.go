package repository

import (
	"errors"
	driver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// LockInput belongs to the resource owners. It locks Profile, file, account and
// output rows in that order, validates current identities, and performs no I/O.
type ExecutionStore struct {
	Store
	LockInput func(*gorm.DB, model.Task) (model.ExecutionInput, error)
	LockScene func(*gorm.DB, uint64, int64) error
}

func lockedTask(tx *gorm.DB, id uint64) (model.Task, error) {
	var v model.Task
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, id).Error
	return v, missing(e)
}
func (s ExecutionStore) lockedInput(tx *gorm.DB, v model.Task) (model.ExecutionInput, error) {
	if s.LockInput == nil {
		return model.ExecutionInput{}, errors.New("publication resource locking is unavailable")
	}
	input, e := s.LockInput(tx, v)
	if e != nil {
		return input, e
	}
	if s.LockFiles == nil {
		return input, errors.New("publication file locking is unavailable")
	}
	return input, s.LockFiles(tx, []int64{v.CoverFileID})
}
func sameCandidate(v, old model.Task) bool {
	return v.ID == old.ID && v.Revision == old.Revision && v.OperatorID == old.OperatorID && v.PlatformAccountUID == old.PlatformAccountUID && v.MediaAccountID == old.MediaAccountID && v.CompositeOutputID == old.CompositeOutputID && v.CoverFileID == old.CoverFileID
}
func (s ExecutionStore) Start(candidate model.Task, revision uint64, actor int64, now time.Time) (model.Task, error) {
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if _, e := s.lockedInput(tx, candidate); e != nil {
			return e
		}
		old, e := lockedTask(tx, candidate.ID)
		if e != nil {
			return e
		}
		if !sameCandidate(candidate, old) || old.Revision != revision || old.Status != "pending" || old.AuthorizedAt != nil || old.CurrentAttemptID != nil || old.CancelRequestedAt != nil {
			return &ConflictError{TaskID: old.ID, Revision: old.Revision}
		}
		status := "queued"
		if old.NotBefore != nil && old.NotBefore.After(now) {
			status = "pending"
		}
		if e := tx.Model(&model.Task{}).Where("id=?", old.ID).Updates(map[string]any{"authorized_at": now, "status": status, "revision": old.Revision + 1, "updated_at": now}).Error; e != nil {
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
	if e := s.DB.Where("status='pending' AND authorized_at IS NOT NULL AND cancel_requested_at IS NULL AND (not_before IS NULL OR not_before<=?)", now).Order("id").Limit(100).Find(&candidates).Error; e != nil {
		return e
	}
	for _, candidate := range candidates {
		e := s.DB.Transaction(func(tx *gorm.DB) error {
			if _, e := s.lockedInput(tx, candidate); e != nil {
				return e
			}
			old, e := lockedTask(tx, candidate.ID)
			if e != nil {
				return e
			}
			if !sameCandidate(candidate, old) || old.Status != "pending" || old.AuthorizedAt == nil || old.CancelRequestedAt != nil || old.NotBefore != nil && old.NotBefore.After(now) {
				return ErrConflict
			}
			if e := tx.Model(&model.Task{}).Where("id=?", old.ID).Updates(map[string]any{"status": "queued", "revision": old.Revision + 1, "updated_at": now}).Error; e != nil {
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
	if e := s.DB.Where("operator_id=? AND status='queued' AND authorized_at IS NOT NULL AND cancel_requested_at IS NULL AND (not_before IS NULL OR not_before<=?)", user, now).Order("id").Limit(100).Find(&candidates).Error; e != nil {
		return nil, e
	}
	for _, candidate := range candidates {
		var claimed model.Claim
		e := pureTransaction(s.DB, func(tx *gorm.DB) error {
			input, e := s.lockedInput(tx, candidate)
			if e != nil {
				return e
			}
			v, e := lockedTask(tx, candidate.ID)
			if e != nil {
				return e
			}
			if !sameCandidate(candidate, v) || v.OperatorID != user || v.Status != "queued" || v.AuthorizedAt == nil || v.CurrentAttemptID != nil || v.CancelRequestedAt != nil || v.NotBefore != nil && v.NotBefore.After(now) {
				return ErrConflict
			}
			var count int64
			if e := tx.Model(&model.Attempt{}).Where("browser_profile_id=? AND (status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL))", input.ProfileID).Count(&count).Error; e != nil {
				return e
			}
			if count > 0 {
				return ErrConflict
			}
			if e := tx.Model(&model.Attempt{}).Where("executor_user_id=? AND (status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL))", user).Count(&count).Error; e != nil {
				return e
			}
			if count >= 3 {
				return ErrConflict
			}
			a := model.Attempt{PublicationID: v.ID, AttemptNo: v.AttemptCount + 1, ExecutorUserID: user, ProfileID: input.ProfileID, ClaimRequestKey: key, Input: input, Status: "running", Stage: "claimed", StartedAt: now, HeartbeatAt: now, Steps: []byte(`[]`), Evidence: []byte(`{}`)}
			if e := tx.Create(&a).Error; e != nil {
				return e
			}
			if e := tx.Model(&model.Task{}).Where("id=?", v.ID).Updates(map[string]any{"status": "running", "current_attempt_id": a.ID, "attempt_count": a.AttemptNo, "revision": v.Revision + 1, "updated_at": now}).Error; e != nil {
				return e
			}
			if e := audit(tx, user, v.ID, "publication.claim", v.Revision+1); e != nil {
				return e
			}
			v.Status = "running"
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
			// The platform/UID unique slot rejected a concurrent claim; nothing committed.
			continue
		}
		if errors.Is(e, ErrConflict) || errors.Is(e, ErrNotFound) {
			continue
		}
		return nil, e
	}
	return nil, nil
}

// Only database deadlocks are retried, before any external action or grant signing.
func pureTransaction(db *gorm.DB, work func(*gorm.DB) error) error {
	for attempt := 0; ; attempt++ {
		e := db.Transaction(work)
		var deadlock *driver.MySQLError
		if !errors.As(e, &deadlock) || deadlock.Number != 1213 || attempt >= 2 {
			return e
		}
	}
}
func (s ExecutionStore) Attempts(task uint64) ([]model.Attempt, error) {
	v := []model.Attempt{}
	e := s.DB.Where("publication_id=?", task).Order("attempt_no").Find(&v).Error
	return v, e
}
func (s ExecutionStore) InterruptStale(now time.Time) error {
	var candidates []model.Attempt
	if e := s.DB.Where("status IN ('running','awaiting_human') AND heartbeat_at<?", now.Add(-90*time.Second)).Order("id").Limit(100).Find(&candidates).Error; e != nil {
		return e
	}
	for _, candidate := range candidates {
		e := s.DB.Transaction(func(tx *gorm.DB) error {
			v, e := lockedTask(tx, candidate.PublicationID)
			if e != nil {
				return e
			}
			var a model.Attempt
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, candidate.ID).Error; e != nil {
				return e
			}
			if v.CurrentAttemptID == nil || *v.CurrentAttemptID != a.ID || a.ResolvedAt != nil || a.Status != "running" && a.Status != "awaiting_human" || !a.HeartbeatAt.Before(now.Add(-90*time.Second)) {
				return nil
			}
			if e := tx.Model(&a).Updates(map[string]any{"status": "interrupted", "stage": "connection_lost", "finished_at": now}).Error; e != nil {
				return e
			}
			if e := tx.Model(&v).Updates(map[string]any{"status": "uncertain", "revision": v.Revision + 1, "updated_at": now}).Error; e != nil {
				return e
			}
			return audit(tx, 0, v.ID, "publication.connection_lost", v.Revision+1)
		})
		if e != nil {
			return e
		}
	}
	return nil
}

// Caller holds the corresponding Profile row, shared with claim admission.
func ProfileExecutionBusy(tx *gorm.DB, profile string) error {
	var n int64
	if e := tx.Model(&model.Attempt{}).Where("browser_profile_id=? AND (status IN ('running','awaiting_human') OR (status='interrupted' AND resolved_at IS NULL))", profile).Count(&n).Error; e != nil {
		return e
	}
	if n > 0 {
		return ErrConflict
	}
	return nil
}
