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

func TestLiveCancelKeepsOccupancyAndFencesClaim(t *testing.T) {
	s, makeTask := ownedStore(t)
	v := makeTask(0)
	now := time.Now().UTC()
	queued, err := s.Start(v, v.Revision, 9, now)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	var cancelled model.Task
	var cancelErr error
	var claim *model.Claim
	var claimErr error
	go func() {
		defer wg.Done()
		cancelled, cancelErr = s.Cancel(queued, model.Cancellation{ExpectedRevision: queued.Revision, Reason: "停止当前安排"}, 9, now)
	}()
	go func() { defer wg.Done(); claim, claimErr = s.Claim(9, "cancel-race", now) }()
	wg.Wait()
	if claimErr != nil {
		t.Fatal(claimErr)
	}
	current, err := s.Get(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelErr == nil {
		if claim != nil || cancelled.Status != model.StatusCancelled || current.CurrentAttemptID != nil || current.CancelRequestedAt == nil {
			t.Fatalf("cancel and claim both won: %+v %+v", current, claim)
		}
	} else if !errors.Is(cancelErr, repo.ErrConflict) || claim == nil || current.Status != model.StatusRunning {
		t.Fatalf("cancel=%v claim=%+v task=%+v", cancelErr, claim, current)
	}
	duplicate := task("replacement", v.CompositeOutputID, v.CoverFileID)
	if _, err := s.Create(duplicate); !errors.Is(err, repo.ErrOccupied) {
		t.Fatalf("cancel released occupancy: %v", err)
	}
}

func TestLiveCancelUnclaimedOnlyAndAuditRollback(t *testing.T) {
	s, makeTask := ownedStore(t)
	v := makeTask(0)
	now := time.Now().UTC()
	if _, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision + 1, Reason: "停止当前安排"}, 9, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal(err)
	}
	if err := s.DB.Exec("DROP TABLE audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, now); err == nil {
		t.Fatal("audit failure ignored")
	}
	after, _ := s.Get(v.ID)
	if after.Status != model.StatusPending || after.Revision != v.Revision || after.CancelRequestedAt != nil {
		t.Fatalf("partial cancellation: %+v", after)
	}
}

func TestLiveAbandonRequiresOriginalSafeFailureAndReleasesOnlyThen(t *testing.T) {
	s, c, original := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	now := time.Now().UTC()
	input := model.Abandonment{AttemptID: c.Attempt.ID, ExpectedRevision: v.Revision, Evidence: resolution(c, v).Evidence}
	if _, err := s.Abandon(v, 9, input, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("unknown abandoned: %v", err)
	}
	failed, err := s.ResolveResult(v, 9, resolution(c, v), now)
	if err != nil {
		t.Fatal(err)
	}
	input.ExpectedRevision = failed.Revision
	bad := input
	bad.Evidence.PlatformUID = "foreign"
	if _, err := s.Abandon(failed, 9, bad, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal(err)
	}
	bad = input
	bad.AttemptID++
	if _, err := s.Abandon(failed, 9, bad, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal(err)
	}
	bad = input
	bad.Evidence.SceneClosed = false
	if _, err := s.Abandon(failed, 9, bad, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal(err)
	}
	abandoned, err := s.Abandon(failed, 9, input, now)
	if err != nil || abandoned.Status != model.StatusAbandoned {
		t.Fatalf("%+v %v", abandoned, err)
	}
	history, err := s.Attempt(v.ID)
	if err != nil || *history[0].ResultHash != original.ResultHash || history[0].ResolvedAt == nil {
		t.Fatal("original attempt changed", err)
	}
	next := task("new-arrangement", v.CompositeOutputID, v.CoverFileID)
	if _, err := s.Create(next); err != nil {
		t.Fatalf("safe abandonment kept occupancy: %v", err)
	}
	if _, err := s.Retry(abandoned, abandoned.Revision, 9, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("abandoned task restarted: %v", err)
	}
}

