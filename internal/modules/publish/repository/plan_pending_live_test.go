package repository_test

import (
	"errors"
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"gorm.io/gorm"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlanPendingPagesShareProgressVerificationFactsBeforeCountAndPagination(t *testing.T) {
	execution, claim, original := reviewingPublication(t)
	execution.RecordWork = service.NewExecutionStore(execution.DB).RecordWork
	public, e := execution.ResolveResult(original, 9, publicResolution(claim, original), time.Now().UTC())
	if e != nil || public.WorkID == nil {
		t.Fatalf("real public-work fixture failed %v", e)
	}
	db := execution.DB
	s := execution.Store
	now := time.Now().UTC().Truncate(time.Microsecond)
	s.PlanClock = func() time.Time { return now }
	expired := now.Add(-25 * time.Hour)
	recent := now.Add(-time.Hour)
	stop := now.Add(-5 * time.Minute)
	manual := now.Add(-time.Minute)
	stale := stop.Add(-time.Minute)
	type sample struct {
		name                      string
		status                    model.Status
		accepted                  *time.Time
		stopped                   *time.Time
		conclusion                model.Status
		verified                  *time.Time
		wrongAttempt, badRevision bool
		work                      *uint64
		pending                   bool
	}
	foreignWork := *public.WorkID
	cases := []sample{
		{name: "pending", status: model.StatusPending, pending: true},
		{name: "queued", status: model.StatusQueued, pending: true},
		{name: "running", status: model.StatusRunning, pending: true},
		{name: "awaiting_human", status: model.StatusAwaitingHuman, pending: true},
		{name: "late_unknown", status: model.StatusUncertain, accepted: &expired, conclusion: model.StatusReviewing, verified: &manual, pending: true},
		{name: "normal_review", status: model.StatusReviewing, accepted: &recent},
		{name: "stopped_review", status: model.StatusReviewing, accepted: &recent, stopped: &stop, pending: true},
		{name: "expired_review", status: model.StatusReviewing, accepted: &expired, pending: true},
		{name: "manual_review", status: model.StatusReviewing, accepted: &expired, stopped: &stop, conclusion: model.StatusReviewing, verified: &manual},
		{name: "manual_unknown", status: model.StatusReviewing, accepted: &expired, stopped: &stop, conclusion: model.StatusUncertain, verified: &manual, pending: true},
		{name: "wrong_attempt", status: model.StatusReviewing, accepted: &expired, stopped: &stop, conclusion: model.StatusReviewing, verified: &manual, wrongAttempt: true, pending: true},
		{name: "future_manual_revision", status: model.StatusReviewing, accepted: &expired, stopped: &stop, conclusion: model.StatusReviewing, verified: &manual, badRevision: true, pending: true},
		{name: "stale_manual", status: model.StatusReviewing, accepted: &expired, stopped: &stop, conclusion: model.StatusReviewing, verified: &stale, pending: true},
		{name: "no_accepted_fact", status: model.StatusReviewing, pending: true},
		{name: "public_without_work", status: model.StatusPublished, pending: true},
		{name: "public_wrong_work", status: model.StatusPublished, work: &foreignWork, pending: true},
		{name: "manual_public", status: model.StatusPublished},
		{name: "manual_failed", status: model.StatusFailed, accepted: &expired, stopped: &stop, conclusion: model.StatusFailed, verified: &manual},
		{name: "cancelled", status: model.StatusCancelled},
		{name: "empty_batch"},
	}
	rows := make([]model.PlanStructureBatch, 0, len(cases))
	for i, c := range cases {
		rows = append(rows, model.PlanStructureBatch{RowKey: c.name, Name: c.name, Platform: "baidu", GroupOrder: 1, BatchOrder: uint32(i + 1), Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}})
	}
	p, batches, e := s.CreatePlan(model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "pending scope", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: int64(len(cases))}}, RequestKey: "pending", RequestHash: "pending"}, rows)
	if e != nil {
		t.Fatal(e)
	}
	expected := map[uint64]bool{}
	for i, c := range cases {
		expected[batches[i].ID] = c.pending
		if c.status == "" {
			continue
		}
		if c.name == "manual_public" {
			if e = db.Model(&model.Task{}).Where("id=?", public.ID).Update("batch_id", batches[i].ID).Error; e != nil {
				t.Fatal(e)
			}
			continue
		}
		if e = db.Exec("INSERT INTO composite_outputs(id) VALUES(?)", 100+i).Error; e != nil {
			t.Fatal(e)
		}
		v := task("pending-"+c.name, int64(100+i), 1)
		v.PlatformAccountUID = fmt.Sprintf("pending-uid-%d", i)
		v.BatchID = &batches[i].ID
		v.Status = c.status
		v.Revision = 3
		v.AcceptedAt = c.accepted
		v.CheckStoppedAt = c.stopped
		v.WorkID = c.work
		attempt := uint64(1000 + i)
		v.CurrentAttemptID = &attempt
		if c.verified != nil {
			v.LatestManualVerification = &model.LatestManualVerification{AttemptID: attempt, TaskRevision: 3, VerifiedAt: *c.verified, Conclusion: c.conclusion}
			if c.wrongAttempt {
				v.LatestManualVerification.AttemptID++
			}
			if c.badRevision {
				v.LatestManualVerification.TaskRevision++
			}
		}
		if e = db.Create(&v).Error; e != nil {
			t.Fatal(e)
		}
	}
	// Controlled fixture positions expose all task facts to the existing progress oracle.
	if e = db.Model(&model.Batch{}).Where("plan_id=?", p.ID).Updates(map[string]any{"started_at": now, "execution_status": model.BatchRunning}).Error; e != nil {
		t.Fatal(e)
	}
	if e = db.Model(&model.Plan{}).Where("id=?", p.ID).Updates(map[string]any{"started_at": now, "status": "running", "current_batch_id": batches[len(batches)-1].ID}).Error; e != nil {
		t.Fatal(e)
	}
	progress, e := s.PlanProgress(p.ID, now)
	if e != nil {
		t.Fatal(e)
	}
	if progress.Progress != "waiting_verification" {
		t.Fatalf("missing progress verification gate: %s", progress.Progress)
	}
	oracle := map[uint64]bool{}
	for _, v := range progress.Tasks {
		switch v.Status {
		case model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain:
			oracle[*v.BatchID] = true
		}
		if v.VerificationRequiredAt(now) {
			oracle[*v.BatchID] = true
		}
	}
	for id, want := range expected {
		if oracle[id] != want {
			t.Fatalf("test oracle mismatch batch %d got=%v expected=%v", id, oracle[id], want)
		}
	}
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, TeamID: &team, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s, BatchRepo: s}
	projectionQueries := 0
	if e = db.Callback().Query().After("gorm:query").Register("chg109_pending_projection_count", func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if strings.Contains(query, " AS matched_public") {
			projectionQueries++
			if strings.Contains(strings.ToUpper(query), "LIMIT") {
				t.Error("verification facts were paginated before filtering")
			}
		}
	}); e != nil {
		t.Fatal(e)
	}

	for _, pending := range []bool{true, false} {
		wantIDs := []uint64{}
		for _, b := range batches {
			if expected[b.ID] == pending {
				wantIDs = append(wantIDs, b.ID)
			}
		}
		gotIDs := []uint64{}
		for page := 1; page < 10; page++ {
			before := projectionQueries
			result, e := svc.PlanBatches(actor, p.ID, page, 3, "", &pending)
			if projectionQueries-before != 1 {
				t.Fatalf("expected one task projection, got %d", projectionQueries-before)
			}
			if e != nil {
				t.Fatal(e)
			}
			if result.Total != int64(len(wantIDs)) {
				t.Fatalf("pending=%v page=%d total=%d expected=%d", pending, page, result.Total, len(wantIDs))
			}
			for _, b := range result.Items {
				gotIDs = append(gotIDs, b.BatchID)
			}
			if len(result.Items) < 3 {
				break
			}
		}
		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Fatalf("pending=%v rows=%v expected=%v", pending, gotIDs, wantIDs)
		}
	}
	// Ownership and whole-goal category scope are checked before any page/count query.
	for _, a := range []identity.PublicUser{func() identity.PublicUser { v := actor; v.ID = 10; return v }(), func() identity.PublicUser { v := actor; v.CategoryIDs = nil; return v }()} {
		yes := true
		before := projectionQueries
		if _, e := svc.PlanBatches(a, p.ID, 1, 3, "", &yes); !errors.Is(e, service.ErrNotFound) {
			t.Fatalf("unauthorized page exposed count %v", e)
		}
		if projectionQueries != before {
			t.Fatal("unauthorized page ran pending count query")
		}
	}
}

