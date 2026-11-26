package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errNoCheck = errors.New("no eligible publication check")

func lockCheckIntent(tx *gorm.DB, actor model.CheckActor, key string, now time.Time) (model.PublicCheckRequest, error) {
	intent := model.PublicCheckRequest{OwnerID: actor.UserID, RequestKey: key, AgentSessionID: actor.SessionID, CreatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&intent).Error; err != nil {
		return intent, err
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id=? AND request_key=?", actor.UserID, key).Take(&intent).Error
	return intent, err
}
func lockedCheckUse(tx *gorm.DB, id uint64, owner int64) (model.ProfileUse, error) {
	var use model.ProfileUse
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=? AND kind='check'", id, owner).Take(&use).Error
	return use, missing(err)
}
func checkReleaseAck(use model.ProfileUse, replay bool) model.PublicCheckReleaseAck {
	out := model.PublicCheckReleaseAck{PublicationID: use.PublicationID, AttemptID: *use.AttemptID,
		Generation: *use.CheckGeneration, ReservationID: use.ID, Released: true}
	if use.ObservationHash != nil {
		out.ObservationHash = *use.ObservationHash
	}
	if replay {
		out.ReservationState, out.Replayed = "released", true
	}
	return out
}

func releaseCheckUse(tx *gorm.DB, use *model.ProfileUse, disposition string, hash *string, now time.Time) error {
	if use.ReleasedAt != nil {
		if use.Disposition == nil || *use.Disposition != disposition || !reflect.DeepEqual(use.ObservationHash, hash) {
			return ErrProfileUse
		}
		return nil
	}
	result := tx.Model(&model.ProfileUse{}).Where("id=? AND released_at IS NULL", use.ID).
		Updates(map[string]any{"released_at": now, "disposition": disposition, "observation_hash": hash})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrProfileUse
	}
	use.ReleasedAt, use.Disposition, use.ObservationHash = &now, &disposition, hash
	return nil
}

