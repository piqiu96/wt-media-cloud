package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/executionpreflight"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type fakeStore struct {
	task           SensitiveTask
	taskFound      bool
	cloudFacts     executionpreflight.CloudFacts
	acquireOutcome PreflightOutcome
	storedPermit   Permit
	finishPermitID string
	finishUserID   identityservice.UserID
	finishOutcome  FinishOutcome
	renewUserID    identityservice.UserID
	renewExpiresAt time.Time
}

func (s *fakeStore) FindAuthorizedTask(taskID string) (SensitiveTask, bool, error) {
	return s.task, s.taskFound && s.task.ID == taskID, nil
}

func (s *fakeStore) LoadCloudFacts(int64, string, string) (executionpreflight.CloudFacts, error) {
	return s.cloudFacts, nil
}

func (s *fakeStore) AcquirePermit(task SensitiveTask, permit Permit, _ time.Time) (PreflightOutcome, error) {
	s.storedPermit = permit
	if s.acquireOutcome.Outcome != "" {
		return s.acquireOutcome, nil
	}
	return PreflightOutcome{Outcome: OutcomeGranted, PermitID: permit.ID, ProfileID: task.ProfileID, ExpiresAt: &permit.ExpiresAt}, nil
}

func (s *fakeStore) RenewPermit(_ string, userID identityservice.UserID, _, expiresAt time.Time) (time.Time, error) {
	s.renewUserID, s.renewExpiresAt = userID, expiresAt
	return expiresAt, nil
}

func (s *fakeStore) FinishPermit(permitID string, userID identityservice.UserID, outcome FinishOutcome, _ time.Time) error {
	s.finishPermitID, s.finishUserID, s.finishOutcome = permitID, userID, outcome
	return nil
}

func testProfileGuard(store *fakeStore, now *time.Time) *Service {
	return NewService(
		store,
		WithClock(func() time.Time { return *now }),
		WithIDGenerator(func(string) string { return "permit-1" }),
	)
}

func authorizedTask() SensitiveTask {
	return SensitiveTask{ID: "task-1", UserID: identityservice.UserID(1), ProfileID: "profile-1", BitProfileID: "bit-profile-1", Operation: OperationInteraction, Status: TaskAuthorized}
}

func greenCloudFacts() executionpreflight.CloudFacts {
	return executionpreflight.CloudFacts{
		UserEnabled:   true,
		BitMainUserID: "main-1",
		Profile:       executionpreflight.ProfileFacts{Found: true, OwnedByUser: true, MainUserID: "main-1", LocalStatus: "active"},
		MediaAccount:  executionpreflight.MediaAccountFacts{Found: true, LoginStatus: "normal"},
	}
}

func liveAgentFacts(now time.Time) executionpreflight.AgentFacts {
	return executionpreflight.AgentFacts{BitBrowserRunning: true, MainUserID: "main-1", ObservedAt: now, ProfileVisible: true, FilesReady: true}
}

func greenPreflightInput(now time.Time) PreflightInput {
	return PreflightInput{AgentFacts: liveAgentFacts(now), DesktopSessionValid: true}
}

// 许可必须记在任务行上：用户、Profile 和操作都取自任务，不取请求。
func TestPreflightTakesTheLockForTheTasksUser(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true, cloudFacts: greenCloudFacts()}
	service := testProfileGuard(store, &now)

	grant, err := service.Preflight(identityservice.UserID(1), "task-1", greenPreflightInput(now))
	if err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	if grant.Outcome != OutcomeGranted || grant.PermitID != "permit-1" || grant.ProfileID != "profile-1" {
		t.Fatalf("grant = %+v", grant)
	}
	permit := store.storedPermit
	if permit.TaskID != "task-1" || permit.UserID != 1 || permit.ProfileID != "profile-1" ||
		permit.Operation != OperationInteraction || permit.Status != PermitActive {
		t.Fatalf("stored permit = %+v", permit)
	}
	if !permit.ExpiresAt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("expires_at = %v, want the permit TTL", permit.ExpiresAt)
	}
}

// 别人的任务按 not-found 处理，既不能取锁，也不能被用来探测任务 id。
func TestPreflightRefusesATaskOfAnotherUser(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true, cloudFacts: greenCloudFacts()}
	service := testProfileGuard(store, &now)

	if _, err := service.Preflight(identityservice.UserID(2), "task-1", greenPreflightInput(now)); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("Preflight() error = %v, want ErrTaskNotFound", err)
	}
	if store.storedPermit.ID != "" {
		t.Fatal("a refused preflight still took a permit")
	}
}

func TestPreflightRefusesAnUnknownOperation(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	task := authorizedTask()
	task.Operation = "unknown"
	store := &fakeStore{task: task, taskFound: true, cloudFacts: greenCloudFacts()}
	service := testProfileGuard(store, &now)

	if _, err := service.Preflight(identityservice.UserID(1), "task-1", greenPreflightInput(now)); !errors.Is(err, ErrTaskAssignmentMismatch) {
		t.Fatalf("Preflight() error = %v, want ErrTaskAssignmentMismatch", err)
	}
}

// 统一准入的失败要带具体错误码透传，且不能留下许可。
func TestPreflightStopsAtTheUnifiedChecksWithoutTakingAPermit(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true, cloudFacts: greenCloudFacts()}
	input := greenPreflightInput(now)
	input.AgentFacts.ObservedAt = now.Add(-time.Hour)

	_, err := testProfileGuard(store, &now).Preflight(identityservice.UserID(1), "task-1", input)
	var failure *executionpreflight.Failure
	if !errors.As(err, &failure) || failure.Code != "BIT_BROWSER_OFFLINE" {
		t.Fatalf("Preflight() error = %v, want BIT_BROWSER_OFFLINE", err)
	}
	if store.storedPermit.ID != "" {
		t.Fatal("a refused preflight still took a permit")
	}
}

func TestPreflightReturnsWaitingOrReviewWithoutAPermit(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	for _, outcome := range []Outcome{OutcomeWaiting, OutcomeReviewRequired} {
		store := &fakeStore{task: authorizedTask(), taskFound: true, cloudFacts: greenCloudFacts(), acquireOutcome: PreflightOutcome{Outcome: outcome, ProfileID: "profile-1"}}
		grant, err := testProfileGuard(store, &now).Preflight(identityservice.UserID(1), "task-1", greenPreflightInput(now))
		if err != nil || grant.Outcome != outcome || grant.PermitID != "" {
			t.Fatalf("outcome=%s grant=%+v error=%v", outcome, grant, err)
		}
	}
}

func TestFinishScopesToTheUserAndPreservesUncertainOutcome(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := testProfileGuard(store, &now)

	if err := service.Finish(identityservice.UserID(1), "permit-1", FinishResultUncertain); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if store.finishPermitID != "permit-1" || store.finishUserID != 1 || store.finishOutcome != FinishResultUncertain {
		t.Fatalf("finish facts = %q %d %q", store.finishPermitID, store.finishUserID, store.finishOutcome)
	}
	if err := service.Finish(identityservice.UserID(1), "permit-1", "maybe"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Finish() with an unknown outcome error = %v, want ErrInvalidInput", err)
	}
}

func TestRenewScopesToTheUserAndCapsLease(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := testProfileGuard(store, &now)

	expiresAt, err := service.Renew(identityservice.UserID(1), "permit-1", 10*time.Minute)
	if err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	if expiresAt != now.Add(2*time.Minute) || store.renewUserID != 1 {
		t.Fatalf("renew = %v for user %d, want the capped lease for the token's user", expiresAt, store.renewUserID)
	}
}
