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

func uncertainTask() model.ComposeTask {
	operatorID := int64(10)
	return model.ComposeTask{
		ID: 77, TeamID: 7, CategoryID: 6, OperatorID: &operatorID,
		Status: model.ComposeTaskFailed, ErrorCode: "execution_uncertain", AttemptCount: 1,
	}
}

func uncertainInput() ResolveComposeUncertainInput {
	return ResolveComposeUncertainInput{OldExecutionStopped: true, NoOutputCommitted: true, Evidence: "核对 Worker 节点进程已退出，数据库没有成片记录"}
}

func TestSameTeamSeniorResolvesUncertainExecution(t *testing.T) {
	task := uncertainTask()
	store := &uncertainReviewMemory{task: &task}
	svc := NewComposeUncertainReviewService(store)
	team := identity.TeamID(7)
	senior := identity.PublicUser{ID: 10, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{6}}
	input := uncertainInput()

	resolved, err := svc.Resolve(senior, 77, input)
	if err != nil || resolved.Task.Status != model.ComposeTaskPending || resolved.Review.ActorUserID != 10 || store.called != 1 {
		t.Fatalf("senior resolve=%+v called=%d err=%v", resolved, store.called, err)
	}
	reviews, err := svc.List(senior, 77)
	if err != nil || len(reviews) != 1 || reviews[0].Evidence != input.Evidence {
		t.Fatalf("senior history=%+v err=%v", reviews, err)
	}
	if _, err := svc.Resolve(senior, 77, input); !errors.Is(err, ErrComposeTaskNotRetryable) || store.called != 2 {
		t.Fatalf("duplicate review: called=%d err=%v", store.called, err)
	}
}

func TestUncertainReviewRefusesOperatorsAndOtherTeams(t *testing.T) {
	team := identity.TeamID(7)
	otherTeam := identity.TeamID(8)
	operator := identity.PublicUser{ID: 10, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{6}}
	seniorElsewhere := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &otherTeam, CategoryIDs: []identity.CategoryID{6}}

	task := uncertainTask()
	store := &uncertainReviewMemory{task: &task}
	svc := NewComposeUncertainReviewService(store)
	// The operator can read this task (it is theirs), but reviewing is not reading.
	if _, err := svc.Resolve(operator, 77, uncertainInput()); !errors.Is(err, ErrComposeTaskForbidden) {
		t.Fatalf("operator review: %v", err)
	}
	if _, err := svc.List(operator, 77); !errors.Is(err, ErrComposeTaskForbidden) {
		t.Fatalf("operator review history: %v", err)
	}
	if store.called != 0 {
		t.Fatalf("refused reviews wrote %d times", store.called)
	}
	if _, err := svc.Resolve(seniorElsewhere, 77, uncertainInput()); !errors.Is(err, ErrComposeTaskNotFound) {
		t.Fatalf("other-team senior review: %v", err)
	}
	if _, err := svc.List(seniorElsewhere, 77); !errors.Is(err, ErrComposeTaskNotFound) {
		t.Fatalf("other-team senior history: %v", err)
	}
}

func TestUncertainReviewKeepsTheEvidenceAndConfirmationRules(t *testing.T) {
	task := uncertainTask()
	store := &uncertainReviewMemory{task: &task}
	svc := NewComposeUncertainReviewService(store)
	admin := identity.PublicUser{ID: 9, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	for _, invalid := range []ResolveComposeUncertainInput{
		{NoOutputCommitted: true, Evidence: uncertainInput().Evidence},
		{OldExecutionStopped: true, Evidence: uncertainInput().Evidence},
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
	resolved, err := svc.Resolve(admin, 77, uncertainInput())
	if err != nil || resolved.Task.Status != model.ComposeTaskPending || resolved.Review.Attempt != 1 || store.called != 1 {
		t.Fatalf("admin resolve=%+v called=%d err=%v", resolved, store.called, err)
	}
}
