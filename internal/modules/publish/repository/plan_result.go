package repository

import (
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

// Matched work rows are authoritative; a status string or foreign work ID cannot
// count toward a plan's public goal.
func verifiedPlanWorks(tx *gorm.DB, ids []uint64) (map[uint64]bool, error) {
	out := map[uint64]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	var matched []uint64
	e := tx.Table("publication_task t").Select("t.id").Joins("JOIN work w ON w.id=t.work_id AND w.publication_id=t.id AND w.attempt_id=t.current_attempt_id AND w.platform=t.platform AND w.operator_id=t.operator_id AND w.composite_output_id=t.composite_output_id").Where("t.batch_id IN ? AND t.status=?", ids, model.StatusPublished).Scan(&matched).Error
	for _, id := range matched {
		out[id] = true
	}
	return out, e
}
func (s ExecutionStore) FinishPlan(id uint64, actor int64, in model.FinishPlanInput, event time.Time) (model.Plan, error) {
	var out model.Plan
	if in.ExpectedRevision == 0 || in.EndReason != "achieved" && in.EndReason != "unmet" || in.EndReason == "unmet" && strings.TrimSpace(in.EndNote) == "" || utf8.RuneCountInString(in.EndNote) > 2000 || len(in.ExpectedShortfalls) == 0 {
		return out, ErrConflict
	}
	e := currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		if p.OwnerID != actor {
			return ErrNotFound
		}
		if p.Revision != in.ExpectedRevision {
			return ErrConflict
		}
		if p.StartedAt == nil || p.StoppedAt != nil || p.EndedAt != nil {
			return ErrParentBlocked
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		now := s.planNow(event)
		state, e := planProgress(tx, p, rows, now)
		if e != nil {
			return e
		}
		ids := []uint64{}
		for _, b := range rows {
			ids = append(ids, b.ID)
			if b.ExecutionStatus != model.BatchCancelled && (b.StartedAt == nil || b.HandedOffAt == nil && (p.CurrentBatchID == nil || b.ID != *p.CurrentBatchID) && b.ExecutionStatus != model.BatchEnded) {
				return ErrUnresolved
			}
		}
		for _, t := range state.Tasks {
			if planTaskUnresolved(t, now) || t.Status == model.StatusReviewing {
				return ErrUnresolved
			}
		}
		works, e := verifiedPlanWorks(tx, ids)
		if e != nil {
			return e
		}
		counts := map[string]int64{}
		key := func(platform string, category int64) string { return fmt.Sprintf("%s/%d", platform, category) }
		for _, t := range state.Tasks {
			if t.Status == model.StatusPublished && !works[t.ID] {
				return ErrUnresolved
			}
			if works[t.ID] {
				counts[key(t.Platform, t.CategoryID)]++
			}
		}
		expected := map[string]int64{}
		for _, g := range in.ExpectedShortfalls {
			k := key(g.Platform, g.CategoryID)
			if _, ok := expected[k]; ok || g.Shortfall < 0 {
				return ErrConflict
			}
			expected[k] = g.Shortfall
		}
		if len(expected) != len(p.Goals) {
			return ErrConflict
		}
		unmet := false
		for _, g := range p.Goals {
			gap := max(0, g.TargetCount-counts[key(g.Platform, g.CategoryID)])
			got, ok := expected[key(g.Platform, g.CategoryID)]
			if !ok || got != gap {
				return ErrConflict
			}
			unmet = unmet || gap > 0
		}
		if unmet && in.EndReason != "unmet" || !unmet && in.EndReason != "achieved" {
			return ErrConflict
		}
		for _, b := range rows {
			if b.ExecutionStatus == model.BatchRunning || b.ExecutionStatus == model.BatchPaused {
				if e = tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(map[string]any{"execution_status": model.BatchEnded, "ended_at": now, "finish_reason": in.EndNote, "pause_json": nil, "revision": b.Revision + 1, "updated_at": now}).Error; e != nil {
					return e
				}
			}
		}
		if e = tx.Model(&model.Plan{}).Where("id=?", id).Updates(map[string]any{"status": "ended", "ended_at": now, "end_reason": in.EndReason, "end_note": in.EndNote, "resource_wait_json": nil, "revision": p.Revision + 1, "updated_at": now}).Error; e != nil {
			return e
		}
		p.Revision++
		if e = auditPlanDetails(tx, p, actor, "publication_plan.finish", in); e != nil {
			return e
		}
		return tx.First(&out, id).Error
	})
	if len(out.DateStart) > 10 {
		out.DateStart = out.DateStart[:10]
	}
	if len(out.DateEnd) > 10 {
		out.DateEnd = out.DateEnd[:10]
	}
	return out, e
}

// Cancellation needs safe execution settlement, not public success. Reviewing
// receipts remain available for original verification after the plan has ended.
func (s ExecutionStore) ReconcileStoppedPlan(id uint64, event time.Time) error {
	return currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, e := lockPlan(tx, id)
		if e != nil {
			return e
		}
		if p.StoppedAt == nil || p.EndedAt != nil {
			return nil
		}
		rows, e := lockPlanBatches(tx, id)
		if e != nil {
			return e
		}
		now := s.planNow(event)
		for _, b := range rows {
			tasks, e := batchTasks(tx, b.ID)
			if e != nil {
				return e
			}
			for _, t := range tasks {
				switch t.Status {
				case model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain:
					return nil
				}
			}
			if b.ExecutionStatus == model.BatchStopping {
				if e = tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(map[string]any{"execution_status": model.BatchCancelled, "ended_at": now, "revision": b.Revision + 1, "updated_at": now}).Error; e != nil {
					return e
				}
			}
		}
		if e = tx.Model(&model.Plan{}).Where("id=?", id).Updates(map[string]any{"status": "ended", "ended_at": now, "revision": p.Revision + 1, "resource_wait_json": nil, "updated_at": now}).Error; e != nil {
			return e
		}
		p.Revision++
		return auditPlan(tx, p, 0, "publication_plan.cancel_settled")
	})
}
