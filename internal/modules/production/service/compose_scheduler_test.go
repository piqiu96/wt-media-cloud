package service

import (
	"errors"
	"testing"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type scheduleFake struct {
	ids            []int64
	options        []repository.ScheduleRunOptions
	failID         int64
	runs           []model.ComposeScheduleRun
	listID         int64
	historyOptions repository.ScheduleRunHistoryOptions
}

func (f *scheduleFake) DueStrategyIDs(time.Time) ([]int64, error) { return f.ids, nil }
func (f *scheduleFake) Run(options repository.ScheduleRunOptions) (model.ComposeScheduleRun, bool, error) {
	f.options = append(f.options, options)
	if options.StrategyID == f.failID {
		return model.ComposeScheduleRun{}, false, errors.New("temporary database failure")
	}
	return model.ComposeScheduleRun{StrategyID: options.StrategyID, TriggerType: "manual"}, true, nil
}
func (f *scheduleFake) ListRuns(strategyID int64, limit int) ([]model.ComposeScheduleRun, error) {
	f.listID = strategyID
	if limit != 20 {
		return nil, errors.New("unexpected run history limit")
	}
	return f.runs, nil
}
func (f *scheduleFake) ListRunHistory(options repository.ScheduleRunHistoryOptions) (model.ComposeScheduleRunHistoryPage, error) {
	f.historyOptions = options
	items := make([]model.ComposeScheduleRunHistoryItem, len(f.runs))
	for i, run := range f.runs {
		items[i].ComposeScheduleRun = run
	}
	return model.ComposeScheduleRunHistoryPage{Items: items, Total: len(items), Page: options.Page, PageSize: options.PageSize}, nil
}

func (f *scheduleFake) BatchTasks(int64, time.Time) ([]model.ComposeTask, model.ComposeBatchCounts, error) {
	return nil, model.ComposeBatchCounts{}, nil
}

func (f *scheduleFake) FindRun(int64, time.Time) (model.ComposeScheduleRun, bool, error) {
	return model.ComposeScheduleRun{}, false, nil
}

type scheduleBatchFake struct {
	scheduleFake
	tasks   []model.ComposeTask
	counts  model.ComposeBatchCounts
	slot    time.Time
	findRun model.ComposeScheduleRun
}

func (f *scheduleBatchFake) BatchTasks(strategyID int64, slot time.Time) ([]model.ComposeTask, model.ComposeBatchCounts, error) {
	f.slot = slot
	return f.tasks, f.counts, nil
}

func (f *scheduleBatchFake) FindRun(int64, time.Time) (model.ComposeScheduleRun, bool, error) {
	return f.findRun, f.findRun.RunKey != "", nil
}

func TestComposeBatchViewScopesReadbackAndCarriesCounts(t *testing.T) {
	_, strategies, actor := strategyFixture()
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "enabled"}}
	slot := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	store := &scheduleBatchFake{
		tasks:   []model.ComposeTask{{ID: 1, Status: "success"}, {ID: 2, Status: "failed", ErrorCode: "execution_failed"}},
		counts:  model.ComposeBatchCounts{Success: 1, Failed: 1},
		findRun: model.ComposeScheduleRun{RunKey: "run-1", Skipped: []model.ComposeScheduleSkip{{MaterialID: 9, Reason: "already_produced"}}},
	}
	svc := NewComposeSchedulerService(store, strategies)
	view, err := svc.Batch(actor, 11, slot)
	if err != nil || !store.slot.Equal(slot) || len(view.Tasks) != 2 || view.Counts.Failed != 1 || view.Run.RunKey != "run-1" || view.Run.Skipped[0].Reason != "already_produced" {
		t.Fatalf("batch=%+v slot=%v err=%v", view, store.slot, err)
	}
	if _, err := svc.Batch(actor, 12, slot); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("missing strategy error=%v", err)
	}
	actor.GameIDs = []string{"other"}
	if _, err := svc.Batch(actor, 11, slot); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("out-of-scope error=%v", err)
	}
	if _, err := svc.Batch(actor, 11, time.Time{}); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("empty slot error=%v", err)
	}
}

