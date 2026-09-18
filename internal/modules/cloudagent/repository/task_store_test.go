package repository

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGetTaskUsesGORMAndMapsStoredTask(t *testing.T) {
	db, mock, closeDB := newMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT task_id, task_type, status")).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"task_id", "task_type", "status", "idempotency_key", "agent_id",
			"created_at", "lease_expires_at", "progress", "message", "updated_at",
			"error_code", "payload_json", "result_json",
		}).AddRow("task-1", "noop_task", "pending", "", "", "2026-01-01", "", 0, "", "", "", []byte(`{"ok":true}`), nil))

	task, err := getTask(db, "task-1")
	if err != nil {
		t.Fatalf("getTask() error = %v", err)
	}
	if task.TaskID != "task-1" || task.TaskType != "noop_task" || task.Payload["ok"] != true {
		t.Fatalf("task = %#v", task)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func newMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}
