package repository_test

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	workmodel "github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	workrepo "github.com/wt-media/wt-media-cloud/internal/modules/work/repository"
	"sync"
	"testing"
	"time"
)

func publicResolution(c model.Claim, v model.Task) model.ResultResolution {
	at := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Microsecond)
	return model.ResultResolution{AttemptID: c.Attempt.ID, ExpectedRevision: v.Revision, Conclusion: "published", PlatformSubmissionID: "1882371730190912814", PlatformWorkID: "7000395122085014122", WorkURL: "https://mbd.baidu.com/newspage/data/videolanding?nid=sv_7000395122085014122", PublishedAt: &at, Evidence: model.VerificationEvidence{PlatformUID: c.Attempt.Input.PlatformUID, Reason: "核实公开", Basis: "已核对指定原稿件、原账号和视频，公开页可访问，实际发布时间由平台核实", SceneClosed: true, WorkMatched: true, PubliclyAccessible: true}}
}
func TestLivePublicWorkResolutionAtomicAndImmutable(t *testing.T) {
	s, c, original := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	before, _ := s.Attempt(v.ID)
	in := publicResolution(c, v)
	now := time.Now().UTC()
	var wg sync.WaitGroup
	result := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.ResolveResult(v, 9, in, now); result <- err }()
	}
	wg.Wait()
	close(result)
	success := 0
	for err := range result {
		if err == nil {
			success++
		} else if !errors.Is(err, repo.ErrConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent results=%d", success)
	}
	v, _ = s.Get(v.ID)
	if v.Status != "published" || v.WorkID == nil || v.PlatformSubmissionID == nil || *v.PlatformSubmissionID != in.PlatformSubmissionID || v.PlatformWorkID == nil || *v.PlatformWorkID != in.PlatformWorkID || !v.PublishedAt.Equal(*in.PublishedAt) {
		t.Fatalf("wrong public projection: %+v", v)
	}
	var works []workmodel.Work
	s.DB.Find(&works)
	if len(works) != 1 || works[0].PublicationID != v.ID || works[0].AttemptID != c.Attempt.ID {
		t.Fatalf("work association %+v", works)
	}
	a, _ := s.Attempt(v.ID)
	if *a[0].ResultHash != original.ResultHash || string(a[0].Evidence) != string(before[0].Evidence) || a[0].ResultStatus == nil || *a[0].ResultStatus != "uncertain" {
		t.Fatal("original result was rewritten")
	}
	state, err := s.OriginalAttempt(v.ID, c.Attempt.ID, 9)
	if err != nil || !state.SceneResolved || state.ResolvedStatus != "published" {
		t.Fatalf("original resolution %+v %v", state, err)
	}
	summary, err := (workrepo.Store{DB: s.DB}).Summary(workrepo.Filter{Actor: identity.PublicUser{Role: identity.RoleAdmin}}, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil || summary.All != 1 || summary.Today != 0 {
		t.Fatalf("observation substituted for publication time: %+v %v", summary, err)
	}
}
func TestLiveReceiptAndPublicIdentitySeparateAndAuditRollback(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	in := publicResolution(c, v)
	now := time.Now().UTC()
	in.Conclusion = "reviewing"
	in.PlatformWorkID = ""
	in.WorkURL = ""
	in.PublishedAt = nil
	in.Evidence.PubliclyAccessible = false
	in.TargetURL = "https://baijiahao.baidu.com/builder/preview/s?id=" + in.PlatformSubmissionID
	reviewing, err := s.ResolveResult(v, 9, in, now)
	if err != nil || reviewing.PlatformSubmissionID == nil || reviewing.PlatformWorkID != nil || reviewing.WorkID != nil {
		t.Fatalf("receipt promoted to work: %+v %v", reviewing, err)
	}
	pub := publicResolution(c, reviewing)
	bad := pub
	bad.PlatformSubmissionID = "1882371730190912815"
	if _, err = s.ResolveResult(reviewing, 9, bad, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("foreign receipt: %v", err)
	}
	if err = s.DB.Exec("CREATE TRIGGER work_audit_failure BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='intentional work audit failure'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResolveResult(reviewing, 9, pub, now); err == nil {
		t.Fatal("audit failure accepted")
	}
	var count int64
	s.DB.Model(&workmodel.Work{}).Count(&count)
	current, _ := s.Get(v.ID)
	if count != 0 || current.Revision != reviewing.Revision || current.Status != "reviewing" || current.WorkID != nil {
		t.Fatalf("partial commit: %d %+v", count, current)
	}
	s.DB.Exec("DROP TRIGGER work_audit_failure")
	if _, err = s.ResolveResult(reviewing, 9, pub, now); err != nil {
		t.Fatal(err)
	}
}

func TestLiveWorkIdentityDeduplicationAndScope(t *testing.T) {
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	in := publicResolution(c, v)
	if _, err := s.ResolveResult(v, 9, in, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	r := workrepo.Store{DB: s.DB}
	var original workmodel.Work
	if err := s.DB.First(&original).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := original
	duplicate.ID = 0
	duplicate.FirstObservedAt = time.Now().UTC()
	reused, err := r.Record(duplicate)
	if err != nil || reused.ID != original.ID || !reused.FirstObservedAt.Equal(original.FirstObservedAt) {
		t.Fatalf("not reused: %+v %v", reused, err)
	}
	for _, change := range []func(*workmodel.Work){func(w *workmodel.Work) { w.PlatformAccountUID = "foreign" }, func(w *workmodel.Work) { w.CompositeOutputID++ }, func(w *workmodel.Work) { w.PublishedAt = w.PublishedAt.Add(time.Second) }} {
		bad := duplicate
		change(&bad)
		if _, err = r.Record(bad); !errors.Is(err, workrepo.ErrConflict) {
			t.Fatalf("conflict overwritten: %v", err)
		}
	}
	f := workrepo.Filter{Actor: identity.PublicUser{ID: 9, Role: identity.RoleOperator, CategoryIDs: []identity.CategoryID{1}}, Page: 1, PageSize: 20}
	rows, n, err := r.List(f)
	if err != nil || n != 1 || len(rows) != 1 {
		t.Fatalf("owner list %+v %d %v", rows, n, err)
	}
	f.Actor.ID = 10
	rows, n, err = r.List(f)
	if err != nil || n != 0 || len(rows) != 0 {
		t.Fatal("foreign user saw work")
	}
	f.Actor.ID = 9
	f.Actor.CategoryIDs = nil
	_, n, err = r.List(f)
	if err != nil || n != 0 {
		t.Fatal("foreign category saw work")
	}
}

func TestLiveWorkMigrationPreservesExternalReceipt(t *testing.T) {
	db := isolatedDB(t, true)
	v := task("receipt-cutover", 42, 1)
	if err := db.Omit("PlatformSubmissionID", "TargetURL", "WorkID").Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	receipt := "1882371730190912814"
	target := "https://baijiahao.baidu.com/builder/preview/s?id=" + receipt
	if err := db.Model(&model.Task{}).Where("id=?", v.ID).Updates(map[string]any{"platform_work_id": receipt, "work_url": target, "status": "reviewing"}).Error; err != nil {
		t.Fatal(err)
	}
	applyFileMigration(t, db, "20261121_112_public_work.sql")
	if err := db.First(&v, v.ID).Error; err != nil {
		t.Fatal(err)
	}
	if v.PlatformSubmissionID == nil || *v.PlatformSubmissionID != receipt || v.TargetURL == nil || *v.TargetURL != target || v.PlatformWorkID != nil || v.WorkURL != nil || v.WorkID != nil || v.Status != "reviewing" {
		t.Fatal("receipt lost or promoted")
	}
}
