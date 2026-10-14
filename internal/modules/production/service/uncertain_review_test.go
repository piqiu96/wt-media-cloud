package service

import (
	"errors"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

type uncertainReviewMemory struct {
	task    *model.ComposeTask
	called  int
	reviews []model.ComposeUncertainReview
}

func (f *uncertainReviewMemory) Find(int64) (model.ComposeTask, bool, error) {
	if f.task == nil {
		return model.ComposeTask{}, false, nil
	}
	return *f.task, true, nil
}

func (f *uncertainReviewMemory) ResolveUncertain(taskID, actorID int64, evidence string) (model.ComposeUncertainReview, bool, error) {
	f.called++
	if f.task.Status != model.ComposeTaskFailed || f.task.ErrorCode != "execution_uncertain" {
		return model.ComposeUncertainReview{}, false, nil
	}
	review := model.ComposeUncertainReview{ID: 5, TaskID: taskID, Attempt: f.task.AttemptCount, ActorUserID: actorID, Evidence: evidence}
	f.reviews = append(f.reviews, review)
	f.task.Status = model.ComposeTaskPending
	return review, true, nil
}

func (f *uncertainReviewMemory) ListUncertainReviews(int64) ([]model.ComposeUncertainReview, error) {
	return f.reviews, nil
}

func TestUncertainReviewRequiresAdminAndExplicitEvidence(t *testing.T) {
	task := model.ComposeTask{ID: 77, TeamID: 7, GameID: "game-a", Status: model.ComposeTaskFailed, ErrorCode: "execution_uncertain", AttemptCount: 1}
	store := &uncertainReviewMemory{task: &task}
	svc := NewComposeUncertainReviewService(store)
	input := ResolveComposeUncertainInput{OldExecutionStopped: true, NoOutputCommitted: true, Evidence: "核对 Worker 节点进程已退出，数据库没有成片记录"}
	admin := identity.PublicUser{ID: 9, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	team := identity.TeamID(7)
	senior := identity.PublicUser{ID: 10, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}
	if _, err := svc.Resolve(senior, 77, input); !errors.Is(err, ErrComposeTaskForbidden) {
		t.Fatalf("senior review: %v", err)
	}
	if _, err := svc.List(senior, 77); !errors.Is(err, ErrComposeTaskForbidden) {
		t.Fatalf("senior review history: %v", err)
	}
	for _, invalid := range []ResolveComposeUncertainInput{
		{NoOutputCommitted: true, Evidence: input.Evidence},
		{OldExecutionStopped: true, Evidence: input.Evidence},
		{OldExecutionStopped: true, NoOutputCommitted: true, Evidence: "太短"},
		{OldExecutionStopped: true, NoOutputCommitted: true, Evidence: "已检查 https://secret.example/signed-token"},
	} {
		if _, err := svc.Resolve(admin, 77, invalid); !errors.Is(err, ErrComposeTaskInvalid) {
			t.Fatalf("invalid declaration %+v: %v", invalid, err)
		}
	}
	if store.called != 0 {
		t.Fatalf("invalid input wrote %d times", store.called)
	}
	resolved, err := svc.Resolve(admin, 77, input)
	if err != nil || resolved.Task.Status != model.ComposeTaskPending || resolved.Review.Attempt != 1 || store.called != 1 {
		t.Fatalf("resolve=%+v called=%d err=%v", resolved, store.called, err)
	}
	if _, err := svc.Resolve(admin, 77, input); !errors.Is(err, ErrComposeTaskNotRetryable) || store.called != 2 {
		t.Fatalf("duplicate review: called=%d err=%v", store.called, err)
	}
	reviews, err := svc.List(admin, 77)
	if err != nil || len(reviews) != 1 || reviews[0].Evidence != input.Evidence {
		t.Fatalf("admin history=%+v err=%v", reviews, err)
	}
}
