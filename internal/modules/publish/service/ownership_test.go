package service

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
	"time"
)

func publicationSenior(id identity.UserID) identity.PublicUser {
	team := identity.TeamID(7)
	return identity.PublicUser{ID: id, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
}
func TestSeniorPublicationExecutionIsOwned(t *testing.T) {
	task := model.Task{ID: 7, OperatorID: 9, TeamID: 7, CategoryID: 1, Platform: "baidu", Revision: 1, Status: model.StatusPublished}
	for _, id := range []identity.UserID{8, 9} {
		actor := publicationSenior(id)
		execution := &executionFake{v: task}
		recovery := &recoveryFake{task: task}
		_, startErr := (&ExecutionService{Repo: execution}).Start(actor, 7, 1)
		_, retryErr := (&RecoveryService{Repo: recovery}).Retry(actor, 7, 1)
		_, copyErr := (&RecoveryService{Repo: recovery}).Copy(actor, 7, 1)
		for name, err := range map[string]error{"start": startErr, "retry": retryErr, "copy": copyErr} {
			if id == 8 && !errors.Is(err, ErrNotFound) {
				t.Errorf("member %s reached execution: %v", name, err)
			}
			if id == 9 && err != nil {
				t.Errorf("owner %s refused: %v", name, err)
			}
		}
		if id == 8 && (execution.starts != 0 || recovery.writes != 0) {
			t.Error("member execution wrote authorization")
		}
	}
}
func TestSeniorCannotCreateWithMemberAccount(t *testing.T) {
	s, _, in := fixture()
	if _, err := s.Create(context.Background(), publicationSenior(8), in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("created member task: %v", err)
	}
	if len(s.Repo.(*memoryTasks).items) != 0 {
		t.Fatal("member task persisted")
	}
	if _, err := s.Create(context.Background(), publicationSenior(9), in); err != nil {
		t.Fatalf("own account refused: %v", err)
	}
}
func TestManagerStopsOnlyUnclaimedPublication(t *testing.T) {
	attempt := uint64(21)
	for _, actor := range []identity.PublicUser{publicationSenior(8), {ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}} {
		f := &recoveryFake{task: model.Task{ID: 7, OperatorID: 9, TeamID: 7, CategoryID: 1, Status: model.StatusQueued}}
		s := RecoveryService{Repo: f}
		in := model.Cancellation{ExpectedRevision: 1, Reason: "取消未开始安排"}
		if _, err := s.Cancel(actor, 7, in); err != nil || f.writes != 1 {
			t.Fatalf("manager cannot cancel queue: %v", err)
		}
		f.task.Status = model.StatusRunning
		f.task.CurrentAttemptID = &attempt
		if _, err := s.Cancel(actor, 7, in); !errors.Is(err, ErrNotFound) || f.writes != 1 {
			t.Fatalf("manager stopped member scene: %v", err)
		}
		if _, err := s.Cancel(publicationSenior(9), 7, in); err != nil || f.writes != 2 {
			t.Fatalf("owner stop refused: %v", err)
		}
	}
}
func TestManagerOnlySupplementsReviewingResult(t *testing.T) {
	attempt := uint64(21)
	now := time.Now().UTC().Add(-time.Hour)
	for _, actor := range []identity.PublicUser{publicationSenior(8), {ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}} {
		f := &recoveryFake{task: model.Task{ID: 7, OperatorID: 9, TeamID: 7, CategoryID: 1, Platform: "baidu", Status: model.StatusUncertain, CurrentAttemptID: &attempt}}
		s := RecoveryService{Repo: f}
		in := model.ResultResolution{AttemptID: 21, ExpectedRevision: 1, Conclusion: model.StatusFailed, Evidence: model.VerificationEvidence{PlatformUID: "original", Reason: "核实结论", Basis: "原执行现场及原账号稿件仍需所属成员实际核实", SceneClosed: true, NoPublishableWork: true}}
		if _, err := s.Resolve(actor, 7, in); !errors.Is(err, ErrNotFound) {
			t.Fatalf("manager asserted local scene: %v", err)
		}
		if f.writes != 0 {
			t.Fatal("manager changed unknown result")
		}
		if _, err := s.Resolve(publicationSenior(9), 7, in); err != nil {
			t.Fatalf("owner verification refused: %v", err)
		}
		f.task.Status = model.StatusReviewing
		in.Conclusion = model.StatusPublished
		in.PlatformSubmissionID = "1882371730190912814"
		in.PlatformWorkID = "7000395122085014122"
		in.WorkURL = "https://mbd.baidu.com/newspage/data/videolanding?nid=sv_7000395122085014122"
		in.PublishedAt = &now
		in.Evidence.NoPublishableWork = false
		in.Evidence.WorkMatched = true
		in.Evidence.PubliclyAccessible = true
		if _, err := s.Resolve(actor, 7, in); err != nil {
			t.Fatalf("manager public supplement refused: %v", err)
		}
	}
}
