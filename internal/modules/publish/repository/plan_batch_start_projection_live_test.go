package repository_test

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPlanUnstartedBatchActionsMatchIndependentStartGuardWithoutTasks(t *testing.T) {
	s, p, bs, start, tasks := preparedPlan(t)
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store, PlanExecutionRepo: s}
	check := func(b model.Batch) {
		t.Helper()
		if slices.Contains(b.AllowedActions, "start") || !slices.Contains(b.AllowedActions, "edit") || !slices.Contains(b.AllowedActions, "stop") {
			t.Fatalf("plan-owned batch independently startable or edit/stop missing: %v", b.AllowedActions)
		}
		found := false
		for _, blocker := range b.ActionBlockers {
			if blocker.Action == "start" && strings.Contains(blocker.Message, "计划统一启动") {
				found = true
			}
		}
		if !found {
			t.Fatal("no unified plan-start blocker")
		}
	}
	for _, batch := range bs {
		detail, e := svc.GetBatch(owner, batch.ID)
		if e != nil {
			t.Fatal(e)
		}
		check(detail.Batch)
	}
	list, e := svc.ListBatches(owner, service.BatchFilter{Page: 1, PageSize: 10})
	if e != nil {
		t.Fatal(e)
	}
	if len(list.Items) != 2 {
		t.Fatal("configured plan batches not listed")
	}
	for _, b := range list.Items {
		check(b)
	}
	plan, e := svc.GetPlan(owner, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if !slices.Contains(plan.Plan.AllowedActions, "start") {
		t.Fatalf("unified plan first-start disappeared: %v", plan.Plan.AllowedActions)
	}
	for _, b := range plan.Batches {
		check(b.Batch.Batch)
		if slices.Contains(b.AllowedActions, "start") {
			t.Fatal("plan batch outer actions still expose independent start")
		}
	}
	page, e := svc.PlanBatches(owner, p.ID, 1, 10, "", nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range page.Items {
		check(b.Batch.Batch)
	}
	confirmation := start.BatchConfirmations[0]
	in := model.BatchStartInput{RequestKey: "independent-plan-bypass", ExpectedRevision: bs[0].Revision, IncludedRowKeys: confirmation.IncludedRowKeys, ExcludedRowKeys: confirmation.ExcludedRowKeys, AllowShortfall: confirmation.AllowShortfall, ExpectedShortfalls: confirmation.ExpectedShortfalls}
	if _, e = s.StartBatch(bs[0].ID, 9, in, "not-plan-start", tasks[bs[0].ID], time.Now().UTC()); !errors.Is(e, repo.ErrParentBlocked) {
		t.Fatalf("existing independent endpoint guard changed: %v", e)
	}
	var n int64
	if e = s.DB.Model(&model.Task{}).Count(&n).Error; e != nil || n != 0 {
		t.Fatalf("readonly projection/rejected start created tasks/occupancy: n=%d err=%v", n, e)
	}
	result, e := s.StartPlan(p.ID, 9, start, "unified-plan-start", tasks[bs[0].ID], time.Now().UTC())
	if e != nil || len(result.FirstBatchReceipt.TaskIDs) != 1 {
		t.Fatalf("shared internal plan startup changed: %v %v", result, e)
	}
}
