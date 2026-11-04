package service

import (
	"errors"
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type composeBatchStore interface {
	FindBatch(int64) (model.ComposeTaskBatch, bool, error)
	ListPage(repository.ComposeTaskFilter, int, int) (repository.ComposeTaskPage, error)
	ListBatchPage(repository.ComposeTaskFilter, int, int) (repository.ComposeBatchPage, error)
	BatchTaskIDs(int64) ([]int64, error)
	BatchActionCounts(int64) (int, int, error)
}
type ComposeBatchActions struct {
	RetryCount    int                            `json:"retry_count"`
	CancelCount   int                            `json:"cancel_count"`
	RetrySkipped  []repository.ComposeActionSkip `json:"retry_skipped,omitempty"`
	CancelSkipped []repository.ComposeActionSkip `json:"cancel_skipped,omitempty"`
}
type ComposeBatchDetail struct {
	Batch   model.ComposeTaskBatch `json:"batch"`
	Actions ComposeBatchActions    `json:"actions"`
}

// Marshal is intentionally avoided here: metadata is nested so batch's own secret redaction always applies.
func taskPageBounds(page, size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return 0, 0, fmt.Errorf("%w: invalid pagination", ErrComposeTaskInvalid)
	}
	return page, size, nil
}
func (s *ComposeTaskService) ListPage(actor identity.PublicUser, input ListComposeTasksInput) (repository.ComposeTaskPage, error) {
	f, err := composeTaskFilter(actor, input)
	if err != nil {
		return repository.ComposeTaskPage{}, err
	}
	p, n, err := taskPageBounds(input.Page, input.PageSize)
	if err != nil {
		return repository.ComposeTaskPage{}, err
	}
	if actor.Role == identity.RoleOperator && len(actor.CategoryIDs) == 0 {
		return repository.ComposeTaskPage{Items: []model.ComposeTask{}, Page: p, PageSize: n}, nil
	}
	store, ok := s.tasks.(composeBatchStore)
	if !ok {
		return repository.ComposeTaskPage{}, errors.New("batch store unavailable")
	}
	out, err := store.ListPage(f, p, n)
	if err == nil {
		out.Items, err = s.withOperatorNames(out.Items)
	}
	return out, err
}
func (s *ComposeTaskService) ListBatches(actor identity.PublicUser, input ListComposeTasksInput) (repository.ComposeBatchPage, error) {
	f, err := composeTaskFilter(actor, input)
	if err != nil {
		return repository.ComposeBatchPage{}, err
	}
	p, n, err := taskPageBounds(input.Page, input.PageSize)
	if err != nil {
		return repository.ComposeBatchPage{}, err
	}
	if actor.Role == identity.RoleOperator && len(actor.CategoryIDs) == 0 {
		return repository.ComposeBatchPage{Items: []model.ComposeTaskBatch{}, Page: p, PageSize: n}, nil
	}
	store, ok := s.tasks.(composeBatchStore)
	if !ok {
		return repository.ComposeBatchPage{}, errors.New("batch store unavailable")
	}
	out, err := store.ListBatchPage(f, p, n)
	if err == nil {
		out.Items, err = s.decorateBatchNames(out.Items)
	}
	return out, err
}
func (s *ComposeTaskService) GetBatch(actor identity.PublicUser, id int64) (ComposeBatchDetail, error) {
	store, ok := s.tasks.(composeBatchStore)
	if !ok {
		return ComposeBatchDetail{}, errors.New("batch store unavailable")
	}
	b, found, err := store.FindBatch(id)
	if err != nil {
		return ComposeBatchDetail{}, err
	}
	if !found || !canReadComposeTask(actor, model.ComposeTask{TeamID: b.TeamID, CategoryID: b.CategoryID, OperatorID: b.OperatorID, CreatedBy: b.CreatedBy}) {
		return ComposeBatchDetail{}, ErrComposeTaskNotFound
	}
	retry, cancel, err := store.BatchActionCounts(id)
	if err != nil {
		return ComposeBatchDetail{}, err
	}
	retrySkipped, cancelSkipped := []repository.ComposeActionSkip{}, []repository.ComposeActionSkip{}
	if details, ok := store.(interface {
		BatchActionSkips(int64) ([]repository.ComposeActionSkip, []repository.ComposeActionSkip, error)
	}); ok {
		retrySkipped, cancelSkipped, err = details.BatchActionSkips(id)
		if err != nil {
			return ComposeBatchDetail{}, err
		}
	}
	if actor.Role != identity.RoleAdmin && actor.Role != identity.RoleSeniorOperator {
		retry = 0
		retrySkipped = []repository.ComposeActionSkip{}
		if b.Counts.Total() > 0 {
			retrySkipped = append(retrySkipped, repository.ComposeActionSkip{Reason: "无操作权限", Count: b.Counts.Total()})
		}
	}
	decorated, err := s.decorateBatchNames([]model.ComposeTaskBatch{b})
	if err != nil {
		return ComposeBatchDetail{}, err
	}
	return ComposeBatchDetail{Batch: decorated[0], Actions: ComposeBatchActions{RetryCount: retry, CancelCount: cancel, RetrySkipped: retrySkipped, CancelSkipped: cancelSkipped}}, nil
}

