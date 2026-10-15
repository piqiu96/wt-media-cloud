package service

import (
	"encoding/json"
	"errors"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type taskMemoryStore struct {
	tasks          []model.ComposeTask
	usages         []model.MaterialUsage
	attempts       []model.ComposeTaskAttempt
	nextID         int64
	creates        int
	cancels        int
	cancelBlocked  bool
	requests       int
	requestBlocked bool
	retryBlocked   bool
	retriedBy      int64
	lastFilter     repository.ComposeTaskFilter
}

func (s *taskMemoryStore) Find(id int64) (model.ComposeTask, bool, error) {
	for _, item := range s.tasks {
		if item.ID == id {
			return item, true, nil
		}
	}
	return model.ComposeTask{}, false, nil
}

func (s *taskMemoryStore) FindByRequestKey(key string) (model.ComposeTask, bool, error) {
	for _, item := range s.tasks {
		if item.RequestKey == key {
			return item, true, nil
		}
	}
	return model.ComposeTask{}, false, nil
}

func (s *taskMemoryStore) List(filter repository.ComposeTaskFilter) ([]model.ComposeTask, error) {
	s.lastFilter = filter
	return s.tasks, nil
}

// CreateForUsage mirrors the repository contract: the usage relation is checked
// inside the write transaction, and found=false means nothing was written.
func (s *taskMemoryStore) CreateForUsage(item model.ComposeTask, _ string) (model.ComposeTask, bool, error) {
	for _, usage := range s.usages {
		if usage.UserID == identity.UserID(item.CreatedBy) && usage.MaterialID == item.MaterialID && usage.Status == model.MaterialUsageActive {
			s.creates++
			s.nextID++
			item.ID = s.nextID
			item.MaterialUsageID = usage.ID
			s.tasks = append(s.tasks, item)
			return item, true, nil
		}
	}
	return model.ComposeTask{}, false, nil
}

func (s *taskMemoryStore) CancelAudited(item model.ComposeTask, _ int64, _ string) (bool, error) {
	s.cancels++
	if s.cancelBlocked {
		return false, nil
	}
	for i := range s.tasks {
		if s.tasks[i].ID == item.ID && s.tasks[i].Status == model.ComposeTaskPending {
			s.tasks[i].Status = model.ComposeTaskCancelled
			return true, nil
		}
	}
	return false, nil
}

// RequestComposeCancel mirrors the repository contract: only a running row
// without a flag yet accepts the request, and the flag lands on the row. The
// counter is writes, not calls — a refused request must leave no trace.
func (s *taskMemoryStore) RequestComposeCancel(taskID, _ int64, _ string) (bool, error) {
	if s.requestBlocked {
		return false, nil
	}
	for i := range s.tasks {
		if s.tasks[i].ID == taskID && s.tasks[i].Status == model.ComposeTaskRunning && !s.tasks[i].CancelRequested {
			s.tasks[i].CancelRequested = true
			s.requests++
			return true, nil
		}
	}
	return false, nil
}

// RetryFailed mirrors the repository contract: only a failed task outside the
// refusals is requeued — the fake flips status pending, keeps attempt_count
// and grows max_attempts, and records the actor for the audit assertion.
func (s *taskMemoryStore) RetryFailed(item model.ComposeTask, actorID int64, _ string) (bool, error) {
	if s.retryBlocked {
		return false, nil
	}
	for i := range s.tasks {
		if s.tasks[i].ID != item.ID || s.tasks[i].Status != model.ComposeTaskFailed {
			continue
		}
		switch s.tasks[i].ErrorCode {
		case "storage_error", "task_timeout", "execution_failed":
			s.tasks[i].Status = model.ComposeTaskPending
			s.tasks[i].MaxAttempts++
			s.tasks[i].Stage = ""
			s.tasks[i].Progress = 0
			s.retriedBy = actorID
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func (s *taskMemoryStore) ListAttempts(taskID int64) ([]model.ComposeTaskAttempt, error) {
	return s.attempts, nil
}

type taskMaterialMemoryStore struct {
	items []model.Material
}

func (s *taskMaterialMemoryStore) FindMaterial(id int64) (model.Material, bool, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return model.Material{}, false, nil
}

func composeTaskFixture() (*ComposeTaskService, *taskMemoryStore, *strategyMemoryStore, *taskMaterialMemoryStore, identity.PublicUser) {
	team := identity.TeamID(7)
	definition := executor.BasicCutV1()
	strategies := &strategyMemoryStore{
		items:    []model.ComposeStrategy{{ID: 5, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "切段", Config: json.RawMessage(`{"start_seconds":0,"end_seconds":10}`), Status: "active", CreatedBy: 9}},
		template: model.ComposeTemplate{ID: 3, Code: definition.Code, Version: definition.Version, Schema: definition.Schema, ExecutorID: definition.ExecutorID, Status: "active"},
	}
	materials := &taskMaterialMemoryStore{items: []model.Material{
		{ID: 42, TeamID: 7, GameID: strPtr("delta"), Title: "素材一"},
		{ID: 43, TeamID: 7, GameID: strPtr("delta"), Title: "素材二"},
	}}
	tasks := &taskMemoryStore{usages: []model.MaterialUsage{
		{ID: 91, TeamID: 7, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive},
		{ID: 92, TeamID: 7, MaterialID: 43, UserID: 9, Status: model.MaterialUsageActive},
		{ID: 93, TeamID: 7, MaterialID: 42, UserID: 10, Status: model.MaterialUsageActive},
	}}
	svc := NewComposeTaskService(tasks, strategies, materials)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	return svc, tasks, strategies, materials, actor
}

func strPtr(s string) *string { return &s }

func composeTaskSnapshot(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("snapshot %s is not an object: %v", raw, err)
	}
	return out
}

func TestCreateComposeTasksCreatesPerItemWithMergedSnapshot(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	input := CreateComposeTasksInput{
		StrategyID:     5,
		ConfigOverride: json.RawMessage(`{"start_seconds":5}`),
		Items: []CreateComposeTaskItemInput{
			{MaterialID: 42, RequestKey: "rk-aaaa-0001"},
			{MaterialID: 43, RequestKey: "rk-aaaa-0002"},
		},
	}
	res, err := svc.Create(actor, input)
	if err != nil || len(res.Created) != 2 || len(res.Failed) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if tasks.creates != 2 {
		t.Fatalf("creates = %d", tasks.creates)
	}
	first := res.Created[0]
	if first.DisplayName != "素材一 · 切段" {
		t.Fatalf("task name must capture creation-time material and strategy: %q", first.DisplayName)
	}
	if first.MaterialID != 42 || first.MaterialUsageID != 91 || first.StrategyID != 5 || first.TeamID != 7 || first.GameID != "delta" || first.TemplateCode != "basic_cut" || first.TemplateVersion != 1 || first.Status != model.ComposeTaskPending || first.CreatedBy != 9 {
		t.Fatalf("first task facts drifted: %+v", first)
	}
	snapshot := composeTaskSnapshot(t, first.ConfigSnapshot)
	if snapshot["start_seconds"] != float64(5) || snapshot["end_seconds"] != float64(10) {
		t.Fatalf("merge did not layer override over strategy config: %v", snapshot)
	}
	if second := res.Created[1]; second.MaterialID != 43 || second.MaterialUsageID != 92 {
		t.Fatalf("second task facts drifted: %+v", second)
	}
}

func TestGuideTaskFreezesStrategyAddressAndRejectsTaskAddressOverride(t *testing.T) {
	svc, tasks, strategies, _, actor := composeTaskFixture()
	definition := executor.GuidePIPV1()
	strategies.template = model.ComposeTemplate{ID: 3, Code: definition.Code, Version: 1, Schema: definition.Schema, ExecutorID: definition.ExecutorID, Status: "active"}
	strategies.items[0].Config = json.RawMessage(`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":2,"guide_duration_seconds":0,"max_duration_seconds":0}`)
	input := CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: "rk-guide-0001"}}}
	result, err := svc.Create(actor, input)
	if err != nil || len(result.Created) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	strategies.items[0].Config = json.RawMessage(`{"guide_video_url":"https://media.example.com/changed.mp4","insert_seconds":2,"guide_duration_seconds":0,"max_duration_seconds":0}`)
	if got := composeTaskSnapshot(t, result.Created[0].ConfigSnapshot)["guide_video_url"]; got != "https://media.example.com/guide.mp4" {
		t.Fatalf("snapshot address = %v", got)
	}
	input.ConfigOverride = json.RawMessage(`{"guide_video_url":"https://media.example.com/other.mp4"}`)
	input.Items[0].RequestKey = "rk-guide-0002"
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrComposeTaskInvalid) || tasks.creates != 1 {
		t.Fatalf("override err=%v writes=%d", err, tasks.creates)
	}
}

