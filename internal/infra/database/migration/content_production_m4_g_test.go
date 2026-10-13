package migration

import (
	"strings"
	"testing"
)

// CHG-20261013-089 (M4-G): per-attempt execution records. The unique key
// pins one row per (task, attempt) so a retry can never open a second row
// for the same attempt, and the worker's claim INSERT relies on it.
func TestContentProductionM4GComposeTaskAttemptsMigration(t *testing.T) {
	sql := readMigration(t, "20261014_056_compose_task_attempts.sql")
	for _, fragment := range []string{
		"CREATE TABLE compose_task_attempts",
		"attempt INT UNSIGNED NOT NULL",
		"source VARCHAR(16) NOT NULL DEFAULT 'auto'",
		"started_at DATETIME(6) NULL",
		"finished_at DATETIME(6) NULL",
		"outcome VARCHAR(16) NOT NULL DEFAULT 'pending'",
		"stage VARCHAR(32) NOT NULL DEFAULT ''",
		"error_code VARCHAR(64) NOT NULL DEFAULT ''",
		"error_message VARCHAR(512) NOT NULL DEFAULT ''",
		"UNIQUE KEY uq_compose_task_attempts_task_attempt (task_id, attempt)",
		"CONSTRAINT fk_compose_task_attempts_task FOREIGN KEY (task_id) REFERENCES compose_tasks(id)",
		"CONSTRAINT chk_compose_task_attempts_source CHECK (source IN ('auto', 'manual'))",
		"CONSTRAINT chk_compose_task_attempts_outcome CHECK (outcome IN ('pending', 'running', 'success', 'failed', 'cancelled'))",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("056 missing %q", fragment)
		}
	}
}