func TestLiveCancelPendingAndQueuedRemainOccupiedAndCannotRestart(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "queued"}[queued], func(t *testing.T) {
			s, makeTask := ownedStore(t)
			v := makeTask(0)
			now := time.Now().UTC()
			if queued {
				var err error
				v, err = s.Start(v, v.Revision, 9, now)
				if err != nil {
					t.Fatal(err)
				}
			}
			cancelled, err := s.Cancel(v, model.Cancellation{ExpectedRevision: v.Revision, Reason: "停止当前安排"}, 9, now)
			if err != nil || cancelled.Status != model.StatusCancelled || cancelled.CancelRequestedAt == nil || cancelled.Revision != v.Revision+1 {
				t.Fatalf("%+v %v", cancelled, err)
			}
			var log struct {
				ActorUserID int64
				SummaryJSON string
			}
			if err = s.DB.Table("audit_logs").Select("actor_user_id,summary_json").Where("target_id=? AND action=?", fmt.Sprint(v.ID), "publication.cancel").Take(&log).Error; err != nil {
				t.Fatal(err)
			}
			var note struct {
				Reason string
				Basis  struct {
					Status    string
					Unclaimed bool
				}
			}
			if err = json.Unmarshal([]byte(log.SummaryJSON), &note); err != nil || log.ActorUserID != 9 || note.Reason != "停止当前安排" || !note.Basis.Unclaimed || note.Basis.Status != string(v.Status) {
				t.Fatalf("lost termination history: %+v %+v %v", log, note, err)
			}
			if _, err = s.Claim(9, "cancelled-claim", now); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Start(cancelled, cancelled.Revision, 9, now); !errors.Is(err, repo.ErrConflict) {
				t.Fatalf("cancelled restarted: %v", err)
			}
			if _, err = s.Create(task("cancelled-replacement", v.CompositeOutputID, v.CoverFileID)); !errors.Is(err, repo.ErrOccupied) {
				t.Fatal(err)
			}
		})
	}
}
func TestLiveAbandonAndRetryCannotBothWin(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	now := time.Now().UTC()
	failed, err := s.ResolveResult(v, 9, resolution(c, v), now)
	if err != nil {
		t.Fatal(err)
	}
	in := model.Abandonment{AttemptID: c.Attempt.ID, ExpectedRevision: failed.Revision, Evidence: resolution(c, failed).Evidence}
	outcomes := make(chan error, 2)
	go func() { _, err := s.Abandon(failed, 9, in, now); outcomes <- err }()
	go func() { _, err := s.Retry(failed, failed.Revision, 9, now); outcomes <- err }()
	successes := 0
	for n := 0; n < 2; n++ {
		if err := <-outcomes; err == nil {
			successes++
		} else if !errors.Is(err, repo.ErrConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful writes=%d", successes)
	}
	current, _ := s.Get(v.ID)
	_, err = s.Create(task("after-race", v.CompositeOutputID, v.CoverFileID))
	if current.Status == model.StatusAbandoned && err != nil || current.Status == model.StatusQueued && !errors.Is(err, repo.ErrOccupied) {
		t.Fatalf("state=%s occupancy=%v", current.Status, err)
	}
}
func TestLiveAbandonAuditFailureDoesNotRelease(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	now := time.Now().UTC()
	failed, err := s.ResolveResult(v, 9, resolution(c, v), now)
	if err != nil {
		t.Fatal(err)
	}
	in := model.Abandonment{AttemptID: c.Attempt.ID, ExpectedRevision: failed.Revision, Evidence: resolution(c, failed).Evidence}
	if err = s.DB.Exec("DROP TABLE audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.Abandon(failed, 9, in, now); err == nil {
		t.Fatal("audit failure ignored")
	}
	current, _ := s.Get(v.ID)
	if current.Status != model.StatusFailed || current.Revision != failed.Revision {
		t.Fatal("partial abandonment")
	}
	var occupied int64
	if err = s.DB.Table("publication_task").Where("id=? AND occupied_output_id=?", v.ID, v.CompositeOutputID).Count(&occupied).Error; err != nil || occupied != 1 {
		t.Fatalf("occupancy=%d %v", occupied, err)
	}
}
