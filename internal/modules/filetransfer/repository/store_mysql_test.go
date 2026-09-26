package repository

import (
	"database/sql/driver"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var testNow = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

const testSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestCreateTaskDeduplicatesTheBusinessCommand(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO file_transfer_tasks (id, team_id, asset_type, asset_id, asset_title, source_object_key, purpose, execution_scope, status, requested_by, assigned_node_id, dependency_task_id, dedupe_key, total_bytes, expected_sha256, max_attempts, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?, 'pending', ?,?,?,?,?,?, ?,?,?) ON DUPLICATE KEY UPDATE id = id")).
		WithArgs("transfer-1", int64(7), "material", int64(42), "示例视频", "materials/42/aaaaaaaa.mp4", "user_download", "local_agent", int64(9), "node-1", nil, "dedupe-1", int64(100), testSHA256, 3, testNow, testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectTaskByDedupeKey(mock, "dedupe-1", testNow, nil)

	task, err := createTask(db, validCreateInput(), testNow)
	if err != nil {
		t.Fatalf("createTask() error = %v", err)
	}
	if task.ID != "transfer-1" || task.Status != model.StatusPending || task.AssignedNodeID != "node-1" {
		t.Fatalf("task = %+v", task)
	}
	if task.ExpectedSHA256 != testSHA256 || task.AssetTitle != "示例视频" {
		t.Fatalf("task = %+v, want the executor's expectations carried on the row", task)
	}
	assertExpectations(t, mock)
}

// The insert is idempotent on the dedupe key, but the row that survives keeps
// its own id — the id this call generated is discarded with the duplicate
// insert. Reading back by id would therefore report "not found" for a request
// that succeeded, so the read-back keys on the dedupe key, and this test pins
// that the returned task is the pre-existing row with the *other* id.
func TestCreateTaskReadsBackTheExistingRowWhenTheDedupeKeyIsTaken(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO file_transfer_tasks (id, team_id, asset_type, asset_id, asset_title, source_object_key, purpose, execution_scope, status, requested_by, assigned_node_id, dependency_task_id, dedupe_key, total_bytes, expected_sha256, max_attempts, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?, 'pending', ?,?,?,?,?,?, ?,?,?) ON DUPLICATE KEY UPDATE id = id")).
		WithArgs("transfer-2", int64(7), "material", int64(42), "示例视频", "materials/42/aaaaaaaa.mp4", "user_download", "local_agent", int64(9), "node-1", nil, "dedupe-1", int64(100), testSHA256, 3, testNow, testNow).
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectTaskByDedupeKey(mock, "dedupe-1", testNow, map[string]any{"id": "transfer-1", "status": "running"})

	input := validCreateInput()
	input.ID = "transfer-2"
	task, err := createTask(db, input, testNow)
	if err != nil {
		t.Fatalf("createTask() error = %v, want the existing row rather than a missing task", err)
	}
	if task.ID != "transfer-1" || task.Status != model.StatusRunning {
		t.Fatalf("task = %+v, want the pre-existing row", task)
	}
	assertExpectations(t, mock)
}

// A local transfer exists so an executor can run it later, on another machine,
// without reading the production tables. Everything that requires is refused at
// creation rather than discovered at lease time, when the failure would reach a
// user as a broken download.
//
// Each case names the fact it withholds and asserts that the refusal says so.
// Asserting only "an error came back" would be satisfied by the un-mocked insert
// that follows a *passing* validation — the test would stay green with the check
// it is about deleted, which is how it read before this assertion was added.
func TestCreateTaskRefusesATaskWithoutTheFactsAnExecutorNeeds(t *testing.T) {
	for name, testCase := range map[string]struct {
		mutate func(*CreateTaskInput)
		want   string
	}{
		"no title to name the file with": {func(in *CreateTaskInput) { in.AssetTitle = "  " }, "asset title"},
		"no object to grant":             {func(in *CreateTaskInput) { in.SourceObjectKey = "" }, "object to download"},
		"no total size to check against": {func(in *CreateTaskInput) { in.TotalBytes = 0 }, "total size"},
		"no expected hash":               {func(in *CreateTaskInput) { in.ExpectedSHA256 = "" }, "expected sha256"},
		"a truncated hash":               {func(in *CreateTaskInput) { in.ExpectedSHA256 = testSHA256[:63] }, "expected sha256"},
		"a hash that is not hex":         {func(in *CreateTaskInput) { in.ExpectedSHA256 = "z" + testSHA256[1:] }, "expected sha256"},
		"a local transfer with no node":  {func(in *CreateTaskInput) { in.AssignedNodeID = "" }, "assigned node"},
		"a negative total size":          {func(in *CreateTaskInput) { in.TotalBytes = -1 }, "invalid transfer total size"},
	} {
		input := validCreateInput()
		testCase.mutate(&input)
		db, mock := newMockGORM(t)
		_, err := createTask(db, input, testNow)
		if err == nil {
			t.Errorf("%s: createTask() accepted an invalid task", name)
			continue
		}
		if !strings.Contains(err.Error(), testCase.want) {
			t.Errorf("%s: error = %q, want it to name the missing fact %q", name, err, testCase.want)
		}
		assertExpectations(t, mock)
	}
}

// The claim is the place a cancellation is honoured. Without this predicate a
// cancelled running task whose node died becomes claimable again, and the new
// owner is then refused by every continuation predicate — progress, heartbeat
// and completion all require `cancel_requested_at IS NULL` — so the task stays
// `running` with no reachable terminal state.
//
// The attempt bound rides along in the same statement for the same reason: the
// claim is what increments `attempt_count`, so without it a task whose lease
// keeps expiring is retried forever.
func TestClaimLocalTaskRefusesATaskThatWasAskedToStopOrHasNoAttemptsLeft(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'local_agent' AND assigned_node_id = ? AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))")).
		WithArgs("node-1", testNow.Add(2*time.Minute), testNow, testNow, testNow, "transfer-1", "node-1", testNow).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, claimed, err := claimLocalTask(db, "transfer-1", "node-1", testNow, 2*time.Minute)
	if err != nil {
		t.Fatalf("claimLocalTask() error = %v", err)
	}
	if claimed {
		t.Fatal("a cancelled, exhausted, or already-leased task must not be claimable")
	}
	assertExpectations(t, mock)
}

