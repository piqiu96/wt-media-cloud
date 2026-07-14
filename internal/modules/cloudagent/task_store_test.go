package cloudagent

import (
	"errors"
	"testing"
	"time"
)

func TestCreateNoopIsIdempotent(t *testing.T) {
	store := NewTaskStoreWithClock(
		func() time.Time { return time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC) },
		func() string { return "task_1" },
	)

	first := store.CreateNoop(CreateTaskRequest{IdempotencyKey: "key-1"})
	second := store.CreateNoop(CreateTaskRequest{IdempotencyKey: "key-1"})

	if first.TaskID != "task_1" || second.TaskID != first.TaskID {
		t.Fatalf("expected idempotent task, got %+v and %+v", first, second)
	}
}

func TestClaimAllowsOnlyOneAgentDuringLease(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := NewTaskStoreWithClock(func() time.Time { return now }, func() string { return "task_1" })
	store.CreateNoop(CreateTaskRequest{})

	claimed, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	if claimed.Status != TaskLeased || claimed.AgentID != "agent-1" {
		t.Fatalf("unexpected claim: %+v", claimed)
	}

	sameAgent, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("same agent claim returned error: %v", err)
	}
	if sameAgent.TaskID != claimed.TaskID {
		t.Fatalf("same agent got different task: %+v", sameAgent)
	}

	_, err = store.Claim(ClaimTaskRequest{AgentID: "agent-2", LeaseSeconds: 30})
	if !errors.Is(err, ErrNoPendingTask) {
		t.Fatalf("err = %v", err)
	}
}

func TestClaimAfterLeaseExpiry(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := NewTaskStoreWithClock(func() time.Time { return now }, func() string { return "task_1" })
	store.CreateNoop(CreateTaskRequest{})
	_, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 1})
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}

	now = now.Add(2 * time.Second)
	claimed, err := store.Claim(ClaimTaskRequest{AgentID: "agent-2", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("Claim after expiry returned error: %v", err)
	}
	if claimed.AgentID != "agent-2" {
		t.Fatalf("AgentID = %q", claimed.AgentID)
	}
}

func TestReportRequiresLeasedAgent(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := NewTaskStoreWithClock(func() time.Time { return now }, func() string { return "task_1" })
	task := store.CreateNoop(CreateTaskRequest{})
	_, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}

	_, err = store.Report(task.TaskID, ReportTaskRequest{AgentID: "agent-2", Status: TaskRunning, Progress: 10})
	if !errors.Is(err, ErrTaskAgentMismatch) {
		t.Fatalf("err = %v", err)
	}

	updated, err := store.Report(task.TaskID, ReportTaskRequest{AgentID: "agent-1", Status: TaskSucceeded, Progress: 100})
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}
	if updated.Status != TaskSucceeded || updated.Progress != 100 {
		t.Fatalf("unexpected report: %+v", updated)
	}
}
