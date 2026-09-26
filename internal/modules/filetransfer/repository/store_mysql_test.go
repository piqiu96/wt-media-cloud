package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestCreateTaskDeduplicatesTheBusinessCommand(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO file_transfer_tasks (id, team_id, asset_type, asset_id, purpose, execution_scope, status, requested_by, assigned_node_id, dependency_task_id, dedupe_key, max_attempts, created_at, updated_at) VALUES (?,?,?,?,?,?, 'pending', ?,?,?,?, ?,?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)")).
		WithArgs("transfer-1", int64(7), "material", int64(42), "user_download", "local_agent", int64(9), "node-1", nil, "dedupe-1", 3, now, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, asset_type, asset_id, purpose, execution_scope, status, requested_by, assigned_node_id, claimed_by_node_id, dependency_task_id, total_bytes, transferred_bytes, speed_bytes_per_sec, eta_seconds, attempt_count, max_attempts, lease_expires_at, heartbeat_at, started_at, finished_at, error_code, error_message, integrity_sha256, integrity_bytes, created_at, updated_at FROM file_transfer_tasks WHERE id = ?")).
		WithArgs("transfer-1").
		WillReturnRows(sqlmock.NewRows(taskColumns()).AddRow("transfer-1", int64(7), "material", int64(42), "user_download", "local_agent", "pending", int64(9), "node-1", nil, nil, 0, 0, 0, nil, 0, 3, nil, nil, nil, nil, nil, nil, nil, nil, now, now))

	task, err := createTask(db, CreateTaskInput{
		ID: "transfer-1", TeamID: identity.TeamID(7), AssetType: model.AssetMaterial, AssetID: 42,
		Purpose: model.PurposeUserDownload, ExecutionScope: model.ExecutionLocalAgent, RequestedBy: identity.UserID(9),
		AssignedNodeID: "node-1", DedupeKey: "dedupe-1", MaxAttempts: 3,
	}, now)
	if err != nil {
		t.Fatalf("createTask() error = %v", err)
	}
	if task.ID != "transfer-1" || task.Status != model.StatusPending || task.AssignedNodeID != "node-1" {
		t.Fatalf("task = %+v", task)
	}
	assertExpectations(t, mock)
}

func TestClaimLocalTaskUsesAConditionalUpdate(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'local_agent' AND assigned_node_id = ? AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))")).
		WithArgs("node-1", leaseUntil, now, now, now, "transfer-1", "node-1", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, asset_type, asset_id, purpose, execution_scope, status, requested_by, assigned_node_id, claimed_by_node_id, dependency_task_id, total_bytes, transferred_bytes, speed_bytes_per_sec, eta_seconds, attempt_count, max_attempts, lease_expires_at, heartbeat_at, started_at, finished_at, error_code, error_message, integrity_sha256, integrity_bytes, created_at, updated_at FROM file_transfer_tasks WHERE id = ?")).
		WithArgs("transfer-1").
		WillReturnRows(sqlmock.NewRows(taskColumns()).AddRow("transfer-1", int64(7), "material", int64(42), "user_download", "local_agent", "running", int64(9), "node-1", "node-1", nil, int64(100), int64(0), int64(0), nil, 1, 3, leaseUntil, now, now, nil, nil, nil, nil, nil, now, now))

	task, claimed, err := claimLocalTask(db, "transfer-1", "node-1", now, 2*time.Minute)
	if err != nil {
		t.Fatalf("claimLocalTask() error = %v", err)
	}
	if !claimed || task.Status != model.StatusRunning || task.ClaimedByNodeID != "node-1" || task.AttemptCount != 1 {
		t.Fatalf("claimed=%v task=%+v", claimed, task)
	}
	assertExpectations(t, mock)
}

func TestReportProgressRejectsCancelledOrUnclaimedTask(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, speed_bytes_per_sec = ?, eta_seconds = ?, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND (lease_expires_at IS NULL OR lease_expires_at > ?) AND transferred_bytes <= ?")).
		WithArgs(int64(50), int64(100), int64(20), int64(3), now, now, "transfer-1", "node-1", now, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err := reportProgress(db, ProgressInput{TaskID: "transfer-1", NodeID: "node-1", TransferredBytes: 50, TotalBytes: 100, SpeedBytesPerSec: 20, ETASeconds: 3}, now)
	if err != nil {
		t.Fatalf("reportProgress() error = %v", err)
	}
	if updated {
		t.Fatal("a cancelled, expired, or unclaimed task must not accept progress")
	}
	assertExpectations(t, mock)
}

func TestCompleteTaskRequiresRunningClaimAndIntegrity(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	sha256 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'success', transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, integrity_sha256 = ?, integrity_bytes = ?, finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND (total_bytes = 0 OR total_bytes = ?) AND ? = ?")).
		WithArgs(int64(100), int64(100), sha256, int64(100), now, now, now, "transfer-1", "node-1", int64(100), int64(100), int64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	completed, err := completeTask(db, CompletionInput{TaskID: "transfer-1", NodeID: "node-1", Bytes: 100, SHA256: sha256}, now)
	if err != nil {
		t.Fatalf("completeTask() error = %v", err)
	}
	if !completed {
		t.Fatal("a claimed running task with matching bytes must complete")
	}
	assertExpectations(t, mock)
}

func TestHeartbeatOnlyExtendsTheCurrentNodeLease(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET heartbeat_at = ?, lease_expires_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND (lease_expires_at IS NULL OR lease_expires_at > ?)")).
		WithArgs(now, leaseUntil, now, "transfer-1", "node-1", now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := heartbeatTask(db, "transfer-1", "node-1", now, 2*time.Minute)
	if err != nil {
		t.Fatalf("heartbeatTask() error = %v", err)
	}
	if !updated {
		t.Fatal("expected current claimant to extend its lease")
	}
	assertExpectations(t, mock)
}

func TestFailTaskOnlyAllowsTheClaimedExecutorToWriteTerminalFailure(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = ?, error_code = ?, error_message = ?, finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ?")).
		WithArgs("failed", "source_unavailable", "source returned 404", now, now, now, "transfer-1", "node-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	finished, err := failTask(db, FailureInput{TaskID: "transfer-1", NodeID: "node-1", Status: model.StatusFailed, ErrorCode: "source_unavailable", ErrorMessage: "source returned 404"}, now)
	if err != nil {
		t.Fatalf("failTask() error = %v", err)
	}
	if !finished {
		t.Fatal("claimed executor must be able to record a terminal failure")
	}
	assertExpectations(t, mock)
}

func taskColumns() []string {
	return []string{"id", "team_id", "asset_type", "asset_id", "purpose", "execution_scope", "status", "requested_by", "assigned_node_id", "claimed_by_node_id", "dependency_task_id", "total_bytes", "transferred_bytes", "speed_bytes_per_sec", "eta_seconds", "attempt_count", "max_attempts", "lease_expires_at", "heartbeat_at", "started_at", "finished_at", "error_code", "error_message", "integrity_sha256", "integrity_bytes", "created_at", "updated_at"}
}