func TestCreateComposeTasksMergedOverrideMustMatchSchema(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	input := CreateComposeTasksInput{
		StrategyID:     5,
		ConfigOverride: json.RawMessage(`{"bgm":"x"}`),
		Items:          []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: "rk-schema-01"}},
	}
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrComposeTaskInvalid) {
		t.Fatalf("schema-external override error = %v", err)
	}
	if tasks.creates != 0 {
		t.Fatalf("rejected command wrote %d tasks", tasks.creates)
	}
}

func TestCreateComposeTasksNamedFailuresAreZeroWrite(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*taskMemoryStore, *strategyMemoryStore, *taskMaterialMemoryStore)
		check  func(*testing.T, CreateComposeTasksResult, error, *taskMemoryStore)
	}{
		{
			name: "removed usage",
			mutate: func(tasks *taskMemoryStore, _ *strategyMemoryStore, _ *taskMaterialMemoryStore) {
				tasks.usages[0].Status = model.MaterialUsageRemoved
			},
			check: func(t *testing.T, res CreateComposeTasksResult, err error, tasks *taskMemoryStore) {
				if err != nil || len(res.Failed) != 1 || len(res.Created) != 0 {
					t.Fatalf("res=%+v err=%v", res, err)
				}
				if !errors.Is(res.Failed[0].Err, ErrComposeTaskInvalid) || res.Failed[0].Message == "" {
					t.Fatalf("failure = %+v", res.Failed[0])
				}
			},
		},
		{
			name: "material outside team scope",
			mutate: func(_ *taskMemoryStore, _ *strategyMemoryStore, materials *taskMaterialMemoryStore) {
				materials.items[0].TeamID = 8
			},
			check: func(t *testing.T, res CreateComposeTasksResult, err error, _ *taskMemoryStore) {
				if err != nil || len(res.Failed) != 1 || !errors.Is(res.Failed[0].Err, ErrComposeTaskForbidden) {
					t.Fatalf("res=%+v err=%v", res, err)
				}
			},
		},
		{
			name: "material game does not match strategy game",
			mutate: func(_ *taskMemoryStore, _ *strategyMemoryStore, materials *taskMaterialMemoryStore) {
				materials.items[0].GameID = strPtr("echo")
			},
			check: func(t *testing.T, res CreateComposeTasksResult, err error, _ *taskMemoryStore) {
				if err != nil || len(res.Failed) != 1 || !errors.Is(res.Failed[0].Err, ErrComposeTaskInvalid) {
					t.Fatalf("res=%+v err=%v", res, err)
				}
			},
		},
		{
			name: "inactive strategy is named for the operator",
			mutate: func(_ *taskMemoryStore, strategies *strategyMemoryStore, _ *taskMaterialMemoryStore) {
				strategies.items[0].Status = "inactive"
			},
			check: func(t *testing.T, res CreateComposeTasksResult, err error, _ *taskMemoryStore) {
				if !errors.Is(err, ErrComposeTaskNotFound) {
					t.Fatalf("inactive strategy error = %v", err)
				}
			},
		},
		{
			name: "strategy out of scope",
			mutate: func(_ *taskMemoryStore, strategies *strategyMemoryStore, _ *taskMaterialMemoryStore) {
				strategies.items[0].TeamID = 8
			},
			check: func(t *testing.T, res CreateComposeTasksResult, err error, _ *taskMemoryStore) {
				if !errors.Is(err, ErrComposeTaskNotFound) {
					t.Fatalf("out-of-scope strategy error = %v", err)
				}
			},
		},
		{
			name: "template version unavailable",
			mutate: func(_ *taskMemoryStore, strategies *strategyMemoryStore, _ *taskMaterialMemoryStore) {
				strategies.template.Status = "inactive"
			},
			check: func(t *testing.T, res CreateComposeTasksResult, err error, _ *taskMemoryStore) {
				if !errors.Is(err, ErrStrategyTemplateUnavailable) {
					t.Fatalf("unavailable template error = %v", err)
				}
			},
		},
	}
	keys := []string{"rk-usage-001", "rk-scope-001", "rk-game-001", "rk-inact-001", "rk-oscope-01", "rk-tmpl-001"}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, tasks, strategies, materials, actor := composeTaskFixture()
			tc.mutate(tasks, strategies, materials)
			res, err := svc.Create(actor, CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: keys[i]}}})
			tc.check(t, res, err, tasks)
			if tasks.creates != 0 {
				t.Fatalf("rejected command still wrote %d tasks", tasks.creates)
			}
		})
	}
}

