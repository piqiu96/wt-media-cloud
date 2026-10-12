package migration

import (
	"strings"
	"testing"
)

// CHG-20261013-082: compose_task is the formal production task row. The
// snapshot columns are immutable by convention (ADR-0021) — once written at
// creation they are never updated by later strategy edits, so the migration
// carries them as plain columns and the executor reads only them.
func TestContentProductionM4C3ComposeTasksMigration(t *testing.T) {
	sql := readMigration(t, "20261013_053_compose_tasks.sql")
	for _, fragment := range []string{
		"CREATE TABLE compose_tasks",
		"trigger_type VARCHAR(16) NOT NULL DEFAULT 'manual'",
		"material_id BIGINT UNSIGNED NOT NULL",
		"material_usage_id BIGINT UNSIGNED NOT NULL",
		"compose_strategy_id BIGINT UNSIGNED NOT NULL",
		"template_code VARCHAR(64) NOT NULL",
		"template_version INT UNSIGNED NOT NULL",
		"config_snapshot_json JSON NOT NULL",
		"status VARCHAR(16) NOT NULL DEFAULT 'pending'",
		"UNIQUE KEY uq_compose_tasks_request_key (request_key)",
		"lease_expires_at DATETIME(6) NULL",
		"heartbeat_at DATETIME(6) NULL",
		"attempt_count INT UNSIGNED NOT NULL DEFAULT 0",
		"stage VARCHAR(32) NOT NULL DEFAULT ''",
		"composite_output_id BIGINT UNSIGNED NULL",
		"CONSTRAINT fk_compose_tasks_team FOREIGN KEY (team_id) REFERENCES operation_teams(id)",
		"CONSTRAINT fk_compose_tasks_game FOREIGN KEY (game_id) REFERENCES operation_games(id)",
		"CONSTRAINT fk_compose_tasks_material FOREIGN KEY (material_id) REFERENCES materials(id)",
		"CONSTRAINT fk_compose_tasks_usage FOREIGN KEY (material_usage_id) REFERENCES material_usages(id)",
		"CONSTRAINT fk_compose_tasks_strategy FOREIGN KEY (compose_strategy_id) REFERENCES compose_strategies(id)",
		"CONSTRAINT chk_compose_tasks_status CHECK (status IN ('pending', 'running', 'success', 'failed', 'cancelled'))",
		"CONSTRAINT chk_compose_tasks_trigger CHECK (trigger_type IN ('manual', 'scheduled'))",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("053 missing %q", fragment)
		}
	}
	// C3 creates and cancels only; the executor lifecycle columns exist but
	// nothing in this migration may pre-populate an execution fact.
	for _, forbidden := range []string{"INSERT INTO compose_tasks", "UPDATE compose_tasks", "composite_outputs"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("053 includes out-of-scope %q", forbidden)
		}
	}
}
