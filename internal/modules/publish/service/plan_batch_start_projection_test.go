package service

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPlanOwnedUnstartedBatchNeverOffersIndependentStart(t *testing.T) {
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team}
	plan := uint64(1)
	b := model.Batch{ID: 2, TeamID: 7, OperatorID: 9, PlanID: &plan, ExecutionStatus: model.BatchNotStarted}
	for _, frozen := range []bool{false, true} {
		b.PlanConfigurationFrozen = frozen
		got := projectBatch(owner, b)
		if slices.Contains(got.AllowedActions, "start") {
			t.Fatalf("independent start offered frozen=%v", frozen)
		}
		if !slices.Contains(got.AllowedActions, "stop") || !frozen && !slices.Contains(got.AllowedActions, "edit") {
			t.Fatalf("unrelated edit/stop removed frozen=%v actions=%v", frozen, got.AllowedActions)
		}
		found := false
		for _, blocker := range got.ActionBlockers {
			if blocker.Action == "start" {
				found = blocker.Code == "plan_position_blocked" && strings.Contains(blocker.Message, "计划统一启动")
			}
		}
		if !found {
			t.Fatalf("missing clear existing-code plan startup blocker frozen=%v blockers=%v", frozen, got.ActionBlockers)
		}
	}
	b.PlanID = nil
	b.PlanConfigurationFrozen = false
	if got := projectBatch(owner, b); !reflect.DeepEqual(got.AllowedActions, []string{"edit", "start", "stop"}) || len(got.ActionBlockers) != 0 {
		t.Fatalf("standalone startup changed: %+v", got)
	}
	now := time.Now().UTC()
	b.PlanID = &plan
	b.StartedAt = &now
	b.PlanConfigurationFrozen = true
	b.ExecutionStatus = model.BatchPaused
	b.Execution.TaskCounts = map[model.Status]int64{model.StatusPending: 1}
	if got := projectBatch(owner, b); !reflect.DeepEqual(got.AllowedActions, []string{"resume", "stop"}) {
		t.Fatalf("current batch resume changed %v", got.AllowedActions)
	}
}
