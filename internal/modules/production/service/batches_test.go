package service

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"testing"
)

type batchTestStore struct {
	*taskMemoryStore
	batches                []model.ComposeTaskBatch
	ids                    []int64
	actionReads, pageReads int
	filter                 repository.ComposeTaskFilter
	page, size             int
}

func (s *batchTestStore) FindBatch(id int64) (model.ComposeTaskBatch, bool, error) {
	for _, b := range s.batches {
		if b.ID == id {
			return b, true, nil
		}
	}
	return model.ComposeTaskBatch{}, false, nil
}
func (s *batchTestStore) ListPage(f repository.ComposeTaskFilter, p, n int) (repository.ComposeTaskPage, error) {
	s.pageReads++
	s.filter = f
	s.page = p
	s.size = n
	return repository.ComposeTaskPage{Items: []model.ComposeTask{}, Page: p, PageSize: n}, nil
}
func (s *batchTestStore) ListBatchPage(f repository.ComposeTaskFilter, p, n int) (repository.ComposeBatchPage, error) {
	s.pageReads++
	s.filter = f
	s.page = p
	s.size = n
	return repository.ComposeBatchPage{Items: append([]model.ComposeTaskBatch(nil), s.batches...), Page: p, PageSize: n}, nil
}
func (s *batchTestStore) BatchTaskIDs(int64) ([]int64, error) { return s.ids, nil }
func (s *batchTestStore) BatchActionCounts(int64) (int, int, error) {
	s.actionReads++
	return 2, 3, nil
}

func batchServiceFixture() (*ComposeTaskService, *batchTestStore, identity.PublicUser) {
	svc, tasks, _, _, actor := composeTaskFixture()
	store := &batchTestStore{taskMemoryStore: tasks, batches: []model.ComposeTaskBatch{{ID: 8, TeamID: 7, CategoryID: 1, OperatorID: operatorIDPtr(9), CreatedBy: 9}}}
	svc.tasks = store
	return svc, store, actor
}
func batchTask(id, owner int64, status model.ComposeTaskStatus) model.ComposeTask {
	return model.ComposeTask{ID: id, TeamID: 7, CategoryID: 1, OperatorID: operatorIDPtr(owner), CreatedBy: owner, Status: status}
}

