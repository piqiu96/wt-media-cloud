package cloudagent

import (
	"errors"
	"testing"
	"time"
)

func TestCreateIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	first := store.Create(CreateTaskRequest{TaskType: "noop_task", IdempotencyKey: "key-1"})
	second := store.Create(CreateTaskRequest{TaskType: "noop_task", IdempotencyKey: "key-1"})
	if first.TaskID != "task_1" || second.TaskID != first.TaskID {
		t.Fatalf("expected idempotent task, got %+v and %+v", first, second)
	}
}

func TestCreateDefaultsToNoop(t *testing.T) {
	store := newTestStore(t)
	task := store.Create(CreateTaskRequest{})
	if task.TaskType != "noop_task" || task.Status != "pending" {
		t.Fatalf("unexpected defaults: %+v", task)
	}
}

func TestClaimAllowsOnlyOneAgentDuringLease(t *testing.T) {
	store := newTestStore(t)
	store.Create(CreateTaskRequest{})
	claimed, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	if claimed.Status != "leased" || claimed.AgentID != "agent-1" {
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
	store := newTestStore(t)
	store.Create(CreateTaskRequest{})
	_, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 1})
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	// Advance clock.
	store.mem.now = func() time.Time { return time.Date(2026, 7, 14, 9, 0, 2, 0, time.UTC) }
	claimed, err := store.Claim(ClaimTaskRequest{AgentID: "agent-2", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("Claim after expiry returned error: %v", err)
	}
	if claimed.AgentID != "agent-2" {
		t.Fatalf("AgentID = %q", claimed.AgentID)
	}
}

func TestReportRequiresLeasedAgent(t *testing.T) {
	store := newTestStore(t)
	task := store.Create(CreateTaskRequest{})
	_, err := store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 30})
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	_, err = store.Report(task.TaskID, ReportTaskRequest{AgentID: "agent-2", Status: "running", Progress: 10})
	if !errors.Is(err, ErrTaskAgentMismatch) {
		t.Fatalf("err = %v", err)
	}
	updated, err := store.Report(task.TaskID, ReportTaskRequest{AgentID: "agent-1", Status: "succeeded", Progress: 100})
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}
	if updated.Status != "succeeded" || updated.Progress != 100 {
		t.Fatalf("unexpected report: %+v", updated)
	}
}

func TestCancelNonTerminalTask(t *testing.T) {
	store := newTestStore(t)
	task := store.Create(CreateTaskRequest{})
	cancelled, err := store.Cancel(task.TaskID, CancelTaskRequest{Message: "operator cancelled"})
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled, got %q", cancelled.Status)
	}
	_, err = store.Cancel(task.TaskID, CancelTaskRequest{})
	if !errors.Is(err, ErrTaskAlreadyTerminal) {
		t.Fatalf("expected ErrTaskAlreadyTerminal, got %v", err)
	}
}

func TestTerminalTaskRejectsReport(t *testing.T) {
	store := newTestStore(t)
	task := store.Create(CreateTaskRequest{})
	store.Claim(ClaimTaskRequest{AgentID: "agent-1", LeaseSeconds: 30})
	store.Report(task.TaskID, ReportTaskRequest{AgentID: "agent-1", Status: "succeeded", Progress: 100})
	_, err := store.Report(task.TaskID, ReportTaskRequest{AgentID: "agent-1", Status: "running", Progress: 50})
	if !errors.Is(err, ErrTaskAlreadyTerminal) {
		t.Fatalf("expected ErrTaskAlreadyTerminal, got %v", err)
	}
}

func newTestStore(t *testing.T) *MySQLTaskStore {
	t.Helper()
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	return NewMySQLTaskStoreWithClock(nil,
		func() time.Time { return now },
		func() string { return "task_1" },
	)
}
