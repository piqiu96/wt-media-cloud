package repository_test

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"slices"
	"testing"
	"time"
)

func TestPlanFinalReviewWaitsForAllPrefixPublicFactsWithoutFinishPermission(t *testing.T) {
	s, p, bs, start, tasks := preparedPlan(t)
	s.RecordWork = service.NewExecutionStore(s.DB).RecordWork
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, e := s.StartPlan(p.ID, 9, start, "final-review", tasks[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	first := acceptanceClaim(t, s, "final-first", now)
	acceptanceReviewing(t, s, first, now)
	state, e := s.PlanProgress(p.ID, now)
	if e != nil {
		t.Fatal(e)
	}
	if state.Progress != "running" || state.Next == nil {
		t.Fatalf("normal review blocked 10%% handoff %+v", state)
	}
	in := model.PlanAdvanceInput{RequestKey: "auto-second", ExpectedRevision: state.Plan.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint}
	if _, e = s.AdvancePlan(p.ID, 9, in, "auto-second", tasks[bs[1].ID], true, now); e != nil {
		t.Fatal(e)
	}
	second := acceptanceClaim(t, s, "final-second", now)
	acceptanceReviewing(t, s, second, now)
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store, PlanExecutionRepo: s}
	check := func(want string, canFinish bool) {
		t.Helper()
		actual, e := s.PlanProgress(p.ID, time.Now().UTC())
		if e != nil {
			t.Fatal(e)
		}
		if actual.Progress != want || actual.Next != nil {
			t.Fatalf("progress=%s next=%v expected=%s", actual.Progress, actual.Next, want)
		}
		detail, e := svc.GetPlan(owner, p.ID)
		if e != nil {
			t.Fatal(e)
		}
		if detail.Plan.Progress != want || slices.Contains(detail.Plan.AllowedActions, "finish") != canFinish {
			t.Fatalf("detail progress/permission mismatch: %+v", detail.Plan)
		}
		page, e := svc.ListPlans(owner, repo.PlanFilter{Page: 1, PageSize: 10})
		if e != nil || len(page.Items) != 1 || page.Items[0].Progress != want {
			t.Fatalf("list progress inconsistent: %+v %v", page, e)
		}
		if want == "awaiting_finish" {
			blocked := false
			for _, item := range detail.Plan.ActionBlockers {
				if b, ok := item.(map[string]string); ok && b["action"] == "finish" && b["code"] == "awaiting_public_results" {
					blocked = true
				}
			}
			if !blocked {
				t.Fatal("no explanation for final public-results wait")
			}
		}
	}
	check("awaiting_finish", false)
	current, e := s.Get(second.Task.ID)
	if e != nil {
		t.Fatal(e)
	}
	pub := publicResolution(*second, current)
	pub.PlatformSubmissionID = *current.PlatformSubmissionID
	pub.PlatformWorkID = "7000395122085014123"
	pub.WorkURL = "https://mbd.baidu.com/newspage/data/videolanding?nid=sv_7000395122085014123"
	if _, e = s.ResolveResult(current, 9, pub, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	check("awaiting_finish", false) // Current is public; the handed-off prefix is still reviewing.
	accepted := now.Add(-25 * time.Hour)
	if e = s.DB.Model(&model.Task{}).Where("id=?", first.Task.ID).Update("accepted_at", accepted).Error; e != nil {
		t.Fatal(e)
	}
	check("waiting_verification", false)
	original, e := s.Get(first.Task.ID)
	if e != nil {
		t.Fatal(e)
	}
	review := publicResolution(*first, original)
	review.Conclusion = model.StatusReviewing
	review.PlatformSubmissionID = *original.PlatformSubmissionID
	review.PlatformWorkID = ""
	review.WorkURL = ""
	review.PublishedAt = nil
	review.Evidence.PubliclyAccessible = false
	original, e = s.ResolveResult(original, 9, review, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	check("awaiting_finish", false)
	unknown := publicResolution(*first, original)
	unknown.Conclusion = model.StatusUncertain
	unknown.PlatformSubmissionID = ""
	unknown.TargetURL = ""
	unknown.PlatformWorkID = ""
	unknown.WorkURL = ""
	unknown.PublishedAt = nil
	unknown.Evidence.PubliclyAccessible = false
	unknown.Evidence.WorkMatched = false
	original, e = s.ResolveResult(original, 9, unknown, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	check("waiting_tasks", false)
	review.ExpectedRevision = original.Revision
	original, e = s.ResolveResult(original, 9, review, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	check("awaiting_finish", false)
	latest, _, e := s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ControlPlan(p.ID, 9, latest.Revision, "pause", "等最终平台公开", 0, 0, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	check("manual_paused", false)
	latest, _, e = s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ControlPlan(p.ID, 9, latest.Revision, "resume", "", 0, 0, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	check("awaiting_finish", false)
	original, e = s.Get(first.Task.ID)
	if e != nil {
		t.Fatal(e)
	}
	pub = publicResolution(*first, original)
	pub.PlatformSubmissionID = *original.PlatformSubmissionID
	if _, e = s.ResolveResult(original, 9, pub, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	check("ready_to_finish", true)
	latest, _, e = s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.FinishPlan(p.ID, 9, model.FinishPlanInput{ExpectedRevision: latest.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 0}}, EndReason: "achieved"}, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	check("ended", false)
}
