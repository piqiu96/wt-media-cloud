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

func TestProfileGuardMigrationUsesDurableTasksAndHashedPermits(t *testing.T) {
	content, err := osReadFile("../../../../migrations/20260714_005_sensitive_profile_locks.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, required := range []string{"CREATE TABLE sensitive_browser_tasks", "CREATE TABLE sensitive_profile_permits", "credential_hash CHAR(64) NOT NULL"} {
		if !containsString(string(content), required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestFindAuthorizedTaskUsesGORM(t *testing.T) {
	db, mock, closeDB := newGuardMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, profile_id, bit_profile_id, node_id, operation, status")).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "profile_id", "bit_profile_id", "node_id", "operation", "status", "created_at", "updated_at"}).
			AddRow("task-1", 1, "profile-1", "bit-profile-1", "node-1", "interaction", "authorized", now, now))

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
