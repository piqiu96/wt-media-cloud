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

// 许可只是 Profile 上的互斥锁，不是凭证：调用方由 Agent Token 认证，
// 许可行只保证同一 Profile 同时只有一个敏感操作。071 因此删掉初版需要的 node 与凭证列。
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

// 许可只做 Profile 互斥：环境与账号检查在统一准入里，这里只锁 Profile、
// 看前一个许可，然后把任务从 authorized 推到 running。多加一条检查会让两处
// 判据各说各话，所以 SQL 断言整段固定。
func TestAcquirePermitIsOnlyTheProfileMutex(t *testing.T) {
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
