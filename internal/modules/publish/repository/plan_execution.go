package repository

import (
	"crypto/sha256"
	"encoding/json"

	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sort"
	"strconv"
	"time"
)

func lockPlanBatches(tx *gorm.DB, id uint64) ([]model.Batch, error) {
	var rows []model.Batch
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("plan_id=?", id).Order("id").Find(&rows).Error
	return rows, e
}
func orderedPlanBatches(rows []model.Batch) []model.Batch {
	out := append([]model.Batch(nil), rows...)
	sort.Slice(out, func(i, j int) bool {
		if *out[i].PlanGroupOrder != *out[j].PlanGroupOrder {
			return *out[i].PlanGroupOrder < *out[j].PlanGroupOrder
		}
		return *out[i].PlanBatchOrder < *out[j].PlanBatchOrder
	})
	return out
}
func confirmationStart(c model.PlanBatchConfirmation, key string) model.BatchStartInput {
	return model.BatchStartInput{RequestKey: key, ExpectedRevision: c.ExpectedRevision, IncludedRowKeys: c.IncludedRowKeys, ExcludedRowKeys: c.ExcludedRowKeys, AllowShortfall: c.AllowShortfall, ExpectedShortfalls: c.ExpectedShortfalls}
}
func planBatchKey(plan, batch uint64) string { return fmt.Sprintf("plan:%d:batch:%d", plan, batch) }
func planDigest(v any) string                { b, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(b)) }
func (s ExecutionStore) StartPlan(id uint64, actor int64, in model.StartPlanInput, digest string, prepared []model.Task, now time.Time) (model.StartPlanResult, error) {
	var out model.StartPlanResult
	now = now.UTC().Truncate(time.Microsecond)
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		if p.OwnerID != actor {
			return ErrNotFound
		}
		if p.StartRequestKey != nil {
			if *p.StartRequestKey != in.RequestKey || p.StartRequestHash == nil || *p.StartRequestHash != digest || len(p.StartReceipt) == 0 {
				return ErrConflict
			}
			if e = json.Unmarshal(p.StartReceipt, &out); e != nil {
				return e
			}
			if out.PlanID != p.ID || out.Revision == 0 || out.FirstStartedAt.IsZero() || len(out.FirstBatchReceipt.TaskIDs) == 0 {
				return ErrConflict
			}
			return nil
		}
		if p.Revision != in.ExpectedRevision {
			return ErrConflict
		}
		if p.Status != "not_started" || p.StartedAt != nil || p.StoppedAt != nil {
			return ErrParentBlocked
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		ordered := orderedPlanBatches(rows)
		confirmed := map[uint64]model.PlanBatchConfirmation{}
		for _, c := range in.BatchConfirmations {
			if _, ok := confirmed[c.BatchID]; ok {
				return ErrConflict
			}
			confirmed[c.BatchID] = c
		}
		var first *model.Batch
		outputs := map[string]struct {
			BatchID uint64
			RowKey  string
		}{}
		count := 0
		for i, b := range ordered {
			if b.ExecutionStatus == model.BatchCancelled {
				tasks, e := batchTasks(tx, b.ID)
				if e != nil {
					return e
				}
				for _, t := range tasks {
					if planTaskUnresolved(t, now) {
						return ErrUnresolved
					}
				}
				continue
			}
			if first == nil {
				first = &ordered[i]
			}
			count++
			c, ok := confirmed[b.ID]
			if !ok || b.Revision != c.ExpectedRevision || b.StartedAt != nil || b.ExecutionStatus != model.BatchNotStarted || b.ConfigurationStatus != model.BatchReady || !b.Readiness.Ready {
				return ErrConflict
			}
			selected, _, e := model.BatchStartSelection(b, confirmationStart(c, planBatchKey(id, b.ID)))
			if e != nil {
				return e
			}
			for _, row := range selected {
				key := b.Platform + ":" + strconv.FormatInt(row.CompositeOutputID, 10)
				if prior, exists := outputs[key]; exists {
					return &PlanConflictError{PlanID: p.ID, CurrentRevision: p.Revision, Issues: []PlanConflictIssue{{Code: "duplicate_plan_output", Message: "同一平台成片被两个批次纳入，请调整其中一行", BatchID: b.ID, RowKey: row.RowKey, RelatedBatchID: prior.BatchID, RelatedRowKey: prior.RowKey}}}
				}
				outputs[key] = struct {
					BatchID uint64
					RowKey  string
				}{b.ID, row.RowKey}
			}
			if e = tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(map[string]any{"plan_frozen_at": now, "frozen_confirmation_json": mustJSON(c)}).Error; e != nil {
				return e
			}
		}
		if first == nil || first.ID != in.FirstBatchID || count != len(confirmed) {
			return ErrConflict
		}
		c := confirmed[first.ID]
		bi := confirmationStart(c, planBatchKey(id, first.ID))
		receipt, e := s.startBatchTx(tx, *first, actor, bi, planDigest(bi), prepared, now)
		if e != nil {
			return e
		}
		out = model.StartPlanResult{PlanID: id, Revision: p.Revision + 1, FirstStartedAt: now, CurrentBatchID: first.ID, BatchConfirmations: in.BatchConfirmations, FirstBatchReceipt: receipt}
		if e = tx.Model(&model.Plan{}).Where("id=?", id).Updates(map[string]any{"status": "running", "started_at": now, "current_batch_id": first.ID, "revision": out.Revision, "start_request_key": in.RequestKey, "start_request_hash": digest, "start_receipt_json": mustJSON(out), "updated_at": now}).Error; e != nil {
			return e
		}
		p.Revision = out.Revision
		return auditPlanDetails(tx, p, actor, "publication_plan.start", out)
	})
	return out, e
}
func planTaskUnresolved(t model.Task, now time.Time) bool {
	switch t.Status {
	case model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain:
		return true
	}
	return t.VerificationRequiredAt(now)
}

