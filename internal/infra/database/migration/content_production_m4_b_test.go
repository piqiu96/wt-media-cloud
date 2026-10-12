package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentProductionM4BMigrationKeepsStrategyVersions(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20261012_049_compose_strategies.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read strategy migration: %v", err)
	}
	sql := string(content)
	for _, fragment := range []string{
		"CREATE TABLE compose_strategies",
		"CREATE TABLE compose_strategy_versions",
		"UNIQUE KEY uq_compose_strategy_version (strategy_id, version)",
		"FOREIGN KEY (strategy_id) REFERENCES compose_strategies(id)",
		"FOREIGN KEY (team_id) REFERENCES operation_teams(id)",
		"FOREIGN KEY (created_by) REFERENCES users(id)",
		"FOREIGN KEY (game_id) REFERENCES operation_games(id)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
	for _, forbidden := range []string{"compose_task", "schedule", "local_path", "ffmpeg"} {
		if strings.Contains(strings.ToLower(sql), forbidden) {
			t.Errorf("migration includes out-of-scope %q", forbidden)
		}
	}
}
