package migration

import (
	"strings"
	"testing"
)

// CHG-20261026-105: scheduled tasks come from the strategy team's material
// pool, so there is no personal material_usage row to reference. Both the task
// and its verified output keep the column nullable; the foreign keys stay.
func TestContentProductionM4JComposeUsageNullable(t *testing.T) {
	sql := readMigration(t, "20261027_075_compose_optional_usage.sql")
	for _, fragment := range []string{
		"ALTER TABLE compose_tasks",
		"ALTER TABLE composite_outputs",
		"MODIFY COLUMN material_usage_id BIGINT UNSIGNED NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("075 missing %q", fragment)
		}
	}
	if got := strings.Count(sql, "MODIFY COLUMN material_usage_id BIGINT UNSIGNED NULL"); got != 2 {
		t.Errorf("075 must loosen both tables, got %d column changes", got)
	}
	if strings.Contains(sql, "NOT NULL") {
		t.Error("075 must not reintroduce NOT NULL")
	}
	for _, forbidden := range []string{"DROP FOREIGN KEY", "DROP COLUMN", "UPDATE ", "INSERT INTO"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("075 includes out-of-scope %q", forbidden)
		}
	}
}
