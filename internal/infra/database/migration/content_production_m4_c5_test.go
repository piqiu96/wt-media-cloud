package migration

import (
	"strings"
	"testing"
)

// CHG-20261013-083 (M4-C5): composite_outputs is the verified-output asset row.
// It is only ever written inside the same transaction that flips the task to
// success (spec §6.3), so every source reference points at facts that already
// exist: the task row carries the immutable snapshot, and the output carries
// the storage facts measured after upload.
func TestContentProductionM4C5CompositeOutputsMigration(t *testing.T) {
	sql := readMigration(t, "20261014_054_composite_outputs.sql")
	for _, fragment := range []string{
		"CREATE TABLE composite_outputs",
		"compose_task_id BIGINT UNSIGNED NOT NULL",
		"UNIQUE KEY uq_composite_outputs_task (compose_task_id)",
		"UNIQUE KEY uq_composite_outputs_object_key (object_key)",
		"material_id BIGINT UNSIGNED NOT NULL",
		"material_usage_id BIGINT UNSIGNED NOT NULL",
		"compose_strategy_id BIGINT UNSIGNED NOT NULL",
		"template_code VARCHAR(64) NOT NULL",
		"template_version INT UNSIGNED NOT NULL",
		"file_size_bytes BIGINT UNSIGNED NOT NULL",
		"sha256 CHAR(64) NOT NULL",
		"media_json JSON NOT NULL",
		"publish_status VARCHAR(16) NOT NULL DEFAULT 'pending_publish'",
		"CONSTRAINT chk_composite_outputs_publish CHECK (publish_status IN ('pending_publish', 'published', 'archived'))",
		"CONSTRAINT fk_composite_outputs_task FOREIGN KEY (compose_task_id) REFERENCES compose_tasks(id)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("054 missing %q", fragment)
		}
	}
	// Nothing may seed an output outside the worker's commit transaction.
	if strings.Contains(sql, "INSERT INTO composite_outputs") {
		t.Errorf("054 includes out-of-scope INSERT INTO composite_outputs")
	}
}

// The running-cancel protocol rides one flag on the task row (CHG083 §7: no
// second table), so 054 alters compose_tasks to carry the worker-visible
// cancel request.
func TestContentProductionM4C5ComposeTasksCancelRequested(t *testing.T) {
	sql := readMigration(t, "20261014_054_composite_outputs.sql")
	for _, fragment := range []string{
		"ALTER TABLE compose_tasks",
		"cancel_requested TINYINT(1) NOT NULL DEFAULT 0",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("054 missing %q", fragment)
		}
	}
}
