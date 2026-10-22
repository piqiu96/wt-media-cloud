package service

import (
	"errors"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type fakeStore struct {
	task           SensitiveTask
	taskFound      bool
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

// The permit is a lock, so what it has to get right is whose lock it is: the
// task's user, its Profile and its operation from the task row, never from the
// caller's request.
func TestPreflightTakesTheLockForTheTasksUser(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true}
	service := testProfileGuard(store, &now)

	grant, err := service.Preflight(identityservice.UserID(1), "task-1")
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

// A permit id must not work as a cross-user bearer: the endpoint answers a task
// belonging to somebody else as not-found, so it cannot be used to probe ids.
func TestPreflightRefusesATaskOfAnotherUser(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true}
	service := testProfileGuard(store, &now)

	if _, err := service.Preflight(identityservice.UserID(2), "task-1"); !errors.Is(err, ErrTaskNotFound) {
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
	store := &fakeStore{task: task, taskFound: true}
	service := testProfileGuard(store, &now)

	if _, err := service.Preflight(identityservice.UserID(1), "task-1"); !errors.Is(err, ErrTaskAssignmentMismatch) {
		t.Fatalf("Preflight() error = %v, want ErrTaskAssignmentMismatch", err)
	}
}

func TestPreflightReturnsWaitingOrReviewWithoutAPermit(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	for _, outcome := range []Outcome{OutcomeWaiting, OutcomeReviewRequired} {
		store := &fakeStore{task: authorizedTask(), taskFound: true, acquireOutcome: PreflightOutcome{Outcome: outcome, ProfileID: "profile-1"}}
		grant, err := testProfileGuard(store, &now).Preflight(identityservice.UserID(1), "task-1")
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