type PlanProgress struct {
	Plan     model.Plan
	Batches  []model.Batch
	Tasks    []model.Task
	Basis    model.PlanHandoffBasis
	Progress string
	Current  *model.Batch
	Next     *model.Batch
}

func planProgress(tx *gorm.DB, p model.Plan, rows []model.Batch, now time.Time) (PlanProgress, error) {
	out := PlanProgress{Plan: p, Batches: rows, Tasks: []model.Task{}, Progress: "preparing"}
	ordered := orderedPlanBatches(rows)
	ids := []uint64{}
	for _, b := range ordered {
		if b.StartedAt != nil {
			ids = append(ids, b.ID)
		}
		if p.CurrentBatchID != nil && b.ID == *p.CurrentBatchID {
			b := b
			out.Current = &b
		}
	}
	if out.Current != nil {
		found := false
		for _, b := range ordered {
			if b.ID == out.Current.ID {
				found = true
				continue
			}
			if found && b.HandedOffAt == nil {
				b := b
				out.Next = &b
				break
			}
		}
	}
	if len(ids) > 0 {
		if e := tx.Where("batch_id IN ?", ids).Order("id").Find(&out.Tasks).Error; e != nil {
			return out, e
		}
	}
	public, e := verifiedPlanWorks(tx, ids)
	if e != nil {
		return out, e
	}
	for i, t := range out.Tasks {
		if t.Status == model.StatusPublished && !public[t.ID] {
			out.Tasks[i].VerificationRequired = true
		}
	}
	out.Basis.CurrentBatchID = p.CurrentBatchID
	if out.Current != nil {
		rev := out.Current.Revision
		out.Basis.CurrentBatchRevision = &rev
	}
	if out.Next != nil {
		id, rev := out.Next.ID, out.Next.Revision
		out.Basis.NextBatchID = &id
		out.Basis.NextBatchRevision = &rev
	}
	fingerprint := []any{p.ID, p.Revision, p.CurrentBatchID}
	for _, b := range ordered {
		fingerprint = append(fingerprint, []any{b.ID, b.Revision, b.HandedOffAt, b.ExecutionStatus, b.Pause})
	}
	unresolved := false
	verification := false
	for _, t := range out.Tasks {
		requires := t.VerificationRequiredAt(now)
		fingerprint = append(fingerprint, []any{t.ID, t.Revision, t.Status, t.CurrentAttemptID, t.CheckStoppedAt, t.AcceptedAt, t.LatestManualVerification, requires})
		if planTaskUnresolved(t, now) {
			unresolved = true
		}
		if requires {
			verification = true
		}
		if p.CurrentBatchID != nil && t.BatchID != nil && *t.BatchID == *p.CurrentBatchID {
			switch t.Status {
			case model.StatusFailed:
				if t.WorkID == nil {
					out.Basis.FailedCount++
					out.Basis.ExplicitCount++
				}
			case model.StatusReviewing, model.StatusPublished:
				out.Basis.ExplicitCount++
			}
		}
	}
	out.Basis.Fingerprint = planDigest(fingerprint)
	switch {
	case p.EndedAt != nil || p.Status == "ended":
		out.Progress = "ended"
	case p.StoppedAt != nil:
		out.Progress = "stopping"
	case p.StartedAt == nil:
		out.Progress = "preparing"
	case p.Status == "paused":
		out.Progress = "manual_paused"
	case verification:
		out.Progress = "waiting_verification"
	case unresolved:
		out.Progress = "waiting_tasks"
	case out.Current != nil && out.Current.ExecutionStatus == model.BatchPaused:
		out.Progress = "batch_paused"
	case out.Next == nil:
		out.Progress = "ready_to_finish"
	case out.Basis.ExplicitCount == 0:
		out.Progress = "waiting_skip"
	case out.Basis.FailedCount*10 > out.Basis.ExplicitCount:
		out.Progress = "waiting_continue"
	default:
		out.Progress = "running"
	}
	if out.Progress == "running" && p.ResourceWait != nil && p.ResourceWait.ExpectedRevision == p.Revision && out.Next != nil && p.CurrentBatchID != nil && p.ResourceWait.CurrentBatchID == *p.CurrentBatchID && p.ResourceWait.NextBatchID == out.Next.ID && p.ResourceWait.Basis == out.Basis.Fingerprint {
		out.Progress = "waiting_resources"
	}
	return out, nil
}
func (s Store) PlanProgress(id uint64, now time.Time) (PlanProgress, error) {
	var out PlanProgress
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		out, e = planProgress(tx, p, rows, s.planNow(now))
		return e
	})
	return out, e
}

