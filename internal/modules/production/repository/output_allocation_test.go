package repository

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"
)

func TestOutputAllocationUsesAttemptIdentityOnlyWhileLeaseIsHeld(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM compose_tasks.*FOR UPDATE").WithArgs(int64(77), int64(9), 2, now).WillReturnRows(sqlmockIDRows(77))
	mock.ExpectExec("INSERT INTO compose_output_allocations").WithArgs(int64(77), 2, now).WillReturnResult(sqlmock.NewResult(81, 1))
	mock.ExpectQuery("SELECT id FROM compose_output_allocations").WithArgs(int64(77), 2).WillReturnRows(sqlmockIDRows(81))
	mock.ExpectCommit()
	id, err := reserveComposeOutputID(db, 77, 9, 2, now)
	if err != nil || id != 81 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	assertExpectations(t, mock)
}

func TestOutputAllocationRefusesAnExpiredOrCancelledClaim(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM compose_tasks.*FOR UPDATE").WithArgs(int64(77), int64(9), 2, now).WillReturnRows(sqlmockIDRows())
	mock.ExpectRollback()
	_, err := reserveComposeOutputID(db, 77, 9, 2, now)
	if !errors.Is(err, ErrComposeCommitLost) {
		t.Fatal(err)
	}
	assertExpectations(t, mock)
}
