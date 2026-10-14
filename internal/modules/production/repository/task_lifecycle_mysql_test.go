package repository

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func sqlmockIDRows(ids ...int64) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id"})
	for _, id := range ids {
		rows.AddRow(id)
	}
	return rows
}

func modelStatusRunning() model.ComposeTaskStatus { return model.ComposeTaskRunning }

func composeOutputFixture(taskID int64) model.CompositeOutput {
	return model.CompositeOutput{TeamID: 7, GameID: "delta", MaterialID: 42, MaterialUsageID: 9, TaskID: taskID, StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1, ObjectKey: "outputs/final", FileSizeBytes: 1024, SHA256: "abc", Media: json.RawMessage(`{"duration_ms":900}`), PublishStatus: "pending_publish", CreatedBy: 11}
}

// CHG-20261013-083 (M4-C5): the worker lifecycle writes. Each test pins the
// conditional-UPDATE shape — a worker may only move a row it still owns.

func TestClaimNextComposeTaskFlipsPendingToRunning(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	lease := 5 * time.Minute
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT max_running FROM compose_worker_capacity WHERE id = 1 FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"max_running"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM compose_tasks WHERE status = 'running'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT id FROM compose_tasks WHERE status = 'pending' AND attempt_count < max_attempts").
		WillReturnRows(sqlmockIDRows(77))
	mock.ExpectExec("UPDATE compose_tasks SET status = 'running'").
		WithArgs(int64(9), now.Add(lease), now, now, now, int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "running", now)...))
	mock.ExpectExec("INSERT INTO compose_task_attempts").
		WithArgs(int64(77), 0, now, now, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	claimed, found, err := claimNextComposeTask(db, 9, lease, now)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if claimed.ID != 77 || claimed.Status != modelStatusRunning() || claimed.AttemptCount != 0 {
		t.Fatalf("claimed=%+v", claimed)
	}
	assertExpectations(t, mock)
}

func TestClaimNextWritesNothingWhenQueueIsEmpty(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT max_running FROM compose_worker_capacity WHERE id = 1 FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"max_running"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM compose_tasks WHERE status = 'running'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT id FROM compose_tasks").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	_, found, err := claimNextComposeTask(db, 9, time.Minute, now)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	assertExpectations(t, mock)
}

func TestClaimNextRefusesTheClusterCapacityBeforeSelectingATask(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT max_running FROM compose_worker_capacity WHERE id = 1 FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"max_running"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM compose_tasks WHERE status = 'running'").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectRollback()
	_, found, err := claimNextComposeTask(db, 9, time.Minute, now)
	if err != nil || found {
		t.Fatalf("full cluster must not claim: found=%v err=%v", found, err)
	}
	assertExpectations(t, mock)
}