type planAdvanceReceipt struct {
	Digest string                  `json:"digest"`
	Result model.PlanAdvanceResult `json:"result"`
}

func (s ExecutionStore) AdvancePlan(id uint64, actor int64, in model.PlanAdvanceInput, digest string, prepared []model.Task, automatic bool, now time.Time) (model.PlanAdvanceResult, error) {
	var out model.PlanAdvanceResult
	now = now.UTC().Truncate(time.Microsecond)
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		if p.OwnerID != actor {
			return ErrNotFound
		}
		receipts := map[string]planAdvanceReceipt{}
		if len(p.HandoffReceipts) > 0 {
			if e = json.Unmarshal(p.HandoffReceipts, &receipts); e != nil {
				return e
			}
		}
		if r, ok := receipts[in.RequestKey]; ok {
			if r.Digest != digest {
				return ErrConflict
			}
			out = r.Result
			return nil
		}
		if p.Revision != in.ExpectedRevision || p.CurrentBatchID == nil || *p.CurrentBatchID != in.CurrentBatchID {
			return ErrConflict
		}
		if p.Status != "running" || p.StoppedAt != nil || p.EndedAt != nil {
			return ErrParentBlocked
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		state, e := planProgress(tx, p, rows, s.planNow(now))
		if e != nil {
			return e
		}
		if state.Current == nil || state.Next == nil || state.Next.ID != in.NextBatchID || state.Basis.Fingerprint != in.ExpectedBasisFingerprint {
			return ErrConflict
		}
		if state.Progress == "waiting_tasks" || state.Progress == "waiting_verification" || state.Progress == "batch_paused" {
			return ErrUnresolved
		}
		if automatic && (state.Progress == "waiting_skip" || state.Progress == "waiting_continue") {
			return ErrParentBlocked
		}
		if !automatic {
			if state.Progress == "waiting_skip" && in.Decision != "skip" || state.Progress == "waiting_continue" && in.Decision != "continue" {
				return ErrConflict
			}
			if state.Progress != "waiting_skip" && state.Progress != "waiting_continue" {
				return ErrConflict
			}
		}
		next := *state.Next
		var receipt *model.BatchStartResult
		if next.ExecutionStatus != model.BatchCancelled {
			if next.FrozenConfirmation == nil || next.PlanFrozenAt == nil {
				return ErrConflict
			}
			bi := confirmationStart(*next.FrozenConfirmation, planBatchKey(id, next.ID))
			if bi.ExpectedRevision != next.Revision {
				return ErrConflict
			}
			r, startErr := s.startBatchTx(tx, next, actor, bi, planDigest(bi), prepared, now)
			e = startErr
			receipt = &r
			if e != nil {
				return e
			}
		}
		if e = tx.Model(&model.Batch{}).Where("id=?", state.Current.ID).Updates(map[string]any{"handed_off_at": now, "handoff_basis_fingerprint": state.Basis.Fingerprint, "revision": state.Current.Revision + 1, "updated_at": now}).Error; e != nil {
			return e
		}
		decision := in.Decision
		if automatic {
			decision = "automatic"
		}
		out = model.PlanAdvanceResult{PlanID: id, Revision: p.Revision + 1, PreviousBatchID: state.Current.ID, CurrentBatchID: next.ID, Decision: decision, AdvancedAt: now, BatchReceipt: receipt, CurrentBatchStatus: func() string {
			if receipt == nil {
				return model.BatchCancelled
			}
			return model.BatchRunning
		}()}
		receipts[in.RequestKey] = planAdvanceReceipt{Digest: digest, Result: out}
		if e = tx.Model(&model.Plan{}).Where("id=?", id).Updates(map[string]any{"current_batch_id": next.ID, "revision": out.Revision, "handoff_receipts_json": mustJSON(receipts), "resource_wait_json": nil, "updated_at": now}).Error; e != nil {
			return e
		}
		p.Revision = out.Revision
		return auditPlanDetails(tx, p, actor, "publication_plan.handoff", map[string]any{"basis": state.Basis, "decision": in.Decision, "reason": in.Reason, "result": out})
	})
	return out, e
}

