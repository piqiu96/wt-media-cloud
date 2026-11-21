package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"gorm.io/gorm"
)

func runningClaim(t *testing.T) (repo.ExecutionStore, model.Claim, time.Time) {
	t.Helper()
	s, makeTask := executionStore(t)
	now := time.Now().UTC()
	v := makeTask("optimistic", 42, "uid-a")
	if _, e := s.Start(v, 1, 9, now); e != nil {
		t.Fatal(e)
	}
	c, e := s.Claim(9, "optimistic-claim", now)
	if e != nil || c == nil {
		t.Fatalf("claim: %+v %v", c, e)
	}
	return s, *c, now
}

// Both requests must be able to read the same revision before either writes.
// Exactly one can commit; a larger report sequence must not bypass that check.
func TestLiveHeartbeatOptimisticConflict(t *testing.T) {
	s, c, now := runningClaim(t)
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	if e := s.DB.Callback().Update().Before("gorm:update").Register("test:concurrent-heartbeats", func(tx *gorm.DB) {
		if tx.Statement.Table == "publication_attempt" {
			ready <- struct{}{}
			<-release
		}
	}); e != nil {
		t.Fatal(e)
	}
	defer s.DB.Callback().Update().Remove("test:concurrent-heartbeats")
	results := make(chan error, 2)
	for seq := uint64(1); seq <= 2; seq++ {
		go func(seq uint64) {
			_, e := s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: seq}, false, now.Add(time.Duration(seq)*time.Second))
			results <- e
		}(seq)
	}
	for n := 0; n < 2; n++ {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("report reads serialized before optimistic update")
		}
	}
	close(release)
	successes, conflicts := 0, 0
	for n := 0; n < 2; n++ {
		e := <-results
		if e == nil {
			successes++
		} else if errors.Is(e, repo.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	var a model.Attempt
	if e := s.DB.First(&a, c.Attempt.ID).Error; e != nil {
		t.Fatal(e)
	}
	if successes != 1 || conflicts != 1 || a.Revision != c.Attempt.Revision+1 {
		t.Fatalf("committed=%d conflicts=%d attempt=%+v", successes, conflicts, a)
	}
}

// Pause after the task update but before the attempt update. A fresh heartbeat
// wins the attempt revision; the losing transaction must also roll back its
// task transition and audit. The sweep skips that race and retries next poll.
func TestLiveHeartbeatPreventsStaleFinalization(t *testing.T) {
	for _, mode := range []string{"completion", "sweep"} {
		t.Run(mode, func(t *testing.T) {
			s, c, now := runningClaim(t)
			reached := make(chan struct{}, 1)
			release := make(chan struct{})
			var once sync.Once
			if e := s.DB.Callback().Update().Before("gorm:update").Register("test:stale-finalization", func(tx *gorm.DB) {
				updates, ok := tx.Statement.Dest.(map[string]any)
				if tx.Statement.Table == "publication_attempt" && ok && updates["finished_at"] != nil {
					once.Do(func() { reached <- struct{}{}; <-release })
				}
			}); e != nil {
				t.Fatal(e)
			}
			defer s.DB.Callback().Update().Remove("test:stale-finalization")
			var before int64
			if e := s.DB.Table("audit_logs").Count(&before).Error; e != nil {
				t.Fatal(e)
			}
			result := make(chan error, 1)
			go func() {
				if mode == "sweep" {
					result <- s.InterruptStale(now.Add(2 * time.Minute))
					return
				}
				raw := json.RawMessage(`{"status":"uncertain","evidence":{"scene_closed":false}}`)
				hash, _ := model.CompletionHash(2, raw)
				_, e := s.Complete(c.Task, 9, model.Completion{AttemptID: c.Attempt.ID, Seq: 2, Result: raw, ResultHash: hash}, now.Add(time.Minute))
				result <- e
			}()
			select {
			case <-reached:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("finalization did not reach update")
			}
			// A locking task read here would block behind finalization, defeating CAS.
			heartbeat := make(chan error, 1)
			go func() {
				_, e := s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1}, false, now.Add(2*time.Minute))
				heartbeat <- e
			}()
			select {
			case e := <-heartbeat:
				if e != nil {
					close(release)
					t.Fatal(e)
				}
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("heartbeat blocked on task snapshot")
			}
			close(release)
			e := <-result
			if mode == "completion" && !errors.Is(e, repo.ErrConflict) || mode == "sweep" && e != nil {
				t.Fatalf("%s: %v", mode, e)
			}
			v, e := s.Get(c.Task.ID)
			if e != nil {
				t.Fatal(e)
			}
			var a model.Attempt
			if e = s.DB.First(&a, c.Attempt.ID).Error; e != nil {
				t.Fatal(e)
			}
			var after int64
			if e = s.DB.Table("audit_logs").Count(&after).Error; e != nil {
				t.Fatal(e)
			}
			if v.Status != model.StatusRunning || v.Revision != c.Task.Revision || a.Status != model.AttemptStatusRunning || a.FinishedAt != nil || a.ResultHash != nil || a.ReportSeq != 1 || a.Revision != c.Attempt.Revision+1 || before != after {
				t.Fatalf("partial stale write: task=%+v attempt=%+v audits=%d/%d", v, a, before, after)
			}
		})
	}
}

