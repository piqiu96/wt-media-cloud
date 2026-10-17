package migration

import (
	"strings"
	"testing"
)

// The (strategy, material) pair is read by the create drawer's aggregate, by
// the create guard — a locking read, so it locks the range it scans — and by
// the scheduler's per-candidate check. This migration adds the key all three
// want; the negative assertion below is the load-bearing one, because a UNIQUE
// key here would silently outlaw the second version of a pair that 再生成一版
// exists to produce.
func TestComposeTaskStrategyMaterialIndexMigration(t *testing.T) {
	sql := stripSQLComments(readMigration(t, "20261016_065_compose_task_strategy_material_index.sql"))
	if !strings.Contains(sql, "ALTER TABLE compose_tasks") {
		t.Fatalf("065 must alter compose_tasks:\n%s", sql)
	}
	if !strings.Contains(sql, "ADD KEY idx_compose_tasks_strategy_material (compose_strategy_id, material_id)") {
		t.Fatalf("065 does not declare the pair key:\n%s", sql)
	}
	if strings.Contains(strings.ToUpper(sql), "UNIQUE") {
		t.Fatalf("065 must not make the pair unique:\n%s", sql)
	}
	// The pair key only helps the three reads above if it is the only change:
	// a second statement here would need its own reason.
	if strings.Count(sql, ";") != 1 {
		t.Fatalf("065 must hold exactly one statement:\n%s", sql)
	}
}

// stripSQLComments drops `--` line comments before the assertions run. The
// migration's own comment says what it is NOT allowed to be ("deliberately not
// UNIQUE"), and scanning the raw text would read that sentence as the
// violation it is warning about.
func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if index := strings.Index(line, "--"); index >= 0 {
			line = line[:index]
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