func TestClaimLocalTaskUsesAConditionalUpdate(t *testing.T) {
	db, mock := newMockGORM(t)
	leaseUntil := testNow.Add(2 * time.Minute)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'local_agent' AND assigned_node_id = ? AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))")).
		WithArgs("node-1", leaseUntil, testNow, testNow, testNow, "transfer-1", "node-1", testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectTaskByID(mock, "transfer-1", testNow, map[string]any{"status": "running", "claimed_by_node_id": "node-1", "attempt_count": 1, "lease_expires_at": leaseUntil, "heartbeat_at": testNow, "started_at": testNow})

	task, claimed, err := claimLocalTask(db, "transfer-1", "node-1", testNow, 2*time.Minute)
	if err != nil {
		t.Fatalf("claimLocalTask() error = %v", err)
	}
	if !claimed || task.Status != model.StatusRunning || task.ClaimedByNodeID != "node-1" || task.AttemptCount != 1 {
		t.Fatalf("claimed=%v task=%+v", claimed, task)
	}
	assertExpectations(t, mock)
}

// A Cloud worker has no task id to offer — nothing told it which task to run —
// so the pick and the lease are two statements: the queue head is selected, then
// leased with the whole predicate re-asserted.
func TestClaimCloudTaskPicksTheQueueHeadThenReassertsThePredicate(t *testing.T) {
	db, mock := newMockGORM(t)
	leaseUntil := testNow.Add(2 * time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM file_transfer_tasks WHERE execution_scope = 'cloud' AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?)) ORDER BY created_at ASC, id ASC LIMIT 1")).
		WithArgs(testNow).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("transfer-1"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'cloud' AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))")).
		WithArgs("cloud-worker:host:1:uuid", leaseUntil, testNow, testNow, testNow, "transfer-1", testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectTaskByID(mock, "transfer-1", testNow, map[string]any{"status": "running", "purpose": "compose_input_prepare", "execution_scope": "cloud", "claimed_by_node_id": "cloud-worker:host:1:uuid", "attempt_count": 1})

	task, claimed, err := claimCloudTask(db, "cloud-worker:host:1:uuid", testNow, 2*time.Minute)
	if err != nil {
		t.Fatalf("claimCloudTask() error = %v", err)
	}
	if !claimed || task.ID != "transfer-1" || task.ExecutionScope != model.ExecutionCloud {
		t.Fatalf("claimed=%v task=%+v", claimed, task)
	}
	assertExpectations(t, mock)
}

