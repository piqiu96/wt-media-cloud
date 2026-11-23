package repository_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	workmodel "github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"gorm.io/gorm"
)

func reviewingPublication(t *testing.T) (repo.ExecutionStore, model.Claim, model.Task) {
	t.Helper()
	s, c, _ := unresolvedPublication(t)
	v, _ := s.Get(c.Task.ID)
	in := publicResolution(c, v)
	in.Conclusion = model.StatusReviewing
	in.PlatformWorkID = ""
	in.WorkURL = ""
	in.PublishedAt = nil
	in.Evidence.PubliclyAccessible = false
	v, err := s.ResolveResult(v, 9, in, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return s, c, v
}
func TestLivePublicCheckIsReadOnlyUntilCrediblePublicReport(t *testing.T) {
	s, c, v := reviewingPublication(t)
	before, _ := s.Attempt(v.ID)
	now := time.Now().UTC()
	if err := s.SchedulePublicCheck(now); err != nil {
		t.Fatal(err)
	}
	if claim, err := s.ClaimPublicCheck(10, now); err != nil || claim != nil {
		t.Fatal("foreign check admitted")
	}
	check, err := s.ClaimPublicCheck(9, now)
	if err != nil || check == nil {
		t.Fatalf("claim %+v %v", check, err)
	}
	if check.Generation != 1 || check.ExpectedRevision != v.Revision || check.Attempt.ID != c.Attempt.ID {
		t.Fatal("wrong original check")
	}
	if again, err := s.ClaimPublicCheck(9, now); err != nil || again != nil {
		t.Fatal("immediate duplicate check")
	}
	in := model.PublicCheckReport{Generation: check.Generation, Observation: model.ResultResolution{AttemptID: c.Attempt.ID, ExpectedRevision: check.ExpectedRevision, Conclusion: model.StatusUncertain, Evidence: model.VerificationEvidence{PlatformUID: c.Attempt.Input.PlatformUID, Reason: "暂时不可访问", Basis: "原投稿仍在核验，当前公开页未取得可加载媒体证据，保持审核中"}}}
	ack, err := s.ObservePublication(v, 9, in, now)
	if err != nil || ack.Generation != check.Generation {
		t.Fatalf("observation %+v %v", ack, err)
	}
	current, _ := s.Get(v.ID)
	if current.Status != model.StatusReviewing || current.WorkID != nil || current.PlatformSubmissionID == nil || *current.PlatformSubmissionID != *v.PlatformSubmissionID || current.LastCheckedAt == nil {
		t.Fatal("unavailable became failure or success")
	}
	if _, err = s.ObservePublication(current, 9, in, now); err != nil {
		t.Fatalf("original reply not replayable %v", err)
	}
	again, _ := s.Get(v.ID)
	if again.Revision != current.Revision {
		t.Fatal("duplicate observation changed task")
	}
	changed := in
	changed.Observation.Evidence.Reason = "changed"
	if _, err = s.ObservePublication(current, 9, changed, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal("observation overwritten")
	}
	after, _ := s.Attempt(v.ID)
	if !reflect.DeepEqual(before, after) || current.AttemptCount != 1 {
		t.Fatal("read check rewrote original attempt")
	}
}
func TestLivePublicCheckGenerationFencesAndPublicWorkIsAtomic(t *testing.T) {
	s, c, v := reviewingPublication(t)
	original, _ := s.Attempt(v.ID)
	now := time.Now().UTC()
	s.SchedulePublicCheck(now)
	first, err := s.ClaimPublicCheck(9, now)
	if err != nil || first == nil {
		t.Fatal(err)
	}
	later, err := s.ClaimPublicCheck(9, now.Add(6*time.Minute))
	if err != nil || later == nil || later.Generation != 2 {
		t.Fatal("read-only check cannot recover next cycle")
	}
	report := model.PublicCheckReport{Generation: first.Generation, Observation: publicResolution(c, v)}
	if _, err = s.ObservePublication(v, 9, report, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal("stale generation accepted")
	}
	report.Generation = later.Generation
	bad := report
	bad.Observation.Evidence.PubliclyAccessible = false
	if _, err = s.ObservePublication(v, 9, bad, now); err == nil {
		t.Fatal("preview alone became public")
	}
	s.DB.Exec("CREATE TRIGGER check_audit_failure BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='intentional check audit failure'")
	if _, err = s.ObservePublication(v, 9, report, now); err == nil {
		t.Fatal("audit failure committed")
	}
	current, _ := s.Get(v.ID)
	var count int64
	s.DB.Model(&model.PublicationObservation{}).Count(&count)
	if current.Revision != v.Revision || current.WorkID != nil || count != 0 {
		t.Fatal("partial public observation")
	}
	s.DB.Exec("DROP TRIGGER check_audit_failure")
	if _, err = s.ObservePublication(v, 9, report, now); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Get(v.ID)
	var works []workmodel.Work
	s.DB.Find(&works)
	if current.Status != model.StatusPublished || current.WorkID == nil || len(works) != 1 || !current.PublishedAt.Equal(*report.Observation.PublishedAt) {
		t.Fatal("public work not associated")
	}
	after, _ := s.Attempt(v.ID)
	if !reflect.DeepEqual(original, after) {
		t.Fatal("periodic observation rewrote original result or manual note")
	}
	if _, err = s.ObservePublication(current, 9, report, now); err != nil {
		t.Fatal("lost public observation reply cannot be confirmed")
	}
}

func TestLivePeriodicAndManualPublicResultCannotOverwriteEachOther(t *testing.T) {
	s, c, v := reviewingPublication(t)
	now := time.Now().UTC()
	if err := s.SchedulePublicCheck(now); err != nil {
		t.Fatal(err)
	}
	check, err := s.ClaimPublicCheck(9, now)
	if err != nil || check == nil {
		t.Fatal(err)
	}
	in := publicResolution(c, v)
	report := model.PublicCheckReport{Generation: check.Generation, Observation: in}
	type outcome struct {
		periodic bool
		err      error
	}
	result := make(chan outcome, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := s.ObservePublication(v, 9, report, now)
		result <- outcome{true, err}
	}()
	go func() { defer wg.Done(); _, err := s.ResolveResult(v, 9, in, now); result <- outcome{false, err} }()
	wg.Wait()
	close(result)
	success := 0
	periodicWon := false
	for r := range result {
		if r.err == nil {
			success++
			periodicWon = r.periodic
		} else if !errors.Is(r.err, repo.ErrConflict) {
			t.Fatal(r.err)
		}
	}
	if success != 1 {
		t.Fatalf("successful public transitions=%d", success)
	}
	var works, observations int64
	s.DB.Model(&workmodel.Work{}).Count(&works)
	s.DB.Model(&model.PublicationObservation{}).Count(&observations)
	if works != 1 || (periodicWon && observations != 1) || (!periodicWon && observations != 0) {
		t.Fatal("partial or duplicate public result")
	}
}

func TestLiveNewCheckClaimFencesAnObservationAlreadyBeingValidated(t *testing.T) {
	s, c, v := reviewingPublication(t)
	now := time.Now().UTC()
	s.SchedulePublicCheck(now)
	first, err := s.ClaimPublicCheck(9, now)
	if err != nil || first == nil {
		t.Fatal(err)
	}
	other := s
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	read := s.ReadInput
	s.ReadInput = func(tx *gorm.DB, task model.Task) (model.ExecutionInput, error) {
		value, err := read(tx, task)
		close(entered)
		<-release
		return value, err
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.ObservePublication(v, 9, model.PublicCheckReport{Generation: first.Generation, Observation: publicResolution(c, v)}, now)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("observation did not reach validation")
	}
	later, err := other.ClaimPublicCheck(9, now.Add(6*time.Minute))
	if err != nil || later == nil || later.Generation != 2 {
		t.Fatalf("next check %+v %v", later, err)
	}
	close(release)
	if err = <-done; !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("older observation committed after new generation: %v", err)
	}
	current, _ := s.Get(v.ID)
	var count int64
	s.DB.Model(&model.PublicationObservation{}).Count(&count)
	if current.Status != model.StatusReviewing || current.WorkID != nil || count != 0 {
		t.Fatal("stale observation wrote public state")
	}
}

