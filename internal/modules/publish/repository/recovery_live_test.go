package repository_test

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

func unresolvedPublication(t *testing.T) (repo.ExecutionStore, model.Claim, model.Completion) {
	t.Helper()
	s, makeTask := ownedStore(t)
	c, packet := unresolvedPublicationFrom(t, s, makeTask(0))
	return s, c, packet
}

func unresolvedPublicationFrom(t *testing.T, s repo.ExecutionStore, v model.Task) (model.Claim, model.Completion) {
	t.Helper()
	now := time.Now().UTC()
	if _, e := s.Start(v, 1, 9, now); e != nil {
		t.Fatal(e)
	}
	c, e := s.Claim(9, "original", now)
	if e != nil || c == nil {
		t.Fatalf("%+v %v", c, e)
	}
	raw := json.RawMessage(`{"status":"uncertain","error_code":"LOST_RESPONSE","evidence":{"scene_closed":false}}`)
	hash, e := model.CompletionHash(1, raw)
	if e != nil {
		t.Fatal(e)
	}
	packet := model.Completion{AttemptID: c.Attempt.ID, Seq: 1, ResultHash: hash, Result: raw}
	if _, e = s.Complete(c.Task, 9, packet, now); e != nil {
		t.Fatal(e)
	}
	return *c, packet
}
func resolution(c model.Claim, v model.Task) model.ResultResolution {
	return model.ResultResolution{AttemptID: c.Attempt.ID, ExpectedRevision: v.Revision, Conclusion: "failed", Evidence: model.VerificationEvidence{PlatformUID: c.Attempt.Input.PlatformUID, Reason: "执行在上传前失败", Basis: "已检查原账号稿件及本机原尝试，确认没有可继续发布稿件，原执行进程与浏览器已经结束", SceneClosed: true, NoPublishableWork: true}}
}
func TestLiveResultResolutionAndRetryKeepOriginalAttemptAndOccupancy(t *testing.T) {
	s, c, original := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	now := time.Now().UTC()
	in := resolution(c, v)
	bad := in
	bad.Evidence.SceneClosed = false
	if _, e := s.ResolveResult(v, 9, bad, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("open scene accepted: %v", e)
	}
	bad = in
	bad.Evidence.PlatformUID = "foreign"
	if _, e := s.ResolveResult(v, 9, bad, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("foreign UID accepted: %v", e)
	}
	if _, e := s.Retry(v, v.Revision, 9, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("unknown retry accepted: %v", e)
	}
	failed, e := s.ResolveResult(v, 9, in, now)
	if e != nil || failed.Status != "failed" {
		t.Fatalf("%+v %v", failed, e)
	}
	attempts, _ := s.Attempt(v.ID)
	if attempts[0].ResultHash == nil || *attempts[0].ResultHash != original.ResultHash || attempts[0].ResultStatus == nil || *attempts[0].ResultStatus != "uncertain" || attempts[0].ResolvedAt == nil {
		t.Fatalf("original result rewritten: %+v", attempts[0])
	}
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Retry(failed, failed.Revision, 9, now); out <- e }()
	}
	wg.Wait()
	close(out)
	success := 0
	for e := range out {
		if e == nil {
			success++
		} else if !errors.Is(e, repo.ErrConflict) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatalf("retry count=%d", success)
	}
	queued, _ := s.Get(v.ID)
	if queued.Status != "queued" || queued.AttemptCount != 1 || queued.CurrentAttemptID != nil {
		t.Fatalf("%+v", queued)
	}
	next, e := s.Claim(9, "retry", now)
	if e != nil || next == nil || next.Task.ID != v.ID || next.Attempt.ID == c.Attempt.ID || next.Attempt.AttemptNo != 2 {
		t.Fatalf("%+v %v", next, e)
	}
	old, e := s.OriginalAttempt(v.ID, c.Attempt.ID, 9)
	if e != nil || !old.SceneResolved || old.ResolvedStatus != "failed" || old.ResultHash == nil || *old.ResultHash != original.ResultHash {
		t.Fatalf("old acknowledgement lost: %+v %v", old, e)
	}
	if _, e := s.OriginalAttempt(v.ID, c.Attempt.ID, 10); !errors.Is(e, repo.ErrNotFound) {
		t.Fatalf("foreign owner can read: %v", e)
	}
	if _, e := s.Complete(next.Task, 9, original, now); e == nil {
		t.Fatal("historical report drove current task")
	}
	again, _ := s.Get(v.ID)
	if again.CurrentAttemptID == nil || *again.CurrentAttemptID != next.Attempt.ID || again.Status != "running" {
		t.Fatalf("late result changed current: %+v", again)
	}
}
func TestLiveResolvedFailureStillRechecksResourcesAndStopBeforeRetry(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	now := time.Now().UTC()
	v, e := s.ResolveResult(v, 9, resolution(c, v), now)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.DB.Exec("UPDATE composite_outputs SET sha256=? WHERE id=?", "bad-digest", v.CompositeOutputID).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.Retry(v, v.Revision, 9, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("changed output accepted: %v", e)
	}
	current, _ := s.Get(v.ID)
	if current.Status != "failed" || current.Revision != v.Revision {
		t.Fatal("failed retry mutated task")
	}
	if e = s.DB.Exec("UPDATE publication_task SET cancel_requested_at=NOW(6) WHERE id=?", v.ID).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.Retry(v, v.Revision, 9, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("stopped retry accepted: %v", e)
	}
}

