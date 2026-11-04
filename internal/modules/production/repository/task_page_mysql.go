package repository

import (
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
	"strings"
)

type ComposeTaskPage struct {
	Items    []model.ComposeTask `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}
type ComposeBatchPage struct {
	Items    []model.ComposeTaskBatch `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

func composeScopeWhere(f ComposeTaskFilter) (string, []any) {
	q := ` WHERE 1=1`
	a := []any{}
	if f.TeamID > 0 {
		q += ` AND team_id=?`
		a = append(a, f.TeamID)
	}
	if f.OperatorID > 0 {
		if f.IncludeUnownedCreatedBy == f.OperatorID {
			q += ` AND (operator_id=? OR (operator_id IS NULL AND created_by=?))`
			a = append(a, f.OperatorID, f.OperatorID)
		} else {
			q += ` AND operator_id=?`
			a = append(a, f.OperatorID)
		}
	}
	if len(f.CategoryIDs) > 0 {
		q += ` AND category_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(f.CategoryIDs)), ",") + `)`
		for _, v := range f.CategoryIDs {
			a = append(a, v)
		}
	}
	if f.StrategyID > 0 {
		q += ` AND compose_strategy_id=?`
		a = append(a, f.StrategyID)
	}
	if f.TriggerType != "" {
		q += ` AND trigger_type=?`
		a = append(a, f.TriggerType)
	}
	if !f.CreatedFrom.IsZero() {
		q += ` AND created_at>=?`
		a = append(a, f.CreatedFrom)
	}
	if !f.CreatedBefore.IsZero() {
		q += ` AND created_at<?`
		a = append(a, f.CreatedBefore)
	}
	return q, a
}
func taskPageWhere(f ComposeTaskFilter) (string, []any) {
	q, a := composeScopeWhere(f)
	if f.Status != "" {
		q += ` AND status=?`
		a = append(a, f.Status)
	}
	if f.BatchID > 0 {
		q += ` AND batch_id=?`
		a = append(a, f.BatchID)
	}
	if f.Search != "" {
		q += ` AND (display_name LIKE ? OR CAST(id AS CHAR)=? OR CAST(material_id AS CHAR)=? OR CAST(batch_id AS CHAR)=?)`
		a = append(a, "%"+f.Search+"%", f.Search, f.Search, f.Search)
	}
	return q, a
}
func (ComposeTaskStore) ListPage(f ComposeTaskFilter, page, size int) (ComposeTaskPage, error) {
	return listComposeTaskPage(database.DB(), f, page, size)
}
func listComposeTaskPage(db *gorm.DB, f ComposeTaskFilter, page, size int) (ComposeTaskPage, error) {
	out := ComposeTaskPage{Items: []model.ComposeTask{}, Page: page, PageSize: size}
	q, a := taskPageWhere(f)
	if err := db.Raw(`SELECT COUNT(*) FROM compose_tasks`+q, a...).Row().Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := db.Raw(`SELECT `+composeTaskColumns+` FROM compose_tasks`+q+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(a, size, (page-1)*size)...).Rows()
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		v, err := scanComposeTask(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, v)
	}
	return out, rows.Err()
}
func (ComposeTaskStore) ListBatchPage(f ComposeTaskFilter, page, size int) (ComposeBatchPage, error) {
	return listComposeBatchPage(database.DB(), f, page, size)
}
func listComposeBatchPage(db *gorm.DB, f ComposeTaskFilter, page, size int) (ComposeBatchPage, error) {
	out := ComposeBatchPage{Items: []model.ComposeTaskBatch{}, Page: page, PageSize: size}
	q, a := composeScopeWhere(f)
	if f.BatchID > 0 {
		q += ` AND id=?`
		a = append(a, f.BatchID)
	}
	if f.Search != "" {
		q += ` AND (name LIKE ? OR CAST(id AS CHAR)=?)`
		a = append(a, "%"+f.Search+"%", f.Search)
	}
	if f.Status != "" {
		q += ` AND EXISTS (SELECT 1 FROM compose_tasks t WHERE t.batch_id=compose_task_batches.id AND t.status=?)`
		a = append(a, f.Status)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM compose_task_batches`+q, a...).Row().Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := db.Raw(`SELECT id FROM compose_task_batches`+q+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(a, size, (page-1)*size)...).Rows()
	if err != nil {
		return out, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		b, found, err := findComposeTaskBatch(db, id)
		if err != nil {
			return out, err
		}
		if !found {
			return out, fmt.Errorf("batch %d disappeared", id)
		}
		out.Items = append(out.Items, b)
	}
	return out, nil
}
func (ComposeTaskStore) BatchTaskIDs(id int64) ([]int64, error) {
	rows, err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE batch_id=? ORDER BY id`, id).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (ComposeTaskStore) BatchActionCounts(id int64) (retry, cancel int, err error) {
	err = database.DB().Raw(`SELECT COALESCE(SUM(status='cancelled' OR (status='failed' AND error_code<>'execution_uncertain')),0),COALESCE(SUM(status='pending' OR (status='running' AND cancel_requested=0)),0) FROM compose_tasks WHERE batch_id=?`, id).Row().Scan(&retry, &cancel)
	return
}

type ComposeActionSkip struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

func (ComposeTaskStore) BatchActionSkips(id int64) ([]ComposeActionSkip, []ComposeActionSkip, error) {
	return batchActionSkips(database.DB(), id)
}
func batchActionSkips(db *gorm.DB, id int64) (retry, cancel []ComposeActionSkip, err error) {
	retry = []ComposeActionSkip{}
	cancel = []ComposeActionSkip{}
	var uncertain, wrongRetryState, cancelRequested, terminal int
	err = db.Raw(`SELECT COALESCE(SUM(status='failed' AND error_code='execution_uncertain'),0),COALESCE(SUM(status NOT IN ('failed','cancelled')),0),COALESCE(SUM(status='running' AND cancel_requested=1),0),COALESCE(SUM(status NOT IN ('pending','running')),0) FROM compose_tasks WHERE batch_id=?`, id).Row().Scan(&uncertain, &wrongRetryState, &cancelRequested, &terminal)
	if err != nil {
		return
	}
	add := func(items *[]ComposeActionSkip, reason string, n int) {
		if n > 0 {
			*items = append(*items, ComposeActionSkip{Reason: reason, Count: n})
		}
	}
	add(&retry, "结果不明确，需人工核实", uncertain)
	add(&retry, "当前状态不可重试", wrongRetryState)
	add(&cancel, "已请求取消", cancelRequested)
	add(&cancel, "已结束", terminal)
	return
}
