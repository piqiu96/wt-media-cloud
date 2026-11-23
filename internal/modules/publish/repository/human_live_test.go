package repository_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"gorm.io/gorm"
)

func waitingScene(t *testing.T) (repo.ExecutionStore, model.Claim, model.Task) {
	t.Helper()
	s, makeTask := ownedStore(t)
	v := makeTask(0)
	now := time.Now().UTC()
	if _, err := s.Start(v, 1, 9, now); err != nil {
		t.Fatal(err)
	}
	c, err := s.Claim(9, "human-scene", now)
	if err != nil || c == nil {
		t.Fatal(err)
	}
	r := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStageReady, PlatformUID: c.Attempt.Input.PlatformUID, BitProfileID: c.Attempt.Input.BitProfileID, VideoSHA256: c.Attempt.Input.VideoSHA256, CoverSHA256: c.Attempt.Input.CoverSHA256, PortraitCoverSHA256: c.Attempt.Input.PortraitCoverSHA256}
	if _, err = s.Report(c.Task, 9, r, true, now); err != nil {
		t.Fatal(err)
	}
	r.Seq = 2
	r.Stage = model.AttemptStageAwaitingHuman
	if _, err = s.Report(c.Task, 9, r, true, now); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get(v.ID)
	return s, *c, v
}
func TestLiveHumanResumeKeepsOriginalSceneAndNeverGrantsSubmit(t *testing.T) {
	s, c, v := waitingScene(t)
	in := model.HumanContinuation{AttemptID: c.Attempt.ID, ExpectedRevision: v.Revision}
	now := time.Now().UTC()
	if _, err := s.GetHumanScene(v, 9, in); err != nil {
		t.Fatal(err)
	}
	bad := in
	bad.ExpectedRevision--
	if _, err := s.Resume(v, 9, bad, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("stale accepted: %v", err)
	}
	if _, err := s.GetHumanScene(v, 10, in); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("foreign owner: %v", err)
	}
	result, err := s.Resume(v, 9, in, now)
	if err != nil || result.AllowSubmit || result.TaskStatus != "running" || result.AttemptStatus != "running" {
		t.Fatalf("resume %+v %v", result, err)
	}
	current, _ := s.Get(v.ID)
	attempts, _ := s.Attempt(v.ID)
	if current.AttemptCount != 1 || *current.CurrentAttemptID != c.Attempt.ID || attempts[0].Stage != model.AttemptStageReady || attempts[0].ResultHash != nil || !reflect.DeepEqual(attempts[0].Input, c.Attempt.Input) {
		t.Fatal("restarted or replaced original input")
	}
	if _, err = s.Resume(current, 9, in, now); err != nil {
		t.Fatalf("lost reply cannot confirm exact resume: %v", err)
	}
	again, _ := s.Get(v.ID)
	if again.Revision != current.Revision {
		t.Fatal("replay changed task")
	}
	r := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 3, Stage: model.AttemptStageSubmitting, PlatformUID: c.Attempt.Input.PlatformUID, BitProfileID: c.Attempt.Input.BitProfileID, VideoSHA256: c.Attempt.Input.VideoSHA256, CoverSHA256: c.Attempt.Input.CoverSHA256, PortraitCoverSHA256: c.Attempt.Input.PortraitCoverSHA256}
	permission, err := s.Report(current, 9, r, true, now)
	if err != nil || !permission.AllowSubmit {
		t.Fatalf("fresh submit recheck %+v %v", permission, err)
	}
	if _, err = s.Resume(current, 9, in, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal("resume accepted after submit barrier")
	}
}
func TestLiveHumanResumeAuditRollbackAndLostScene(t *testing.T) {
	s, c, v := waitingScene(t)
	in := model.HumanContinuation{AttemptID: c.Attempt.ID, ExpectedRevision: v.Revision}
	now := time.Now().UTC()
	if err := s.DB.Exec("CREATE TRIGGER human_audit_failure BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='intentional human audit failure'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resume(v, 9, in, now); err == nil {
		t.Fatal("audit failure accepted")
	}
	current, _ := s.Get(v.ID)
	a, _ := s.Attempt(v.ID)
	if current.Status != "awaiting_human" || current.Revision != v.Revision || a[0].Status != "awaiting_human" {
		t.Fatal("partial resume")
	}
	s.DB.Exec("DROP TRIGGER human_audit_failure")
	if err := s.InterruptStale(now.Add(24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resume(v, 9, in, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatal("lost original scene reopened")
	}
}

func TestLiveHumanSceneRejectsChangedInputStopAndSubmitBarrier(t *testing.T) {
	for _, scenario := range []string{"input", "stop", "submit", "authorization", "attempt"} {
		t.Run(scenario, func(t *testing.T) {
			s, c, v := waitingScene(t)
			in := model.HumanContinuation{AttemptID: c.Attempt.ID, ExpectedRevision: v.Revision}
			switch scenario {
			case "input":
				read := s.ReadInput
				s.ReadInput = func(tx *gorm.DB, task model.Task) (model.ExecutionInput, error) {
					value, err := read(tx, task)
					value.PlatformUID = "different"
					return value, err
				}
			case "stop":
				s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("cancel_requested_at", time.Now().UTC())
			case "submit":
				s.DB.Model(&model.Attempt{}).Where("id=?", c.Attempt.ID).Update("submit_started_at", time.Now().UTC())
			case "authorization":
				s.DB.Model(&model.Task{}).Where("id=?", v.ID).Update("authorized_at", nil)
			case "attempt":
				in.AttemptID++
			}
			if _, err := s.GetHumanScene(v, 9, in); err == nil {
				t.Fatal("unsafe original scene accepted")
			}
			if _, err := s.Resume(v, 9, in, time.Now().UTC()); err == nil {
				t.Fatal("unsafe continuation accepted")
			}
			current, _ := s.Get(v.ID)
			if current.Status != model.StatusAwaitingHuman || current.AttemptCount != 1 {
				t.Fatal("refusal changed business state")
			}
		})
	}
}