func TestLiveResolutionAuditFailureRollsBackAndUncertainKeepsHold(t *testing.T) {
	s, c, original := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	now := time.Now().UTC()
	in := resolution(c, v)
	in.Conclusion = "uncertain"
	in.Evidence.SceneClosed = false
	in.Evidence.NoPublishableWork = false
	waiting, e := s.ResolveResult(v, 9, in, now)
	if e != nil || waiting.Status != "uncertain" || waiting.Revision != v.Revision+1 {
		t.Fatalf("%+v %v", waiting, e)
	}
	old, e := s.OriginalAttempt(v.ID, c.Attempt.ID, 9)
	if e != nil || old.SceneResolved || old.ResolvedStatus != "" {
		t.Fatalf("unknown scene was released: %+v %v", old, e)
	}
	if _, e = s.Retry(waiting, waiting.Revision, 9, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("unknown scene retried: %v", e)
	}
	if e = s.DB.Exec("DROP TABLE audit_logs").Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.ResolveResult(waiting, 9, resolution(c, waiting), now); e == nil {
		t.Fatal("audit failure ignored")
	}
	after, _ := s.Get(v.ID)
	attempts, _ := s.Attempt(v.ID)
	if after.Status != "uncertain" || after.Revision != waiting.Revision || attempts[0].ResolvedAt != nil || attempts[0].ResultHash == nil || *attempts[0].ResultHash != original.ResultHash {
		t.Fatalf("failed audit changed original state: %+v %+v", after, attempts)
	}
}

func TestLiveVerificationPreservesMaximumAllowedEvidence(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	in := resolution(c, v)
	in.Evidence.Reason = strings.Repeat("因", 200)
	in.Evidence.Basis = strings.Repeat("💡", 2000)
	if !in.Valid() {
		t.Fatal("contract-valid maximum evidence rejected")
	}
	failed, e := s.ResolveResult(v, 9, in, time.Now().UTC())
	if e != nil || failed.Status != "failed" {
		t.Fatalf("valid evidence did not persist: %v", e)
	}
	attempts, e := s.Attempt(v.ID)
	if e != nil || attempts[0].ResolutionNote == nil {
		t.Fatal(e)
	}
	var saved model.ResultResolution
	if e = json.Unmarshal([]byte(*attempts[0].ResolutionNote), &saved); e != nil || saved != in {
		t.Fatalf("verification evidence truncated: %v", e)
	}
}

// Same-scene verification serializes at its original Profile fence before task
// updates. Exactly one old revision may commit task, attempt and audit.
func TestLiveConcurrentResolutionUsesVersions(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	in := resolution(c, v)
	reached := make(chan struct{}, 2)
	release := make(chan struct{})
	if err := s.DB.Callback().Update().Before("gorm:update").Register("test:resolve-versions", func(tx *gorm.DB) {
		if tx.Statement.Table == "publication_task" {
			reached <- struct{}{}
			<-release
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer s.DB.Callback().Update().Remove("test:resolve-versions")
	result := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := s.ResolveResult(v, 9, in, time.Now().UTC()); result <- err }()
	}
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("first resolver did not reach task update")
	}
	select {
	case <-reached:
		close(release)
		t.Fatal("same Profile resolver bypassed the original-scene fence")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	successes := 0
	for i := 0; i < 2; i++ {
		if err := <-result; err == nil {
			successes++
		} else if !errors.Is(err, repo.ErrConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("committed resolutions=%d", successes)
	}
	history, err := s.Attempt(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if history[0].ResolvedAt == nil || history[0].ResultHash == nil {
		t.Fatal("original result or resolution lost")
	}
}
