package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestManifestFreezeGuardsCurrentUncancelledLease(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT execution_manifest_json FROM compose_tasks WHERE id = \\? AND claimed_node_id = \\? AND attempt_count = \\? AND status = 'running' AND cancel_requested = 0 AND lease_expires_at > NOW\\(3\\) FOR UPDATE").WithArgs(int64(1), int64(9), 1).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	if _, err := freezeComposeManifest(db, 1, 9, 1, json.RawMessage(`{}`)); !errors.Is(err, ErrComposeCommitLost) {
		t.Fatalf("err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestManifestFreezeReadsExistingWithoutReplacingAndPreservesDBErrors(t *testing.T) {
	for _, existing := range []bool{true, false} {
		db, mock := newMockGORM(t)
		mock.ExpectBegin()
		query := mock.ExpectQuery("SELECT execution_manifest_json").WithArgs(int64(1), int64(9), 1)
		if existing {
			query.WillReturnRows(sqlmock.NewRows([]string{"execution_manifest_json"}).AddRow(`{"first":1}`))
			mock.ExpectCommit()
		} else {
			query.WillReturnError(errors.New("temporary DB failure"))
			mock.ExpectRollback()
		}
		raw, err := freezeComposeManifest(db, 1, 9, 1, json.RawMessage(`{"new":2}`))
		if existing && (err != nil || string(raw) != `{"first":1}`) {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
		if !existing && (err == nil || errors.Is(err, ErrComposeCommitLost)) {
			t.Fatalf("DB failure hidden as lease loss: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
