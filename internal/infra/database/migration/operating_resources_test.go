package migration

import (
	"strings"
	"testing"
)

func TestOperatingResourcesMigrationConvergesTheSevenTables(t *testing.T) {
	sql := readMigration(t, "20261029_079_operating_resources.sql")
	for _, fragment := range []string{
		"RENAME TABLE proxy_configs TO proxies",
		"RENAME TABLE account_groups TO media_account_groups",
		"CREATE TABLE browser_profile_syncs",
		"CHANGE COLUMN business_status enabled TINYINT(1)",
		"CHANGE COLUMN main_user_id bit_main_user_id",
		"CHANGE COLUMN profile_user_id bit_user_id",
		"ADD COLUMN reserved_proxy_id VARCHAR(64) NULL",
		"ADD COLUMN reserved_since DATETIME(6) NULL",
		"PRIMARY KEY (media_account_id, tag_name)",
		"DROP TABLE profile_sync_candidates",
		"DROP TABLE profile_sync_scans",
		"DROP TABLE IF EXISTS tasks",
		"DROP TABLE IF EXISTS sensitive_browser_tasks",
		"DROP TABLE IF EXISTS sensitive_profile_permits",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("operating-resource migration missing %q", fragment)
		}
	}
	for _, forbidden := range []string{"CREATE TABLE proxy_operations", "CREATE TABLE last_check_history", "CREATE TABLE sensitive_tasks"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("operating-resource migration must not introduce %q", forbidden)
		}
	}
}