func TestCreateComposeTasksSameRequestKeyIsIdempotent(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	input := CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: "rk-retry-001"}}}
	first, err := svc.Create(actor, input)
	if err != nil || len(first.Created) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := svc.Create(actor, input)
	if err != nil || len(second.Created) != 1 || second.Created[0].ID != first.Created[0].ID {
		t.Fatalf("second=%+v first=%+v err=%v", second, first, err)
	}
	if tasks.creates != 1 {
		t.Fatalf("retry created %d tasks", tasks.creates)
	}
}

func TestCreateComposeTasksRejectsKeyClaimedByAnotherUser(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	tasks.tasks = append(tasks.tasks, model.ComposeTask{ID: 500, TeamID: 7, GameID: "delta", Status: model.ComposeTaskPending, RequestKey: "rk-taken-001", CreatedBy: 9})
	other := actor
	other.ID = 10
	res, err := svc.Create(other, CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: "rk-taken-001"}}})
	if err != nil || len(res.Failed) != 1 || !errors.Is(res.Failed[0].Err, ErrComposeTaskForbidden) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if tasks.creates != 0 {
		t.Fatalf("foreign request key still wrote %d tasks", tasks.creates)
	}
}

func TestCreateComposeTasksDuplicateKeysInOneRequestRejected(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	input := CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{
		{MaterialID: 42, RequestKey: "rk-dupes-001"},
		{MaterialID: 43, RequestKey: "rk-dupes-001"},
	}}
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrComposeTaskInvalid) {
		t.Fatalf("duplicate key error = %v", err)
	}
	if tasks.creates != 0 {
		t.Fatalf("duplicate keys still wrote %d tasks", tasks.creates)
	}
}