type ComposeBulkInput struct {
	BatchID int64   `json:"batch_id"`
	TaskIDs []int64 `json:"task_ids"`
}
type ComposeBulkItem struct {
	TaskID int64              `json:"task_id"`
	Status string             `json:"status"`
	Reason string             `json:"reason,omitempty"`
	Task   *model.ComposeTask `json:"task,omitempty"`
}
type ComposeBulkResult struct {
	Items    []ComposeBulkItem `json:"items"`
	Accepted int               `json:"accepted"`
	Skipped  int               `json:"skipped"`
	Failed   int               `json:"failed"`
}

func (s *ComposeTaskService) Bulk(actor identity.PublicUser, input ComposeBulkInput, retry bool) (ComposeBulkResult, error) {
	out := ComposeBulkResult{Items: []ComposeBulkItem{}}
	ids := input.TaskIDs
	if (input.BatchID > 0) == (len(ids) > 0) || input.BatchID < 0 || len(ids) > 200 {
		return out, fmt.Errorf("%w: choose batch_id or 1-200 task_ids", ErrComposeTaskInvalid)
	}
	if input.BatchID > 0 {
		if _, err := s.GetBatch(actor, input.BatchID); err != nil {
			return out, err
		}
		var err error
		ids, err = s.tasks.(composeBatchStore).BatchTaskIDs(input.BatchID)
		if err != nil {
			return out, err
		}
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return out, fmt.Errorf("%w: invalid or duplicate task id", ErrComposeTaskInvalid)
		}
		seen[id] = true
	}
	for _, id := range ids {
		item := ComposeBulkItem{TaskID: id, Status: "skipped"}
		task, err := s.Get(actor, id)
		if err == nil {
			if retry {
				if actor.Role != identity.RoleAdmin && actor.Role != identity.RoleSeniorOperator {
					err = ErrComposeTaskForbidden
				} else if task.Status != model.ComposeTaskCancelled && (task.Status != model.ComposeTaskFailed || task.ErrorCode == "execution_uncertain") {
					err = ErrComposeTaskNotRetryable
				}
			} else if task.Status != model.ComposeTaskPending && (task.Status != model.ComposeTaskRunning || task.CancelRequested) {
				err = ErrComposeTaskNotCancellable
			}
		}
		if err == nil {
			if retry {
				task, err = s.Retry(actor, id)
			} else {
				task, err = s.Cancel(actor, id)
			}
		}
		if err == nil {
			item.Status = "accepted"
			item.Task = &task
			out.Accepted++
		} else {
			switch {
			case errors.Is(err, ErrComposeTaskForbidden):
				item.Reason = "无操作权限"
			case errors.Is(err, ErrComposeTaskNotFound):
				item.Reason = "任务不可见"
			case errors.Is(err, ErrComposeTaskNotRetryable):
				item.Reason = "当前状态不可重试"
			case errors.Is(err, ErrComposeTaskNotCancellable):
				item.Reason = "已结束或已请求取消"
			default:
				item.Status = "failed"
				item.Reason = "操作失败，请重试"
			}
			if item.Status == "skipped" {
				out.Skipped++
			} else {
				out.Failed++
			}
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func ListComposeTaskBatches(actor identity.PublicUser, input ListComposeTasksInput) (repository.ComposeBatchPage, error) {
	return wiredComposeTaskService().ListBatches(actor, input)
}
func GetComposeTaskBatch(actor identity.PublicUser, id int64) (ComposeBatchDetail, error) {
	return wiredComposeTaskService().GetBatch(actor, id)
}
func BulkComposeTasks(actor identity.PublicUser, input ComposeBulkInput, retry bool) (ComposeBulkResult, error) {
	return wiredComposeTaskService().Bulk(actor, input, retry)
}
func ListComposeTasksPage(actor identity.PublicUser, input ListComposeTasksInput) (repository.ComposeTaskPage, error) {
	return wiredComposeTaskService().ListPage(actor, input)
}

// Names are current presentation labels; immutable batch ownership and command
// identity remain the stored IDs and request hash.
func (s *ComposeTaskService) decorateBatchNames(items []model.ComposeTaskBatch) ([]model.ComposeTaskBatch, error) {
	ids := []int64{}
	for _, batch := range items {
		if batch.OperatorID != nil {
			ids = append(ids, *batch.OperatorID)
		}
	}
	names, err := composeOperatorNameLookup(s.operators, ids)
	if err != nil {
		return nil, err
	}
	strategies := map[int64]string{}
	for i := range items {
		if items[i].OperatorID != nil {
			items[i].OperatorName = names[*items[i].OperatorID]
		}
		strategyID := items[i].StrategyID
		if strategyID <= 0 || s.strategies == nil {
			continue
		}
		name, seen := strategies[strategyID]
		if !seen {
			strategy, found, err := s.strategies.Find(strategyID)
			if err != nil {
				return nil, err
			}
			if found {
				name = strategy.Name
			}
			strategies[strategyID] = name
		}
		items[i].StrategyName = name
	}
	return items, nil
}