func TestPlanPendingEmptySetAndManualReviewStillCannotFinish(t *testing.T) {
	s, p, bs, _, now := settledPlan(t, 1, 0)
	s.PlanClock = func() time.Time { return now }
	yes, no := true, false
	rows, total, e := s.PlanBatchPage(p.ID, 1, 1, "", &yes)
	if e != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("normal reviewing pending %d %v %v", total, rows, e)
	}
	rows, total, e = s.PlanBatchPage(p.ID, 2, 1, "", &no)
	if e != nil || total != 2 || len(rows) != 1 || rows[0].ID != bs[1].ID {
		t.Fatalf("empty pending complement %d %v %v", total, rows, e)
	}
	var v model.Task
	if e = s.DB.Where("batch_id=?", bs[0].ID).First(&v).Error; e != nil {
		t.Fatal(e)
	}
	accepted := now.Add(-25 * time.Hour)
	stop := now.Add(-time.Minute)
	latest := model.LatestManualVerification{AttemptID: *v.CurrentAttemptID, TaskRevision: v.Revision, Conclusion: model.StatusReviewing, VerifiedAt: now}
	raw := fmt.Sprintf(`{"attempt_id":"%d","task_revision":"%d","conclusion":"reviewing","verified_at":"%s"}`, latest.AttemptID, latest.TaskRevision, latest.VerifiedAt.Format(time.RFC3339Nano))
	if e = s.DB.Model(&model.Task{}).Where("id=?", v.ID).Updates(map[string]any{"accepted_at": accepted, "check_stopped_at": stop, "latest_manual_verification_json": raw}).Error; e != nil {
		t.Fatal(e)
	}
	_, total, e = s.PlanBatchPage(p.ID, 1, 1, "", &yes)
	if e != nil || total != 0 {
		t.Fatalf("valid manual reviewing remained pending total=%d %v", total, e)
	}
	if _, e = s.FinishPlan(p.ID, 9, model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 2}}, EndReason: "unmet", EndNote: "不能把仍审核当最终结果"}, now); e == nil {
		t.Fatal("manual reviewing allowed finish")
	}
}

func TestPlanPendingUnstartedPlanHasNoTasksAndExactEmptyComplement(t *testing.T) {
	s, p, bs, _, _ := preparedPlan(t)
	yes, no := true, false
	rows, total, e := s.PlanBatchPage(p.ID, 1, 1, "", &yes)
	if e != nil || len(rows) != 0 || total != 0 {
		t.Fatalf("empty plan pending rows=%v total=%d error=%v", rows, total, e)
	}
	rows, total, e = s.PlanBatchPage(p.ID, 2, 1, "", &no)
	if e != nil || len(rows) != 1 || total != 2 || rows[0].ID != bs[1].ID {
		t.Fatalf("empty plan nonpending rows=%v total=%d error=%v", rows, total, e)
	}
}
