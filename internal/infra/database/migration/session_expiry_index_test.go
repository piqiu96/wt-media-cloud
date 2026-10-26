package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration 072 gives the expiry sweep an index it can range over. Deleting
// the rows themselves is the job's business, not the migration's.
func TestUserSessionsExpiresAtIndexMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20261023_072_user_sessions_expires_at_index.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)

	requiredFragments := []string{
		"ALTER TABLE user_sessions",
		"ADD KEY idx_user_sessions_expires_at (expires_at)",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	for _, forbidden := range []string{"DELETE FROM user_sessions", "DROP ", "CREATE TABLE", "MODIFY COLUMN", "ADD COLUMN"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("migration must only add the index, found %q", forbidden)
		}
	}
}
