package repository_test

import (
	"errors"
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"reflect"
	"testing"
	"time"
)

func TestReviewingManualFourConclusionsRetainOriginalAttemptAndOccupancy(t *testing.T) {
	for _, conclusion := range []model.Status{model.StatusFailed, model.StatusUncertain, model.StatusReviewing, model.StatusPublished} {
		t.Run(string(conclusion), func(t *testing.T) {
			s, c, v := reviewingPublication(t)
			before, _ := s.Attempt(v.ID)
			stop := time.Now().UTC().Add(-time.Second)
			s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("check_stopped_at", stop)
			v, _ = s.Get(v.ID)
			in := publicResolution(c, v)
			in.Conclusion = conclusion
			if conclusion != model.StatusPublished {
				in.PlatformWorkID = ""
				in.WorkURL = ""
				in.PublishedAt = nil
				in.Evidence.PubliclyAccessible = false
			}
			if conclusion == model.StatusFailed || conclusion == model.StatusUncertain {
				in.PlatformSubmissionID = ""
				in.TargetURL = ""
				in.Evidence.WorkMatched = false
				in.Evidence.NoPublishableWork = conclusion == model.StatusFailed
			}
			got, e := s.ResolveResult(v, 9, in, time.Now().UTC())
			if e != nil || got.Status != conclusion {
				t.Fatalf("resolve %+v %v", got, e)
			}
			if got.LatestManualVerification == nil || got.LatestManualVerification.Conclusion != conclusion || got.LatestManualVerification.AttemptID != c.Attempt.ID || got.LatestManualVerification.TaskRevision != got.Revision {
				t.Fatal("missing latest manual fact")
			}
			if got.PlatformSubmissionID == nil || *got.PlatformSubmissionID != *v.PlatformSubmissionID {
				t.Fatal("original submission lost")
			}
			after, _ := s.Attempt(v.ID)
			if !reflect.DeepEqual(before[0].Evidence, after[0].Evidence) || *before[0].ResultHash != *after[0].ResultHash {
				t.Fatal("original attempt result rewritten")
			}
			if _, e = s.Create(task("duplicate", v.CompositeOutputID, 1)); !errors.Is(e, repo.ErrOccupied) {
				t.Fatal("manual resolution released occupancy")
			}
			if conclusion == model.StatusUncertain {
				again := publicResolution(c, got)
				again.Conclusion = model.StatusReviewing
				again.PlatformWorkID = ""
				again.WorkURL = ""
				again.PublishedAt = nil
				again.Evidence.PubliclyAccessible = false
				got, e = s.ResolveResult(got, 9, again, time.Now().UTC())
				if e != nil || got.Status != model.StatusReviewing {
					t.Fatalf("cannot reverify manual unknown %+v %v", got, e)
				}
			}
		})
	}
}
func TestPlanFinishRefusesReviewingAndRequiresExactGoalShortfall(t *testing.T) {
	s, p, _, _, now := settledPlan(t, 1, 0)
	if _, e := s.FinishPlan(p.ID, 9, model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 2}}, EndReason: "unmet", EndNote: "核实未达成"}, now); e == nil {
		t.Fatal("ended plan with reviewing/future batch")
	}
}