func TestCreateComposeTasksSnapshotFrozenAfterStrategyEdit(t *testing.T) {
	svc, tasks, strategies, materials, actor := composeTaskFixture()
	first, err := svc.Create(actor, CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: "rk-frozen-001"}}})
	if err != nil || len(first.Created) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	strategies.items[0].Config = json.RawMessage(`{"start_seconds":8,"end_seconds":9}`)
	strategies.items[0].Name = "新策略名"
	materials.items[0].Title = "新素材名"
	stored, found, err := tasks.Find(first.Created[0].ID)
	if err != nil || !found {
		t.Fatalf("stored=%+v found=%v err=%v", stored, found, err)
	}
	if string(stored.ConfigSnapshot) != `{"start_seconds":0,"end_seconds":10}` {
		t.Fatalf("strategy edit leaked into stored snapshot: %s", stored.ConfigSnapshot)
	}
	if stored.DisplayName != "素材一 · 切段" {
		t.Fatalf("source rename changed historical task name: %q", stored.DisplayName)
	}
	second, err := svc.Create(actor, CreateComposeTasksInput{StrategyID: 5, Items: []CreateComposeTaskItemInput{{MaterialID: 43, RequestKey: "rk-frozen-002"}}})
	if err != nil || len(second.Created) != 1 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	snapshot := composeTaskSnapshot(t, second.Created[0].ConfigSnapshot)
	if snapshot["start_seconds"] != float64(8) {
		t.Fatalf("new task must read the edited config: %v", snapshot)
	}
}

func TestCancelComposeTaskPendingOnlyAndNamed(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	tasks.tasks = append(tasks.tasks,
		model.ComposeTask{ID: 500, TeamID: 7, GameID: "delta", Status: model.ComposeTaskPending, CreatedBy: 9},
		model.ComposeTask{ID: 501, TeamID: 7, GameID: "delta", Status: model.ComposeTaskSuccess, CreatedBy: 9},
	)

	cancelled, err := svc.Cancel(actor, 500)
	if err != nil || cancelled.Status != model.ComposeTaskCancelled || tasks.cancels != 1 {
		t.Fatalf("cancelled=%+v err=%v", cancelled, err)
	}
	if _, err := svc.Cancel(actor, 500); !errors.Is(err, ErrComposeTaskNotCancellable) {
		t.Fatalf("double cancel error = %v", err)
	}
	if _, err := svc.Cancel(actor, 501); !errors.Is(err, ErrComposeTaskNotCancellable) {
		t.Fatalf("non-pending cancel error = %v", err)
	}
	other := actor
	other.ID = 10
	if _, err := svc.Cancel(other, 500); !errors.Is(err, ErrComposeTaskNotFound) {
		t.Fatalf("foreign task cancel error = %v", err)
	}
}