func TestLivePublicCheckStopsAfterThreeMissingReadsAndSurvivesSchedulerRestart(t *testing.T) {
	s, c, v := reviewingPublication(t)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if err := s.SchedulePublicCheck(now); err != nil {
			t.Fatal(err)
		}
		check, err := s.ClaimPublicCheck(9, now)
		if err != nil || check == nil {
			t.Fatalf("check %d: %+v %v", i, check, err)
		}
		in := model.PublicCheckReport{Generation: check.Generation, Observation: model.ResultResolution{AttemptID: c.Attempt.ID, ExpectedRevision: check.ExpectedRevision, Conclusion: model.StatusUncertain, Evidence: model.VerificationEvidence{PlatformUID: c.Attempt.Input.PlatformUID, Reason: "原账号内容列表未找到指定投稿", Basis: "已核对原账号并完成平台列表全部实际分页，没有找到原投稿", SceneClosed: true}}}
		raw, _ := json.Marshal(in)
		var body map[string]any
		json.Unmarshal(raw, &body)
		body["lookup_result"] = "not_found"
		raw, _ = json.Marshal(body)
		json.Unmarshal(raw, &in)
		if _, err = s.ObservePublication(v, 9, in, now); err != nil {
			t.Fatal(err)
		}
		v, _ = s.Get(v.ID)
		if i < 2 {
			if v.NextCheckAt == nil {
				t.Fatal("missing retry")
			}
			delay := v.NextCheckAt.Sub(now)
			lo, hi := 45*time.Second, 75*time.Second
			if i == 1 {
				lo, hi = 150*time.Second, 210*time.Second
			}
			if delay < lo || delay > hi {
				t.Fatalf("retry %d delay %v", i, delay)
			}
			now = *v.NextCheckAt
		}
	}
	if v.Status != model.StatusReviewing || v.WorkID != nil {
		t.Fatal("missing read changed publication")
	}
	if err := s.SchedulePublicCheck(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ClaimPublicCheck(9, now.Add(time.Hour)); err != nil || got != nil {
		t.Fatal("stopped task rescheduled")
	}
}
func TestLivePublicCheckExpiresFromOriginalAcceptance(t *testing.T) {
	s, _, v := reviewingPublication(t)
	now := v.AcceptedAt.Add(24 * time.Hour)
	if err := s.SchedulePublicCheck(now); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ClaimPublicCheck(9, now); err != nil || got != nil {
		t.Fatal("expired task admitted")
	}
	v, _ = s.Get(v.ID)
	if v.Status != model.StatusReviewing || v.LastCheckMessage == nil {
		t.Fatal("expiry changed status or omitted explanation")
	}
	if err := s.SchedulePublicCheck(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ClaimPublicCheck(9, now.Add(time.Hour)); got != nil {
		t.Fatal("restart resurrected expired check")
	}
}

func TestLivePublicCheckUnknownDoesNotConsumeMissingRetriesAndFallbackPersists(t *testing.T) {
	s, c, v := reviewingPublication(t)
	now := time.Now().UTC()
	s.SchedulePublicCheck(now)
	check, err := s.ClaimPublicCheck(9, now)
	if err != nil || check == nil {
		t.Fatal(err)
	}
	if !check.BrowserFallbackAllowed {
		t.Fatal("first fallback not allowed")
	}
	report := model.PublicCheckReport{Generation: check.Generation, BrowserUsed: true, Observation: model.ResultResolution{AttemptID: c.Attempt.ID, ExpectedRevision: check.ExpectedRevision, Conclusion: model.StatusUncertain, Evidence: model.VerificationEvidence{PlatformUID: c.Attempt.Input.PlatformUID, Reason: "网络暂不可用", Basis: "本轮网络请求异常，未完成完整列表查询，不能判定为未找到"}}}
	if _, err = s.ObservePublication(v, 9, report, now); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get(v.ID)
	if v.CheckNotFoundCount != 0 || v.CheckStoppedAt != nil || v.NextCheckAt == nil {
		t.Fatal("network error counted as missing or stopped")
	}
	next, err := s.ClaimPublicCheck(9, *v.NextCheckAt)
	if err != nil || next == nil || next.BrowserFallbackAllowed {
		t.Fatal("restart restored browser fallback")
	}
}
