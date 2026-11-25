package service

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"slices"
	"testing"
	"time"
)

func TestTakeoverProjectionSharesCurrentFrozenExecutionPolicy(t *testing.T) {
	now := time.Now().UTC()
	id, current := uint64(1), uint64(2)
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	p := model.Plan{ID: id, TeamID: 7, OwnerID: 9, Status: "running", StartedAt: &now, CurrentBatchID: &current}
	b := model.Batch{ID: current, TeamID: 7, OperatorID: 9, PlanID: &id, Revision: 2, StartedAt: &now, PlanFrozenAt: &now, FrozenConfirmation: &model.PlanBatchConfirmation{BatchID: current, ExpectedRevision: 1, IncludedRowKeys: []string{"r"}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1, Shortfall: 0}}}, ExecutionStatus: model.BatchRunning}
	if !model.CanTakeoverPlanBatch(p, b) {
		t.Fatal("legal current execution denied")
	}
	projected := projectBatch(a, model.ProjectPlanBatch(p, b))
	if !slices.Contains(projected.AllowedActions, "takeover") {
		t.Fatal("no current batch takeover projection")
	}
	for _, role := range []identity.Role{identity.RoleAdmin, identity.RoleSeniorOperator} {
		other := a
		other.Role = role
		other.ID = 11
		if slices.Contains(projectBatch(other, model.ProjectPlanBatch(p, b)).AllowedActions, "takeover") {
			t.Fatal("other manager takeover offered")
		}
	}
	cases := []struct {
		name string
		p    model.Plan
		b    model.Batch
	}{{"paused-plan", p, b}, {"future", p, b}, {"handoff", p, b}, {"stopped", p, b}, {"ended-batch", p, b}, {"unfrozen", p, b}, {"unstarted", p, b}}
	cases[0].p.Status = "paused"
	future := uint64(3)
	cases[1].p.CurrentBatchID = &future
	cases[2].b.HandedOffAt = &now
	cases[3].p.StoppedAt = &now
	cases[4].b.ExecutionStatus = model.BatchEnded
	cases[5].b.FrozenConfirmation = nil
	cases[6].b.StartedAt = nil
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if model.CanTakeoverPlanBatch(tc.p, tc.b) || slices.Contains(projectBatch(a, model.ProjectPlanBatch(tc.p, tc.b)).AllowedActions, "takeover") {
				t.Fatal("illegal takeover offered")
			}
		})
	}
}