func TestPlanFinishRejectsEveryUnresolvedStateAndKeepsEndExplanation(t *testing.T) {
	for _, status := range []model.Status{model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain, model.StatusReviewing, model.StatusPublished} {
		t.Run(string(status), func(t *testing.T) {
			s, p, bs, _, now := settledPlan(t, 0, 1)
			s.DB.Model(&model.Batch{}).Where("id=?", bs[1].ID).Updates(map[string]any{"execution_status": model.BatchCancelled, "ended_at": now})
			s.DB.Model(&model.Task{}).Where("batch_id=?", bs[0].ID).Update("status", status)
			in := model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 2}}, EndReason: "unmet", EndNote: "保留目标缺口"}
			if _, e := s.FinishPlan(p.ID, 9, in, now); !errors.Is(e, repo.ErrUnresolved) {
				t.Fatalf("ended %s: %v", status, e)
			}
			got, _, _ := s.GetPlan(p.ID)
			if got.EndedAt != nil || got.EndNote != "" {
				t.Fatal("failed finish changed explanation")
			}
		})
	}
	s, p, bs, _, now := settledPlan(t, 0, 1)
	s.DB.Model(&model.Batch{}).Where("id=?", bs[1].ID).Updates(map[string]any{"execution_status": model.BatchCancelled, "ended_at": now})
	in := model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 1}}, EndReason: "unmet", EndNote: "核实失败，保留原目标"}
	if _, e := s.FinishPlan(p.ID, 9, in, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("stale goal gap %v", e)
	}
	in.ExpectedShortfalls[0].Shortfall = 2
	in.EndNote = ""
	if _, e := s.FinishPlan(p.ID, 9, in, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatal("ended without reason")
	}
	in.EndNote = "核实失败，保留原目标"
	ended, e := s.FinishPlan(p.ID, 9, in, now)
	if e != nil || ended.Status != "ended" || ended.EndReason != "unmet" || ended.EndNote != in.EndNote {
		t.Fatalf("finish %+v %v", ended, e)
	}
	if _, e = s.FinishPlan(p.ID, 9, in, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("stale revision %v", e)
	}
}
func TestPlanCancelledReviewingLatePublicUpdatesOnlyOriginalFacts(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	s.RecordWork = service.NewExecutionStore(s.DB).RecordWork
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	c := acceptanceClaim(t, s, "plan-original", now)
	acceptanceReviewing(t, s, c, now)
	v, _ := s.Get(c.Task.ID)
	p, _, _ = s.GetPlan(p.ID)
	if _, e := s.ControlPlan(p.ID, 9, p.Revision, "cancel", "整体终止原因", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	before, _, _ := s.GetPlan(p.ID)
	if before.Status != "ended" {
		t.Fatal("safe reviewing cannot end cancel")
	}
	original, _ := s.Attempt(v.ID)
	pub := publicResolution(*c, v)
	pub.PlatformSubmissionID = *v.PlatformSubmissionID
	pub.TargetURL = ""
	published, e := s.ResolveResult(v, 9, pub, now.Add(time.Second))
	if e != nil || published.WorkID == nil {
		t.Fatal(e)
	}
	after, _, _ := s.GetPlan(p.ID)
	if after.Status != "ended" || after.EndNote != before.EndNote || !after.EndedAt.Equal(*before.EndedAt) || *after.CurrentBatchID != *before.CurrentBatchID {
		t.Fatal("late public reopened plan")
	}
	history, _ := s.Attempt(v.ID)
	if *history[0].ResultHash != *original[0].ResultHash || !reflect.DeepEqual(history[0].Evidence, original[0].Evidence) {
		t.Fatal("late public rewrote original receipt")
	}
	state, e := s.PlanProgress(p.ID, now.Add(time.Second))
	if e != nil || state.Progress != "ended" || len(state.Tasks) != 1 || state.Tasks[0].VerificationRequired {
		t.Fatal("late public not projected")
	}
}
func TestPlanCancellationWaitsForUnknownOriginalAndKeepsReason(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	c := acceptanceClaim(t, s, "original", now)
	acceptanceUnknown(t, s, c, now)
	p, _, _ = s.GetPlan(p.ID)
	if _, e := s.ControlPlan(p.ID, 9, p.Revision, "cancel", "永久终止原因", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	if e := s.ReconcileStoppedPlan(p.ID, now); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.Status != "stopping" || p.EndedAt != nil {
		t.Fatal("forced unknown terminal")
	}
	v, _ := s.Get(c.Task.ID)
	if _, e := s.ResolveResult(v, 9, resolution(*c, v), now.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if e := s.ReconcileStoppedPlan(p.ID, now.Add(2*time.Second)); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.Status != "ended" || p.EndNote != "永久终止原因" || p.EndReason != "cancelled" {
		t.Fatalf("wrong cancel result %+v", p)
	}
}

func TestManagerReviewingMayOnlySupplementPublicOriginal(t *testing.T) {
	s, c, v := reviewingPublication(t)
	team := identity.TeamID(7)
	manager := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	recovery := service.RecoveryService{Repo: s}
	for _, conclusion := range []model.Status{model.StatusFailed, model.StatusUncertain, model.StatusReviewing} {
		in := resolution(c, v)
		in.Conclusion = conclusion
		if conclusion == model.StatusReviewing {
			in = publicResolution(c, v)
			in.Conclusion = conclusion
			in.PlatformWorkID = ""
			in.WorkURL = ""
			in.PublishedAt = nil
			in.Evidence.PubliclyAccessible = false
		}
		if _, e := recovery.Resolve(manager, v.ID, in); !errors.Is(e, service.ErrNotFound) {
			t.Fatalf("manager %s accepted %v", conclusion, e)
		}
	}
	pub := publicResolution(c, v)
	got, e := recovery.Resolve(manager, v.ID, pub)
	if e != nil || got.Status != model.StatusPublished || got.WorkID == nil {
		t.Fatalf("legal manager public supplement %+v %v", got, e)
	}
}
func TestPlanAchievedFinishUsesMatchedWorkAndLateResultsNeverReopen(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	s.RecordWork = service.NewExecutionStore(s.DB).RecordWork
	p.Goals[0].TargetCount = 1
	rows := []model.PlanStructureBatch{{BatchID: &bs[0].ID, RowKey: "first", Name: bs[0].Name, Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: bs[0].Targets}}
	var e error
	p, bs, e = s.EditPlanStructure(p, p.Revision, rows)
	if e != nil {
		t.Fatal(e)
	}
	in.ExpectedRevision = p.Revision
	in.BatchConfirmations = in.BatchConfirmations[:1]
	in.BatchConfirmations[0].ExpectedRevision = bs[0].Revision
	now := time.Now().UTC()
	if _, e = s.StartPlan(p.ID, 9, in, "one-start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	c := acceptanceClaim(t, s, "public-original", now)
	acceptanceReviewing(t, s, c, now)
	v, _ := s.Get(c.Task.ID)
	pub := publicResolution(*c, v)
	pub.PlatformSubmissionID = *v.PlatformSubmissionID
	pub.TargetURL = ""
	v, e = s.ResolveResult(v, 9, pub, now)
	if e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	finish := model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 0}}, EndReason: "achieved", EndNote: "已核实目标达成"}
	ended, e := s.FinishPlan(p.ID, 9, finish, now)
	if e != nil || ended.EndReason != "achieved" {
		t.Fatalf("achieved %+v %v", ended, e)
	}
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store}
	detail, e := svc.GetPlan(actor, p.ID)
	if e != nil || detail.Plan.PublishedCount != 1 || detail.Plan.GoalResults[0].Shortfall != 0 || detail.Plan.Progress != "ended" {
		t.Fatalf("final summary %+v %v", detail.Plan, e)
	}
}

