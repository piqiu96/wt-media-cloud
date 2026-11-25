package repository

import (
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"strings"
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
func (s Store) PlanBatchPage(id uint64, page, size int, status string, pending *bool) ([]model.Batch, int64, error) {
	q := s.DB.Model(&model.Batch{}).Where("plan_id=?", id)
	if status != "" {
		q = q.Where("execution_status=?", status)
	}
	if pending != nil {
		sql := `EXISTS(SELECT 1 FROM publication_task t WHERE t.batch_id=publication_batch.id AND t.status IN ('pending','queued','running','awaiting_human','uncertain'))`
		if !*pending {
			sql = "NOT " + sql
		}
		q = q.Where(sql)
	}
	var n int64
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	rows := []model.Batch{}
	e := q.Order("plan_group_order,plan_batch_order,id").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	for i := range rows {
		if e == nil {
			rows[i], e = s.enrichBatch(rows[i])
		}
	}
	return rows, n, e
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
	for _, b := range rows {
		if b.PlanID != nil {
			out[*b.PlanID] = append(out[*b.PlanID], b)
		}
	}
	return out, e
}
