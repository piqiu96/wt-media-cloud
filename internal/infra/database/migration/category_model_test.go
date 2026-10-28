package migration

import (
	"strings"
	"testing"
)

func TestCategoryModelMigrationPreservesEveryLegacyReference(t *testing.T) {
	sql := readMigration(t, "20261029_078_category_model.sql")
	for _, fragment := range []string{
		"CREATE TABLE categories",
		"id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT",
		"type VARCHAR(32) NOT NULL",
		"enabled TINYINT(1) NOT NULL DEFAULT 1",
		"legacy_id VARCHAR(32) NULL",
		"SELECT 'game', name, status = 'enabled', remark, id, created_at, updated_at",
		"ALTER TABLE compose_strategies ADD COLUMN category_id",
		"ALTER TABLE compose_tasks ADD COLUMN category_id",
		"ALTER TABLE composite_outputs ADD COLUMN category_id",
		"ALTER TABLE discovery_strategies ADD COLUMN category_id",
		"ALTER TABLE source_contents ADD COLUMN category_id",
		"ALTER TABLE user_game_scopes ADD COLUMN category_id",
		"ALTER TABLE media_account_games ADD COLUMN category_id",
		"RENAME TABLE user_game_scopes TO user_category_scopes",
		"RENAME TABLE media_account_games TO media_account_categories",
		"DROP TABLE operation_games",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("category migration missing %q", fragment)
		}
	}
	if strings.Contains(strings.ToLower(sql), "'other'") {
		t.Fatal("category migration must not assume that an `other` category exists")
	}
	if got := strings.Count(sql, "ADD COLUMN category_id BIGINT UNSIGNED NULL"); got != 7 {
		t.Fatalf("category migration backfills %d dependent columns, want 7", got)
	}
}
