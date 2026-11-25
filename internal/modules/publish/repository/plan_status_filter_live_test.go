package repository_test

import (
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"reflect"
	"testing"
	"time"
)

func TestPlanRunningListIncludesManualPausedBeforeScopedCountAndPagination(t *testing.T) {
	db := isolated(t)
	r := store(db)
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	now := time.Now().UTC().Truncate(time.Microsecond)
	expectedRunning, expectedPaused := []uint64{}, []uint64{}
	createdIDs := []uint64{}
	for i, status := range []string{"running", "paused", "not_started", "running", "paused", "stopping", "ended", "paused", "paused"} {
		owner, category := int64(9), int64(1)
		if i == 7 {
			owner = 10
		}
		if i == 8 {
			category = 2
		}
		p, bs, e := r.CreatePlan(model.Plan{TeamID: 7, OwnerID: owner, CreatedBy: owner, Name: "status-" + fmt.Sprint(i), DateStart: "2026-11-25", DateEnd: "2026-11-26", RequestKey: fmt.Sprint("status-", i), RequestHash: fmt.Sprint("hash-", i), Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: category, TargetCount: 1}}, CreatedAt: now.Add(time.Duration(i) * time.Second)}, []model.PlanStructureBatch{{RowKey: "row", Name: "row", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: category, TargetCount: 1}}}})
		if e != nil {
			t.Fatal(e)
		}
		createdIDs = append(createdIDs, p.ID)
		updates := map[string]any{"status": status}
		if status != "not_started" {
			updates["started_at"] = now
			updates["current_batch_id"] = bs[0].ID
			if e = db.Model(&model.Batch{}).Where("id=?", bs[0].ID).Updates(map[string]any{"started_at": now, "execution_status": model.BatchRunning}).Error; e != nil {
				t.Fatal(e)
			}
		}
		if status == "stopping" {
			updates["stopped_at"] = now
		}
		if status == "ended" {
			updates["ended_at"] = now
		}
		if e = db.Model(&model.Plan{}).Where("id=?", p.ID).Updates(updates).Error; e != nil {
			t.Fatal(e)
		}
		if owner == 9 && category == 1 && (status == "running" || status == "paused") {
			expectedRunning = append([]uint64{p.ID}, expectedRunning...)
		}
		if owner == 9 && category == 1 && status == "paused" {
			expectedPaused = append([]uint64{p.ID}, expectedPaused...)
		}
	}
	svc := service.Service{PlanRepo: r, BatchRepo: r}
	for _, filter := range []struct {
		status string
		ids    []uint64
	}{{"running", expectedRunning}, {"paused", expectedPaused}} {
		got := []uint64{}
		for page := 1; page < 5; page++ {
			result, e := svc.ListPlans(actor, repo.PlanFilter{Status: filter.status, Page: page, PageSize: 1})
			if e != nil {
				t.Fatal(e)
			}
			if result.Total != int64(len(filter.ids)) {
				t.Fatalf("filter=%s total=%d expected=%d", filter.status, result.Total, len(filter.ids))
			}
			for _, p := range result.Items {
				got = append(got, p.ID)
				if p.Status == "paused" && p.Progress != "manual_paused" {
					t.Fatalf("manual pause progress changed: %s", p.Progress)
				}
			}
			if len(result.Items) == 0 {
				break
			}
		}
		if !reflect.DeepEqual(got, filter.ids) {
			t.Fatalf("filter=%s ids=%v expected=%v", filter.status, got, filter.ids)
		}
	}
	ownIDs := []uint64{}
	for i := 6; i >= 0; i-- {
		ownIDs = append(ownIDs, createdIDs[i])
	}
	senior := actor
	senior.ID = 11
	senior.Role = identity.RoleSeniorOperator
	otherTeam := identity.TeamID(8)
	foreignSenior := senior
	foreignSenior.TeamID = &otherTeam
	for _, tc := range []struct {
		name   string
		actor  identity.PublicUser
		filter repo.PlanFilter
		want   []uint64
	}{
		{"ordinary search and status", actor, repo.PlanFilter{Status: "running", Search: "status"}, expectedRunning},
		{"category only", actor, repo.PlanFilter{CategoryID: 1}, ownIDs},
		{"platform only", actor, repo.PlanFilter{Platform: "baidu"}, ownIDs},
		{"nonexistent category", actor, repo.PlanFilter{CategoryID: 99}, []uint64{}},
		{"ordinary platform and category", actor, repo.PlanFilter{Status: "running", Platform: "baidu", CategoryID: 1}, expectedRunning},
		{"foreign category filter", actor, repo.PlanFilter{CategoryID: 2}, []uint64{}},
		{"foreign category search", actor, repo.PlanFilter{Search: "status-8"}, []uint64{}},
		{"foreign category exact key", actor, repo.PlanFilter{RequestKey: "status-8"}, []uint64{}},
		{"other owner exact key", actor, repo.PlanFilter{RequestKey: "status-7"}, []uint64{}},
		{"own exact key ignores status", actor, repo.PlanFilter{RequestKey: "status-2", Status: "running"}, []uint64{createdIDs[2]}},
		{"senior own team category", senior, repo.PlanFilter{Status: "running"}, append([]uint64{createdIDs[7]}, expectedRunning...)},
		{"senior foreign category search", senior, repo.PlanFilter{Search: "status-8"}, []uint64{}},
		{"senior other creator exact key", senior, repo.PlanFilter{RequestKey: "status-7"}, []uint64{}},
		{"senior foreign team", foreignSenior, repo.PlanFilter{Status: "running"}, []uint64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := []uint64{}
			for page := 1; page <= len(tc.want)+1; page++ {
				f := tc.filter
				f.Page = page
				f.PageSize = 1
				out, e := svc.ListPlans(tc.actor, f)
				if e != nil {
					t.Fatal(e)
				}
				if out.Total != int64(len(tc.want)) {
					t.Fatalf("total=%d expected=%d", out.Total, len(tc.want))
				}
				for _, p := range out.Items {
					got = append(got, p.ID)
					if _, e := svc.GetPlan(tc.actor, p.ID); e != nil {
						t.Fatalf("list scope disagrees with detail permission %v", e)
					}
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("rows=%v expected=%v", got, tc.want)
			}
		})
	}

}

