package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"reflect"
	"testing"
	"time"
)

func TestPlanConcurrentAdvanceCreatesOneNextExecutionAndReceipt(t *testing.T) {
	for _, mode := range []string{"auto_auto", "confirm_confirm", "auto_confirm"} {
		t.Run(mode, func(t *testing.T) {
			accepted, failed := 9, 1
			if mode == "confirm_confirm" {
				accepted, failed = 8, 2
			}
			s, p, bs, tasks, now := settledPlan(t, accepted, failed)
			state, e := s.PlanProgress(p.ID, now)
			if e != nil {
				t.Fatal(e)
			}
			gate := s.DB.Begin()
			if gate.Error != nil {
				t.Fatal(gate.Error)
			}
			defer gate.Rollback()
			if e = gate.Exec("SELECT id FROM publication_plan WHERE id=? FOR UPDATE", p.ID).Error; e != nil {
				t.Fatal(e)
			}
			type outcome struct {
				result    model.PlanAdvanceResult
				err       error
				input     model.PlanAdvanceInput
				digest    string
				automatic bool
			}
			done := make(chan outcome, 2)
			ready := make(chan struct{}, 2)
			for i := 0; i < 2; i++ {
				go func(i int) {
					automatic := mode == "auto_auto" || mode == "auto_confirm" && i == 0
					in := model.PlanAdvanceInput{RequestKey: fmt.Sprint("competing-", i), ExpectedRevision: p.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint, Decision: "continue", Reason: "独立确认本次推进"}
					digest := fmt.Sprint("digest-", i)
					ready <- struct{}{}
					result, err := s.AdvancePlan(p.ID, 9, in, digest, tasks[bs[1].ID], automatic, now)
					done <- outcome{result, err, in, digest, automatic}
				}(i)
			}
			<-ready
			<-ready
			select {
			case r := <-done:
				t.Fatalf("command bypassed parent lock %+v", r)
			case <-time.After(50 * time.Millisecond):
			}
			if e = gate.Commit().Error; e != nil {
				t.Fatal(e)
			}
			successes := 0
			var winner outcome
			for i := 0; i < 2; i++ {
				select {
				case r := <-done:
					if r.err == nil {
						successes++
						winner = r
					} else if !errors.Is(r.err, repo.ErrConflict) && !errors.Is(r.err, repo.ErrParentBlocked) {
						t.Fatal(r.err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("concurrent advance stuck")
				}
			}
			if successes != 1 {
				t.Fatalf("next position started %d times", successes)
			}
			after, rows, e := s.GetPlan(p.ID)
			if e != nil {
				t.Fatal(e)
			}
			if after.Revision != p.Revision+1 || after.CurrentBatchID == nil || *after.CurrentBatchID != bs[1].ID || rows[0].HandedOffAt == nil {
				t.Fatal("cursor/revision/handoff not atomic")
			}
			var receipts map[string]json.RawMessage
			if e = json.Unmarshal(after.HandoffReceipts, &receipts); e != nil || len(receipts) != 1 {
				t.Fatalf("receipts=%v err=%v", receipts, e)
			}
			var nextTasks []model.Task
			if e = s.DB.Where("batch_id=?", bs[1].ID).Find(&nextTasks).Error; e != nil || len(nextTasks) != 1 || nextTasks[0].AuthorizedAt == nil {
				t.Fatalf("next tasks/auth count=%d err=%v", len(nextTasks), e)
			}
			var occupied, audits int64
			if e = s.DB.Table("publication_task").Where("composite_output_id=? AND occupied_output_id IS NOT NULL", 43).Count(&occupied).Error; e != nil || occupied != 1 {
				t.Fatalf("occupancy=%d %v", occupied, e)
			}
			if e = s.DB.Table("audit_logs").Where("action=?", "publication_plan.handoff").Count(&audits).Error; e != nil || audits != 1 {
				t.Fatalf("handoff audits=%d %v", audits, e)
			}
			replay, e := s.AdvancePlan(p.ID, 9, winner.input, winner.digest, nil, winner.automatic, now)
			if e != nil || !reflect.DeepEqual(replay, winner.result) {
				t.Fatalf("winning immutable receipt changed %v", e)
			}
		})
	}
}

func TestPlanSingleBatchStopPreservesParentAndRequiresOriginalAdvanceDecision(t *testing.T) {
	for _, threshold := range []bool{false, true} {
		t.Run(fmt.Sprint("threshold_", threshold), func(t *testing.T) {
			var s repo.ExecutionStore
			var p model.Plan
			var bs []model.Batch
			var prepared map[uint64][]model.Task
			var now time.Time
			if threshold {
				s, p, bs, prepared, now = settledPlan(t, 8, 2)
			} else {
				var in model.StartPlanInput
				s, p, bs, in, prepared = preparedPlan(t)
				now = time.Now().UTC()
				if _, e := s.StartPlan(p.ID, 9, in, "batch-stop", prepared[bs[0].ID], now); e != nil {
					t.Fatal(e)
				}
			}
			b, e := s.GetBatch(bs[0].ID)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "stop", ExpectedRevision: b.Revision, Reason: "仅终止本批"}, now); e != nil {
				t.Fatal(e)
			}
			state, e := s.PlanProgress(p.ID, now)
			if e != nil {
				t.Fatal(e)
			}
			want, decision := "waiting_skip", "skip"
			if threshold {
				want, decision = "waiting_continue", "continue"
			}
			if state.Plan.Status != "running" || state.Plan.StoppedAt != nil || state.Plan.EndedAt != nil || state.Progress != want {
				t.Fatalf("single-batch stop permanently affected parent %+v", state)
			}
			in := model.PlanAdvanceInput{RequestKey: "after-local-stop", ExpectedRevision: state.Plan.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint, Decision: decision, Reason: "核对原结果后继续"}
			if _, e = s.AdvancePlan(p.ID, 9, in, "auto-refused", prepared[bs[1].ID], true, now); !errors.Is(e, repo.ErrParentBlocked) {
				t.Fatalf("automatic bypassed explicit %s %v", decision, e)
			}
			result, e := s.AdvancePlan(p.ID, 9, in, "explicit", prepared[bs[1].ID], false, now)
			if e != nil || result.CurrentBatchID != bs[1].ID || result.BatchReceipt == nil {
				t.Fatalf("local stop prevented legal continuation %v %v", result, e)
			}
		})
	}
}

