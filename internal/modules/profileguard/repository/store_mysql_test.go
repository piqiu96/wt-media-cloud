package repository

import (
	"os"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// The permits are a mutual-exclusion lock on a Profile, not a credential: the
// Agent Token is what admits the caller, and the permit row only guarantees one
// sensitive operation runs per Profile at a time. 071 therefore removes the
// node and credential columns the first design needed.
func TestProfileGuardMigrationDropsThePermitCredentialLayer(t *testing.T) {
	content, err := osReadFile("../../../../migrations/20261023_071_remove_agent_device_model.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, required := range []string{
		"ALTER TABLE sensitive_profile_permits",
		"DROP FOREIGN KEY fk_sensitive_profile_permit_node",
		"DROP INDEX uq_sensitive_profile_permit_credential_hash",
		"DROP COLUMN node_id",
		"DROP COLUMN credential_hash",
	} {
		if !containsString(string(content), required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestFindAuthorizedTaskUsesGORM(t *testing.T) {
	db, mock, closeDB := newGuardMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, profile_id, bit_profile_id, operation, status")).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "profile_id", "bit_profile_id", "operation", "status", "created_at", "updated_at"}).
			AddRow("task-1", 1, "profile-1", "bit-profile-1", "interaction", "authorized", now, now))

	task, found, err := findAuthorizedTask(db, "task-1")
	if err != nil || !found {
		t.Fatalf("findAuthorizedTask() = %v, %v, want found without error", found, err)
	}
	if task.ID != "task-1" || task.Operation != model.OperationInteraction || task.Status != model.TaskAuthorized {
		t.Fatalf("task = %#v", task)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

// The permit gate proves the task still matches the user's bound main account
// and the Profile is in scope, and must not ask whether the login session that
// started it is still alive: renew and finish are report-safe, and a replaced
// Desktop must not revoke a sensitive operation that is already running. The
// COUNT text is pinned whole, so re-adding a session JOIN or `invalidated_at`
// fails this arm.
func TestAcquirePermitScopesToTheBoundMainAccountButNotALiveSession(t *testing.T) {
	db, mock, closeDB := newGuardMockGORM(t)
	defer closeDB()
	at := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	task := model.SensitiveTask{
		ID: "task-1", UserID: 1, ProfileID: "profile-1", BitProfileID: "bit-profile-1",
		Operation: model.OperationInteraction, Status: model.TaskAuthorized,
	}
	permit := model.Permit{
		ID: "permit-1", TaskID: "task-1", UserID: 1, ProfileID: "profile-1",
		Operation: model.OperationInteraction, Status: model.PermitActive,
		AcquiredAt: at, ExpiresAt: at.Add(10 * time.Minute),
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM browser_profiles\s+WHERE id = \? FOR UPDATE`).
		WithArgs("profile-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("profile-1"))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sensitive_browser_tasks t\s+JOIN browser_profiles bp ON bp\.id = t\.profile_id\s+JOIN users u ON u\.id = t\.user_id\s+WHERE t\.id = \? AND t\.user_id = \? AND t\.profile_id = \? AND t\.operation = \?\s+AND t\.bit_profile_id = \? AND t\.status = 'authorized' AND bp\.local_status = 'active'\s+AND bp\.user_id = u\.id\s+AND bp\.bit_profile_id = t\.bit_profile_id AND bp\.main_user_id = u\.bit_main_user_id\s+AND u\.status = 'enabled'`).
		WithArgs("task-1", int64(1), "profile-1", "interaction", "bit-profile-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id, status, expires_at FROM sensitive_profile_permits\s+WHERE profile_id = \? ORDER BY acquired_at DESC LIMIT 1`).
		WithArgs("profile-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "expires_at"}))
	mock.ExpectExec(`INSERT INTO sensitive_profile_permits`).
		WithArgs("permit-1", "task-1", int64(1), "profile-1", "interaction", "active", at, at.Add(10*time.Minute), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE sensitive_browser_tasks SET status = \?, updated_at = \?\s+WHERE id = \? AND status = \?`).
		WithArgs("running", at, "task-1", "authorized").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	outcome, err := acquirePermit(db, task, permit, at)
	if err != nil {
		t.Fatalf("acquirePermit() error = %v", err)
	}
	if outcome.Outcome != model.OutcomeGranted || outcome.PermitID != "permit-1" {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func newGuardMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func containsString(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