func TestCancelComposeTaskLostRaceReloadsToNotCancellable(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	tasks.tasks = append(tasks.tasks, model.ComposeTask{ID: 500, TeamID: 7, GameID: "delta", Status: model.ComposeTaskPending, CreatedBy: 9})
	tasks.cancelBlocked = true
	if _, err := svc.Cancel(actor, 500); !errors.Is(err, ErrComposeTaskNotCancellable) {
		t.Fatalf("lost-race cancel error = %v", err)
	}
	if tasks.cancels != 1 {
		t.Fatalf("cancels = %d", tasks.cancels)
	}
}

// M4-C5 T-05: a running task is not refused — cancelling it requests the
// worker's controlled stop and keeps answering running until the worker
// confirms. Only a terminal row is not cancellable.
func TestCancelComposeTaskRunningRequestsControlledStop(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	running := model.ComposeTask{ID: 502, TeamID: 7, GameID: "delta", Status: model.ComposeTaskRunning, CreatedBy: 9}
	tasks.tasks = append(tasks.tasks, running)

	stored, err := svc.Cancel(actor, 502)
	if err != nil {
		t.Fatalf("running cancel error = %v", err)
	}
	if stored.Status != model.ComposeTaskRunning || !stored.CancelRequested {
		t.Fatalf("the row must stay running with the flag set: %+v", stored)
	}
	if tasks.requests != 1 || tasks.cancels != 0 {
		t.Fatalf("a running cancel must request, not flip: requests=%d cancels=%d", tasks.requests, tasks.cancels)
	}

	again, err := svc.Cancel(actor, 502)
	if err != nil || !again.CancelRequested {
		t.Fatalf("a repeated request is answered by the flag, not an error: %+v err=%v", again, err)
	}
	if tasks.requests != 1 {
		t.Fatalf("a second request must write nothing: %d", tasks.requests)
	}
}

func TestCancelComposeTaskTerminalRowsAreNamed(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	for _, status := range []model.ComposeTaskStatus{model.ComposeTaskSuccess, model.ComposeTaskFailed, model.ComposeTaskCancelled} {
		id := 503 + int64(statusIndex(status))
		tasks.tasks = append(tasks.tasks, model.ComposeTask{ID: id, TeamID: 7, GameID: "delta", Status: status, CreatedBy: 9})
		if _, err := svc.Cancel(actor, id); !errors.Is(err, ErrComposeTaskNotCancellable) {
			t.Fatalf("%s cancel error = %v", status, err)
		}
	}
	if tasks.requests != 0 || tasks.cancels != 0 {
		t.Fatalf("terminal rows must not be written: requests=%d cancels=%d", tasks.requests, tasks.cancels)
	}
}

func TestCancelComposeTaskLostRequestOnTerminalRowRefuses(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	// The worker finished the task between the service's read and the flag
	// write: the request lands on nothing and the reloaded row is terminal.
	tasks.tasks = append(tasks.tasks, model.ComposeTask{ID: 506, TeamID: 7, GameID: "delta", Status: model.ComposeTaskRunning, CreatedBy: 9})
	tasks.requestBlocked = true
	if _, err := svc.Cancel(actor, 506); !errors.Is(err, ErrComposeTaskNotCancellable) {
		t.Fatalf("lost-race running cancel error = %v", err)
	}
	if tasks.requests != 0 {
		t.Fatalf("a refused request must write nothing: %d", tasks.requests)
	}
}

func statusIndex(status model.ComposeTaskStatus) int {
	switch status {
	case model.ComposeTaskSuccess:
		return 0
	case model.ComposeTaskFailed:
		return 1
	case model.ComposeTaskCancelled:
		return 2
	}
	return 3
}