// Losing the race for the queue head is not a failure. If another worker leased
// the candidate between the select and the update, the update matches nothing
// and the caller is told there was no task this round.
func TestClaimCloudTaskReportsNoTaskWhenItLosesTheRace(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM file_transfer_tasks WHERE execution_scope = 'cloud' AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?)) ORDER BY created_at ASC, id ASC LIMIT 1")).
		WithArgs(testNow).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("transfer-1"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'cloud' AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))")).
		WithArgs("cloud-worker:host:1:uuid", testNow.Add(2*time.Minute), testNow, testNow, testNow, "transfer-1", testNow).
		WillReturnResult(sqlmock.NewResult(0, 0))

	task, claimed, err := claimCloudTask(db, "cloud-worker:host:1:uuid", testNow, 2*time.Minute)
	if err != nil {
		t.Fatalf("claimCloudTask() error = %v, want no task rather than an error", err)
	}
	if claimed || task.ID != "" {
		t.Fatalf("claimed=%v task=%+v, want no task", claimed, task)
	}
	assertExpectations(t, mock)
}

// An empty queue is not an error either.
func TestClaimCloudTaskReportsNoTaskForAnEmptyQueue(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM file_transfer_tasks WHERE execution_scope = 'cloud' AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?)) ORDER BY created_at ASC, id ASC LIMIT 1")).
		WithArgs(testNow).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, claimed, err := claimCloudTask(db, "cloud-worker:host:1:uuid", testNow, 2*time.Minute)
	if err != nil {
		t.Fatalf("claimCloudTask() error = %v", err)
	}
	if claimed {
		t.Fatal("an empty queue has no task to lease")
	}
	assertExpectations(t, mock)
}

// The candidate read is what lets Cloud mint a download grant before it leases
// anything, so it must not lease: no `UPDATE` is expected here at all, and
// `assertExpectations` fails the test if one happens. That is the whole point —
// a version of this that leased first would leave a task `running` with an
// incremented `attempt_count` whenever minting failed.
func TestNextLocalTaskReportsTheCandidateWithoutLeasingIt(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM file_transfer_tasks WHERE execution_scope = 'local_agent' AND assigned_node_id = ? AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?)) ORDER BY created_at ASC, id ASC LIMIT 1")).
		WithArgs("node-1", testNow).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("transfer-9"))
	expectTaskByID(mock, "transfer-9", testNow, map[string]any{"id": "transfer-9", "status": "pending", "execution_scope": "local_agent", "purpose": "user_download", "assigned_node_id": "node-1", "attempt_count": 0})

	task, found, err := nextLocalTask(db, "node-1", testNow)
	if err != nil {
		t.Fatalf("nextLocalTask() error = %v", err)
	}
	if !found || task.ID != "transfer-9" || task.Status != model.StatusPending || task.AttemptCount != 0 {
		t.Fatalf("found=%v task=%+v", found, task)
	}
	assertExpectations(t, mock)
}

