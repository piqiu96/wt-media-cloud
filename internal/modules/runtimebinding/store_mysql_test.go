package runtimebinding

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRuntimeMigrationStoresOnlyHashesAndSeparatesPresence(t *testing.T) {
	content, err := os.ReadFile("../../../migrations/20260714_004_agent_runtime.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE local_agent_binding_tickets",
		"token_hash CHAR(64) NOT NULL",
		"CREATE TABLE local_agent_nodes",
		"credential_hash CHAR(64) NOT NULL",
		"CREATE TABLE browser_profile_runtime_presence",
		"FOREIGN KEY (profile_id) REFERENCES browser_profiles(id)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"binding_token ", "node_credential ", "cookie", "proxy_password", "local_path", "hostname"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("migration contains forbidden storage %q", forbidden)
		}
	}
}

func TestMySQLStoreConsumesBindingTicketTransactionally(t *testing.T) {
	store, mock, closeDB := newRuntimeMockStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	expires := now.Add(time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, session_id, token_hash, created_at, expires_at, used_at FROM local_agent_binding_tickets WHERE token_hash = ? FOR UPDATE`)).
		WithArgs("hash-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "session_id", "token_hash", "created_at", "expires_at", "used_at"}).
			AddRow("ticket-1", "user-1", "session-1", "hash-1", now.Add(-time.Minute), expires, nil))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE local_agent_binding_tickets SET used_at = ? WHERE id = ? AND used_at IS NULL`)).
		WithArgs(now, "ticket-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ticket, found, err := store.ConsumeTicket("hash-1", now)
	if err != nil || !found || ticket.SessionID != "session-1" || ticket.UsedAt == nil {
		t.Fatalf("ConsumeTicket() ticket=%+v found=%v error=%v", ticket, found, err)
	}
}

func TestMySQLStoreValidatesBoundOwnerAndEveryActiveProfile(t *testing.T) {
	store, mock, closeDB := newRuntimeMockStore(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT bit_main_user_id FROM users WHERE id = ? AND status = 'enabled'`)).
		WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id"}).AddRow("main-user-1"))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM browser_profiles WHERE user_id = \? AND main_user_id = \? AND local_status = 'active' AND bit_profile_id IN \(\?, \?\)`).
		WithArgs("user-1", "main-user-1", "profile-1", "profile-2").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	valid, err := store.ValidateRuntimeProfiles("user-1", "main-user-1", []string{"profile-1", "profile-2"})
	if err != nil || !valid {
		t.Fatalf("ValidateRuntimeProfiles() valid=%v error=%v", valid, err)
	}
}

func TestMySQLStoreAppliesRuntimePresenceTransactionally(t *testing.T) {
	store, mock, closeDB := newRuntimeMockStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	node := AgentNode{ID: "node-1", UserID: "user-1", LastHeartbeatAt: now, Status: "online"}
	report := validRuntimeReport()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE local_agent_nodes SET status = ?, last_heartbeat_at = ?, operating_system = ?, cpu_architecture = ?, agent_version = ?, python_version = ?, ffmpeg_status = ?, ffmpeg_version = ?, workdir_status = ?, disk_status = ?, disk_free_megabytes = ?, bitbrowser_status = ?, reported_main_user_id = ?, updated_at = ? WHERE id = ?`)).
		WithArgs(node.Status, now, report.OperatingSystem, report.CPUArchitecture, report.AgentVersion, report.PythonVersion, report.FFmpeg.Status, report.FFmpeg.Version, report.WorkdirStatus, report.Disk.Status, report.Disk.FreeMegabytes, report.BitBrowserStatus, report.MainUserID, now, node.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, profileID := range report.BitProfileIDs {
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO browser_profile_runtime_presence`)).
			WithArgs(sqlmock.AnyArg(), node.ID, node.UserID, report.MainUserID, "visible", now, now, now, profileID, node.UserID, report.MainUserID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectExec(`UPDATE browser_profile_runtime_presence SET status = 'not_visible'.*bit_profile_id NOT IN \(\?, \?\)`).
		WithArgs(now, now, node.UserID, "profile-1", "profile-2").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := store.ApplyRuntimeReport(node, report, now); err != nil {
		t.Fatalf("ApplyRuntimeReport() error = %v", err)
	}
}

func newRuntimeMockStore(t *testing.T) (*MySQLStore, sqlmock.Sqlmock, func()) {
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