func TestPlanCancelStillWaitsForLateUnknownInAlreadyEndedBatch(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	c := acceptanceClaim(t, s, "original", now)
	acceptanceReviewing(t, s, c, now)
	v, _ := s.Get(c.Task.ID)
	unknown := resolution(*c, v)
	unknown.Conclusion = model.StatusUncertain
	unknown.Evidence.NoPublishableWork = false
	if _, e := s.ResolveResult(v, 9, unknown, now.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	b, _ := s.GetBatch(bs[0].ID)
	if b.ExecutionStatus != model.BatchEnded {
		t.Fatal("fixture batch not ended")
	}
	p, _, _ = s.GetPlan(p.ID)
	if _, e := s.ControlPlan(p.ID, 9, p.Revision, "cancel", "旧批仍待核实", 0, 0, now.Add(2*time.Second)); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.EndedAt != nil || p.Status != "stopping" {
		t.Fatal("cancel forced late unknown terminal")
	}
	if e := s.ReconcileStoppedPlan(p.ID, now.Add(3*time.Second)); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.EndedAt != nil {
		t.Fatal("reconcile ignored old unknown")
	}
}

func TestPlanRealWorkOverageInOneCellCannotOffsetAnotherCell(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	s.RecordWork = service.NewExecutionStore(s.DB).RecordWork
	// Legacy-data robustness fixture: permit an extra original task in cell A.
	// Both public records below still go through actual task/attempt/work writes.
	p.Goals = []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 1}, {Platform: "baidu", CategoryID: 2, TargetCount: 1}}
	rows := []model.PlanStructureBatch{{BatchID: &bs[0].ID, RowKey: "first", Name: "two cells", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}, {CategoryID: 2, TargetCount: 1}}}}
	var e error
	p, bs, e = s.EditPlanStructure(p, p.Revision, rows)
	if e != nil {
		t.Fatal(e)
	}
	b := bs[0]
	b.Config.Candidates[0].Content.Confirmed = true
	b.ConfigurationStatus = model.BatchReady
	b.Readiness.Ready = true
	b, e = s.EditBatch(b, b.Revision)
	if e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	in.ExpectedRevision = p.Revision
	in.BatchConfirmations = in.BatchConfirmations[:1]
	in.BatchConfirmations[0].ExpectedRevision = b.Revision
	in.BatchConfirmations[0].AllowShortfall = true
	in.BatchConfirmations[0].ExpectedShortfalls = []model.ExpectedShortfall{{CategoryID: 1, Shortfall: 0}, {CategoryID: 2, Shortfall: 1}}
	now := time.Now().UTC()
	if _, e = s.StartPlan(p.ID, 9, in, "two-cell-start", prepared[b.ID], now); e != nil {
		t.Fatal(e)
	}
	extra := prepared[b.ID][0]
	extra.ID = 0
	extra.BatchID = &b.ID
	extra.TaskOrder = 1
	extra.CompositeOutputID = 43
	extra.RequestKey = "legacy-extra"
	extra.RequestHash = "legacy-extra"
	extra.Status = model.StatusPending
	extra.AuthorizedAt = &now
	extra.Revision = 1
	extra.CreatedBy = 9
	key := "legacy-extra"
	extra.RowKey = &key
	if _, e = s.Create(extra); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		c := acceptanceClaim(t, s, fmt.Sprint("public-", i), now.Add(time.Duration(i)*time.Second))
		acceptanceReviewing(t, s, c, now.Add(time.Duration(i)*time.Second))
		v, _ := s.Get(c.Task.ID)
		pub := publicResolution(*c, v)
		pub.PlatformSubmissionID = *v.PlatformSubmissionID
		pub.TargetURL = ""
		pub.PlatformWorkID = fmt.Sprint("700039512208501412", i)
		pub.WorkURL = "https://mbd.baidu.com/newspage/data/videolanding?nid=sv_" + pub.PlatformWorkID
		if _, e = s.ResolveResult(v, 9, pub, now.Add(time.Duration(i)*time.Second)); e != nil {
			t.Fatal(e)
		}
	}
	p, _, _ = s.GetPlan(p.ID)
	bad := model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 0}, {Platform: "baidu", CategoryID: 2, Shortfall: 0}}, EndReason: "achieved"}
	if _, e = s.FinishPlan(p.ID, 9, bad, now.Add(3*time.Second)); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("cross-cell offset accepted %v", e)
	}
	bad.ExpectedShortfalls[1].Shortfall = 1
	bad.EndReason = "unmet"
	bad.EndNote = "另一类目仍缺1条，保留原目标"
	if _, e = s.FinishPlan(p.ID, 9, bad, now.Add(3*time.Second)); e != nil {
		t.Fatal(e)
	}
}