// A node with nothing waiting is the ordinary case for a polling executor, so it
// reports absence rather than an error.
func TestNextLocalTaskReportsNothingForANodeWithNoWork(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM file_transfer_tasks WHERE execution_scope = 'local_agent' AND assigned_node_id = ? AND cancel_requested_at IS NULL AND attempt_count < max_attempts AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?)) ORDER BY created_at ASC, id ASC LIMIT 1")).
		WithArgs("node-1", testNow).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	task, found, err := nextLocalTask(db, "node-1", testNow)
	if err != nil {
		t.Fatalf("nextLocalTask() error = %v", err)
	}
	if found || task.ID != "" {
		t.Fatalf("found=%v task=%+v, want nothing", found, task)
	}
	assertExpectations(t, mock)
}

func TestReportProgressRejectsCancelledOrUnclaimedTask(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, speed_bytes_per_sec = ?, eta_seconds = ?, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND cancel_requested_at IS NULL AND (lease_expires_at IS NULL OR lease_expires_at > ?) AND transferred_bytes <= ?")).
		WithArgs(int64(50), int64(100), int64(20), int64(3), testNow, testNow, "transfer-1", "node-1", testNow, int64(50)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err := reportProgress(db, ProgressInput{TaskID: "transfer-1", NodeID: "node-1", TransferredBytes: 50, TotalBytes: 100, SpeedBytesPerSec: 20, ETASeconds: 3}, testNow)
	if err != nil {
		t.Fatalf("reportProgress() error = %v", err)
	}
	if updated {
		t.Fatal("a cancelled, expired, or unclaimed task must not accept progress")
	}
	assertExpectations(t, mock)
}

// A completion has to agree with what the task declared. Both expectations are
// asserted in the predicate, so bytes that were never verified cannot be
// recorded as a success even if a handler's own check is bypassed.
func TestCompleteTaskRequiresRunningClaimAndIntegrity(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'success', transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, integrity_sha256 = ?, integrity_bytes = ?, file_name = COALESCE(?, file_name), finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND cancel_requested_at IS NULL AND (total_bytes = 0 OR total_bytes = ?) AND (expected_sha256 IS NULL OR expected_sha256 = ?)")).
		WithArgs(int64(100), int64(100), testSHA256, int64(100), "示例视频-42.mp4", testNow, testNow, testNow, "transfer-1", "node-1", int64(100), testSHA256).
		WillReturnResult(sqlmock.NewResult(0, 1))

	completed, err := completeTask(db, CompletionInput{TaskID: "transfer-1", NodeID: "node-1", Bytes: 100, SHA256: testSHA256, FileName: "示例视频-42.mp4"}, testNow)
	if err != nil {
		t.Fatalf("completeTask() error = %v", err)
	}
	if !completed {
		t.Fatal("a claimed running task with matching bytes and hash must complete")
	}
	assertExpectations(t, mock)
}

func TestHeartbeatOnlyExtendsTheCurrentNodeLease(t *testing.T) {
	db, mock := newMockGORM(t)
	leaseUntil := testNow.Add(2 * time.Minute)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET heartbeat_at = ?, lease_expires_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND cancel_requested_at IS NULL AND (lease_expires_at IS NULL OR lease_expires_at > ?)")).
		WithArgs(testNow, leaseUntil, testNow, "transfer-1", "node-1", testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := heartbeatTask(db, "transfer-1", "node-1", testNow, 2*time.Minute)
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
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = ?, error_code = ?, error_message = ?, finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ?")).
		WithArgs("failed", "source_unavailable", "source returned 404", testNow, testNow, testNow, "transfer-1", "node-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	finished, err := failTask(db, FailureInput{TaskID: "transfer-1", NodeID: "node-1", Status: model.StatusFailed, ErrorCode: "source_unavailable", ErrorMessage: "source returned 404"}, testNow)
	if err != nil {
		t.Fatalf("failTask() error = %v", err)
	}
	if !finished {
		t.Fatal("claimed executor must be able to record a terminal failure")
	}
	assertExpectations(t, mock)
}

func TestCancelTaskMarksPendingTerminalAndRequestsRunningCancellation(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = CASE WHEN status = 'pending' THEN 'cancelled' ELSE status END, cancel_requested_at = CASE WHEN status = 'running' THEN ? ELSE cancel_requested_at END, finished_at = CASE WHEN status = 'pending' THEN ? ELSE finished_at END, lease_expires_at = CASE WHEN status = 'pending' THEN NULL ELSE lease_expires_at END, error_code = CASE WHEN status = 'pending' THEN 'cancelled_by_user' ELSE error_code END, error_message = CASE WHEN status = 'pending' THEN 'cancelled by user' ELSE error_message END, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status IN ('pending', 'running')")).
		WithArgs(testNow, testNow, testNow, "transfer-1", int64(7), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	cancelled, err := cancelTask(db, "transfer-1", identity.TeamID(7), identity.UserID(9), testNow)
	if err != nil {
		t.Fatalf("cancelTask() error = %v", err)
	}
	if !cancelled {
		t.Fatal("pending task should cancel or running task should record a cancellation request")
	}
	assertExpectations(t, mock)
}

// Retrying does not reset the attempt count. It is the count of attempts this
// task has already consumed, so clearing it would make the bound unbounded
// through repeated retries.
func TestRetryTaskRequeuesAFailedTaskWithinItsAttemptBound(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'pending', claimed_by_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, started_at = NULL, finished_at = NULL, cancel_requested_at = NULL, transferred_bytes = 0, speed_bytes_per_sec = 0, eta_seconds = NULL, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status = 'failed' AND attempt_count < max_attempts")).
		WithArgs(testNow, "transfer-1", int64(7), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectTaskByID(mock, "transfer-1", testNow, map[string]any{"status": "pending", "attempt_count": 1})

	task, requeued, err := retryTask(db, "transfer-1", identity.TeamID(7), identity.UserID(9), testNow)
	if err != nil {
		t.Fatalf("retryTask() error = %v", err)
	}
	if !requeued || task.Status != model.StatusPending || task.AttemptCount != 1 {
		t.Fatalf("requeued=%v task=%+v", requeued, task)
	}
	assertExpectations(t, mock)
}

// A task that is not failed, or that has used every attempt, is simply not
// retryable — an unmatched row, not a permission the caller can forget to check.
func TestRetryTaskReportsNoRequeueWhenStatusOrBoundRefuses(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'pending', claimed_by_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, started_at = NULL, finished_at = NULL, cancel_requested_at = NULL, transferred_bytes = 0, speed_bytes_per_sec = 0, eta_seconds = NULL, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND team_id = ? AND requested_by = ? AND status = 'failed' AND attempt_count < max_attempts")).
		WithArgs(testNow, "transfer-1", int64(7), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, requeued, err := retryTask(db, "transfer-1", identity.TeamID(7), identity.UserID(9), testNow)
	if err != nil {
		t.Fatalf("retryTask() error = %v", err)
	}
	if requeued {
		t.Fatal("a task that is not failed, or is out of attempts, must not be requeued")
	}
	assertExpectations(t, mock)
}

// Cancelling a running transfer only records the request, because the executor
// owns the transition out of `running`. An executor that stopped reporting will
// never make it, and a task with a cancellation request is not leasable, so
// nothing else could either. This is the write that ends it.
func TestReconcileCancelledTasksFinishesCancellationsTheirExecutorAbandoned(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'cancelled', finished_at = ?, lease_expires_at = NULL, error_code = 'cancelled_by_user', error_message = 'cancelled by user', updated_at = ? WHERE status = 'running' AND cancel_requested_at IS NOT NULL AND (lease_expires_at IS NULL OR lease_expires_at <= ?)")).
		WithArgs(testNow, testNow, testNow).
		WillReturnResult(sqlmock.NewResult(0, 2))

	reconciled, err := reconcileCancelledTasks(db, testNow)
	if err != nil {
		t.Fatalf("reconcileCancelledTasks() error = %v", err)
	}
	if reconciled != 2 {
		t.Fatalf("reconciled = %d, want 2", reconciled)
	}
	assertExpectations(t, mock)
}

// The bound in the claim predicate is what makes this necessary: the last lease
// expiry leaves a task no one may claim. It is a failure, not a cancellation —
// nobody asked it to stop — so it is recorded as `failed` with its own code.
func TestReconcileExhaustedTasksFailsTasksThatUsedEveryAttempt(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'failed', error_code = 'lease_expired', error_message = 'transfer lease expired without a reporting executor', finished_at = ?, lease_expires_at = NULL, updated_at = ? WHERE status = 'running' AND cancel_requested_at IS NULL AND attempt_count >= max_attempts AND (lease_expires_at IS NULL OR lease_expires_at <= ?)")).
		WithArgs(testNow, testNow, testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))

	reconciled, err := reconcileExhaustedTasks(db, testNow)
	if err != nil {
		t.Fatalf("reconcileExhaustedTasks() error = %v", err)
	}
	if reconciled != 1 {
		t.Fatalf("reconciled = %d, want 1", reconciled)
	}
	assertExpectations(t, mock)
}

func TestGetTaskReportsNotFoundForAnAbsentOrEmptyID(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + taskColumnList + " FROM file_transfer_tasks WHERE id = ?")).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows(taskColumns()))

	if _, err := getTask(db, "missing"); err != ErrNotFound {
		t.Fatalf("getTask() error = %v, want ErrNotFound", err)
	}
	// An empty id is refused before it reaches SQL, so it adds no expectation.
	if _, err := getTask(db, "  "); err != ErrNotFound {
		t.Fatalf("getTask(\"\") error = %v, want ErrNotFound", err)
	}
	assertExpectations(t, mock)
}

// The list is what a client polls, so its order has to be a total order: with
// `created_at` alone, two tasks created in the same microsecond come back in an
// arbitrary order and rows visibly swap places between polls.
func TestListTasksScopesToTheRequestingUserAndOrdersDeterministically(t *testing.T) {
	db, mock := newMockGORM(t)
	requestedBy := identity.UserID(9)
	assetID := int64(42)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+taskColumnList+" FROM file_transfer_tasks WHERE requested_by = ? AND asset_id = ? AND status IN (?, ?) AND purpose IN (?) ORDER BY created_at DESC, id DESC LIMIT 50")).
		WithArgs(int64(9), int64(42), "running", "pending", "user_download").
		WillReturnRows(sqlmock.NewRows(taskColumns()).
			AddRow(taskRowValues(testNow, map[string]any{"id": "transfer-2", "status": "running"})...).
			AddRow(taskRowValues(testNow, nil)...))

	tasks, err := listTasks(db, TaskFilter{
		RequestedBy: &requestedBy,
		AssetID:     &assetID,
		Statuses:    []model.Status{model.StatusRunning, model.StatusPending, ""},
		Purposes:    []model.Purpose{model.PurposeUserDownload},
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("listTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].ID != "transfer-2" || tasks[0].Status != model.StatusRunning {
		t.Fatalf("tasks[0] = %+v", tasks[0])
	}
	assertExpectations(t, mock)
}

// The row builder above is only trustworthy while it covers every column. A
// column it does not know would silently read as NULL in every test, which is
// how a scan-order bug hides.
func TestTaskRowDefaultsCoverEveryColumnAndEveryColumnIsScanned(t *testing.T) {
	columns := taskColumns()
	for _, column := range columns {
		if _, ok := taskDefaults(column, testNow); !ok {
			t.Errorf("column %q has no default in the row builder", column)
		}
	}
	if want := strings.Count(taskColumnList, ",") + 1; len(columns) != want {
		t.Errorf("taskColumns() has %d entries, taskColumnList has %d columns", len(columns), want)
	}
	// A duplicate column would make scanTask read the wrong value for both
	// positions while still having the right length.
	seen := map[string]bool{}
	for _, column := range columns {
		if seen[column] {
			t.Errorf("column %q appears twice", column)
		}
		seen[column] = true
	}
}

func validCreateInput() CreateTaskInput {
	return CreateTaskInput{
		ID:              "transfer-1",
		TeamID:          identity.TeamID(7),
		AssetType:       model.AssetMaterial,
		AssetID:         42,
		AssetTitle:      "示例视频",
		SourceObjectKey: "materials/42/aaaaaaaa.mp4",
		Purpose:         model.PurposeUserDownload,
		ExecutionScope:  model.ExecutionLocalAgent,
		RequestedBy:     identity.UserID(9),
		AssignedNodeID:  "node-1",
		DedupeKey:       "dedupe-1",
		TotalBytes:      100,
		ExpectedSHA256:  testSHA256,
		MaxAttempts:     3,
	}
}

func expectTaskByID(mock sqlmock.Sqlmock, taskID string, now time.Time, overrides map[string]any) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + taskColumnList + " FROM file_transfer_tasks WHERE id = ?")).
		WithArgs(taskID).
		WillReturnRows(taskRow(now, overrides))
}

