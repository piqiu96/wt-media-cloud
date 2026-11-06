package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUncertainReviewRollsBackRequeueWhenEvidenceInsertFails(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending', error_code = '', error_message = ''").
		WithArgs(now, int64(77), int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	mock.ExpectExec("UPDATE compose_task_attempts SET uncertain_review_actor_id").
		WithArgs(int64(9), "管理员核实旧进程已停止且无成片", now, now, int64(77), 0).
		WillReturnError(errors.New("review write failed"))
	mock.ExpectRollback()
	_, changed, err := resolveComposeUncertain(db, 77, 9, "管理员核实旧进程已停止且无成片", now)
	if err == nil || changed {
		t.Fatalf("requeue escaped failed review insert: changed=%v err=%v", changed, err)
	}
	assertExpectations(t, mock)
}

func TestUncertainReviewDoesNotWriteWhenTaskGuardMisses(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending', error_code = '', error_message = ''").
		WithArgs(now, int64(77), int64(77)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	_, changed, err := resolveComposeUncertain(db, 77, 9, "管理员核实旧进程已停止且无成片", now)
	if err != nil || changed {
		t.Fatalf("guard miss changed=%v err=%v", changed, err)
	}
	assertExpectations(t, mock)
}

func TestUncertainReviewRollsBackWhenOriginalAttemptIsMissing(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	row := composeTaskRow(77, "pending", now)
	for i, column := range composeTaskRowColumns {
		if column == "attempt_count" {
			row[i] = 1
		}
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending', error_code = '', error_message = ''").
		WithArgs(now, int64(77), int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(row...))
	mock.ExpectExec("UPDATE compose_task_attempts SET uncertain_review_actor_id").
		WithArgs(int64(9), "管理员核实旧进程已停止且无成片", now, now, int64(77), 1).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// No INSERT expectation: a missing original execution cannot create a retry
	// record or audit, and the preceding task requeue must roll back.
	mock.ExpectRollback()
	review, changed, err := resolveComposeUncertain(db, 77, 9, "管理员核实旧进程已停止且无成片", now)
	if err == nil || changed || review.ID != 0 {
		t.Fatalf("missing attempt escaped rollback: review=%+v changed=%v err=%v", review, changed, err)
	}
	assertExpectations(t, mock)
}

func TestUncertainReviewRequiresAllExistingOutputGuardsBeforeAnyWrite(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	// Each representation of a saved output independently blocks requeueing:
	// the task's output ID, its object key, and an independently saved output row.
	mock.ExpectExec(`UPDATE compose_tasks SET status = 'pending'.*AND composite_output_id IS NULL AND output_object_key = '' AND NOT EXISTS \(SELECT 1 FROM composite_outputs WHERE compose_task_id = \?\)`).
		WithArgs(now, int64(77), int64(77)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	review, changed, err := resolveComposeUncertain(db, 77, 9, "管理员核实旧进程已停止且无成片", now)
	if err != nil || changed || review.ID != 0 {
		t.Fatalf("existing output guard changed state: review=%+v changed=%v err=%v", review, changed, err)
	}
	assertExpectations(t, mock)
}
