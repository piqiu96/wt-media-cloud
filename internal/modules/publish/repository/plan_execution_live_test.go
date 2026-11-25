package repository_test

import (
	"errors"
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func preparedPlan(t *testing.T) (repo.ExecutionStore, model.Plan, []model.Batch, model.StartPlanInput, map[uint64][]model.Task) {
	t.Helper()
	s, _ := executionStore(t)
	p := model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "execution", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 2}}, RequestKey: "execution", RequestHash: "hash"}
	rows := []model.PlanStructureBatch{{RowKey: "first", Name: "first", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}, {RowKey: "second", Name: "second", Platform: "baidu", GroupOrder: 1, BatchOrder: 2, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}
	p, bs, e := s.CreatePlan(p, rows)
	if e != nil {
		t.Fatal(e)
	}
	in := model.StartPlanInput{RequestKey: "start", ExpectedRevision: 1, FirstBatchID: bs[0].ID}
	tasks := map[uint64][]model.Task{}
	for i, b := range bs {
		v := task("", int64(42+i), 1)
		v.RowKey = &rows[i].RowKey
		v.ResourceSnapshot = []byte(`{"profile_id":37}`)
		b.Config.Candidates = []model.BatchCandidate{{BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: int64(42 + i)}, RowKey: rows[i].RowKey, Content: v.Content}}
		b.ConfigurationStatus = model.BatchReady
		b.Readiness.Ready = true
		b, e = s.EditBatch(b, b.Revision)
		if e != nil {
			t.Fatal(e)
		}
		bs[i] = b
		in.BatchConfirmations = append(in.BatchConfirmations, model.PlanBatchConfirmation{BatchID: b.ID, ExpectedRevision: b.Revision, IncludedRowKeys: []string{rows[i].RowKey}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1, Shortfall: 0}}})
		tasks[b.ID] = []model.Task{v}
	}
	p, _, e = s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	in.ExpectedRevision = p.Revision
	return s, p, bs, in, tasks
}
func TestPlanStartFreezesAllAndReplaysImmutableFirstReceipt(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	now := time.Now().UTC()
	first, e := s.StartPlan(p.ID, 9, in, "digest", tasks[bs[0].ID], now)
	if e != nil {
		t.Fatal(e)
	}
	if len(first.FirstBatchReceipt.TaskIDs) != 1 {
		t.Fatal(first)
	}
	got, rows, e := s.GetPlan(p.ID)
	if e != nil || got.StartedAt == nil || *got.CurrentBatchID != bs[0].ID {
		t.Fatal(got, e)
	}
	for _, b := range rows {
		if b.FrozenConfirmation == nil || b.PlanFrozenAt == nil {
			t.Fatal("future configuration not frozen")
		}
	}
	if rows[1].StartedAt != nil {
		t.Fatal("future started early")
	}
	again, e := s.StartPlan(p.ID, 9, in, "digest", nil, now)
	if e != nil || !reflect.DeepEqual(first, again) {
		t.Fatalf("replay %v", e)
	}
	if _, e = s.StartPlan(p.ID, 9, in, "other", nil, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatal(e)
	}
}
func TestPlanStartResourceConflictRollsBackWholeFreeze(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	if _, e := s.Create(task("occupied", 42, 1)); e != nil {
		t.Fatal(e)
	}
	if _, e := s.StartPlan(p.ID, 9, in, "digest", tasks[bs[0].ID], time.Now().UTC()); !errors.Is(e, repo.ErrOccupied) {
		t.Fatalf("conflict %v", e)
	}
	got, rows, e := s.GetPlan(p.ID)
	if e != nil || got.StartedAt != nil {
		t.Fatal("partial plan start")
	}
	for _, b := range rows {
		if b.PlanFrozenAt != nil || b.StartedAt != nil {
			t.Fatal("partial freeze")
		}
	}
}