func TestClaimNextAbortsWhenTheClaimUpdateMisses(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT max_running FROM compose_worker_capacity WHERE id = 1 FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"max_running"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM compose_tasks WHERE status = 'running'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT id FROM compose_tasks").WillReturnRows(sqlmockIDRows(77))
	mock.ExpectExec("UPDATE compose_tasks SET status = 'running'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	_, found, err := claimNextComposeTask(db, 9, time.Minute, now)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	assertExpectations(t, mock)
}

func TestRenewComposeLeaseOnlyExtendsTheCallerOwnRow(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 5, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE compose_tasks SET lease_expires_at").
		WithArgs(now.Add(time.Minute), now, now, int64(77), int64(9), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	ok, err := renewComposeLease(db, 77, 9, 1, time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	mock.ExpectExec("UPDATE compose_tasks SET lease_expires_at").WillReturnResult(sqlmock.NewResult(0, 0))
	ok, err = renewComposeLease(db, 78, 9, 1, time.Minute, now)
	if err != nil || ok {
		t.Fatalf("foreign row must not renew: ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestRenewComposeLeaseChecksTheClaimAttempt(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 5, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE compose_tasks SET lease_expires_at.*attempt_count = \\?").
		WithArgs(now.Add(time.Minute), now, now, int64(77), int64(9), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	ok, err := renewComposeLease(db, 77, 9, 1, time.Minute, now)
	if err != nil || ok {
		t.Fatalf("old attempt must not renew: ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestRequestComposeCancelAuditsOnlyWhenTheFlagLands(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 6, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET cancel_requested = 1").
		WithArgs(now, int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "running", now)...))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	ok, err := requestComposeCancel(db, 77, 3, "compose_task.cancel", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET cancel_requested = 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	ok, err = requestComposeCancel(db, 78, 3, "compose_task.cancel", now)
	if err != nil || ok {
		t.Fatalf("second request writes nothing: ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestCommitComposeOutputWritesAssetAndSuccessInOneTransaction(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 7, 0, 0, time.UTC)
	out := composeOutputFixture(77)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `composite_outputs`").
		WithArgs(int64(7), "delta", int64(42), int64(9), int64(77), int64(3), "basic_cut", uint32(1), "outputs/final", int64(1024), "abc", out.Media, "pending_publish", int64(11), now, now).
		WillReturnResult(sqlmock.NewResult(55, 1))
	mock.ExpectExec("UPDATE compose_tasks SET status = 'success'").
		WithArgs("outputs/final", int64(55), now, now, int64(77), int64(9), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE compose_task_attempts a JOIN compose_tasks t").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	commit, err := commitComposeOutput(db, out, "outputs/final", 9, 1, now)
	if err != nil || commit.OutputID != 55 || commit.TaskID != 77 {
		t.Fatalf("commit=%+v err=%v", commit, err)
	}
	assertExpectations(t, mock)
}

func TestCommitComposeOutputRollsBackWhenTheTaskRowIsGone(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 7, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `composite_outputs`").WillReturnResult(sqlmock.NewResult(55, 1))
	mock.ExpectExec("UPDATE compose_tasks SET status = 'success'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if _, err := commitComposeOutput(db, composeOutputFixture(77), "outputs/final", 9, 1, now); !errors.Is(err, ErrComposeCommitLost) {
		t.Fatalf("want ErrComposeCommitLost, got %v", err)
	}
	assertExpectations(t, mock)
}

func TestReleaseComposeForRetryRequeuesWithoutErasingAttempts(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 8, 0, 0, time.UTC)
	// The stage and attempt number are captured before the requeue clears
	// them, and the closed attempt row keeps the phase the run reached.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT stage, attempt_count FROM compose_tasks WHERE id = \\? AND claimed_node_id = \\? AND attempt_count = \\?").
		WithArgs(int64(77), int64(9), 1, now).
		WillReturnRows(sqlmock.NewRows([]string{"stage", "attempt_count"}).AddRow("uploading", 1))
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending'").
		WithArgs("storage_flaky", "object storage timeout", now, int64(77), int64(9), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE compose_task_attempts SET outcome").
		WithArgs("failed", "storage_flaky", "object storage timeout", now, "uploading", now, int64(77), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ok, err := releaseComposeForRetry(db, 77, 9, 1, "storage_flaky", "object storage timeout", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestFailComposeTaskWritesTheTerminalError(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 9, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'failed'").
		WithArgs("input_missing", "source video is gone", now, now, int64(77), int64(9), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE compose_task_attempts a JOIN compose_tasks t").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ok, err := failComposeTask(db, 77, 9, 1, "input_missing", "source video is gone", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

// CHG-20261013-089 (M4-G): a manual retry is one transaction — guarded task
// requeue, manual attempt row, audit. A refusal (not failed, or an error code
// outside the retryable set) writes nothing.
func TestRetryFailedComposeTaskOpensTheManualAttempt(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 13, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending'").
		WithArgs(now, int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	mock.ExpectExec("INSERT INTO compose_task_attempts").
		WithArgs(int64(77), 1, now, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	ok, err := retryFailedComposeTask(db, 77, 5, "compose_task.retry", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestRetryFailedComposeTaskWritesNothingWhenRefused(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 13, 30, 0, time.UTC)
	for _, taskID := range []int64{77, 78} {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE compose_tasks SET status = 'pending'").
			WithArgs(now, taskID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()
		ok, err := retryFailedComposeTask(db, taskID, 5, "compose_task.retry", now)
		if err != nil || ok {
			t.Fatalf("refused retry must write nothing: ok=%v err=%v", ok, err)
		}
	}
	assertExpectations(t, mock)
}

func TestFindExpiredComposeLeasesReturnsOnlyLapsedRunningRows(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 10, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < \\?").
		WithArgs(now).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "running", now)...))
	items, err := findExpiredComposeLeases(db, now)
	if err != nil || len(items) != 1 || items[0].ID != 77 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}

// M4-C5 T-04: the recovery scan's writes. A reaper may only touch a row whose
// lease has already lapsed under the dead node — a live worker's row is untouchable.

func TestRequeueExpiredComposeLeaseTakesBackTheDeadWorkersRow(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 11, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT stage, attempt_count FROM compose_tasks").
		WithArgs(int64(77), int64(9), 2, now).
		WillReturnRows(sqlmock.NewRows([]string{"stage", "attempt_count"}).AddRow("cutting", 2))
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending'").
		WithArgs("lease_expired", "worker lease expired", now, int64(77), int64(9), 2, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE compose_task_attempts SET outcome").
		WithArgs("failed", "lease_expired", "worker lease expired", now, "cutting", now, int64(77), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ok, err := requeueExpiredComposeLease(db, 77, 9, 2, "lease_expired", "worker lease expired", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// A row whose lease has not lapsed reads as no rows in the guarded scan.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT stage, attempt_count FROM compose_tasks").
		WithArgs(int64(78), int64(9), 2, now).
		WillReturnRows(sqlmock.NewRows([]string{"stage", "attempt_count"}))
	mock.ExpectCommit()
	ok, err = requeueExpiredComposeLease(db, 78, 9, 2, "lease_expired", "worker lease expired", now)
	if err != nil || ok {
		t.Fatalf("a row without a lapsed lease must not be reaped: ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestFailExpiredComposeLeaseTerminalizesTheDeadWorkersRow(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 12, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'failed'").
		WithArgs("lease_expired", "attempts exhausted", now, now, int64(77), int64(9), 2, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE compose_task_attempts a JOIN compose_tasks t").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ok, err := failExpiredComposeLease(db, 77, 9, 2, "lease_expired", "attempts exhausted", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

// M4-G: the attempts read is oldest first — one row per attempt, the manual
// retry's pending row before the worker's claimed rows.
func TestListComposeTaskAttemptsReadsOldestFirst(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 14, 0, 0, time.UTC)
	attemptColumns := []string{"id", "task_id", "attempt", "source", "started_at", "finished_at", "outcome", "stage", "error_code", "error_message", "created_at", "updated_at"}
	mock.ExpectQuery("SELECT .* FROM compose_task_attempts WHERE task_id = \\? ORDER BY attempt ASC").
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(attemptColumns).
			AddRow(int64(1), int64(77), 1, "manual", nil, now, "failed", "uploading", "storage_error", "boom", now, now).
			AddRow(int64(2), int64(77), 2, "auto", now, nil, "running", "cutting", "", "", now, now))
	items, err := listComposeTaskAttempts(db, 77)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if items[0].Source != "manual" || items[0].StartedAt != nil || items[0].FinishedAt == nil || items[1].StartedAt == nil || items[1].FinishedAt != nil {
		t.Fatalf("attempt rows = %+v", items)
	}
	assertExpectations(t, mock)
}

func TestArchiveComposeOutputFlipsOnlyPendingPublishAndAudits(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 9, 8, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE composite_outputs SET publish_status = 'archived', updated_at = \\? WHERE id = \\? AND publish_status = 'pending_publish'").
		WithArgs(now, int64(55)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	ok, err := archiveComposeOutput(db, 55, 9, now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// A published or already-archived row matches nothing: no write, no audit.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE composite_outputs SET publish_status = 'archived'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	ok, err = archiveComposeOutput(db, 55, 9, now)
	if err != nil || ok {
		t.Fatalf("refused archive: ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}