// Whole-plan control is a fence, not evidence of browser closure.
func (s ExecutionStore) ControlPlan(id uint64, actor int64, revision uint64, action, reason string, expectedBatch uint64, batchRevision uint64, now time.Time) (model.PlanCancelResult, error) {
	out := model.PlanCancelResult{AffectedBatchIDs: []string{}, SettlingBatchIDs: []string{}}
	now = now.UTC().Truncate(time.Microsecond)
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		if p.OwnerID != actor {
			return ErrNotFound
		}
		if p.Revision != revision {
			return ErrConflict
		}
		if p.StoppedAt != nil || p.EndedAt != nil {
			return ErrParentBlocked
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		updates := map[string]any{"revision": p.Revision + 1, "updated_at": now}
		switch action {
		case "pause", "resume", "takeover":
			if p.StartedAt == nil {
				return ErrParentBlocked
			}
			if action == "resume" {
				if p.Status != "paused" {
					return ErrParentBlocked
				}
				updates["status"] = "running"
				updates["pause_reason"] = ""
			} else {
				if action == "takeover" {
					if p.CurrentBatchID == nil || *p.CurrentBatchID != expectedBatch {
						return ErrConflict
					}
					found := false
					for _, b := range rows {
						if b.ID == expectedBatch && b.Revision == batchRevision && model.CanTakeoverPlanBatch(p, b) {
							found = true
						}
					}
					if !found {
						return ErrConflict
					}
				} else if p.Status != "running" {
					return ErrParentBlocked
				}
				updates["status"] = "paused"
				updates["pause_reason"] = reason
			}
		case "cancel":
			updates["status"] = "stopping"
			updates["stopped_at"] = now
			updates["end_reason"] = "cancelled"
			updates["end_note"] = reason
			updates["resource_wait_json"] = nil
			settling := false
			for _, b := range rows {
				if b.ExecutionStatus == model.BatchEnded || b.ExecutionStatus == model.BatchCancelled {
					oldTasks, readErr := batchTasks(tx, b.ID)
					if readErr != nil {
						return readErr
					}
					oldUnresolved := false
					for _, task := range oldTasks {
						if pendingSettlement(task.Status) || task.Status == model.StatusPending || task.Status == model.StatusQueued {
							oldUnresolved = true
						}
					}
					if oldUnresolved {
						settling = true
						out.SettlingBatchIDs = append(out.SettlingBatchIDs, strconv.FormatUint(b.ID, 10))
					}
					continue
				}
				out.AffectedBatchIDs = append(out.AffectedBatchIDs, strconv.FormatUint(b.ID, 10))
				tasks, e := batchTasks(tx, b.ID)
				if e != nil {
					return e
				}
				active := false
				for _, t := range tasks {
					if t.CurrentAttemptID == nil && (t.Status == model.StatusPending || t.Status == model.StatusQueued) {
						if e = updateTask(tx, t, map[string]any{"status": model.StatusCancelled, "cancel_requested_at": now, "updated_at": now}); e != nil {
							return e
						}
					} else if pendingSettlement(t.Status) {
						active = true
						if t.CancelRequestedAt == nil {
							if e = updateTask(tx, t, map[string]any{"cancel_requested_at": now, "updated_at": now}); e != nil {
								return e
							}
						}
					}
				}
				status := model.BatchCancelled
				patch := map[string]any{"stopped_at": now, "stop_reason": reason, "pause_json": nil, "revision": b.Revision + 1, "updated_at": now}
				if active {
					status = model.BatchStopping
					settling = true
					out.SettlingBatchIDs = append(out.SettlingBatchIDs, strconv.FormatUint(b.ID, 10))
				} else {
					patch["ended_at"] = now
				}
				patch["execution_status"] = status
				if e = tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(patch).Error; e != nil {
					return e
				}
			}
			if !settling {
				updates["status"] = "ended"
				updates["ended_at"] = now
			}
		default:
			return ErrConflict
		}
		if e = tx.Model(&model.Plan{}).Where("id=?", id).Updates(updates).Error; e != nil {
			return e
		}
		p.Revision++
		if e = auditPlanDetails(tx, p, actor, "publication_plan."+action, map[string]any{"revision": p.Revision, "reason": reason, "current_batch_id": p.CurrentBatchID, "affected_batch_ids": out.AffectedBatchIDs, "settling_batch_ids": out.SettlingBatchIDs}); e != nil {
			return e
		}
		return tx.First(&out.Plan, id).Error
	})
	return out, e
}

