package repository

import (
	"crypto/sha256"
	"errors"
	"fmt"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// ValidatePlanStructure checks per-cell conservation and canonical platform groups.
// Authorization is additionally performed by the owning Service before this store.
func ValidatePlanStructure(p model.Plan, rows []model.PlanStructureBatch) error {
	if p.OwnerID <= 0 || p.TeamID <= 0 || p.CreatedBy <= 0 || strings.TrimSpace(p.Name) == "" || len(rows) == 0 || len(rows) > 1000 {
		return ErrConflict
	}
	start, e := time.Parse("2006-01-02", p.DateStart)
	if e != nil {
		return ErrConflict
	}
	end, e := time.Parse("2006-01-02", p.DateEnd)
	if e != nil || end.Before(start) {
		return ErrConflict
	}
	goals := map[string]int64{}
	sums := map[string]int64{}
	keys := map[string]bool{}
	ids := map[uint64]bool{}
	platforms := map[string]bool{}
	cell := func(platform string, category int64) string { return fmt.Sprintf("%s/%d", platform, category) }
	for _, g := range p.Goals {
		key := cell(g.Platform, g.CategoryID)
		if g.Platform != "baidu" || g.CategoryID <= 0 || g.TargetCount <= 0 || goals[key] != 0 {
			return ErrConflict
		}
		goals[key] = g.TargetCount
	}
	var group, order uint32
	platform := ""
	for _, r := range rows {
		if r.RowKey == "" || len(r.RowKey) > 64 || keys[r.RowKey] || strings.TrimSpace(r.Name) == "" || utf8.RuneCountInString(r.Name) > 100 || len(r.Targets) == 0 {
			return ErrConflict
		}
		keys[r.RowKey] = true
		if r.BatchID != nil {
			if *r.BatchID == 0 || ids[*r.BatchID] {
				return ErrConflict
			}
			ids[*r.BatchID] = true
		}
		if r.Platform != platform {
			if platforms[r.Platform] || r.GroupOrder != group+1 || r.BatchOrder != 1 {
				return ErrConflict
			}
			platforms[r.Platform] = true
			platform = r.Platform
			group = r.GroupOrder
			order = 0
		}
		if r.GroupOrder != group || r.BatchOrder != order+1 {
			return ErrConflict
		}
		order = r.BatchOrder
		seen := map[int64]bool{}
		for _, g := range r.Targets {
			key := cell(r.Platform, g.CategoryID)
			if seen[g.CategoryID] || g.TargetCount <= 0 || goals[key] == 0 || g.TargetCount > goals[key]-sums[key] {
				return ErrConflict
			}
			seen[g.CategoryID] = true
			sums[key] += g.TargetCount
		}
	}
	if !reflect.DeepEqual(goals, sums) {
		return ErrConflict
	}
	return nil
}
func lockPlan(tx *gorm.DB, id uint64) (model.Plan, error) {
	var p model.Plan
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, id).Error
	return p, missing(e)
}

// Association is immutable: take the parent before a batch row in every writer.
func lockBatchPlan(tx *gorm.DB, id uint64) (*model.Plan, error) {
	var b struct{ PlanID *uint64 }
	if e := tx.Model(&model.Batch{}).Select("plan_id").Where("id=?", id).Take(&b).Error; e != nil {
		return nil, missing(e)
	}
	if b.PlanID == nil {
		return nil, nil
	}
	p, e := lockPlan(tx, *b.PlanID)
	return &p, e
}
func (s Store) GetPlan(id uint64) (model.Plan, []model.Batch, error) {
	var p model.Plan
	if e := s.DB.First(&p, id).Error; e != nil {
		return p, nil, missing(e)
	}
	if len(p.DateStart) > 10 {
		p.DateStart = p.DateStart[:10]
	}
	if len(p.DateEnd) > 10 {
		p.DateEnd = p.DateEnd[:10]
	}
	var b []model.Batch
	e := s.DB.Where("plan_id=?", id).Order("plan_group_order,plan_batch_order,id").Find(&b).Error
	if e == nil {
		b, e = s.enrichBatches(b, map[uint64]model.Plan{p.ID: p})
	}
	return p, b, e
}
func (s Store) PlanByRequest(actor int64, key string) (model.Plan, []model.Batch, error) {
	var p model.Plan
	if e := s.DB.Where("created_by=? AND request_key=?", actor, key).First(&p).Error; e != nil {
		return p, nil, missing(e)
	}
	return s.GetPlan(p.ID)
}
func auditPlan(tx *gorm.DB, p model.Plan, actor int64, event string) error {
	return auditPlanDetails(tx, p, actor, event, map[string]any{"revision": p.Revision})
}
func auditPlanDetails(tx *gorm.DB, p model.Plan, actor int64, event string, details any) error {
	return tx.Exec("INSERT INTO audit_logs(id,actor_user_id,action,target_type,target_id,summary_json,created_at) VALUES(?,?,?,'publication_plan',?,?,?)", id.NewID("audit"), actor, event, fmt.Sprint(p.ID), mustJSON(details), time.Now().UTC()).Error
}