// M4-G: retry is senior-or-admin only, scoped by canRead, and refuses
// error codes outside the retryable set with the named error.
func TestRetryComposeTaskRequiresSeniorRoleAndRetryableCode(t *testing.T) {
	svc, tasks, _, _, operator := composeTaskFixture()
	team := identity.TeamID(7)
	senior := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	failed := func(id int64, code string) model.ComposeTask {
		return model.ComposeTask{ID: id, TeamID: 7, GameID: "delta", Status: model.ComposeTaskFailed, ErrorCode: code, CreatedBy: 9}
	}

	// A plain operator who can read the task is forbidden, not 404.
	tasks.tasks = append(tasks.tasks, failed(510, "storage_error"))
	if _, err := svc.Retry(operator, 510); !errors.Is(err, ErrComposeTaskForbidden) {
		t.Fatalf("operator retry error = %v", err)
	}

	// A senior retrying a non-retryable code is named, and the row stays failed.
	tasks.tasks[0].ErrorCode = "input_missing"
	if _, err := svc.Retry(senior, 510); !errors.Is(err, ErrComposeTaskNotRetryable) {
		t.Fatalf("input_missing retry error = %v", err)
	}
	if tasks.tasks[0].Status != model.ComposeTaskFailed {
		t.Fatalf("refused retry must not move the row: %+v", tasks.tasks[0])
	}
	tasks.tasks[0].ErrorCode = "execution_uncertain"
	if _, err := svc.Retry(senior, 510); !errors.Is(err, ErrComposeTaskNotRetryable) {
		t.Fatalf("uncertain execution must use administrator review: %v", err)
	}

	// A retryable code requeues the row and records who authorized it.
	tasks.tasks[0].ErrorCode = "storage_error"
	tasks.retryBlocked = true
	if _, err := svc.Retry(senior, 510); !errors.Is(err, ErrComposeTaskNotRetryable) {
		t.Fatalf("blocked retry error = %v", err)
	}
	tasks.retryBlocked = false
	stored, err := svc.Retry(senior, 510)
	if err != nil || stored.Status != model.ComposeTaskPending || tasks.retriedBy != 11 {
		t.Fatalf("stored=%+v retriedBy=%d err=%v", stored, tasks.retriedBy, err)
	}

	// A task the senior cannot read is 404, same as the list scope.
	tasks.tasks = append(tasks.tasks, model.ComposeTask{ID: 511, TeamID: 7, GameID: "delta", Status: model.ComposeTaskFailed, ErrorCode: "storage_error", CreatedBy: 10})
	if _, err := svc.Retry(operator, 511); !errors.Is(err, ErrComposeTaskNotFound) {
		t.Fatalf("foreign retry error = %v", err)
	}
}

func TestListComposeTaskAttemptsFollowsTheTaskScope(t *testing.T) {
	svc, _, _, _, actor := composeTaskFixture()
	if _, err := svc.ListAttempts(actor, 404); !errors.Is(err, ErrComposeTaskNotFound) {
		t.Fatalf("missing task attempts error = %v", err)
	}
}

func TestListComposeTasksScopesByRole(t *testing.T) {
	svc, tasks, _, _, actor := composeTaskFixture()
	if _, err := svc.List(actor, ListComposeTasksInput{Status: "pending"}); err != nil {
		t.Fatalf("list err=%v", err)
	}
	if tasks.lastFilter.CreatedBy != 9 || tasks.lastFilter.TeamID != 7 || len(tasks.lastFilter.GameIDs) != 1 || tasks.lastFilter.GameIDs[0] != "delta" || tasks.lastFilter.Status != model.ComposeTaskPending {
		t.Fatalf("operator filter = %+v", tasks.lastFilter)
	}

	team := identity.TeamID(7)
	senior := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	if _, err := svc.List(senior, ListComposeTasksInput{}); err != nil {
		t.Fatalf("senior list err=%v", err)
	}
	if tasks.lastFilter.CreatedBy != 0 || tasks.lastFilter.TeamID != 7 {
		t.Fatalf("senior filter = %+v", tasks.lastFilter)
	}

	admin := identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	if _, err := svc.List(admin, ListComposeTasksInput{}); err != nil {
		t.Fatalf("admin list err=%v", err)
	}
	if tasks.lastFilter.TeamID != 0 || tasks.lastFilter.CreatedBy != 0 {
		t.Fatalf("admin filter = %+v", tasks.lastFilter)
	}

	disabled := actor
	disabled.Status = identity.UserStatusDisabled
	if _, err := svc.List(disabled, ListComposeTasksInput{}); !errors.Is(err, ErrComposeTaskForbidden) {
		t.Fatalf("disabled actor error = %v", err)
	}
}