func TestComposeScheduleRunsScopedReadback(t *testing.T) {
	_, strategies, actor := strategyFixture()
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "enabled"}}
	store := &scheduleFake{runs: []model.ComposeScheduleRun{{StrategyID: 11, Skipped: []model.ComposeScheduleSkip{{MaterialID: 4, Reason: "already_produced"}}}}}
	svc := NewComposeSchedulerService(store, strategies)
	runs, err := svc.ListRuns(actor, 11)
	if err != nil || len(runs) != 1 || store.listID != 11 || runs[0].Skipped[0].Reason != "already_produced" {
		t.Fatalf("runs=%+v read id=%d err=%v", runs, store.listID, err)
	}
	if _, err := svc.ListRuns(actor, 12); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("missing strategy error=%v", err)
	}
	actor.GameIDs = []string{"other"}
	if _, err := svc.ListRuns(actor, 11); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("out-of-scope error=%v", err)
	}
}

func TestComposeScheduleRunHistoryScopesAndPassesFilters(t *testing.T) {
	_, strategies, actor := strategyFixture()
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "enabled"}}
	store := &scheduleFake{runs: []model.ComposeScheduleRun{{StrategyID: 11}}}
	svc := NewComposeSchedulerService(store, strategies)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	options := repository.ScheduleRunHistoryOptions{Page: 2, PageSize: 10, TriggerType: "manual", CreatedFrom: &from}
	page, err := svc.RunHistory(actor, 11, options)
	if err != nil || page.Total != 1 || store.historyOptions.StrategyID != 11 || store.historyOptions.Page != 2 || store.historyOptions.TriggerType != "manual" || !store.historyOptions.CreatedFrom.Equal(from) {
		t.Fatalf("page=%+v options=%+v err=%v", page, store.historyOptions, err)
	}
	actor.GameIDs = []string{"other"}
	if _, err := svc.RunHistory(actor, 11, options); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("out-of-scope error=%v", err)
	}
}

func TestComposeSchedulerContinuesAfterOneStrategyFails(t *testing.T) {
	store := &scheduleFake{ids: []int64{1, 2}, failID: 1}
	svc := NewComposeSchedulerService(store, nil)
	count, err := svc.RunDue(time.Now())
	if count != 1 || err == nil || len(store.options) != 2 {
		t.Fatalf("count=%d err=%v options=%d", count, err, len(store.options))
	}
}

func TestComposeRunNowChecksActorAndUsesStableRequestKey(t *testing.T) {
	_, strategies, actor := strategyFixture()
	schedule := model.ComposeSchedule{Mode: "manual", Timezone: "Asia/Shanghai"}
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "enabled", OperatorID: operatorIDPtr(9), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2, Schedule: &schedule}}
	store := &scheduleFake{}
	svc := NewComposeSchedulerService(store, strategies)
	svc.operators = func() ([]identity.PublicUser, error) { return []identity.PublicUser{actor}, nil }
	actor.Role = identity.RoleOperator
	if _, err := svc.RunNow(actor, 11, "request-001"); !errors.Is(err, ErrStrategyForbidden) {
		t.Fatalf("operator error=%v", err)
	}
	actor.Role = identity.RoleSeniorOperator
	if _, err := svc.RunNow(actor, 11, "bad"); !errors.Is(err, ErrStrategyInvalid) {
		t.Fatalf("short key error=%v", err)
	}
	if _, err := svc.RunNow(actor, 11, "request-001"); err != nil || len(store.options) != 1 || store.options[0].ManualKey != "request-001" {
		t.Fatalf("run error=%v options=%+v", err, store.options)
	}
}

func TestComposeRunNowRefusesAnUnavailableOperatorBeforeRunning(t *testing.T) {
	_, strategies, actor := strategyFixture()
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "enabled", OperatorID: operatorIDPtr(10), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2}}
	store := &scheduleFake{}
	svc := NewComposeSchedulerService(store, strategies)
	team := identity.TeamID(7)
	svc.operators = func() ([]identity.PublicUser, error) {
		return []identity.PublicUser{{ID: 9, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}}, nil
	}
	if _, err := svc.RunNow(actor, 11, "request-001"); !errors.Is(err, ErrStrategyInvalid) {
		t.Fatalf("unavailable operator error = %v", err)
	}
	if len(store.options) != 0 {
		t.Fatalf("a refused run must not reach the store: %+v", store.options)
	}
}
