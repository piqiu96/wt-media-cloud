package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration 068 makes user_sessions the only authentication fact source:
// every session gains an expiry and a machine-readable invalidation reason,
// and pre-existing duplicate active Desktop sessions are collapsed before the
// login path starts relying on "at most one".
func TestIdentitySessionStateMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20261023_068_identity_session_state.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)

	requiredFragments := []string{
		"ADD COLUMN expires_at DATETIME(6) NULL",
		"ADD COLUMN invalidated_reason VARCHAR(32) NULL",
		"DATE_ADD(created_at, INTERVAL 30 DAY)",
		"MODIFY COLUMN expires_at DATETIME(6) NOT NULL",
		"CREATE TEMPORARY TABLE m2_f_superseded_desktop_sessions",
		"ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC)",
		"invalidated_reason = 'superseded'",
		"CREATE TEMPORARY TABLE m2_f_session_guard",
		"ADD KEY idx_user_sessions_user_client_active (user_id, client_type, invalidated_at, expires_at)",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	if strings.Contains(sql, "ALTER TABLE users") {
		t.Fatal("session state must not add authentication columns to users")
	}
	if strings.Contains(sql, "CREATE TABLE user_sessions") {
		t.Fatal("user_sessions already exists; migration 068 must only alter it")
	}
}
