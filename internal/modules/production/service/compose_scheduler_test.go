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
	ids     []int64
	options []repository.ScheduleRunOptions
	failID  int64
	runs    []model.ComposeScheduleRun
	listID  int64
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

func TestComposeScheduleRunsScopedReadback(t *testing.T) {
	_, strategies, actor := strategyFixture()
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "active"}}
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
	schedule := model.ComposeSchedule{Enabled: false, Mode: "manual", Timezone: "Asia/Shanghai", PerRunLimit: 2, DailyLimit: 4, MaterialScope: model.ComposeMaterialScopeCreatorUsages}
	strategies.items = []model.ComposeStrategy{{ID: 11, TeamID: 7, GameID: "delta", Status: "active", Schedule: &schedule}}
	store := &scheduleFake{}
	svc := NewComposeSchedulerService(store, strategies)
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