func TestBatchReadHidesOtherOwnersAndOrdinaryRetryActions(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	detail, err := svc.GetBatch(actor, 8)
	if err != nil || detail.Actions.RetryCount != 0 || detail.Actions.CancelCount != 3 {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	store.batches[0].OperatorID = operatorIDPtr(10)
	if _, err = svc.GetBatch(actor, 8); !errors.Is(err, ErrComposeTaskNotFound) || store.actionReads != 1 {
		t.Fatalf("crossowner err=%v reads=%d", err, store.actionReads)
	}
	actor.Role = identity.RoleSeniorOperator
	if _, err = svc.GetBatch(actor, 8); err != nil {
		t.Fatal(err)
	}
	team := identity.TeamID(99)
	actor.TeamID = &team
	if _, err = svc.GetBatch(actor, 8); !errors.Is(err, ErrComposeTaskNotFound) {
		t.Fatalf("crossteam=%v", err)
	}
}
func TestBulkValidatesEveryIDBeforeAnyMutation(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	store.tasks = []model.ComposeTask{batchTask(1, 9, model.ComposeTaskPending)}
	for _, input := range []ComposeBulkInput{{TaskIDs: []int64{1, 0}}, {TaskIDs: []int64{1, 1}}, {BatchID: 8, TaskIDs: []int64{1}}, {BatchID: -1, TaskIDs: []int64{1}}, {}} {
		if _, err := svc.Bulk(actor, input, false); !errors.Is(err, ErrComposeTaskInvalid) {
			t.Fatalf("input=%+v err=%v", input, err)
		}
		if store.cancels != 0 || store.tasks[0].Status != model.ComposeTaskPending {
			t.Fatal("invalid command partially mutated")
		}
	}
}
func TestBulkOrdinaryCancelOwnRunningRemainsRunningAndOtherOwnerHidden(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	store.tasks = []model.ComposeTask{batchTask(1, 9, model.ComposeTaskPending), batchTask(2, 9, model.ComposeTaskRunning), batchTask(3, 10, model.ComposeTaskPending)}
	got, err := svc.Bulk(actor, ComposeBulkInput{TaskIDs: []int64{1, 2, 3}}, false)
	if err != nil || got.Accepted != 2 || got.Skipped != 1 || got.Failed != 0 {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if store.tasks[0].Status != model.ComposeTaskCancelled || store.tasks[1].Status != model.ComposeTaskRunning || !store.tasks[1].CancelRequested || store.tasks[2].Status != model.ComposeTaskPending {
		t.Fatalf("tasks=%+v", store.tasks)
	}
	if got.Items[2].Reason != "任务不可见" || got.Items[2].Task != nil {
		t.Fatalf("crossowner leaked %+v", got.Items[2])
	}
}
func TestBulkRetryRespectsRoleAndUncertainGuard(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	store.tasks = []model.ComposeTask{batchTask(1, 9, model.ComposeTaskFailed), batchTask(2, 9, model.ComposeTaskFailed), batchTask(3, 9, model.ComposeTaskCancelled)}
	store.tasks[1].ErrorCode = "execution_uncertain"
	got, err := svc.Bulk(actor, ComposeBulkInput{TaskIDs: []int64{1, 2, 3}}, true)
	if err != nil || got.Accepted != 0 || got.Skipped != 3 || store.retriedBy != 0 {
		t.Fatalf("ordinary=%+v err=%v", got, err)
	}
	actor.Role = identity.RoleSeniorOperator
	got, err = svc.Bulk(actor, ComposeBulkInput{TaskIDs: []int64{1, 2, 3}}, true)
	if err != nil || got.Accepted != 2 || got.Skipped != 1 || store.tasks[1].Status != model.ComposeTaskFailed || store.tasks[1].ErrorCode != "execution_uncertain" {
		t.Fatalf("senior=%+v err=%v tasks=%+v", got, err, store.tasks)
	}
}
func TestBulkWholeBatchUsesAllIDsIndependentOfPageAndStatus(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	store.tasks = []model.ComposeTask{batchTask(1, 9, model.ComposeTaskPending), batchTask(2, 9, model.ComposeTaskRunning), batchTask(3, 9, model.ComposeTaskSuccess)}
	store.ids = []int64{1, 2, 3}
	if _, err := svc.ListPage(actor, ListComposeTasksInput{BatchID: 8, Status: "pending", Page: 4, PageSize: 1}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Bulk(actor, ComposeBulkInput{BatchID: 8}, false)
	if err != nil || len(result.Items) != 3 || result.Accepted != 2 || result.Skipped != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func TestBatchPageBoundsAndPermissionScope(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	if _, err := svc.ListBatches(actor, ListComposeTasksInput{Page: -1}); !errors.Is(err, ErrComposeTaskInvalid) || store.pageReads != 0 {
		t.Fatalf("invalid page err=%v reads=%d", err, store.pageReads)
	}
	if _, err := svc.ListPage(actor, ListComposeTasksInput{Page: 2, PageSize: 5}); err != nil {
		t.Fatal(err)
	}
	if store.filter.TeamID != 7 || store.filter.OperatorID != 9 || len(store.filter.CategoryIDs) != 1 || store.filter.CategoryIDs[0] != 1 || store.page != 2 || store.size != 5 {
		t.Fatalf("filter=%+v page=%d size=%d", store.filter, store.page, store.size)
	}
}
func TestBatchListsDoNotReadStoreWithoutAnyCategoryScope(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	actor.CategoryIDs = nil
	tasks, err := svc.ListPage(actor, ListComposeTasksInput{})
	if err != nil || len(tasks.Items) != 0 {
		t.Fatalf("tasks=%+v err=%v", tasks, err)
	}
	batches, err := svc.ListBatches(actor, ListComposeTasksInput{})
	if err != nil || len(batches.Items) != 0 {
		t.Fatalf("batches=%+v err=%v", batches, err)
	}
	if store.pageReads != 0 {
		t.Fatalf("unscoped category query executed %d times", store.pageReads)
	}
}

func (s *batchTestStore) BatchActionSkips(int64) ([]repository.ComposeActionSkip, []repository.ComposeActionSkip, error) {
	return []repository.ComposeActionSkip{{Reason: "结果不明确，需人工核实", Count: 1}, {Reason: "当前状态不可重试", Count: 2}}, []repository.ComposeActionSkip{{Reason: "已结束", Count: 2}}, nil
}
func TestBatchActionReasonsPreserveRoleAndWholeBatchCounts(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	store.batches[0].Counts = model.ComposeBatchCounts{Pending: 3, Failed: 2}
	ordinary, err := svc.GetBatch(actor, 8)
	if err != nil || ordinary.Actions.RetryCount != 0 || len(ordinary.Actions.RetrySkipped) != 1 || ordinary.Actions.RetrySkipped[0].Count != 5 || ordinary.Actions.RetrySkipped[0].Reason != "无操作权限" || len(ordinary.Actions.CancelSkipped) != 1 {
		t.Fatalf("ordinary=%+v err=%v", ordinary, err)
	}
	actor.Role = identity.RoleSeniorOperator
	senior, err := svc.GetBatch(actor, 8)
	if err != nil || senior.Actions.RetryCount != 2 || len(senior.Actions.RetrySkipped) != 2 || senior.Actions.RetrySkipped[0].Reason != "结果不明确，需人工核实" {
		t.Fatalf("senior=%+v err=%v", senior, err)
	}
}

func TestBatchNamesAreDecoratedOnDetailAndLists(t *testing.T) {
	svc, store, actor := batchServiceFixture()
	store.batches[0].StrategyID = 5
	svc.operators = func() ([]identity.PublicUser, error) {
		return []identity.PublicUser{{ID: 9, Nickname: "负责人九号"}}, nil
	}
	detail, err := svc.GetBatch(actor, 8)
	if err != nil || detail.Batch.OperatorName != "负责人九号" || detail.Batch.StrategyName != "切段" {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	page, err := svc.ListBatches(actor, ListComposeTasksInput{})
	if err != nil || len(page.Items) != 1 || page.Items[0].OperatorName != "负责人九号" || page.Items[0].StrategyName != "切段" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
