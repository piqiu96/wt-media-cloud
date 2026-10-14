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
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending'").
		WithArgs(now, int64(77), int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	mock.ExpectExec("INSERT INTO compose_uncertain_reviews").
		WithArgs(int64(77), 0, int64(9), "管理员核实旧进程已停止且无成片", now).
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
	mock.ExpectExec("UPDATE compose_tasks SET status = 'pending'").
		WithArgs(now, int64(77), int64(77)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	_, changed, err := resolveComposeUncertain(db, 77, 9, "管理员核实旧进程已停止且无成片", now)
	if err != nil || changed {
		t.Fatalf("guard miss changed=%v err=%v", changed, err)
	}
	assertExpectations(t, mock)
}
