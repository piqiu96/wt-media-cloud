package repository

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestRuntimeMigrationStoresOnlyHashesAndSeparatesPresence(t *testing.T) {
	content, err := os.ReadFile("../../../../migrations/20260714_004_agent_runtime.sql")
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
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestFindNodeByCredentialHashUsesGORM(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, agent_id, device_id, user_id, session_id, mode, agent_version")).
		WithArgs("hash").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_id", "device_id", "user_id", "session_id", "mode", "agent_version",
			"contract_major_version", "contract_revision", "credential_hash", "status", "registered_at", "last_heartbeat_at",
		}).AddRow("node-1", "agent-1", "device-1", 1, "session-1", "local", "1.0", "v1", "2026.07.15.1", "hash", "online", now, now))

	node, found, err := findNodeByCredentialHash(db, "hash")
	if err != nil || !found {
		t.Fatalf("findNodeByCredentialHash() = %v, %v, want found without error", found, err)
	}
	if node.ID != "node-1" || node.UserID != 1 || node.Status != "online" {
		t.Fatalf("node = %#v", node)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

// The freshness bound is applied by the query rather than by the caller, and the
// tie-break is what makes two nodes reporting in the same second resolve to the
// same one on every call. Both are asserted by text because a sqlmock expectation
// is exact: dropping `n.id ASC` or the heartbeat comparison fails this arm, and
// replacing `n.user_id = ?` with `n.id = ?` fails it on the arguments.
func TestFindFreshLocalNodeScopesByUserAndPicksDeterministically(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM local_agent_nodes n\s+JOIN users u ON u\.id = n\.user_id\s+JOIN user_sessions s ON s\.id = n\.session_id\s+WHERE n\.user_id = \? AND n\.mode = 'local' AND n\.status = 'online'\s+AND n\.last_heartbeat_at >= \? AND s\.invalidated_at IS NULL AND u\.status = 'enabled'\s+AND u\.bit_main_user_id IS NOT NULL AND n\.bitbrowser_status = 'normal'\s+AND n\.reported_main_user_id = u\.bit_main_user_id\s+ORDER BY n\.last_heartbeat_at DESC, n\.id ASC LIMIT 1`).
		WithArgs(int64(1), now.Add(-90*time.Second)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_id", "device_id", "user_id", "session_id", "mode", "agent_version",
			"contract_major_version", "contract_revision", "credential_hash", "status", "registered_at", "last_heartbeat_at",
		}).AddRow("node-1", "agent-1", "device-1", 1, "session-1", "local", "0.2.0", "v1", "2026.09.26.1", "hash", "online", now, now))

	node, found, err := findFreshLocalNode(db, 1, now, 90*time.Second)
	if err != nil || !found {
		t.Fatalf("findFreshLocalNode() = %v, %v, want found without error", found, err)
	}
	if node.ID != "node-1" || node.UserID != 1 || node.LastHeartbeatAt != now {
		t.Fatalf("node = %#v", node)
	}
}

// No live device is an ordinary answer — the Agent is closed — so it is absence,
// not an error, and the caller turns it into one error of its own.
func TestFindFreshLocalNodeReportsAbsenceForAUserWithNoLiveDevice(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`FROM local_agent_nodes n`).
		WithArgs(int64(1), now.Add(-90*time.Second)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, found, err := findFreshLocalNode(db, 1, now, 90*time.Second); err != nil || found {
		t.Fatalf("findFreshLocalNode() = %v, %v, want absence without error", found, err)
	}
	// A nonsensical user id short-circuits before the query, which the expectation
	// below cannot be met by — so this also pins that no query is issued without a
	// user.
	if _, found, err := findFreshLocalNode(db, 0, now, 90*time.Second); err != nil || found {
		t.Fatalf("findFreshLocalNode(0) = %v, %v", found, err)
	}
}

func newRuntimeMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	db, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db, mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		_ = sqlDB.Close()
	}
}