func TestLiveConcurrentSubmitPermissionGrantedOnce(t *testing.T) {
	s, c, now := runningClaim(t)
	r := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStageReady, PlatformUID: c.Attempt.Input.PlatformUID, BitProfileID: c.Attempt.Input.BitProfileID, VideoSHA256: c.Attempt.Input.VideoSHA256, CoverSHA256: c.Attempt.Input.CoverSHA256}
	if _, e := s.Report(c.Task, 9, r, true, now); e != nil {
		t.Fatal(e)
	}
	r.Seq = 2
	r.Stage = model.AttemptStageSubmitting
	type result struct {
		f model.ExecutionFeedback
		e error
	}
	results := make(chan result, 2)
	for n := 0; n < 2; n++ {
		go func() { f, e := s.Report(c.Task, 9, r, true, now); results <- result{f, e} }()
	}
	grants := 0
	for n := 0; n < 2; n++ {
		got := <-results
		if got.e != nil && !errors.Is(got.e, repo.ErrConflict) {
			t.Fatal(got.e)
		}
		if got.f.AllowSubmit && got.e == nil {
			grants++
		}
	}
	if grants != 1 {
		t.Fatalf("submit grants=%d", grants)
	}
}

func TestLiveConcurrentEditsCommitOneRevisionAndAudit(t *testing.T) {
	db := isolated(t)
	s := store(db)
	v, e := s.Create(task("concurrent-edit", 42, 1))
	if e != nil {
		t.Fatal(e)
	}
	reached := make(chan struct{}, 2)
	release := make(chan struct{})
	if e = db.Callback().Update().Before("gorm:update").Register("test:concurrent-edits", func(tx *gorm.DB) {
		if tx.Statement.Table == "publication_task" {
			reached <- struct{}{}
			<-release
		}
	}); e != nil {
		t.Fatal(e)
	}
	defer db.Callback().Update().Remove("test:concurrent-edits")
	results := make(chan error, 2)
	// Distinct new covers must not serialize independent edits on a file lock.
	for file := int64(2); file <= 3; file++ {
		go func(file int64) {
			draft := v
			draft.CoverFileID = file
			draft.Content.CoverFileID = file
			_, e := s.Edit(draft, 1)
			results <- e
		}(file)
	}
	for n := 0; n < 2; n++ {
		select {
		case <-reached:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("edit reads serialized before optimistic update")
		}
	}
	close(release)
	successes, conflicts := 0, 0
	for n := 0; n < 2; n++ {
		e := <-results
		if e == nil {
			successes++
		} else if errors.Is(e, repo.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	got, e := s.Get(v.ID)
	if e != nil {
		t.Fatal(e)
	}
	var audits int64
	if e = db.Table("audit_logs").Where("action=?", "publication.edit").Count(&audits).Error; e != nil {
		t.Fatal(e)
	}
	if successes != 1 || conflicts != 1 || got.Revision != 2 || audits != 1 || got.CoverFileID != got.Content.CoverFileID {
		t.Fatalf("committed=%d conflicts=%d task=%+v audits=%d", successes, conflicts, got, audits)
	}
}

func TestLiveAttemptRevisionMigrationPreservesExistingReport(t *testing.T) {
	db := isolatedDB(t, true)
	s := repo.ExecutionStore{Store: store(db)}
	// No file locking is needed to seed a pre-migration task.
	v := task("migration-revision", 42, 1)
	if e := db.Omit("PlatformSubmissionID", "TargetURL", "WorkID").Create(&v).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Exec("INSERT INTO browser_profiles VALUES(37)").Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Exec(`INSERT INTO publication_attempt(publication_id,attempt_no,executor_user_id,browser_profile_id,claim_request_key,input_snapshot_json,status,stage,report_seq,started_at,heartbeat_at,steps_json,evidence_json) VALUES(?,1,9,37,'legacy','{}',?, ?,7,NOW(6),NOW(6),'[]','{"original":true}')`, v.ID, model.AttemptStatusRunning, model.AttemptStageReady).Error; e != nil {
		t.Fatal(e)
	}
	applyFileMigration(t, db, "20261119_107_publication_attempt_revision.sql")
	attempts, e := s.Attempt(v.ID)
	if e != nil || len(attempts) != 1 {
		t.Fatalf("attempts=%+v error=%v", attempts, e)
	}
	a := attempts[0]
	var evidence struct {
		Original bool `json:"original"`
	}
	if e = json.Unmarshal(a.Evidence, &evidence); e != nil {
		t.Fatal(e)
	}
	if a.Revision != 1 || a.ReportSeq != 7 || a.Status != model.AttemptStatusRunning || a.Stage != model.AttemptStageReady || !evidence.Original {
		t.Fatalf("migration changed report: %+v", a)
	}
}

func TestLiveReportKeepaliveDoesNotRecordProgressOrRepeatPermission(t *testing.T) {
	s, c, now := runningClaim(t)
	r := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStageReady, PlatformUID: c.Attempt.Input.PlatformUID, BitProfileID: c.Attempt.Input.BitProfileID, VideoSHA256: c.Attempt.Input.VideoSHA256, CoverSHA256: c.Attempt.Input.CoverSHA256}
	if _, e := s.Report(c.Task, 9, r, true, now); e != nil {
		t.Fatal(e)
	}
	before, e := s.Get(c.Task.ID)
	if e != nil {
		t.Fatal(e)
	}
	var original model.Attempt
	if e = s.DB.First(&original, c.Attempt.ID).Error; e != nil {
		t.Fatal(e)
	}
	lockInput := s.ReadInput
	s.ReadInput = func(*gorm.DB, model.Task) (model.ExecutionInput, error) {
		return model.ExecutionInput{}, errors.New("keepalive must not recheck resources")
	}
	keepalive := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 2}
	for _, at := range []time.Time{now.Add(time.Second), now.Add(2 * time.Second)} {
		f, e := s.Report(c.Task, 9, keepalive, false, at)
		if e != nil || f.AllowSubmit || f.Seq != 2 || f.TaskStatus != model.StatusRunning {
			t.Fatalf("keepalive feedback=%+v error=%v", f, e)
		}
	}
	var saved model.Attempt
	if e = s.DB.First(&saved, c.Attempt.ID).Error; e != nil {
		t.Fatal(e)
	}
	after, e := s.Get(c.Task.ID)
	if e != nil {
		t.Fatal(e)
	}
	if saved.Stage != original.Stage || string(saved.Steps) != string(original.Steps) || saved.SubmitStartedAt != nil || saved.Revision != original.Revision+2 || saved.HeartbeatAt.Before(now.Add(time.Second)) || after.Revision != before.Revision {
		t.Fatalf("keepalive mutated progress: before=%+v after=%+v", original, saved)
	}
	// A stage-less packet cannot smuggle progress evidence; foreign, stale and
	// resolved attempts retain the same identity protections as stage reports.
	keepalive.Seq = 3
	keepalive.PlatformUID = "unexpected"
	if _, e = s.Report(c.Task, 9, keepalive, false, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("ambiguous report=%v", e)
	}
	keepalive.PlatformUID = ""
	if _, e = s.Report(c.Task, 10, keepalive, false, now); !errors.Is(e, repo.ErrNotFound) {
		t.Fatalf("foreign report=%v", e)
	}
	s.ReadInput = lockInput
	r.Seq = 3
	r.Stage = model.AttemptStageSubmitting
	if _, e = s.Report(c.Task, 9, r, false, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("stale session submitted=%v", e)
	}
	f, e := s.Report(c.Task, 9, r, true, now)
	if e != nil || !f.AllowSubmit {
		t.Fatalf("submit=%+v error=%v", f, e)
	}
	keepalive.Seq = 4
	f, e = s.Report(c.Task, 9, keepalive, false, now)
	if e != nil || f.AllowSubmit {
		t.Fatalf("keepalive replayed permission=%+v error=%v", f, e)
	}
	r.Seq = 5
	if _, e = s.Report(c.Task, 9, r, true, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("new sequence repeated submission=%v", e)
	}
}

