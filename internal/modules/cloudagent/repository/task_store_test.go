package repository

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGetTaskForUserScansStoredTask(t *testing.T) {
	db, mock, closeDB := newMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT task_id, task_type, status")).
		WithArgs("task-1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_id", "task_type", "status", "user_id", "attempt", "attempt_id",
			"idempotency_key", "agent_id", "created_at", "started_at", "last_report_at",
			"finished_at", "lease_expires_at", "progress", "message", "updated_at",
			"error_code", "error_message", "execution_timeout_seconds", "external_side_effect",
			"retryable", "recovery_action", "settled_by", "payload_json", "result_json",
		}).AddRow(
			"task-1", "noop_task", "running", 7, 2, "attempt-2",
			"", "user-7", "2026-01-01", "2026-01-01", "2026-01-01",
			"", "", 40, "working", "2026-01-01",
			"", "", 300, true,
			true, "retry", "agent", []byte(`{"ok":true}`), nil,
		))

	task, err := getTaskForUser(db, "task-1", 7)
	if err != nil {
		t.Fatalf("getTaskForUser() error = %v", err)
	}
	if task.TaskID != "task-1" || task.TaskType != "noop_task" || task.Payload["ok"] != true {
		t.Fatalf("task = %#v", task)
	}
	if task.UserID != 7 || task.Attempt != 2 || task.AttemptID != "attempt-2" ||
		!task.ExternalSideEffect || !task.Retryable || task.RecoveryAction != "retry" ||
		task.SettledBy != "agent" || task.ExecutionTimeoutSeconds != 300 {
		t.Fatalf("task runtime fields = %#v", task)
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