func (s Store) CreatePlan(p model.Plan, rows []model.PlanStructureBatch) (model.Plan, []model.Batch, error) {
	if prior, b, e := s.PlanByRequest(p.CreatedBy, p.RequestKey); e == nil {
		if prior.RequestHash != p.RequestHash {
			return model.Plan{}, nil, ErrConflict
		}
		return prior, b, nil
	} else if !errors.Is(e, ErrNotFound) {
		return model.Plan{}, nil, e
	}
	if e := ValidatePlanStructure(p, rows); e != nil {
		return model.Plan{}, nil, e
	}
	for _, r := range rows {
		if r.BatchID != nil {
			return model.Plan{}, nil, ErrConflict
		}
	}
	p.Revision = 1
	p.Status = "not_started"
	p.CurrentBatchID = nil
	p.StartedAt = nil
	p.StoppedAt = nil
	p.EndedAt = nil
	p.StartRequestKey = nil
	p.StartRequestHash = nil
	p.StartReceipt = nil
	p.HandoffReceipts = nil
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&p).Error; e != nil {
			return e
		}
		if e := s.writePlanRows(tx, p, nil, rows); e != nil {
			return e
		}
		return auditPlan(tx, p, p.CreatedBy, "publication_plan.create")
	})
	if e != nil {
		var dup *mysql.MySQLError
		if errors.As(e, &dup) && dup.Number == 1062 {
			prior, b, re := s.PlanByRequest(p.CreatedBy, p.RequestKey)
			if re == nil && prior.RequestHash == p.RequestHash {
				return prior, b, nil
			}
			if re == nil {
				return model.Plan{}, nil, ErrConflict
			}
		}
		return model.Plan{}, nil, e
	}
	return s.GetPlan(p.ID)
}
func (s Store) EditPlanStructure(p model.Plan, revision uint64, rows []model.PlanStructureBatch) (model.Plan, []model.Batch, error) {
	if e := ValidatePlanStructure(p, rows); e != nil {
		return model.Plan{}, nil, e
	}
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		prior, e := lockPlan(tx, p.ID)
		if e != nil {
			return e
		}
		if prior.OwnerID != p.OwnerID || prior.TeamID != p.TeamID {
			return ErrNotFound
		}
		if prior.StartedAt != nil || prior.StoppedAt != nil || prior.Status != "not_started" {
			return ErrParentBlocked
		}
		if prior.Revision != revision {
			return ErrConflict
		}
		var old []model.Batch
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("plan_id=?", p.ID).Order("id").Find(&old).Error; e != nil {
			return e
		}
		if e = s.writePlanRows(tx, p, old, rows); e != nil {
			return e
		}
		p.Revision = revision + 1
		p.UpdatedAt = time.Now().UTC()
		if e = tx.Model(&model.Plan{}).Where("id=? AND revision=?", p.ID, revision).Updates(map[string]any{"name": p.Name, "date_start": p.DateStart, "date_end": p.DateEnd, "note": p.Note, "goals_json": mustJSON(p.Goals), "revision": p.Revision, "updated_at": p.UpdatedAt}).Error; e != nil {
			return e
		}
		return auditPlan(tx, p, p.OwnerID, "publication_plan.structure")
	})
	if e != nil {
		return model.Plan{}, nil, e
	}
	return s.GetPlan(p.ID)
}
func (s Store) writePlanRows(tx *gorm.DB, p model.Plan, old []model.Batch, rows []model.PlanStructureBatch) error {
	byID := map[uint64]model.Batch{}
	keep := map[uint64]bool{}
	for _, b := range old {
		byID[b.ID] = b
	}
	for _, r := range rows {
		if r.BatchID != nil {
			b, ok := byID[*r.BatchID]
			if !ok || b.PlanRowKey == nil || *b.PlanRowKey != r.RowKey {
				return ErrNotFound
			}
			keep[b.ID] = true
			if b.StartedAt != nil || b.ExecutionStatus != model.BatchNotStarted {
				if b.Name != r.Name || b.Platform != r.Platform || !reflect.DeepEqual(b.Targets, r.Targets) || b.PlanGroupOrder == nil || *b.PlanGroupOrder != r.GroupOrder || b.PlanBatchOrder == nil || *b.PlanBatchOrder != r.BatchOrder {
					return ErrParentBlocked
				}
			}
		}
	}
	// Clear positions under the same plan lock so legal permutations do not violate unique order mid-update.
	if len(old) > 0 {
		if e := tx.Model(&model.Batch{}).Where("plan_id=?", p.ID).Updates(map[string]any{"plan_group_order": nil, "plan_batch_order": nil}).Error; e != nil {
			return e
		}
	}
	for _, b := range old {
		if !keep[b.ID] {
			if b.StartedAt != nil || b.ExecutionStatus != model.BatchNotStarted {
				return ErrParentBlocked
			}
			if e := tx.Where("batch_id=?", b.ID).Delete(&batchFile{}).Error; e != nil {
				return e
			}
			if e := tx.Delete(&model.Batch{}, b.ID).Error; e != nil {
				return e
			}
		}
	}
	now := time.Now().UTC()
	for _, r := range rows {
		if r.BatchID == nil {
			b := model.Batch{PlanID: &p.ID, PlanRowKey: &r.RowKey, PlanGroupOrder: &r.GroupOrder, PlanBatchOrder: &r.BatchOrder, TeamID: p.TeamID, OperatorID: p.OwnerID, CreatedBy: p.CreatedBy, Name: r.Name, Platform: r.Platform, Targets: r.Targets, Config: model.BatchConfig{Candidates: []model.BatchCandidate{}}, Readiness: model.BatchReadiness{Issues: []model.BatchIssue{}, Shortfalls: []model.BatchShortfall{}, CoverResources: []model.BatchCoverResource{}}, ConfigurationStatus: model.BatchIncomplete, ExecutionStatus: model.BatchNotStarted, ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 1, BetweenGroupPolicy: "pause_on_exception", ExceptionTaskLimit: 1}, Revision: 1, RequestKey: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("plan:%d:%s", p.ID, r.RowKey)))), RequestHash: p.RequestHash, CreatedAt: now, UpdatedAt: now}
			if e := tx.Create(&b).Error; e != nil {
				return e
			}
			if e := auditBatch(tx, b, "publication_batch.plan_skeleton"); e != nil {
				return e
			}
			continue
		}
		b := byID[*r.BatchID]
		changes := map[string]any{"name": r.Name, "platform": r.Platform, "targets_json": mustJSON(r.Targets), "plan_group_order": r.GroupOrder, "plan_batch_order": r.BatchOrder, "revision": b.Revision + 1, "updated_at": now}
		if b.Platform != r.Platform || !reflect.DeepEqual(b.Targets, r.Targets) {
			b.Readiness.Ready = false
			b.Readiness.Issues = []model.BatchIssue{{Field: "targets", Code: "plan_scope_changed", Message: "计划目标已变化，请重新检查候选配置"}}
			for i := range b.Config.Candidates {
				b.Config.Candidates[i].Content.Confirmed = false
			}
			changes["configuration_status"] = model.BatchIncomplete
			changes["plan_configured_at"] = nil
			changes["readiness_json"] = mustJSON(b.Readiness)
			changes["config_json"] = mustJSON(b.Config)
		}
		if e := tx.Model(&model.Batch{}).Where("id=?", b.ID).Updates(changes).Error; e != nil {
			return e
		}
	}
	return nil
}
