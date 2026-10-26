package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeleteExpiredSessionsBeforeUsesExpiryOnlyAndLimit(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	cutoff := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM user_sessions WHERE expires_at < ? LIMIT ?")).
		WithArgs(cutoff, 500).
		WillReturnResult(sqlmock.NewResult(0, 269))

	deleted, err := deleteExpiredSessionsBefore(context.Background(), db, cutoff, 500)
	if err != nil {
		t.Fatalf("deleteExpiredSessionsBefore() error = %v", err)
	}
	if deleted != 269 {
		t.Fatalf("deleted = %d, want 269", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteExpiredSessionsBeforeReturnsZeroRowsAffected(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	cutoff := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM user_sessions WHERE expires_at < ? LIMIT ?")).
		WithArgs(cutoff, 500).
		WillReturnResult(sqlmock.NewResult(0, 0))

	deleted, err := deleteExpiredSessionsBefore(context.Background(), db, cutoff, 500)
	if err != nil {
		t.Fatalf("deleteExpiredSessionsBefore() error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}

func TestDeleteExpiredSessionsBeforePropagatesError(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	cutoff := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	boom := errors.New("database is gone")
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM user_sessions WHERE expires_at < ? LIMIT ?")).
		WithArgs(cutoff, 500).
		WillReturnError(boom)

	deleted, err := deleteExpiredSessionsBefore(context.Background(), db, cutoff, 500)
	if !errors.Is(err, boom) {
		t.Fatalf("deleteExpiredSessionsBefore() error = %v, want %v", err, boom)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}
