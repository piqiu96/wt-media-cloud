package profileguard

import (
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

func TestMySQLStoreGrantsPermitWhenProfileHasNoPriorLock(t *testing.T) {
	store, mock, closeDB := newGuardMockStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	task := authorizedTask()
	node := runtimebinding.AgentNode{ID: "node-1", UserID: "user-1", SessionID: "session-1", Status: "online"}
	permit := Permit{ID: "permit-1", TaskID: task.ID, UserID: task.UserID, ProfileID: task.ProfileID, NodeID: node.ID, Operation: task.Operation, Status: PermitActive, CredentialHash: "hash", AcquiredAt: now, ExpiresAt: now.Add(2 * time.Minute)}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM browser_profiles WHERE id = ? FOR UPDATE`)).WithArgs(task.ProfileID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(task.ProfileID))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*browser_profile_runtime_presence`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, status, expires_at FROM sensitive_profile_permits WHERE profile_id = ? ORDER BY acquired_at DESC LIMIT 1`)).WithArgs(task.ProfileID).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO sensitive_profile_permits`)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = ? AND status = ?`)).
		WithArgs(TaskRunning, now, task.ID, TaskAuthorized).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	outcome, err := store.AcquirePermit(task, node, permit, now, 90*time.Second)
	if err != nil || outcome.Outcome != OutcomeGranted || outcome.PermitID != permit.ID {
		t.Fatalf("AcquirePermit() outcome=%+v error=%v", outcome, err)
	}
}

func TestProfileGuardMigrationHasDurableTasksAndHashedPermits(t *testing.T) {
	content, err := os.ReadFile("../../../migrations/20260714_005_sensitive_profile_locks.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(content))
	for _, required := range []string{"create table sensitive_browser_tasks", "create table sensitive_profile_permits", "credential_hash char(64) not null", "foreign key (profile_id) references browser_profiles(id)"} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"permit_credential ", "cookie_value", "cookie_text", "proxy_password", "command_payload"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("migration contains forbidden %q", forbidden)
		}
	}
}

func TestMySQLStoreAcquiresPermitAfterAtomicRuntimeValidation(t *testing.T) {
	store, mock, closeDB := newGuardMockStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	task := authorizedTask()
	node := runtimebinding.AgentNode{ID: "node-1", UserID: "user-1", SessionID: "session-1", Status: "online"}
	permit := Permit{ID: "permit-1", TaskID: task.ID, UserID: task.UserID, ProfileID: task.ProfileID, NodeID: node.ID, Operation: task.Operation, Status: PermitActive, CredentialHash: "hash", AcquiredAt: now, ExpiresAt: now.Add(2 * time.Minute)}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM browser_profiles WHERE id = ? FOR UPDATE`)).WithArgs(task.ProfileID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(task.ProfileID))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*browser_profile_runtime_presence`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, status, expires_at FROM sensitive_profile_permits WHERE profile_id = ? ORDER BY acquired_at DESC LIMIT 1`)).WithArgs(task.ProfileID).WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()

	_, err := store.AcquirePermit(task, node, permit, now, 90*time.Second)
	if err == nil {
		t.Fatal("AcquirePermit() should surface non-NoRows lookup error")
	}
}

func TestMySQLStoreExpiredPermitRequiresReviewInsteadOfReuse(t *testing.T) {
	store, mock, closeDB := newGuardMockStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	task := authorizedTask()
	node := runtimebinding.AgentNode{ID: "node-1", UserID: "user-1", SessionID: "session-1", Status: "online"}
	permit := Permit{ID: "permit-new", TaskID: task.ID, UserID: task.UserID, ProfileID: task.ProfileID, NodeID: node.ID, Operation: task.Operation, Status: PermitActive, CredentialHash: "hash", AcquiredAt: now, ExpiresAt: now.Add(2 * time.Minute)}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM browser_profiles WHERE id = ? FOR UPDATE`)).WithArgs(task.ProfileID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(task.ProfileID))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*browser_profile_runtime_presence`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, status, expires_at FROM sensitive_profile_permits WHERE profile_id = ? ORDER BY acquired_at DESC LIMIT 1`)).WithArgs(task.ProfileID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "expires_at"}).AddRow("permit-old", PermitActive, now.Add(-time.Second)))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sensitive_profile_permits SET status = ?, finished_at = ? WHERE id = ? AND status = ?`)).WithArgs(PermitReviewRequired, now, "permit-old", PermitActive).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = (SELECT task_id FROM sensitive_profile_permits WHERE id = ?)`)).WithArgs(TaskReviewRequired, now, "permit-old").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	outcome, err := store.AcquirePermit(task, node, permit, now, 90*time.Second)
	if err != nil || outcome.Outcome != OutcomeReviewRequired {
		t.Fatalf("AcquirePermit() outcome=%+v error=%v", outcome, err)
	}
}

func newGuardMockStore(t *testing.T) (*MySQLStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	return NewMySQLStore(db), mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		db.Close()
	}
}
