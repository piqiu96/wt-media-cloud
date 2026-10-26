package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 顺序是语义的一部分：先回填归属与 attempt，再终结无归属行，最后换唯一键。
func TestTaskExecutionRecoveryMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20261023_070_task_execution_recovery.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)

	for _, fragment := range []string{
		"ADD COLUMN user_id BIGINT UNSIGNED NULL",
		"ADD COLUMN attempt INT NOT NULL DEFAULT 0",
		"ADD COLUMN attempt_id VARCHAR(64) NULL",
		"ADD COLUMN last_report_at DATETIME(6) NULL",
		"ADD COLUMN execution_timeout_seconds INT NOT NULL DEFAULT 300",
		"ADD COLUMN external_side_effect TINYINT(1) NOT NULL DEFAULT 0",
		"ADD COLUMN retryable TINYINT(1) NOT NULL DEFAULT 0",
		"ADD COLUMN recovery_action VARCHAR(32) NULL",
		"ADD COLUMN settled_by VARCHAR(16) NULL",
		"DROP INDEX uq_idempotency_key",
		"ADD UNIQUE KEY uq_tasks_user_idempotency (user_id, idempotency_key)",
		"ADD KEY idx_tasks_user_status_created (user_id, status, created_at)",
		"ADD KEY idx_tasks_status_last_report (status, last_report_at)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	if strings.Contains(sql, "CREATE TABLE") {
		t.Fatal("task recovery must not create tables")
	}
	if strings.Contains(sql, "node_id") || strings.Contains(sql, "device_id") {
		t.Fatal("task runtime must not reintroduce node or device columns")
	}

	backfillAt := strings.Index(sql, "UPDATE tasks t\n    JOIN media_accounts")
	terminateAt := strings.Index(sql, "error_code = 'EXECUTION_INTERRUPTED'")
	keyAt := strings.Index(sql, "DROP INDEX uq_idempotency_key")
	if backfillAt < 0 || terminateAt < 0 || keyAt < 0 {
		t.Fatalf("migration is missing backfill=%d terminate=%d keys=%d", backfillAt, terminateAt, keyAt)
	}
	if !(backfillAt < terminateAt && terminateAt < keyAt) {
		t.Fatalf("order must be backfill -> terminate -> keys, got %d -> %d -> %d", backfillAt, terminateAt, keyAt)
	}
}