func TestLiveInputChangeStopsWithoutGrantAndSurvivesKeepalive(t *testing.T) {
	s, c, now := runningClaim(t)
	s.ReadInput = func(*gorm.DB, model.Task) (model.ExecutionInput, error) {
		return model.ExecutionInput{}, repo.ErrInputChanged
	}
	report := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStagePreparing}
	feedback, e := s.Report(c.Task, 9, report, true, now)
	if e != nil || !feedback.StopRequested || feedback.AllowSubmit || feedback.StopReason != model.ExecutionInputChanged {
		t.Fatalf("stop=%+v error=%v", feedback, e)
	}
	s.ReadInput = func(*gorm.DB, model.Task) (model.ExecutionInput, error) {
		t.Fatal("keepalive rechecked input")
		return model.ExecutionInput{}, nil
	}
	feedback, e = s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 2}, false, now)
	if e != nil || !feedback.StopRequested || feedback.StopReason != model.ExecutionInputChanged {
		t.Fatalf("keepalive=%+v error=%v", feedback, e)
	}
	var a model.Attempt
	if e = s.DB.First(&a, c.Attempt.ID).Error; e != nil {
		t.Fatal(e)
	}
	if a.SubmitStartedAt != nil || a.Stage != model.AttemptStageClaimed {
		t.Fatal("invalid input advanced execution")
	}
}