func settledPlan(t *testing.T, accepted, failed int) (repo.ExecutionStore, model.Plan, []model.Batch, map[uint64][]model.Task, time.Time) {
	t.Helper()
	s, p, bs, in, prepared := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "original", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	// Controlled explicit-result fixtures. Reviewing has a fixed original receipt
	// identity and acceptance time; it participates in handoff but is never public.
	s.DB.Where("batch_id=?", bs[0].ID).Delete(&model.Task{})
	for i := 0; i < accepted+failed; i++ {
		output := int64(100 + i)
		if e := s.DB.Exec("INSERT INTO composite_outputs(id) VALUES(?)", output).Error; e != nil {
			t.Fatal(e)
		}
		v := task("result-"+fmt.Sprint(i), output, 1)
		v.BatchID = &bs[0].ID
		v.TaskOrder = uint32(i)
		v.Status = model.StatusFailed
		if i < accepted {
			v.Status = model.StatusReviewing
			attempt := uint64(1000 + i)
			v.CurrentAttemptID = &attempt
			v.AcceptedAt = &now
			receipt := "123"
			v.PlatformSubmissionID = &receipt
		}
		if e := s.DB.Create(&v).Error; e != nil {
			t.Fatal(e)
		}
	}
	p, _, _ = s.GetPlan(p.ID)
	return s, p, bs, prepared, now
}
func TestPlanTenPercentAutomaticAndTwentyPercentExactConfirmation(t *testing.T) {
	for _, tc := range []struct {
		accepted, failed int
		progress         string
	}{{9, 1, "running"}, {8, 2, "waiting_continue"}, {0, 0, "waiting_skip"}} {
		t.Run(tc.progress, func(t *testing.T) {
			s, p, bs, prepared, now := settledPlan(t, tc.accepted, tc.failed)
			state, e := s.PlanProgress(p.ID, now)
			if e != nil || state.Progress != tc.progress {
				t.Fatalf("basis %+v %v", state, e)
			}
			in := model.PlanAdvanceInput{RequestKey: "next", ExpectedRevision: p.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint, Decision: "continue", Reason: "核对当前结果"}
			automatic := tc.progress == "running"
			if tc.progress == "waiting_skip" {
				in.Decision = "skip"
			}
			if !automatic {
				if _, e = s.AdvancePlan(p.ID, 9, in, "auto-refused", prepared[bs[1].ID], true, now); !errors.Is(e, repo.ErrParentBlocked) {
					t.Fatalf("auto bypass %v", e)
				}
			}
			result, e := s.AdvancePlan(p.ID, 9, in, "next-digest", prepared[bs[1].ID], automatic, now)
			if e != nil || result.CurrentBatchID != bs[1].ID || result.BatchReceipt == nil {
				t.Fatalf("advance %+v %v", result, e)
			}
			replay, e := s.AdvancePlan(p.ID, 9, in, "next-digest", nil, automatic, now)
			if e != nil || !reflect.DeepEqual(result, replay) {
				t.Fatalf("replay %v", e)
			}
			old, _, _ := s.GetPlan(p.ID)
			if *old.CurrentBatchID != bs[1].ID {
				t.Fatal("cursor")
			}
			var failed model.Task
			s.DB.Where("batch_id=? AND status='failed'", bs[0].ID).First(&failed)
			if failed.ID != 0 {
				if _, e = s.Retry(failed, failed.Revision, 9, now); !errors.Is(e, repo.ErrParentBlocked) {
					t.Fatalf("old batch retry %v", e)
				}
			}
		})
	}
}
func TestPlanPausePrefixUnknownAndPermanentCancelFence(t *testing.T) {
	s, p, bs, prepared, now := settledPlan(t, 1, 0)
	state, _ := s.PlanProgress(p.ID, now)
	in := model.PlanAdvanceInput{RequestKey: "next", ExpectedRevision: p.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint}
	if _, e := s.ControlPlan(p.ID, 9, p.Revision, "pause", "人工暂停", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	if _, e := s.AdvancePlan(p.ID, 9, in, "next", prepared[bs[1].ID], true, now); !errors.Is(e, repo.ErrConflict) && !errors.Is(e, repo.ErrParentBlocked) {
		t.Fatalf("paused %v", e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if _, e := s.ControlPlan(p.ID, 9, p.Revision, "resume", "", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	s.DB.Model(&model.Task{}).Where("batch_id=?", bs[0].ID).Updates(map[string]any{"status": model.StatusUncertain, "revision": 3})
	state, e := s.PlanProgress(p.ID, now)
	if e != nil || state.Progress != "waiting_tasks" {
		t.Fatalf("unknown %+v %v", state, e)
	}
	in.ExpectedRevision = p.Revision
	in.ExpectedBasisFingerprint = state.Basis.Fingerprint
	if _, e = s.AdvancePlan(p.ID, 9, in, "next", prepared[bs[1].ID], true, now); !errors.Is(e, repo.ErrUnresolved) {
		t.Fatalf("bypassed unknown %v", e)
	}
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "cancel", "整体终止", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.StoppedAt == nil {
		t.Fatal("no permanent fence")
	}
	var remaining int64
	s.DB.Model(&model.Task{}).Where("batch_id=?", bs[1].ID).Count(&remaining)
	if remaining != 0 {
		t.Fatal("future created")
	}
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "resume", "", 0, 0, now); !errors.Is(e, repo.ErrParentBlocked) {
		t.Fatalf("resumed stopped %v", e)
	}
}

func TestPlanConcurrentFirstStartOneTaskAndLateBasisIsRejected(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	now := time.Now().UTC()
	type result struct {
		receipt model.StartPlanResult
		e       error
	}
	done := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { r, e := s.StartPlan(p.ID, 9, in, "same", tasks[bs[0].ID], now); done <- result{r, e} }()
	}
	a, b := <-done, <-done
	if a.e != nil || b.e != nil || !reflect.DeepEqual(a.receipt, b.receipt) {
		t.Fatalf("race %v %v", a.e, b.e)
	}
	var n int64
	s.DB.Model(&model.Task{}).Where("batch_id=?", bs[0].ID).Count(&n)
	if n != 1 {
		t.Fatal("duplicate start")
	}
	var task model.Task
	s.DB.Where("batch_id=?", bs[0].ID).First(&task)
	s.DB.Model(&model.Task{}).Where("id=?", task.ID).Updates(map[string]any{"status": model.StatusFailed, "revision": task.Revision + 1})
	state, e := s.PlanProgress(p.ID, now)
	if e != nil {
		t.Fatal(e)
	}
	next := model.PlanAdvanceInput{RequestKey: "confirm", ExpectedRevision: state.Plan.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint, Decision: "continue", Reason: "核对失败结果"}
	s.DB.Model(&model.Task{}).Where("id=?", task.ID).Updates(map[string]any{"status": model.StatusUncertain, "revision": task.Revision + 2})
	if _, e = s.AdvancePlan(p.ID, 9, next, "digest", tasks[bs[1].ID], false, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("stale basis %v", e)
	}
}
func TestPlanResourceWaitCannotOverwritePauseOrNewPosition(t *testing.T) {
	s, p, bs, _, now := settledPlan(t, 1, 0)
	state, e := s.PlanProgress(p.ID, now)
	if e != nil {
		t.Fatal(e)
	}
	wait := model.PlanResourceWait{NextBatchID: bs[1].ID, CurrentBatchID: bs[0].ID, ExpectedRevision: p.Revision, Basis: state.Basis.Fingerprint, Code: "original_resources_unavailable", CheckedAt: now}
	if e = s.RecordPlanResourceWait(p.ID, wait); e != nil {
		t.Fatal(e)
	}
	state, e = s.PlanProgress(p.ID, now)
	if e != nil || state.Progress != "waiting_resources" {
		t.Fatalf("resource wait %+v %v", state, e)
	}
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "pause", "人工暂停", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	if e = s.RecordPlanResourceWait(p.ID, wait); e != nil {
		t.Fatal(e)
	}
	state, e = s.PlanProgress(p.ID, now)
	if e != nil || state.Progress != "manual_paused" {
		t.Fatalf("stale probe cleared pause %+v", state)
	}
	s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Updates(map[string]any{"status": "running", "current_batch_id": bs[1].ID, "revision": p.Revision + 2, "resource_wait_json": nil})
	if e = s.RecordPlanResourceWait(p.ID, wait); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.ResourceWait != nil {
		t.Fatal("stale probe overwrote new position")
	}
}
func TestPlanFutureOccupancyConflictDoesNotMoveCursor(t *testing.T) {
	s, p, bs, prepared, now := settledPlan(t, 1, 0)
	if _, e := s.Create(task("other-use", 43, 1)); e != nil {
		t.Fatal(e)
	}
	state, e := s.PlanProgress(p.ID, now)
	if e != nil {
		t.Fatal(e)
	}
	in := model.PlanAdvanceInput{RequestKey: "next", ExpectedRevision: p.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint}
	if _, e = s.AdvancePlan(p.ID, 9, in, "digest", prepared[bs[1].ID], true, now); !errors.Is(e, repo.ErrOccupied) {
		t.Fatalf("resource conflict %v", e)
	}
	p, rows, e := s.GetPlan(p.ID)
	if e != nil || *p.CurrentBatchID != bs[0].ID || rows[0].HandedOffAt != nil || rows[1].StartedAt != nil {
		t.Fatal("partial handoff")
	}
}

func TestPlanPauseKeepsCurrentPermissionButCancelFencesSubmitting(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	c, e := s.ClaimWithCapacity(9, "claim-plan", model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}, now)
	if e != nil || c == nil {
		t.Fatalf("claim %v", e)
	}
	if _, e = s.Report(c.Task, 9, terminationReport(*c, 1, model.AttemptStageReady), true, now); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "pause", "只停后续批", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	report := terminationReport(*c, 2, model.AttemptStageSubmitting)
	feedback, e := s.Report(c.Task, 9, report, true, now)
	if e != nil || !feedback.AllowSubmit || feedback.StopRequested {
		t.Fatalf("manual pause stopped current %+v %v", feedback, e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "cancel", "停止所有后续提交", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	report.Seq = 3
	feedback, e = s.Report(c.Task, 9, report, true, now)
	if e != nil && !errors.Is(e, repo.ErrConflict) || feedback.AllowSubmit || e == nil && !feedback.StopRequested {
		t.Fatalf("cancel did not fence %+v %v", feedback, e)
	}
	feedback, e = s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 4}, true, now)
	if e != nil || !feedback.StopRequested || feedback.AllowSubmit {
		t.Fatalf("heartbeat failed parent fence %+v %v", feedback, e)
	}
}

func TestPlanPermanentFenceAloneBlocksNewSubmitBeforeChildCleanup(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	c, e := s.ClaimWithCapacity(9, "claim-plan", model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}, now)
	if e != nil || c == nil {
		t.Fatal(e)
	}
	if _, e = s.Report(c.Task, 9, terminationReport(*c, 1, model.AttemptStageReady), true, now); e != nil {
		t.Fatal(e)
	}
	// Simulate the durable parent fence before a cleanup worker has touched any
	// batch/task: the parent itself must deny a fresh submit permission.
	if e = s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Updates(map[string]any{"stopped_at": now, "status": "stopping"}).Error; e != nil {
		t.Fatal(e)
	}
	f, e := s.Report(c.Task, 9, terminationReport(*c, 2, model.AttemptStageSubmitting), true, now)
	if e != nil || f.AllowSubmit || !f.StopRequested {
		t.Fatalf("parent-only fence %+v %v", f, e)
	}
}

