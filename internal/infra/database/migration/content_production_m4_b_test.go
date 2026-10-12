package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMigration(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "migrations", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(content)
}

func TestContentProductionM4BMigrationsFollowTheFourLayerRework(t *testing.T) {
	original := readMigration(t, "20261012_049_compose_strategies.sql")
	for _, fragment := range []string{
		"CREATE TABLE compose_strategies",
		"CREATE TABLE compose_strategy_versions",
		"FOREIGN KEY (team_id) REFERENCES operation_teams(id)",
		"FOREIGN KEY (game_id) REFERENCES operation_games(id)",
	} {
		if !strings.Contains(original, fragment) {
			t.Errorf("049 missing %q", fragment)
		}
	}

	rework := readMigration(t, "20261012_050_compose_templates.sql")
	for _, fragment := range []string{
		"CREATE TABLE compose_templates",
		"UNIQUE KEY uq_compose_templates_code_version (code, version)",
		"schema_json JSON NOT NULL",
		"executor VARCHAR(64) NOT NULL",
		"ADD COLUMN compose_template_id BIGINT UNSIGNED NOT NULL",
		"ADD COLUMN compose_template_version INT UNSIGNED NOT NULL",
		"FOREIGN KEY (compose_template_id) REFERENCES compose_templates(id)",
		"DROP TABLE compose_strategy_versions",
		"DROP COLUMN current_version",
		"DROP COLUMN template_key",
		"DROP COLUMN template_version",
	} {
		if !strings.Contains(rework, fragment) {
			t.Errorf("050 missing %q", fragment)
		}
	}
	if strings.Contains(rework, "INSERT INTO compose_templates") {
		t.Error("050 must not publish a template before its executor is available")
	}
	for _, forbidden := range []string{"compose_task", "schedule", "local_path", "ffmpeg"} {
		if strings.Contains(strings.ToLower(rework), forbidden) {
			t.Errorf("050 includes out-of-scope %q", forbidden)
		}
	}
}

func TestComposeStrategyConfigurationHasOneSchemaOwnedSource(t *testing.T) {
	correction := readMigration(t, "20261012_051_compose_strategy_config.sql")
	for _, fragment := range []string{
		"RENAME COLUMN parameters_json TO config_json",
		"DROP COLUMN allowed_overrides_json",
	} {
		if !strings.Contains(correction, fragment) {
			t.Errorf("051 missing %q", fragment)
		}
	}
}
