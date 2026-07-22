package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserTeamModelMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "20260722_012_user_team_model.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)

	requiredFragments := []string{
		"CREATE TABLE operation_teams",
		"CREATE TEMPORARY TABLE m2_a1_preflight_guard",
		"user.role IN ('technician', 'admin')",
		"NOT EXISTS (SELECT 1 FROM user_game_scopes scopes WHERE scopes.user_id = user.id)",
		"legacy_id VARCHAR(64)",
		"BIGINT UNSIGNED NOT NULL AUTO_INCREMENT",
		"UPDATE users SET role = 'admin' WHERE role = 'technician'",
		"CHECK ((role = 'admin' AND team_id IS NULL) OR (role IN ('operator', 'senior_operator') AND team_id IS NOT NULL))",
		"ALTER TABLE media_accounts",
		"ADD COLUMN team_id BIGINT UNSIGNED",
		"ALTER TABLE browser_profiles",
		"actor_user_id IS NOT NULL AND actor_user_uid IS NULL",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
	if strings.Contains(sql, "DELETE scopes FROM user_game_scopes") {
		t.Fatal("migration must not silently delete administrator game scopes")
	}

	for _, table := range []string{
		"user_game_scopes",
		"user_sessions",
		"audit_logs",
		"media_accounts",
		"media_account_tags",
		"browser_profiles",
		"profile_sync_scans",
		"local_agent_binding_tickets",
		"local_agent_nodes",
		"browser_profile_runtime_presence",
		"sensitive_browser_tasks",
		"sensitive_profile_permits",
	} {
		if !strings.Contains(sql, "ALTER TABLE "+table) {
			t.Errorf("migration does not migrate user reference in %s", table)
		}
	}
}
