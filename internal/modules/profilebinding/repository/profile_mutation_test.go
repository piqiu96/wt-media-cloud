package repository

import (
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"
)

func TestProfileMetadataUpdateNeedsNoPublicationLock(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Now()
	mock.ExpectExec("UPDATE browser_profiles SET enabled = \\?, updated_at = \\? WHERE id = \\?").WithArgs(false, now, "37").WillReturnResult(sqlmock.NewResult(0, 1))
	enabled := false
	if e := updateProfile(db, "37", nil, &enabled, now); e != nil {
		t.Fatal(e)
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
