package repository_test

import (
	"encoding/json"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"testing"
	"time"
)

func TestBatchVerificationCountUsesOriginalReviewFactsAcrossAllPlanAndBatchViews(t *testing.T) {
	s, p, bs, start, tasks := preparedPlan(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	clock := now
	s.PlanClock = func() time.Time { return clock }
	s.RecordWork = service.NewExecutionStore(s.DB).RecordWork
	if _, e := s.StartPlan(p.ID, 9, start, "verification-count", tasks[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	claim := acceptanceClaim(t, s, "verification-count", now)
	acceptanceReviewing(t, s, claim, now)
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store, PlanExecutionRepo: s}
	check := func(stage string, want, reviewing, unknown int64) {
		t.Helper()
		checkExecution := func(v model.BatchExecution) {
			t.Helper()
			raw, e := json.Marshal(v)
			if e != nil {
				t.Fatal(e)
			}
			var wire struct {
				Count *int64 `json:"verification_required_count"`
			}
			if e = json.Unmarshal(raw, &wire); e != nil || wire.Count == nil || *wire.Count != want {
				t.Fatalf("%s missing/wrong verification field on wire: %s err=%v", stage, raw, e)
			}
			if v.VerificationRequiredCount != want || v.TaskCounts[model.StatusReviewing] != reviewing || v.TaskCounts[model.StatusUncertain] != unknown {
				t.Fatalf("%s verification=%d reviewing=%d unknown=%d expected=%d/%d/%d", stage, v.VerificationRequiredCount, v.TaskCounts[model.StatusReviewing], v.TaskCounts[model.StatusUncertain], want, reviewing, unknown)
			}
		}
		detail, e := svc.GetBatch(owner, bs[0].ID)
		if e != nil {
			t.Fatal(e)
		}
		checkExecution(detail.Execution)
		batchPage, e := svc.ListBatches(owner, service.BatchFilter{Page: 1, PageSize: 10})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, b := range batchPage.Items {
			if b.ID == bs[0].ID {
				found = true
				checkExecution(b.Execution)
			}
		}
		if !found {
			t.Fatal("batch summary missing current batch")
		}
		plan, e := svc.GetPlan(owner, p.ID)
		if e != nil {
			t.Fatal(e)
		}
		checkExecution(plan.Batches[0].Batch.Execution)
		if plan.Plan.VerificationRequiredCount != want {
			t.Fatalf("plan detail verification=%d expected=%d", plan.Plan.VerificationRequiredCount, want)
		}
		page, e := svc.PlanBatches(owner, p.ID, 1, 10, "", nil)
		if e != nil {
			t.Fatal(e)
		}
		checkExecution(page.Items[0].Batch.Execution)
		plans, e := svc.ListPlans(owner, repo.PlanFilter{Page: 1, PageSize: 10})
		if e != nil {
			t.Fatal(e)
		}
		if len(plans.Items) != 1 {
			t.Fatal("plan summary missing")
		}
		if plans.Items[0].VerificationRequiredCount != want {
			t.Fatalf("plan summary verification=%d expected=%d", plans.Items[0].VerificationRequiredCount, want)
		}
		raw, e := s.PlanRowsByIDs([]uint64{p.ID})
		if e != nil {
			t.Fatal(e)
		}
		checkExecution(raw[p.ID][0].Execution)
	}
	check("normal platform review", 0, 1, 0)
	stopped := now.Add(-time.Second)
	if e := s.DB.Model(&model.Task{}).Where("id=?", claim.Task.ID).Update("check_stopped_at", stopped).Error; e != nil {
		t.Fatal(e)
	}
	check("automatic check stopped", 1, 1, 0)
	accepted := now.Add(-25 * time.Hour)
	if e := s.DB.Model(&model.Task{}).Where("id=?", claim.Task.ID).Updates(map[string]any{"accepted_at": accepted, "check_stopped_at": nil}).Error; e != nil {
		t.Fatal(e)
	}
	check("expired original review", 1, 1, 0)
	v, e := s.Get(claim.Task.ID)
	if e != nil {
		t.Fatal(e)
	}
	reviewing := publicResolution(*claim, v)
	reviewing.Conclusion = model.StatusReviewing
	reviewing.PlatformWorkID = ""
	reviewing.WorkURL = ""
	reviewing.PublishedAt = nil
	reviewing.Evidence.PubliclyAccessible = false
	reviewing.PlatformSubmissionID = *v.PlatformSubmissionID
	v, e = s.ResolveResult(v, 9, reviewing, now)
	if e != nil {
		t.Fatal(e)
	}
	check("fresh explicit manual review", 0, 1, 0)
	unknown := publicResolution(*claim, v)
	unknown.Conclusion = model.StatusUncertain
	unknown.PlatformWorkID = ""
	unknown.WorkURL = ""
	unknown.PublishedAt = nil
	unknown.PlatformSubmissionID = ""
	unknown.TargetURL = ""
	unknown.Evidence.PubliclyAccessible = false
	unknown.Evidence.WorkMatched = false
	v, e = s.ResolveResult(v, 9, unknown, now)
	if e != nil {
		t.Fatal(e)
	}
	check("manual unknown is not double counted", 0, 0, 1)
	reviewing.ExpectedRevision = v.Revision
	v, e = s.ResolveResult(v, 9, reviewing, now)
	if e != nil {
		t.Fatal(e)
	}
	check("renewed explicit manual review", 0, 1, 0)
	newStop := now.Add(time.Second)
	clock = now.Add(2 * time.Second)
	if e = s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("check_stopped_at", newStop).Error; e != nil {
		t.Fatal(e)
	}
	check("newer stop invalidates older manual review", 1, 1, 0)
	v, e = s.Get(v.ID)
	if e != nil {
		t.Fatal(e)
	}
	pub := publicResolution(*claim, v)
	pub.PlatformSubmissionID = *v.PlatformSubmissionID
	if _, e = s.ResolveResult(v, 9, pub, clock); e != nil {
		t.Fatal(e)
	}
	check("actual matched public work", 0, 0, 0)
}
