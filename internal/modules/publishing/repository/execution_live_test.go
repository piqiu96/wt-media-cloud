package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publishing/repository"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func executionStore(t *testing.T) (repo.ExecutionStore, func(string, int64, string) model.Task) {
	t.Helper()
	db := isolated(t)
	if e := db.Exec("INSERT INTO browser_profiles VALUES(37),(38)").Error; e != nil {
		t.Fatal(e)
	}
	s := repo.ExecutionStore{Store: store(db), LockInput: func(tx *gorm.DB, v model.Task) (model.ExecutionInput, error) {
		var snap struct {
			ProfileID uint64 `json:"profile_id"`
		}
		if e := json.Unmarshal(v.ResourceSnapshot, &snap); e != nil {
			return model.ExecutionInput{}, e
		}
		var id uint64
		if e := tx.Raw("SELECT id FROM browser_profiles WHERE id=? FOR UPDATE", snap.ProfileID).Scan(&id).Error; e != nil {
			return model.ExecutionInput{}, e
		}
		if id == 0 {
			return model.ExecutionInput{}, repo.ErrNotFound
		}
		return model.ExecutionInput{ProfileID: id, BitProfileID: fmt.Sprint("bit-", id), PlatformUID: v.PlatformAccountUID, Content: v.Content, VideoSHA256: strings.Repeat("a", 64), CoverSHA256: strings.Repeat("b", 64), VideoSize: 16, CoverSize: 8}, nil
	}}
	makeTask := func(key string, output int64, uid string) model.Task {
		v := task(key, output, 1)
		v.PlatformAccountUID = uid
		v.ResourceSnapshot = []byte(`{"profile_id":37}`)
		v, e := s.Create(v)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	return s, makeTask
}
func TestLiveExecutionStartAndClaimOwnerTimeAndReplay(t *testing.T) {
	s, makeTask := executionStore(t)
	v := makeTask("first", 42, "uid-1")
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	s.DB.Model(&v).Update("not_before", future)
	v.NotBefore = &future
	authorized, e := s.Start(v, 1, 9, now)
	if e != nil {
		t.Fatal(e)
	}
	if authorized.Status != "pending" || authorized.AuthorizedAt == nil {
		t.Fatalf("%+v", authorized)
	}
	if _, e = s.Edit(v, authorized.Revision); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("authorized input editable: %v", e)
	}
	if claimed, e := s.Claim(9, "before-time", now); e != nil || claimed != nil {
		t.Fatalf("early claim %v %v", claimed, e)
	}
	if e = s.QueueDue(future); e != nil {
		t.Fatal(e)
	}
	if claimed, e := s.Claim(10, "foreign", future); e != nil || claimed != nil {
		t.Fatalf("foreign claim %v %v", claimed, e)
	}
	first, e := s.Claim(9, "claim-1", future)
	if e != nil || first == nil {
		t.Fatalf("%+v %v", first, e)
	}
	again, e := s.Claim(9, "claim-1", future)
	if e != nil || again == nil || again.Attempt.ID != first.Attempt.ID || !again.Replayed {
		t.Fatalf("replay %+v %v", again, e)
	}
	if first.Task.Status != "running" || first.Task.AttemptCount != 1 || first.Attempt.ExecutorUserID != 9 || first.Attempt.ProfileID != 37 {
		t.Fatalf("%+v", first)
	}
	var n int64
	s.DB.Model(&model.Attempt{}).Count(&n)
	if n != 1 {
		t.Fatal(n)
	}
}
func TestLiveExecutionProfileRaceAndLostHeartbeatKeepScene(t *testing.T) {
	s, makeTask := executionStore(t)
	now := time.Now().UTC()
	a := makeTask("a", 42, "uid-a")
	b := makeTask("b", 43, "uid-b")
	for _, v := range []model.Task{a, b} {
		if _, e := s.Start(v, 1, 9, now); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	results := make(chan *model.Claim, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); v, e := s.Claim(9, fmt.Sprint("key-", i), now); results <- v; errs <- e }(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	count := 0
	var claimed *model.Claim
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	for v := range results {
		if v != nil {
			count++
			claimed = v
		}
	}
	if count != 1 {
		t.Fatalf("parallel profile claims=%d", count)
	}
	if e := s.InterruptStale(now.Add(91 * time.Second)); e != nil {
		t.Fatal(e)
	}
	current, e := s.Get(claimed.Task.ID)
	if e != nil || current.Status != "uncertain" {
		t.Fatalf("%+v %v", current, e)
	}
	if v, e := s.Claim(9, "after-timeout", now.Add(92*time.Second)); e != nil || v != nil {
		t.Fatalf("scene incorrectly released: %+v %v", v, e)
	}
}

func TestLiveExecutionReportsNeverCacheSubmitPermissionOrRewriteResult(t *testing.T) {
	s, makeTask := executionStore(t)
	now := time.Now().UTC()
	v := makeTask("reports", 42, "uid-a")
	if _, e := s.Start(v, 1, 9, now); e != nil {
		t.Fatal(e)
	}
	claim, e := s.Claim(9, "report-claim", now)
	if e != nil || claim == nil {
		t.Fatalf("%+v %v", claim, e)
	}
	report := model.ExecutionReport{AttemptID: claim.Attempt.ID, Seq: 1, Stage: "ready", PlatformUID: "uid-a", BitProfileID: "bit-37", VideoSHA256: claim.Attempt.Input.VideoSHA256, CoverSHA256: claim.Attempt.Input.CoverSHA256}
	if _, e = s.Progress(claim.Task, 9, report, true, now); e != nil {
		t.Fatal(e)
	}
	report.Seq = 2
	report.Stage = "submitting"
	permission, e := s.Progress(claim.Task, 9, report, true, now)
	if e != nil || !permission.AllowSubmit {
		t.Fatalf("%+v %v", permission, e)
	}
	duplicate, e := s.Progress(claim.Task, 9, report, true, now)
	if e != nil || duplicate.AllowSubmit {
		t.Fatalf("replayed permission %+v %v", duplicate, e)
	}
	report.Seq = 1
	if _, e = s.Progress(claim.Task, 9, report, true, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatal(e)
	}
	report.Seq = 3
	if _, e = s.Progress(claim.Task, 10, report, true, now); !errors.Is(e, repo.ErrNotFound) {
		t.Fatal(e)
	}
	result := json.RawMessage(`{"status":"uncertain","evidence":{"scene_closed":false}}`)
	hash, e := model.CompletionHash(3, result)
	if e != nil {
		t.Fatal(e)
	}
	completion := model.Completion{AttemptID: claim.Attempt.ID, Seq: 3, ResultHash: hash, Result: result}
	first, e := s.Complete(claim.Task, 9, completion, now)
	if e != nil || first.TaskStatus != "uncertain" {
		t.Fatalf("%+v %v", first, e)
	}
	before, _ := s.Get(claim.Task.ID)
	if _, e = s.Complete(claim.Task, 9, completion, now); e != nil {
		t.Fatal(e)
	}
	after, _ := s.Get(claim.Task.ID)
	if before.Revision != after.Revision {
		t.Fatal("duplicate changed revision")
	}
	completion.Result = json.RawMessage(`{"status":"failed","evidence":{"not_submitted":true,"scene_closed":true}}`)
	completion.ResultHash, _ = model.CompletionHash(3, completion.Result)
	if _, e = s.Complete(claim.Task, 9, completion, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatal(e)
	}
	if current, e := s.Claim(9, "new-scene", now); e != nil || current != nil {
		t.Fatalf("unknown released scene %+v %v", current, e)
	}
}
func TestLiveExecutionStaleSessionAndStopCannotStartSubmission(t *testing.T) {
	s, makeTask := executionStore(t)
	now := time.Now().UTC()
	v := makeTask("stop", 42, "uid-a")
	s.Start(v, 1, 9, now)
	claim, e := s.Claim(9, "claim", now)
	if e != nil || claim == nil {
		t.Fatal(e)
	}
	ready := model.ExecutionReport{AttemptID: claim.Attempt.ID, Seq: 1, Stage: "ready", PlatformUID: "uid-a", BitProfileID: "bit-37", VideoSHA256: claim.Attempt.Input.VideoSHA256, CoverSHA256: claim.Attempt.Input.CoverSHA256}
	if _, e = s.Progress(claim.Task, 9, ready, false, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatal(e)
	}
	if _, e = s.Progress(claim.Task, 9, ready, true, now); e != nil {
		t.Fatal(e)
	}
	s.DB.Model(&model.Task{}).Where("id=?", claim.Task.ID).Update("cancel_requested_at", now)
	ready.Stage = "submitting"
	ready.Seq = 2
	feedback, e := s.Progress(claim.Task, 9, ready, true, now)
	if e != nil || feedback.AllowSubmit || !feedback.StopRequested {
		t.Fatalf("%+v %v", feedback, e)
	}
	if feedback, e := s.Heartbeat(claim.Task.ID, 9, claim.Attempt.ID, 3, now); e != nil || feedback.AllowSubmit || !feedback.StopRequested {
		t.Fatalf("%+v %v", feedback, e)
	}
}

func TestLiveLateSafeUnsubmittedResultRequiresMatchingFrozenEvidence(t *testing.T) {
	s, makeTask := executionStore(t)
	now := time.Now().UTC()
	v := makeTask("late-safe-close", 42, "uid-a")
	if _, e := s.Start(v, 1, 9, now); e != nil {
		t.Fatal(e)
	}
	c, e := s.Claim(9, "late-safe-claim", now)
	if e != nil || c == nil {
		t.Fatal(e)
	}
	if e = s.InterruptStale(now.Add(91 * time.Second)); e != nil {
		t.Fatal(e)
	}
	result := model.ExecutionResult{Status: "failed", ErrorCode: "UNSUBMITTED_TEST", Evidence: model.ResultEvidence{PlatformUID: "wrong", BitProfileID: c.Attempt.Input.BitProfileID, VideoSHA256: c.Attempt.Input.VideoSHA256, CoverSHA256: c.Attempt.Input.CoverSHA256, NotSubmitted: true, SceneClosed: true}}
	raw, _ := json.Marshal(result)
	hash, _ := model.CompletionHash(1, raw)
	completion := model.Completion{AttemptID: c.Attempt.ID, Seq: 1, ResultHash: hash, Result: raw}
	if _, e = s.Complete(c.Task, 9, completion, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("wrong evidence released scene: %v", e)
	}
	result.Evidence.PlatformUID = c.Attempt.Input.PlatformUID
	raw, _ = json.Marshal(result)
	hash, _ = model.CompletionHash(1, raw)
	completion.Result = raw
	completion.ResultHash = hash
	out, e := s.Complete(c.Task, 9, completion, now.Add(92*time.Second))
	if e != nil || out.TaskStatus != "failed" {
		t.Fatalf("%+v %v", out, e)
	}
	if e = repo.ProfileExecutionBusy(s.DB, "37"); e != nil {
		t.Fatal(e)
	}
	saved, e := s.Attempts(c.Task.ID)
	if e != nil || len(saved) != 1 {
		t.Fatalf("%+v %v", saved, e)
	}
	var observed model.ExecutionResult
	if e = json.Unmarshal(saved[0].Evidence, &observed); e != nil || observed.ErrorCode != "UNSUBMITTED_TEST" || !observed.Evidence.SceneClosed {
		t.Fatalf("original result missing: %+v %v", observed, e)
	}

	current, _ := s.Get(c.Task.ID)
	if current.PublishedAt != nil || current.AcceptedAt != nil || current.PlatformWorkID != nil {
		t.Fatal("unsubmitted result fabricated platform success")
	}
}
