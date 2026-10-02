package repository

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
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

// The stage-2 migration severs the node row from user_sessions: the session
// foreign key (and with it the column) must go, while the binding ticket keeps
// its own session link — registration stays session-gated, the node does not.
func TestNodeSessionColumnIsDroppedAndTicketKeepsItsSession(t *testing.T) {
	content, err := os.ReadFile("../../../../migrations/20261002_047_local_agent_node_device_scoped.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"ALTER TABLE local_agent_nodes",
		"DROP FOREIGN KEY fk_local_agent_node_session",
		"DROP COLUMN session_id",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(text, "ALTER TABLE local_agent_binding_tickets") {
		t.Fatalf("migration must not touch the ticket table: %s", text)
	}
}

func TestFindNodeByCredentialHashUsesGORM(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, agent_id, device_id, user_id, mode, agent_version")).
		WithArgs("hash").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_id", "device_id", "user_id", "mode", "agent_version",
			"contract_major_version", "contract_revision", "credential_hash", "status", "registered_at", "last_heartbeat_at",
		}).AddRow("node-1", "agent-1", "device-1", 1, "local", "1.0", "v1", "2026.07.15.1", "hash", "online", now, now))

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

// What a download click needs is a node that is *bound*, not one that reported in
// the last ninety seconds: the click only queues a task, and the machine picks it
// up on its own schedule. So this query has no heartbeat bound while
// `checkLocalTrust` — the check behind the sensitive flows — keeps its own. Both
// texts are asserted because a sqlmock expectation is exact, and because the pair
// is the property: an edit that "unified" them would fail this arm. Neither query
// touches `user_sessions` any more (contract v2) — the session JOIN would be a
// regression, and this text pins its absence.
func TestTheHeartbeatWindowSeparatesQueueingFromTrustChecks(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM local_agent_nodes n\s+JOIN users u ON u\.id = n\.user_id\s+WHERE n\.user_id = \? AND n\.mode = 'local' AND n\.status = 'online'\s+AND u\.status = 'enabled' AND u\.device_id = n\.device_id\s+AND u\.bit_main_user_id IS NOT NULL AND n\.bitbrowser_status = 'normal'\s+AND n\.reported_main_user_id = u\.bit_main_user_id\s+ORDER BY n\.last_heartbeat_at DESC, n\.id ASC LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_id", "device_id", "user_id", "mode", "agent_version",
			"contract_major_version", "contract_revision", "credential_hash", "status", "registered_at", "last_heartbeat_at",
		}).AddRow("node-1", "agent-1", "device-1", 1, "local", "0.2.0", "v1", "2026.09.26.1", "hash", "online", now, now.Add(-time.Hour)))

	node, found, err := findTrustedLocalNode(db, 1)
	if err != nil || !found {
		t.Fatalf("findTrustedLocalNode() = %v, %v, want found without error", found, err)
	}
	// The heartbeat is still read — the ordering depends on it — and it is an hour
	// old here, which is the point: finding a node no longer requires a recent one.
	if node.ID != "node-1" || node.UserID != 1 || node.LastHeartbeatAt != now.Add(-time.Hour) {
		t.Fatalf("node = %#v", node)
	}

	mock.ExpectQuery(`SELECT EXISTS\(\s+SELECT 1 FROM local_agent_nodes n\s+JOIN users u ON u\.id = n\.user_id\s+WHERE n\.id = \? AND n\.user_id = \? AND n\.mode = 'local' AND n\.status = 'online'\s+AND n\.last_heartbeat_at >= \? AND u\.status = 'enabled' AND u\.device_id = n\.device_id\s+AND u\.bit_main_user_id IS NOT NULL AND n\.bitbrowser_status = 'normal'\s+AND n\.reported_main_user_id = u\.bit_main_user_id\s+\)`).
		WithArgs("node-1", int64(1), now.Add(-90*time.Second)).
		WillReturnRows(sqlmock.NewRows([]string{"trusted"}).AddRow(true))
	if _, err := checkLocalTrust(db, 1, "node-1", now, 90*time.Second); err != nil {
		t.Fatalf("checkLocalTrust() error = %v", err)
	}
}