func (s ExecutionStore) ClaimPublicCheckV2(actor model.CheckActor, in model.CheckClaimInput, now time.Time) (any, error) {
	supported, valid := model.SupportedPublicationPlatforms(in.SupportedPlatforms)
	if !valid {
		return nil, ErrConflict
	}
	if actor.UserID <= 0 || actor.SessionID == "" {
		return nil, ErrNotFound
	}
	if !in.SettleUnreceived && (!actor.Current || !in.Capacity.Valid()) {
		return nil, ErrAgentUpgrade
	}
	var result any
	err := pureTransaction(s.DB, func(tx *gorm.DB) error {
		intent, err := lockCheckIntent(tx, actor, in.RequestKey, now)
		if err != nil {
			return err
		}
		if in.SettleUnreceived && !actor.Current && intent.AgentSessionID != actor.SessionID {
			return ErrNotFound
		}
		if intent.CancelledAt != nil {
			result = model.CancelledCheckClaim{RequestKey: intent.RequestKey, ReservationState: "cancelled"}
			return nil
		}
		var digest string
		if !in.SettleUnreceived {
			hash := sha256.Sum256([]byte(mustJSON(in.Capacity)))
			digest = hex.EncodeToString(hash[:])
			if intent.RequestHash != nil && *intent.RequestHash != digest {
				return ErrConflict
			}
		}
		if intent.ReservationID != nil {
			use, e := lockedCheckUse(tx, *intent.ReservationID, actor.UserID)
			if e != nil {
				return e
			}
			if use.AttemptID == nil || use.CheckGeneration == nil || use.CheckRevision == nil {
				return ErrProfileUse
			}
			if in.SettleUnreceived && use.ReleasedAt == nil {
				if e = releaseCheckUse(tx, &use, "not_opened", nil, now); e != nil {
					return e
				}
			}
			if use.ReleasedAt != nil {
				result = checkReleaseAck(use, true)
				return nil
			}
			if intent.AgentSessionID != actor.SessionID {
				return ErrProfileUse
			}
			// Existing replay exposes frozen scope to the original current session.
			// The Agent journal is still responsible for the no-open recovery gate.
			v, e := taskSnapshot(tx, use.PublicationID)
			if e != nil {
				return e
			}
			if v.Status != model.StatusReviewing || v.Revision != *use.CheckRevision ||
				v.CurrentAttemptID == nil || *v.CurrentAttemptID != *use.AttemptID || v.CheckGeneration != *use.CheckGeneration || v.CheckStoppedAt != nil {
				return ErrProfileUse
			}
			var a model.Attempt
			if e = tx.First(&a, *use.AttemptID).Error; e != nil {
				return e
			}
			input, e := s.readInput(tx, v)
			if e != nil {
				return e
			}
			if !reflect.DeepEqual(input, a.Input) {
				return ErrInputChanged
			}
			result = checkClaim(v, a, use, true)
			return nil
		}
		if in.SettleUnreceived {
			if err = tx.Model(&model.PublicCheckRequest{}).Where("owner_id=? AND request_key=?", actor.UserID, in.RequestKey).Update("cancelled_at", now).Error; err != nil {
				return err
			}
			result = model.CancelledCheckClaim{RequestKey: in.RequestKey, ReservationState: "cancelled"}
			return nil
		}
		if intent.AgentSessionID != actor.SessionID {
			return ErrProfileUse
		}
		if err = admissionOpen(tx); err != nil {
			return err
		}
		capacity := s.localCapacity(in.Capacity)
		var afterTime *time.Time
		var afterID uint64
		for {
			// The request intent was locked and any original winner replayed
			// above. A full owner window must not scan the remaining backlog.
			window, e := freeWindowSlot(tx, actor.UserID, capacity.WindowSlots)
			if e != nil {
				return e
			}
			if window == 0 {
				return errNoCheck
			}
			rows := []model.Task{}
			query := tx.Where("operator_id=? AND status=? AND platform IN ? AND next_check_at<=? AND check_stopped_at IS NULL AND accepted_at>?",
				actor.UserID, model.StatusReviewing, supported, now, now.Add(-publicCheckLifetime)).Where(`
				NOT EXISTS (
					SELECT 1 FROM publication_attempt a
					JOIN publication_profile_use p
					ON p.active_main_id=JSON_UNQUOTE(JSON_EXTRACT(a.input_snapshot_json,'$.bit_main_user_id'))
					AND p.active_profile_id=JSON_UNQUOTE(JSON_EXTRACT(a.input_snapshot_json,'$.bit_profile_id'))
					WHERE a.id=publication_task.current_attempt_id
				)`)
			if afterTime != nil {
				query = query.Where("next_check_at>? OR (next_check_at=? AND id>?)", *afterTime, *afterTime, afterID)
			}
			if err = query.Order("next_check_at,id").Limit(20).Find(&rows).Error; err != nil {
				return err
			}
			// The actual Profile unique key resolves contention with claims made
			// after this read. Hold at most one batch lock per transaction.
			for _, row := range rows {
				v, a, e := s.checkCandidate(tx, row, actor.UserID, now)
				if errors.Is(e, ErrInputChanged) || errors.Is(e, ErrNotFound) || errors.Is(e, ErrConflict) {
					continue
				}
				if e != nil {
					return e
				}
				// Reviewing work remains eligible after its parent stops or finishes.
				if _, e = lockTaskBatch(tx, v); e != nil {
					return e
				}
				use, e := tryProfileUse(tx, actor.UserID, v, a.Input, "check", capacity.WindowSlots, now)
				if errors.Is(e, ErrProfileUse) || errors.Is(e, ErrCapacityWait) {
					return errNoCheck
				}
				if e != nil {
					return e
				}
				current, e := taskSnapshot(tx.Clauses(clause.Locking{Strength: "UPDATE"}), v.ID)
				if e != nil {
					return e
				}
				if !sameCandidate(v, current) || !eligibleCheck(current, actor.UserID, now) ||
					current.CheckGeneration != v.CheckGeneration || *current.CurrentAttemptID != a.ID {
					return errNoCheck
				}
				generation := v.CheckGeneration + 1
				next := now.Add(publicCheckDelay(now.Sub(*v.AcceptedAt)))
				if deadline := v.AcceptedAt.Add(publicCheckLifetime); next.After(deadline) {
					next = deadline
				}
				if e = tx.Model(&model.Task{}).Where("id=?", v.ID).UpdateColumns(map[string]any{
					"check_generation": generation, "next_check_at": next,
				}).Error; e != nil {
					return e
				}
				use.AttemptID, use.CheckGeneration, use.CheckRevision = &a.ID, &generation, &v.Revision
				use.CheckBrowserFallbackAllowed = v.CheckBrowserUsedAt == nil
				if e = tx.Model(&model.ProfileUse{}).Where("id=?", use.ID).Updates(map[string]any{
					"attempt_id": a.ID, "check_generation": generation, "check_revision": v.Revision, "check_browser_fallback_allowed": use.CheckBrowserFallbackAllowed,
				}).Error; e != nil {
					return e
				}
				if e = tx.Model(&model.PublicCheckRequest{}).Where("owner_id=? AND request_key=?", actor.UserID, in.RequestKey).Updates(map[string]any{
					"request_hash": digest, "reservation_id": use.ID,
				}).Error; e != nil {
					return e
				}
				result = checkClaim(v, a, use, false)
				return nil
			}
			if len(rows) < 20 {
				return errNoCheck
			}
			last := rows[len(rows)-1]
			afterTime, afterID = last.NextCheckAt, last.ID
		}
	})
	if errors.Is(err, errNoCheck) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func eligibleCheck(v model.Task, user int64, now time.Time) bool {
	return v.OperatorID == user && v.Status == model.StatusReviewing && (v.Platform == "baidu" || v.Platform == "bilibili") && v.CurrentAttemptID != nil &&
		v.PlatformSubmissionID != nil && v.NextCheckAt != nil && !v.NextCheckAt.After(now) && v.CheckStoppedAt == nil &&
		v.AcceptedAt != nil && now.Before(v.AcceptedAt.Add(publicCheckLifetime))
}
func (s ExecutionStore) checkCandidate(tx *gorm.DB, v model.Task, user int64, now time.Time) (model.Task, model.Attempt, error) {
	if !eligibleCheck(v, user, now) {
		return v, model.Attempt{}, ErrConflict
	}
	var a model.Attempt
	if err := tx.Where("id=? AND publication_id=? AND executor_user_id=?", v.CurrentAttemptID, v.ID, user).First(&a).Error; err != nil {
		return v, a, missing(err)
	}
	if safeDisposition(a) != model.StatusReviewing {
		return v, a, ErrConflict
	}
	in, err := s.readInput(tx, v)
	if err != nil {
		return v, a, err
	}
	if !reflect.DeepEqual(in, a.Input) {
		return v, a, ErrInputChanged
	}
	return v, a, nil
}
func checkClaim(v model.Task, a model.Attempt, use model.ProfileUse, replay bool) model.PublicCheckClaim {
	return model.PublicCheckClaim{Task: v, Attempt: a, Generation: *use.CheckGeneration, ExpectedRevision: *use.CheckRevision,
		BrowserFallbackAllowed: use.CheckBrowserFallbackAllowed, ReservationID: use.ID, ReservationState: "active", Replayed: replay}
}
func (s ExecutionStore) ReleasePublicCheck(actor model.CheckActor, task uint64, in model.PublicCheckRelease, now time.Time) (model.PublicCheckReleaseAck, error) {
	var out model.PublicCheckReleaseAck
	err := currentTransaction(s.DB, func(tx *gorm.DB) error {
		var intent model.PublicCheckRequest
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id=? AND reservation_id=?", actor.UserID, in.ReservationID).Take(&intent).Error
		if e != nil {
			return missing(e)
		}
		if !actor.Current && intent.AgentSessionID != actor.SessionID {
			return ErrNotFound
		}
		use, e := lockedCheckUse(tx, in.ReservationID, actor.UserID)
		if e != nil {
			return e
		}
		if use.PublicationID != task || use.CheckGeneration == nil || *use.CheckGeneration != in.Generation ||
			use.AttemptID == nil || *use.AttemptID != in.AttemptID {
			return ErrProfileUse
		}
		if use.ReleasedAt != nil {
			if use.Disposition == nil || *use.Disposition != in.ProfileDisposition {
				return ErrProfileUse
			}
		} else if e = releaseCheckUse(tx, &use, in.ProfileDisposition, nil, now); e != nil {
			return e
		}
		out = checkReleaseAck(use, false)
		out.ObservationHash = ""
		return nil
	})
	return out, err
}