func TestExpiredManualReviewingRemainsEffectiveAfterLateCheckSweep(t *testing.T) {
	s, c, v := reviewingPublication(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	cutoff := now.Add(-time.Second)
	accepted := cutoff.Add(-24 * time.Hour)
	if e := s.DB.Model(&model.Task{}).Where("id=?", v.ID).Updates(map[string]any{"accepted_at": accepted, "check_stopped_at": nil, "next_check_at": nil}).Error; e != nil {
		t.Fatal(e)
	}
	v, _ = s.Get(v.ID)
	in := publicResolution(c, v)
	in.Conclusion = model.StatusReviewing
	in.PlatformWorkID = ""
	in.WorkURL = ""
	in.PublishedAt = nil
	in.Evidence.PubliclyAccessible = false
	verified, e := s.ResolveResult(v, 9, in, cutoff.Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if verified.VerificationRequiredAt(cutoff.Add(time.Second)) {
		t.Fatal("fresh explicit manual reviewing still requires verification")
	}
	if e = s.SchedulePublicCheck(cutoff.Add(2 * time.Second)); e != nil {
		t.Fatal(e)
	}
	verified, e = s.Get(v.ID)
	if e != nil {
		t.Fatal(e)
	}
	if verified.VerificationRequiredAt(cutoff.Add(2 * time.Second)) {
		t.Fatal("late sweep invalidated newer manual verification without a new observation")
	}
	p := model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "manual-review", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 2}}, RequestKey: "manual-review", RequestHash: "manual-review"}
	rows := []model.PlanStructureBatch{{RowKey: "first", Name: "first", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}, {RowKey: "next", Name: "next", Platform: "baidu", GroupOrder: 1, BatchOrder: 2, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}
	p, bs, e := s.CreatePlan(p, rows)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Updates(map[string]any{"started_at": accepted, "current_batch_id": bs[0].ID, "status": "running"}).Error; e != nil {
		t.Fatal(e)
	}
	if e = s.DB.Model(&model.Batch{}).Where("id=?", bs[0].ID).Updates(map[string]any{"started_at": accepted, "execution_status": model.BatchEnded}).Error; e != nil {
		t.Fatal(e)
	}
	if e = s.DB.Model(&model.Task{}).Where("id=?", v.ID).Updates(map[string]any{"batch_id": bs[0].ID, "task_order": 0}).Error; e != nil {
		t.Fatal(e)
	}
	state, e := s.PlanProgress(p.ID, cutoff.Add(2*time.Second))
	if e != nil || state.Progress != "running" || state.Basis.ExplicitCount != 1 {
		t.Fatalf("manual result no longer participates in handoff %+v %v", state, e)
	}
	// Even after all other batches are legally terminated, reviewing is never final public proof.
	if e = s.DB.Model(&model.Batch{}).Where("id=?", bs[1].ID).Updates(map[string]any{"execution_status": model.BatchCancelled, "ended_at": now}).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.FinishPlan(p.ID, 9, model.FinishPlanInput{ExpectedRevision: p.Revision, ExpectedShortfalls: []model.PlanGoalShortfall{{Platform: "baidu", CategoryID: 1, Shortfall: 2}}, EndReason: "unmet", EndNote: "不可将审核当公开"}, cutoff.Add(2*time.Second)); !errors.Is(e, repo.ErrUnresolved) {
		t.Fatalf("manual reviewing incorrectly allows finish %v", e)
	}
	// A genuinely later stopped/unclear check is a new basis and must block again.
	newer := cutoff.Add(3 * time.Second)
	if e = s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("check_stopped_at", newer).Error; e != nil {
		t.Fatal(e)
	}
	verified, _ = s.Get(v.ID)
	if !verified.VerificationRequiredAt(newer) {
		t.Fatal("new unclear observation did not require verification")
	}
}
