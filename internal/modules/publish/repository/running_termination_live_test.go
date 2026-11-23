package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"sync"
	"testing"
	"time"
)

func runningTermination(t *testing.T) (repo.ExecutionStore, model.Claim) {
	t.Helper()
	s, makeTask := ownedStore(t)
	v := makeTask(0)
	now := time.Now().UTC()
	if _, err := s.Start(v, v.Revision, 9, now); err != nil {
		t.Fatal(err)
	}
	c, err := s.Claim(9, "safe-stop", now)
	if err != nil || c == nil {
		t.Fatalf("claim: %v", err)
	}
	return s, *c
}
func terminationReport(c model.Claim, seq uint64, stage model.AttemptStage) model.ExecutionReport {
	in := c.Attempt.Input
	return model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: seq, Stage: stage, PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID, VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256, PortraitCoverSHA256: in.PortraitCoverSHA256}
}
func terminationCompletion(t *testing.T, c model.Claim, seq uint64, status model.Status, closed bool) model.Completion {
	t.Helper()
	in := c.Attempt.Input
	r := model.ExecutionResult{Status: status, ErrorCode: "EXECUTION_STOPPED", Evidence: model.ResultEvidence{PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID, VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256, PortraitCoverSHA256: in.PortraitCoverSHA256, NotSubmitted: true, SceneClosed: closed}}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := model.CompletionHash(seq, raw)
	if err != nil {
		t.Fatal(err)
	}
	return model.Completion{AttemptID: c.Attempt.ID, Seq: seq, Result: raw, ResultHash: hash}
}
func TestLiveRunningTerminationWaitsForOriginalSafeEvidence(t *testing.T) {
	for _, status := range []model.Status{model.StatusFailed, model.StatusUncertain} {
		t.Run(string(status), func(t *testing.T) {
			s, c := runningTermination(t)
			v, err := s.Get(c.Task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !v.CanCancel {
				t.Fatal("missing running termination action")
			}
			requested, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if requested.Status != model.StatusRunning || requested.CancelRequestedAt == nil {
				t.Fatalf("premature cancellation: %+v", requested)
			}
			f, err := s.Report(v, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1}, true, time.Now().UTC())
			if err != nil || !f.StopRequested || f.AllowSubmit {
				t.Fatalf("stop not delivered: %+v %v", f, err)
			}
			packet := terminationCompletion(t, c, 2, status, status == model.StatusFailed)
			f, err = s.Complete(v, 9, packet, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			want := status
			if status == model.StatusFailed {
				want = model.StatusCancelled
			}
			actual, _ := s.Get(v.ID)
			if actual.CanCancel {
				t.Fatal("finished task still cancellable")
			}
			if actual.Status != want || f.TaskStatus != want {
				t.Fatalf("task=%s feedback=%s want=%s", actual.Status, f.TaskStatus, want)
			}
			history, err := s.Attempt(v.ID)
			if err != nil || *history[0].ResultStatus != status || *history[0].ResultHash != packet.ResultHash {
				t.Fatal("original result changed", err)
			}
			if _, err = s.Complete(v, 9, packet, time.Now().UTC()); err != nil {
				t.Fatal("lost reply replay", err)
			}
			if _, err = s.Create(task("stopped-replacement", v.CompositeOutputID, v.CoverFileID)); !errors.Is(err, repo.ErrOccupied) {
				t.Fatal("released occupancy", err)
			}
		})
	}
}
func TestLiveRunningTerminationCannotPassSubmissionBarrier(t *testing.T) {
	for _, stopFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "stop_first", false: "permit_first"}[stopFirst], func(t *testing.T) {
			s, c := runningTermination(t)
			for n, stage := range []model.AttemptStage{model.AttemptStagePreparing, model.AttemptStageReady} {
				if _, err := s.Report(c.Task, 9, terminationReport(c, uint64(n+1), stage), true, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			v, _ := s.Get(c.Task.ID)
			if stopFirst {
				if _, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			f, err := s.Report(c.Task, 9, terminationReport(c, 3, model.AttemptStageSubmitting), true, time.Now().UTC())
			if err != nil || f.AllowSubmit == stopFirst || f.StopRequested != stopFirst {
				t.Fatalf("permission=%+v err=%v", f, err)
			}
			if !stopFirst {
				v, _ = s.Get(v.ID)
				if _, err = s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, time.Now().UTC()); !errors.Is(err, repo.ErrConflict) {
					t.Fatal("cancel after permit", err)
				}
			}
		})
	}
}
func TestLiveRunningTerminationRejectsUnprovenAndForeignClose(t *testing.T) {
	s, c := runningTermination(t)
	v, _ := s.Get(c.Task.ID)
	if _, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	p := terminationCompletion(t, c, 1, model.StatusFailed, false)
	if _, err := s.Complete(v, 9, p, time.Now().UTC()); !errors.Is(err, repo.ErrConflict) {
		t.Fatal("open scene accepted", err)
	}
	p = terminationCompletion(t, c, 1, model.StatusFailed, true)
	var r model.ExecutionResult
	json.Unmarshal(p.Result, &r)
	r.Evidence.CoverSHA256 = "foreign"
	p.Result, _ = json.Marshal(r)
	p.ResultHash, _ = model.CompletionHash(p.Seq, p.Result)
	if _, err := s.Complete(v, 9, p, time.Now().UTC()); !errors.Is(err, repo.ErrConflict) {
		t.Fatal("foreign evidence accepted", err)
	}
	actual, _ := s.Get(v.ID)
	if actual.Status != model.StatusRunning {
		t.Fatal("unproven scene cancelled")
	}
}

func TestLiveRunningTerminationRacesPermissionAndRollsBackAudit(t *testing.T) {
	for n := 0; n < 5; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, c := runningTermination(t)
			for n, stage := range []model.AttemptStage{model.AttemptStagePreparing, model.AttemptStageReady} {
				if _, err := s.Report(c.Task, 9, terminationReport(c, uint64(n+1), stage), true, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			v, _ := s.Get(c.Task.ID)
			var cancelErr, reportErr error
			var f model.ExecutionFeedback
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, cancelErr = s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, time.Now().UTC())
			}()
			go func() {
				defer wg.Done()
				f, reportErr = s.Report(c.Task, 9, terminationReport(c, 3, model.AttemptStageSubmitting), true, time.Now().UTC())
			}()
			wg.Wait()
			if reportErr != nil && !errors.Is(reportErr, repo.ErrConflict) {
				t.Fatal(reportErr)
			}
			current, _ := s.Get(v.ID)
			if errors.Is(reportErr, repo.ErrConflict) {
				if cancelErr != nil || current.CancelRequestedAt == nil || f.AllowSubmit {
					t.Fatal("unsafe report conflict", cancelErr)
				}
				return
			}
			if f.AllowSubmit {
				if !errors.Is(cancelErr, repo.ErrConflict) || current.CancelRequestedAt != nil {
					t.Fatalf("both granted: %v %+v", cancelErr, current)
				}
			} else if cancelErr != nil || !f.StopRequested || current.CancelRequestedAt == nil {
				t.Fatalf("no safe winner: %v %+v", cancelErr, f)
			}
		})
	}
	s, c := runningTermination(t)
	v, _ := s.Get(c.Task.ID)
	if err := s.DB.Exec("DROP TABLE audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, time.Now().UTC()); err == nil {
		t.Fatal("missing audit ignored")
	}
	current, _ := s.Get(v.ID)
	if current.CancelRequestedAt != nil || current.Revision != v.Revision || !current.CanCancel {
		t.Fatal("partial stop persisted")
	}
}