func TestPlanDuplicateOutputErrorIdentifiesOnlyBothOwnedRowsAndRollsBack(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	second := bs[1]
	second.Config.Candidates[0].CompositeOutputID = 42
	var e error
	second, e = s.EditBatch(second, second.Revision)
	if e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	in.ExpectedRevision = p.Revision
	in.BatchConfirmations[1].ExpectedRevision = second.Revision
	_, e = s.StartPlan(p.ID, 9, in, "duplicate", prepared[bs[0].ID], time.Now().UTC())
	var conflict *repo.PlanConflictError
	if !errors.As(e, &conflict) {
		t.Fatalf("missing structured issue %v", e)
	}
	if conflict.PlanID != p.ID || len(conflict.Issues) != 1 {
		t.Fatal(conflict)
	}
	issue := conflict.Issues[0]
	if issue.BatchID != bs[1].ID || issue.RowKey != "second" || issue.RelatedBatchID != bs[0].ID || issue.RelatedRowKey != "first" {
		t.Fatalf("wrong rows %+v", issue)
	}
	got, rows, _ := s.GetPlan(p.ID)
	if got.StartedAt != nil || rows[0].PlanFrozenAt != nil || rows[1].PlanFrozenAt != nil {
		t.Fatal("duplicate partially froze plan")
	}
	var n int64
	s.DB.Model(&model.Task{}).Where("batch_id IN ?", []uint64{bs[0].ID, bs[1].ID}).Count(&n)
	if n != 0 {
		t.Fatal("duplicate created formal tasks")
	}
}