func TestPlanResumeDoesNotClearCurrentBatchPause(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	now := time.Now().UTC()
	if _, e := s.StartPlan(p.ID, 9, in, "independent-pause", tasks[bs[0].ID], now); e != nil {
		t.Fatal(e)
	}
	b, e := s.GetBatch(bs[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "pause", ExpectedRevision: b.Revision, Reason: "批次独立暂停"}, now); e != nil {
		t.Fatal(e)
	}
	before, e := s.GetBatch(b.ID)
	if e != nil {
		t.Fatal(e)
	}
	current, _, e := s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ControlPlan(p.ID, 9, current.Revision, "pause", "计划人工暂停", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	current, _, e = s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ControlPlan(p.ID, 9, current.Revision, "resume", "", 0, 0, now); e != nil {
		t.Fatal(e)
	}
	after, e := s.GetBatch(b.ID)
	if e != nil {
		t.Fatal(e)
	}
	if after.ExecutionStatus != model.BatchPaused || after.Revision != before.Revision || !reflect.DeepEqual(after.Pause, before.Pause) || after.Pause == nil {
		t.Fatal("plan resume erased/changed independent batch pause")
	}
	state, e := s.PlanProgress(p.ID, now)
	if e != nil || state.Plan.Status != "running" {
		t.Fatal(e)
	}
	next := model.PlanAdvanceInput{RequestKey: "cannot-bypass-pause", ExpectedRevision: state.Plan.Revision, CurrentBatchID: b.ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint}
	if _, e = s.AdvancePlan(p.ID, 9, next, "blocked", tasks[bs[1].ID], true, now); !errors.Is(e, repo.ErrUnresolved) {
		t.Fatalf("plan resume bypassed paused batch: %v", e)
	}
	var n int64
	if e = s.DB.Model(&model.Task{}).Where("batch_id=?", bs[1].ID).Count(&n).Error; e != nil || n != 0 {
		t.Fatalf("next batch created while independent pause retained %d %v", n, e)
	}
}