func TestLiveProfileReferencesIncludeHistoryAndUnclaimedTask(t *testing.T) {
	s, makeTask := executionStore(t)
	v := makeTask("reference", 42, "uid-ref")
	if e := s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("resource_snapshot_json", `{"browser_profile_id":"37"}`).Error; e != nil {
		t.Fatal(e)
	}
	if used, e := repo.ProfileReferenced(s.DB, "37"); e != nil || !used {
		t.Fatalf("unclaimed input reference: %v %v", used, e)
	}
	if e := s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("resource_snapshot_json", `{"profile_id":37}`).Error; e != nil {
		t.Fatal(e)
	}
	if _, e := s.Start(v, 1, 9, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	c, e := s.Claim(9, "ref-claim", time.Now().UTC())
	if e != nil || c == nil {
		t.Fatalf("claim %v %v", c, e)
	}
	if e = s.DB.Model(&model.Attempt{}).Where("id=?", c.Attempt.ID).Update("status", model.AttemptStatusCompleted).Error; e != nil {
		t.Fatal(e)
	}
	if used, e := repo.ProfileReferenced(s.DB, "37"); e != nil || !used {
		t.Fatalf("history reference: %v %v", used, e)
	}
	if used, e := repo.ProfileReferenced(s.DB, "38"); e != nil || used {
		t.Fatalf("unrelated Profile: %v %v", used, e)
	}
}

func TestLiveTaskProfileReferencePreventsDeleteCreateRace(t *testing.T) {
	for _, deletionFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(deletionFirst), func(t *testing.T) {
			db := isolated(t)
			if err := db.Exec("INSERT INTO browser_profiles(id) VALUES(37)").Error; err != nil {
				t.Fatal(err)
			}
			candidate := task("profile-reference-race", 42, 1)
			candidate.ResourceSnapshot = []byte(`{"browser_profile_id":"37"}`)
			if deletionFirst {
				if err := db.Exec("DELETE FROM browser_profiles WHERE id=37").Error; err != nil {
					t.Fatal(err)
				}
				if _, err := store(db).Create(candidate); err == nil {
					t.Fatal("saved reference to deleted Profile")
				}
			} else {
				if _, err := store(db).Create(candidate); err != nil {
					t.Fatal(err)
				}
				if err := db.Exec("DELETE FROM browser_profiles WHERE id=37").Error; err == nil {
					t.Fatal("deleted Profile referenced by unclaimed task")
				}
			}
		})
	}
}