func ReplayPlanStart(p model.Plan, key, digest string) (*model.StartPlanResult, error) {
	if p.StartRequestKey == nil {
		return nil, nil
	}
	if *p.StartRequestKey != key || p.StartRequestHash == nil || *p.StartRequestHash != digest {
		return nil, ErrConflict
	}
	var r model.StartPlanResult
	if json.Unmarshal(p.StartReceipt, &r) != nil || r.PlanID != p.ID || r.Revision == 0 || r.FirstStartedAt.IsZero() || len(r.FirstBatchReceipt.TaskIDs) == 0 {
		return nil, ErrConflict
	}
	return &r, nil
}
func ReplayPlanAdvance(p model.Plan, key, digest string) (*model.PlanAdvanceResult, error) {
	if len(p.HandoffReceipts) == 0 {
		return nil, nil
	}
	var rs map[string]planAdvanceReceipt
	if e := json.Unmarshal(p.HandoffReceipts, &rs); e != nil {
		return nil, e
	}
	r, ok := rs[key]
	if !ok {
		return nil, nil
	}
	if r.Digest != digest {
		return nil, ErrConflict
	}
	return &r.Result, nil
}
func (s Store) RunningPlanCandidates(after uint64, limit int) ([]model.Plan, error) {
	out := []model.Plan{}
	e := s.DB.Where("id>? AND status IN ('running','stopping')", after).Order("id").Limit(limit).Find(&out).Error
	return out, e
}

func (s Store) RecordPlanResourceWait(id uint64, wait model.PlanResourceWait) error {
	return currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		if p.Revision != wait.ExpectedRevision || p.CurrentBatchID == nil || *p.CurrentBatchID != wait.CurrentBatchID || p.Status != "running" || p.StoppedAt != nil || p.EndedAt != nil {
			return nil
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		state, e := planProgress(tx, p, rows, s.planNow(wait.CheckedAt))
		if e != nil {
			return e
		}
		if state.Next == nil || state.Next.ID != wait.NextBatchID || state.Basis.Fingerprint != wait.Basis {
			return nil
		}
		return tx.Model(&model.Plan{}).Where("id=?", id).Update("resource_wait_json", mustJSON(wait)).Error
	})
}

type PlanConflictIssue struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	BatchID        uint64 `json:"batch_id,string"`
	RowKey         string `json:"row_key"`
	RelatedBatchID uint64 `json:"related_batch_id,string"`
	RelatedRowKey  string `json:"related_row_key"`
}
type PlanConflictError struct {
	PlanID          uint64              `json:"plan_id,string"`
	CurrentRevision uint64              `json:"current_revision,string"`
	Issues          []PlanConflictIssue `json:"issues"`
}

func (e *PlanConflictError) Error() string { return ErrConflict.Error() }
func (e *PlanConflictError) Unwrap() error { return ErrConflict }

// Command timestamps remain audit facts; admission cannot use a clock captured
// before waiting for the parent fence. Tests can move only this internal clock.
func (s Store) planNow(event time.Time) time.Time {
	now := time.Now().UTC()
	if s.PlanClock != nil {
		now = s.PlanClock().UTC()
	}
	if event.After(now) {
		return event
	}
	return now
}