func expectTaskByDedupeKey(mock sqlmock.Sqlmock, dedupeKey string, now time.Time, overrides map[string]any) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + taskColumnList + " FROM file_transfer_tasks WHERE dedupe_key = ?")).
		WithArgs(dedupeKey).
		WillReturnRows(taskRow(now, overrides))
}

func taskColumns() []string {
	return []string{"id", "team_id", "asset_type", "asset_id", "asset_title", "source_object_key", "purpose", "execution_scope", "status", "requested_by", "assigned_node_id", "claimed_by_node_id", "dependency_task_id", "total_bytes", "transferred_bytes", "speed_bytes_per_sec", "eta_seconds", "attempt_count", "max_attempts", "lease_expires_at", "heartbeat_at", "started_at", "finished_at", "cancel_requested_at", "expected_sha256", "file_name", "error_code", "error_message", "integrity_sha256", "integrity_bytes", "created_at", "updated_at"}
}

// taskDefaults is the value each column reads as in a row builder call that did
// not override it, so a test states only the columns it is about.
func taskDefaults(column string, now time.Time) (driver.Value, bool) {
	switch column {
	case "id":
		return "transfer-1", true
	case "team_id":
		return int64(7), true
	case "asset_type":
		return "material", true
	case "asset_id":
		return int64(42), true
	case "asset_title":
		return "示例视频", true
	case "source_object_key":
		return "materials/42/aaaaaaaa.mp4", true
	case "purpose":
		return "user_download", true
	case "execution_scope":
		return "local_agent", true
	case "status":
		return "pending", true
	case "requested_by":
		return int64(9), true
	case "assigned_node_id":
		return "node-1", true
	case "total_bytes":
		return int64(100), true
	case "transferred_bytes", "speed_bytes_per_sec", "attempt_count":
		return int64(0), true
	case "max_attempts":
		return int64(3), true
	case "expected_sha256":
		return testSHA256, true
	case "created_at", "updated_at":
		return now, true
	case "claimed_by_node_id", "dependency_task_id", "eta_seconds",
		"lease_expires_at", "heartbeat_at", "started_at", "finished_at",
		"cancel_requested_at", "file_name", "error_code", "error_message",
		"integrity_sha256", "integrity_bytes":
		return nil, true
	default:
		return nil, false
	}
}

func taskRowValues(now time.Time, overrides map[string]any) []driver.Value {
	columns := taskColumns()
	values := make([]driver.Value, len(columns))
	for index, column := range columns {
		value, ok := taskDefaults(column, now)
		if !ok {
			panic("no default for column " + column)
		}
		values[index] = value
	}
	for column, value := range overrides {
		index := slices.Index(columns, column)
		if index < 0 {
			// A typo in an override name would otherwise be ignored, and the test
			// would assert against the default while looking like it set something.
			panic("unknown column override " + column)
		}
		values[index] = value
	}
	return values
}

func taskRow(now time.Time, overrides map[string]any) *sqlmock.Rows {
	return sqlmock.NewRows(taskColumns()).AddRow(taskRowValues(now, overrides)...)
}
