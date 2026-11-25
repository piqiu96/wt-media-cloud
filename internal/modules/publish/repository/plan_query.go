package repository

import (
	"database/sql"
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"strings"
	"time"
)

type PlanFilter struct {
	Actor                                identity.PublicUser
	Page, PageSize                       int
	Search, Status, Platform, RequestKey string
	CategoryID                           int64
}

func planScope(q *gorm.DB, a identity.PublicUser) *gorm.DB {
	if a.Role == identity.RoleAdmin {
		return q
	}
	if a.Role == identity.RoleSeniorOperator && a.TeamID != nil {
		q = q.Where("team_id=?", *a.TeamID)
	} else {
		q = q.Where("owner_id=?", a.ID)
	}
	if len(a.CategoryIDs) == 0 {
		return q.Where("1=0")
	}
	return q.Where(`NOT EXISTS(SELECT 1 FROM JSON_TABLE(publication_plan.goals_json,'$[*]' COLUMNS(category_id BIGINT PATH '$.category_id')) goal WHERE goal.category_id NOT IN ?)`, a.CategoryIDs)
}
func (s Store) ListPlans(f PlanFilter) ([]model.Plan, int64, error) {
	q := planScope(s.DB.Model(&model.Plan{}), f.Actor)
	if f.RequestKey != "" {
		q = q.Where("created_by=? AND request_key=?", f.Actor.ID, f.RequestKey)
	} else {
		if f.Search != "" {
			escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(f.Search)
			q = q.Where("CAST(id AS CHAR)=? OR name LIKE ? ESCAPE '!'", f.Search, "%"+escaped+"%")
		}
		if f.Status != "" {
			q = q.Where("status=?", f.Status)
		}
		if f.Platform != "" || f.CategoryID > 0 {
			clause := `EXISTS(SELECT 1 FROM JSON_TABLE(goals_json,'$[*]' COLUMNS(platform VARCHAR(32) PATH '$.platform',category_id BIGINT PATH '$.category_id')) goal WHERE 1=1`
			args := []any{}
			if f.Platform != "" {
				clause += " AND goal.platform=?"
				args = append(args, f.Platform)
			}
			if f.CategoryID > 0 {
				clause += " AND goal.category_id=?"
				args = append(args, f.CategoryID)
			}
			q = q.Where(clause+")", args...)
		}
	}
	var count int64
	if e := q.Count(&count).Error; e != nil {
		return nil, 0, e
	}
	rows := []model.Plan{}
	e := q.Order("created_at DESC,id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&rows).Error
	for i := range rows {
		if len(rows[i].DateStart) > 10 {
			rows[i].DateStart = rows[i].DateStart[:10]
		}
		if len(rows[i].DateEnd) > 10 {
			rows[i].DateEnd = rows[i].DateEnd[:10]
		}
	}
	return rows, count, e
}

// One minimal task projection reuses the same verification predicate as progress.
// Matching public work follows verifiedPlanWorks' original task/attempt/output scope.
func planPendingBatchIDs(tx *gorm.DB, id uint64, now time.Time) ([]uint64, error) {
	var facts []struct {
		model.Task
		MatchedPublic bool `gorm:"column:matched_public"`
	}
	e := tx.Table("publication_task AS t").Select("t.id,t.batch_id,t.status,t.accepted_at,t.current_attempt_id,t.check_stopped_at,t.latest_manual_verification_json,t.revision,t.work_id,w.id IS NOT NULL AS matched_public").
		Joins("JOIN publication_batch b ON b.id=t.batch_id AND b.plan_id=?", id).
		Joins("LEFT JOIN work w ON w.id=t.work_id AND w.publication_id=t.id AND w.attempt_id=t.current_attempt_id AND w.platform=t.platform AND w.operator_id=t.operator_id AND w.composite_output_id=t.composite_output_id").
		Where("t.status IN ?", []model.Status{model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain, model.StatusReviewing, model.StatusPublished}).Find(&facts).Error
	if e != nil {
		return nil, e
	}
	ids := []uint64{}
	seen := map[uint64]bool{}
	for _, fact := range facts {
		v := fact.Task
		if v.Status == model.StatusPublished && !fact.MatchedPublic {
			v.VerificationRequired = true
		}
		if v.BatchID != nil && planTaskUnresolved(v, now) && !seen[*v.BatchID] {
			seen[*v.BatchID] = true
			ids = append(ids, *v.BatchID)
		}
	}
	return ids, nil
}
func (s Store) PlanBatchPage(id uint64, page, size int, status string, pending *bool) (rows []model.Batch, n int64, e error) {
	rows = []model.Batch{}
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		now := s.planNow(time.Now().UTC())
		q := tx.Model(&model.Batch{}).Where("plan_id=?", id)
		if status != "" {
			q = q.Where("execution_status=?", status)
		}
		if pending != nil {
			ids, err := planPendingBatchIDs(tx, id, now)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				if *pending {
					q = q.Where("1=0")
				}
			} else if *pending {
				q = q.Where("id IN ?", ids)
			} else {
				q = q.Where("id NOT IN ?", ids)
			}
		}
		if err := q.Count(&n).Error; err != nil {
			return err
		}
		if err := q.Order("plan_group_order,plan_batch_order,id").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
			return err
		}
		reader := s
		reader.DB = tx
		var err error
		rows, err = reader.enrichBatches(rows, nil)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return
}
func (s Store) ListPlanRecords(id uint64, page, size int) ([]model.PlanRecord, int64, error) {
	q := s.DB.Table("audit_logs").Where("target_type='publication_plan' AND target_id=?", fmt.Sprint(id))
	var n int64
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	rows := []model.PlanRecord{}
	e := q.Select("id,actor_user_id AS actor_id,action AS event,created_at,summary_json AS details").Order("created_at DESC,id DESC").Offset((page - 1) * size).Limit(size).Scan(&rows).Error
	return rows, n, e
}

// Page-local parents are loaded together rather than one batch query per list row.
func (s Store) PlanRowsByIDs(ids []uint64) (map[uint64][]model.Batch, error) {
	out := map[uint64][]model.Batch{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []model.Batch
	e := s.DB.Where("plan_id IN ?", ids).Order("plan_id,plan_group_order,plan_batch_order,id").Find(&rows).Error
	if e == nil {
		rows, e = s.enrichBatches(rows, nil)
	}
	for _, b := range rows {
		if b.PlanID != nil {
			out[*b.PlanID] = append(out[*b.PlanID], b)
		}
	}
	return out, e
}