// No bound device is an ordinary answer — the operator has not bound this machine
// yet — so it is absence, not an error, and the caller turns it into one error of
// its own.
func TestFindTrustedLocalNodeReportsAbsenceForAUserWithNoBoundDevice(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()

	mock.ExpectQuery(`FROM local_agent_nodes n`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, found, err := findTrustedLocalNode(db, 1); err != nil || found {
		t.Fatalf("findTrustedLocalNode() = %v, %v, want absence without error", found, err)
	}
	// A nonsensical user id short-circuits before the query, which the expectation
	// below cannot be met by — so this also pins that no query is issued without a
	// user.
	if _, found, err := findTrustedLocalNode(db, 0); err != nil || found {
		t.Fatalf("findTrustedLocalNode(0) = %v, %v", found, err)
	}
}

// Unbinding a device is one transaction, and stage 3 adds one statement to it:
// the unbound device's in-flight local downloads are cancelled so they become
// re-takeable (重新下载) instead of sitting forever assigned to a node that has
// just been marked replaced. Every statement is pinned, so a future edit that
// drops the cancellation, re-scopes it to the wrong user, or stops scoping it by
// the unbound device's nodes fails this run.
func TestUnbindDeviceMarksTheUnboundDevicesInflightDownloadsReclaimable(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT device_id FROM users WHERE id = ? AND status = 'enabled' FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"device_id"}).AddRow("device-7"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET device_id = NULL, device_public_key = NULL, device_name = NULL, device_bound_at = NULL, device_last_verified_at = NULL WHERE id = ?")).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE local_agent_nodes SET status = 'replaced', updated_at = ? WHERE user_id = ? AND mode = 'local' AND status <> 'replaced'")).
		WithArgs(now, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET status = 'cancelled', finished_at = ?, lease_expires_at = NULL, error_code = 'device_unbound', error_message = '设备已解绑，可重新下载', updated_at = ? WHERE requested_by = ? AND purpose = 'user_download' AND execution_scope = 'local_agent' AND status IN ('pending', 'running') AND assigned_node_id IN (SELECT id FROM local_agent_nodes WHERE device_id = ? AND mode = 'local')")).
		WithArgs(now, now, int64(1), "device-7").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, 'user.device.unbind', 'user', ?, ?, ?)")).
		WithArgs(sqlmock.AnyArg(), int64(1), "1", `{"result":"unbound"}`, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := unbindDevice(db, sharedidentity.UserID(1), now); err != nil {
		t.Fatalf("unbindDevice() error = %v", err)
	}
}

// Re-registering for a device mints a new node id and marks the previous node
// `replaced`, and a replaced node can never be claimed: `nextLocalTask` and
// `claimLocalTask` both match `assigned_node_id` exactly. So the registration
// transaction has to move the downloads that were pointed at the old node onto
// the new one, or a download clicked shortly before a re-login waits forever
// with no claimant that could ever match it.
//
// The statement and its scope are pinned in the same style as the unbind test
// above, because the failure mode here is silent: widening it to every task
// would still pass a "does it re-point" reading, and narrowing it to pending
// only would strand exactly the tasks whose lease outlives their executor.
func TestRegisteringForADeviceRePointsItsInflightDownloadsAtTheNewNode(t *testing.T) {
	db, mock, closeDB := newRuntimeMockGORM(t)
	defer closeDB()
	at := time.Date(2026, 10, 2, 22, 9, 59, 0, time.UTC)
	key := []byte("device-public-key")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT device_id, device_public_key FROM users WHERE id = ? AND status = 'enabled' FOR UPDATE")).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"device_id", "device_public_key"}).AddRow("device-7", key))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET device_last_verified_at = ? WHERE id = ?")).
		WithArgs(at, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE local_agent_nodes SET status = 'replaced', updated_at = ? WHERE user_id = ? AND device_id = ? AND status <> 'replaced'")).
		WithArgs(at, int64(3), "device-7").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE file_transfer_tasks SET assigned_node_id = ?, updated_at = ? WHERE requested_by = ? AND purpose = 'user_download' AND execution_scope = 'local_agent' AND status IN ('pending', 'running') AND assigned_node_id IN (SELECT id FROM local_agent_nodes WHERE user_id = ? AND device_id = ? AND mode = 'local')")).
		WithArgs("agent-node_new", at, int64(3), int64(3), "device-7").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO local_agent_nodes")).
		WithArgs("agent-node_new", "local-agent-dev", "device-7", int64(3), "local", "0.2.2", "1", "r1", "hash", "online", at, at, at).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := saveNode(db, model.AgentNode{
		ID: "agent-node_new", AgentID: "local-agent-dev", DeviceID: "device-7",
		DevicePublicKey: key, UserID: 3, Mode: "local", AgentVersion: "0.2.2",
		ContractMajorVersion: "1", ContractRevision: "r1", CredentialHash: "hash",
		Status: "online", RegisteredAt: at, LastHeartbeatAt: at,
	})
	if err != nil {
		t.Fatalf("saveNode() error = %v", err)
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
