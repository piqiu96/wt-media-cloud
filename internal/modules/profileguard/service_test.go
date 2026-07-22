package profileguard

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type fakeNodeAuth struct {
	node runtimebinding.AgentNode
	err  error
}

func (a fakeNodeAuth) AuthenticateNode(nodeID, credential string) (runtimebinding.AgentNode, error) {
	if a.err != nil || credential != "node-secret" || nodeID != a.node.ID {
		return runtimebinding.AgentNode{}, runtimebinding.ErrNodeCredentialInvalid
	}
	return a.node, nil
}

type fakeStore struct {
	task             SensitiveTask
	taskFound        bool
	acquireOutcome   PreflightOutcome
	storedPermit     Permit
	finishPermitID   string
	finishCredential string
	finishOutcome    FinishOutcome
}

func (s *fakeStore) FindAuthorizedTask(taskID string) (SensitiveTask, bool, error) {
	return s.task, s.taskFound && s.task.ID == taskID, nil
}

func (s *fakeStore) AcquirePermit(task SensitiveTask, node runtimebinding.AgentNode, permit Permit, at time.Time, freshness time.Duration) (PreflightOutcome, error) {
	s.storedPermit = permit
	if s.acquireOutcome.Outcome != "" {
		return s.acquireOutcome, nil
	}
	return PreflightOutcome{Outcome: OutcomeGranted, PermitID: permit.ID, ProfileID: task.ProfileID, ExpiresAt: &permit.ExpiresAt}, nil
}

func (s *fakeStore) RenewPermit(permitID, nodeID, credentialHash string, at, expiresAt time.Time) (time.Time, error) {
	if credentialHash == "permit-secret" {
		return time.Time{}, errors.New("plaintext credential reached store")
	}
	return expiresAt, nil
}

func (s *fakeStore) FinishPermit(permitID, nodeID, credentialHash string, outcome FinishOutcome, at time.Time) error {
	s.finishPermitID, s.finishCredential, s.finishOutcome = permitID, credentialHash, outcome
	return nil
}

func testProfileGuard(store *fakeStore, now *time.Time) *Service {
	return NewService(
		store,
		fakeNodeAuth{node: runtimebinding.AgentNode{ID: "node-1", UserID: identity.UserID(1), Mode: "local", Status: "online"}},
		WithClock(func() time.Time { return *now }),
		WithIDGenerator(func(prefix string) string { return "permit-1" }),
		WithSecretGenerator(func() string { return "permit-secret" }),
	)
}

func authorizedTask() SensitiveTask {
	return SensitiveTask{ID: "task-1", UserID: identity.UserID(1), ProfileID: "profile-1", BitProfileID: "bit-profile-1", NodeID: "node-1", Operation: OperationInteraction, Status: TaskAuthorized}
}

func TestPreflightGrantsHashedPermitOnlyForAssignedTask(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true}
	service := testProfileGuard(store, &now)

	grant, err := service.Preflight("node-1", "node-secret", "task-1")
	if err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	if grant.Outcome != OutcomeGranted || grant.PermitCredential != "permit-secret" || grant.ProfileID != "profile-1" {
		t.Fatalf("grant = %+v", grant)
	}
	if store.storedPermit.CredentialHash == "" || store.storedPermit.CredentialHash == grant.PermitCredential {
		t.Fatalf("stored credential = %q", store.storedPermit.CredentialHash)
	}

	store.task.NodeID = "other-node"
	if _, err := service.Preflight("node-1", "node-secret", "task-1"); !errors.Is(err, ErrTaskAssignmentMismatch) {
		t.Fatalf("assignment mismatch error = %v", err)
	}
}

func TestPreflightReturnsWaitingOrReviewWithoutCredential(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	for _, outcome := range []Outcome{OutcomeWaiting, OutcomeReviewRequired} {
		store := &fakeStore{task: authorizedTask(), taskFound: true, acquireOutcome: PreflightOutcome{Outcome: outcome, ProfileID: "profile-1"}}
		grant, err := testProfileGuard(store, &now).Preflight("node-1", "node-secret", "task-1")
		if err != nil || grant.Outcome != outcome || grant.PermitCredential != "" {
			t.Fatalf("outcome=%s grant=%+v error=%v", outcome, grant, err)
		}
	}
}

func TestFinishHashesCredentialAndPreservesUncertainOutcome(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := testProfileGuard(store, &now)

	if err := service.Finish("node-1", "node-secret", "permit-1", "permit-secret", FinishResultUncertain); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if store.finishPermitID != "permit-1" || store.finishCredential == "permit-secret" || store.finishOutcome != FinishResultUncertain {
		t.Fatalf("finish facts = %q %q %q", store.finishPermitID, store.finishCredential, store.finishOutcome)
	}
}

func TestRenewRequiresPermitCredentialAndCapsLease(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	service := testProfileGuard(&fakeStore{}, &now)

	expiresAt, err := service.Renew("node-1", "node-secret", "permit-1", "permit-secret", 10*time.Minute)
	if err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	if expiresAt != now.Add(2*time.Minute) {
		t.Fatalf("expiresAt = %v", expiresAt)
	}
}
