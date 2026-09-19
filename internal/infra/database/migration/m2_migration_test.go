package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM2MigrationsUseCompatibleStringForeignKeys(t *testing.T) {
	migrationsDir := filepath.Join("..", "..", "..", "..", "migrations")
	entries, err := filepath.Glob(filepath.Join(migrationsDir, "20260714_*.sql"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected five M2 migrations, got %d", len(entries))
	}
	for _, path := range entries {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		text := string(content)
		if strings.Contains(text, "COLLATE=utf8mb4_unicode_ci") {
			t.Fatalf("%s uses utf8mb4_unicode_ci; string foreign keys must match users.id collation", filepath.Base(path))
		}
	}

	mediaMigration, err := os.ReadFile(filepath.Join(migrationsDir, "20260714_002_media_accounts.sql"))
	if err != nil {
		t.Fatalf("ReadFile(media migration) error = %v", err)
	}
	if !strings.Contains(string(mediaMigration), "browser_profile_id VARCHAR(64) NULL") {
		t.Fatal("media_accounts.browser_profile_id must match browser_profiles.id VARCHAR(64)")
	}
}