func TestPlanScopeRejectsNullEmptyAndMalformedGoalCategories(t *testing.T) {
	s, p, _, _, _ := preparedPlan(t)
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store}
	for _, raw := range []string{`null`, `[]`, `{}`, `[{}]`, `[{"platform":"baidu","category_id":null,"target_count":1}]`, `[{"platform":"baidu","category_id":"bad","target_count":1}]`, `[{"platform":"baidu","category_id":"2","target_count":1}]`} {
		if e := s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("goals_json", raw).Error; e != nil {
			t.Fatal(e)
		}
		for _, f := range []repo.PlanFilter{{Page: 1, PageSize: 1}, {Page: 1, PageSize: 1, Search: "execution"}, {Page: 1, PageSize: 1, RequestKey: "execution"}} {
			out, e := svc.ListPlans(actor, f)
			if e != nil || out.Total != 0 || len(out.Items) != 0 {
				t.Fatalf("malformed goals read raw=%s out=%+v err=%v", raw, out, e)
			}
		}
		if _, e := svc.GetPlan(actor, p.ID); e == nil {
			t.Fatalf("detail accepted malformed goals %s", raw)
		}
	}
}

func TestPlanGoalFilterKeepsPlatformAndCategoryInTheSameGoal(t *testing.T) {
	s, p, _, _, _ := preparedPlan(t)
	// Controlled legacy/future-platform fixture exercises same-goal SQL matching;
	// this does not permit creating or executing another platform through the API.
	raw := `[{"platform":"other","category_id":"1","target_count":1},{"platform":"baidu","category_id":"2","target_count":1}]`
	if e := s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("goals_json", raw).Error; e != nil {
		t.Fatal(e)
	}
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1, 2}}
	for _, tc := range []struct {
		platform string
		category int64
		count    int64
	}{{"baidu", 1, 0}, {"baidu", 2, 1}, {"baidu", 0, 1}, {"", 1, 1}, {"missing", 0, 0}, {"", 99, 0}} {
		rows, total, e := s.ListPlans(repo.PlanFilter{Actor: actor, Page: 1, PageSize: 1, Platform: tc.platform, CategoryID: tc.category})
		if e != nil || total != tc.count || int64(len(rows)) != tc.count {
			t.Fatalf("platform=%s category=%d rows=%v total=%d expected=%d err=%v", tc.platform, tc.category, rows, total, tc.count, e)
		}
		rows, total, e = s.ListPlans(repo.PlanFilter{Actor: actor, Page: 2, PageSize: 1, Platform: tc.platform, CategoryID: tc.category})
		if e != nil || total != tc.count || len(rows) != 0 {
			t.Fatalf("second page total=%d expected=%d rows=%v err=%v", total, tc.count, rows, e)
		}
	}
}
