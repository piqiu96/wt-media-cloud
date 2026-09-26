package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentProductionM4AMigrationDefinesSingleUsageAndTransferTaskFacts(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20260926_039_content_production_m4_a.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read M4-A migration: %v", err)
	}
	sql := string(content)
	for _, fragment := range []string{
		"ALTER TABLE materials ADD COLUMN video_status",
		"CREATE TABLE material_usages",
		"UNIQUE KEY uq_material_usages_user_material (user_id, material_id)",
		"CREATE TABLE file_transfer_tasks",
		"UNIQUE KEY uq_file_transfer_tasks_dedupe (dedupe_key)",
		"CONSTRAINT fk_file_transfer_tasks_dependency FOREIGN KEY (dependency_task_id) REFERENCES file_transfer_tasks(id)",
		"CHECK (execution_scope IN ('cloud', 'local_agent'))",
		"CHECK (status IN ('pending', 'running', 'success', 'failed', 'cancelled'))",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
	if strings.Contains(sql, "local_path") || strings.Contains(sql, "signed_url") || strings.Contains(sql, "storage_credential") {
		t.Fatal("Cloud migration must not persist a local path, signed URL, or storage credential")
	}
	if strings.Contains(sql, "FOREIGN KEY (asset_id) REFERENCES materials(id)") {
		t.Fatal("the executor-neutral transfer table must not have a production-domain asset foreign key")
	}
}