func TestPlanAdmissionClockRechecksReviewExpiryAfterBlockedParentLock(t *testing.T) {
	s, p, bs, prepared, event := settledPlan(t, 1, 0)
	var clock atomic.Value
	clock.Store(event)
	s.PlanClock = func() time.Time { return clock.Load().(time.Time) }
	accepted := event.Add(-24*time.Hour + time.Second)
	if e := s.DB.Model(&model.Task{}).Where("batch_id=?", bs[0].ID).Update("accepted_at", accepted).Error; e != nil {
		t.Fatal(e)
	}
	state, e := s.PlanProgress(p.ID, event)
	if e != nil || state.Progress != "running" {
		t.Fatalf("before expiry %+v %v", state, e)
	}
	in := model.PlanAdvanceInput{RequestKey: "clock", ExpectedRevision: p.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint}
	tx := s.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if e = tx.Exec("SELECT id FROM publication_plan WHERE id=? FOR UPDATE", p.ID).Error; e != nil {
		tx.Rollback()
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := s.AdvancePlan(p.ID, 9, in, "clock", prepared[bs[1].ID], true, event); done <- e }()
	select {
	case e = <-done:
		tx.Rollback()
		t.Fatalf("did not block on parent lock %v", e)
	case <-time.After(50 * time.Millisecond):
	}
	clock.Store(event.Add(2 * time.Second))
	if e = tx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
		if !errors.Is(e, repo.ErrConflict) && !errors.Is(e, repo.ErrUnresolved) {
			t.Fatalf("expired result advanced %v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("advance stuck")
	}
	p, rows, _ := s.GetPlan(p.ID)
	if *p.CurrentBatchID != bs[0].ID || rows[1].StartedAt != nil {
		t.Fatal("old admission timestamp passed expiry")
	}
	fresh, e := s.PlanProgress(p.ID, event)
	if e != nil || fresh.Progress != "waiting_verification" {
		t.Fatalf("read used stale clock %+v %v", fresh, e)
	}
	wait := model.PlanResourceWait{NextBatchID: bs[1].ID, CurrentBatchID: bs[0].ID, ExpectedRevision: p.Revision, Basis: state.Basis.Fingerprint, Code: "old-probe", CheckedAt: event}
	if e = s.RecordPlanResourceWait(p.ID, wait); e != nil {
		t.Fatal(e)
	}
	p, _, _ = s.GetPlan(p.ID)
	if p.ResourceWait != nil {
		t.Fatal("old resource probe crossed review deadline")
	}
}

func TestPlanTakeoverActionsMatchGuardAndNeverReauthorizeCurrentExecution(t *testing.T) {
	s, p, bs, in, prepared := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "takeover-start", prepared[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	p, bs, _ = s.GetPlan(p.ID)
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store, PlanExecutionRepo: s}
	detail, e := svc.GetPlan(owner, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if !slices.Contains(detail.Plan.AllowedActions, "takeover") || !slices.Contains(detail.Batches[0].AllowedActions, "takeover") || slices.Contains(detail.Batches[1].AllowedActions, "takeover") {
		t.Fatalf("plan actions don't match current execution %+v", detail)
	}
	independent, e := svc.GetBatch(owner, bs[0].ID)
	if e != nil || !slices.Contains(independent.AllowedActions, "takeover") {
		t.Fatalf("batch projection missing takeover %+v %v", independent.AllowedActions, e)
	}
	for _, role := range []identity.Role{identity.RoleAdmin, identity.RoleSeniorOperator} {
		manager := owner
		manager.ID = 11
		manager.Role = role
		read, e := svc.GetPlan(manager, p.ID)
		if e != nil {
			t.Fatal(e)
		}
		if slices.Contains(read.Plan.AllowedActions, "takeover") || slices.Contains(read.Batches[0].AllowedActions, "takeover") {
			t.Fatal("manager takeover action offered")
		}
	}
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "takeover", "不得接管未来位置", bs[1].ID, bs[1].Revision, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("future takeover %v", e)
	}
	if e = s.DB.Model(&model.Batch{}).Where("id=?", bs[0].ID).Update("execution_status", model.BatchEnded).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "takeover", "不得接管已结束批", bs[0].ID, bs[0].Revision, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("ended takeover %v", e)
	}
	if e = s.DB.Model(&model.Batch{}).Where("id=?", bs[0].ID).Update("execution_status", model.BatchRunning).Error; e != nil {
		t.Fatal(e)
	}
	var before []model.Task
	if e = s.DB.Where("batch_id=?", bs[0].ID).Find(&before).Error; e != nil {
		t.Fatal(e)
	}
	var attemptsBefore int64
	s.DB.Model(&model.Attempt{}).Count(&attemptsBefore)
	out, e := s.ControlPlan(p.ID, 9, p.Revision, "takeover", "运营接管当前已有执行", bs[0].ID, bs[0].Revision, now)
	if e != nil || out.Plan.Status != "paused" {
		t.Fatalf("takeover %+v %v", out, e)
	}
	var after []model.Task
	s.DB.Where("batch_id=?", bs[0].ID).Find(&after)
	var attemptsAfter int64
	s.DB.Model(&model.Attempt{}).Count(&attemptsAfter)
	if !reflect.DeepEqual(before, after) || attemptsAfter != attemptsBefore {
		t.Fatal("takeover created or reauthorized task/attempt")
	}
	p, _, _ = s.GetPlan(p.ID)
	paused, e := svc.GetPlan(owner, p.ID)
	if e != nil || slices.Contains(paused.Plan.AllowedActions, "takeover") {
		t.Fatal("already paused takeover still offered")
	}
	if _, e = s.ControlPlan(p.ID, 9, p.Revision, "takeover", "重复接管", bs[0].ID, bs[0].Revision, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("paused repeated takeover %v", e)
	}
}
