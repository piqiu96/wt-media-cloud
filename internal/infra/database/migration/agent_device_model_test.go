package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration 071 removes the fixed-device model: the per-user device identity,
// the registered node, its presence and binding tables, and the node columns
// the sensitive-operation tables carried. The Agent authenticates with a
// session-bound token now, so none of this state has an owner.
func TestRemoveAgentDeviceModelMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "20261023_071_remove_agent_device_model.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)

	requiredFragments := []string{
		"DROP FOREIGN KEY fk_sensitive_profile_permit_node",
		"DROP INDEX uq_sensitive_profile_permit_credential_hash",
		"DROP FOREIGN KEY fk_sensitive_browser_task_node",
		"DROP INDEX idx_sensitive_browser_tasks_node_status",
		"DROP TABLE browser_profile_runtime_presence",
		"DROP TABLE local_agent_binding_tickets",
		"DROP TABLE agent_nodes",
		"DROP TABLE local_agent_nodes",
		"DROP COLUMN device_public_key",
		"DROP COLUMN device_name",
		"DROP COLUMN device_bound_at",
		"DROP COLUMN device_last_verified_at",
		"DROP COLUMN device_id",
		"DROP COLUMN assigned_node_id",
		"DROP COLUMN assigned_device_id",
		"CHANGE COLUMN claimed_by_node_id claimed_by VARCHAR(128) NULL",
		"ADD COLUMN attempt_id VARCHAR(64) NULL",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	// MySQL refuses to drop a column a foreign key still uses and refuses to
	// drop a table a child still references, so the child constraints have to
	// go first. A reordered file would fail only on a real database.
	parentDrop := strings.Index(sql, "DROP TABLE local_agent_nodes")
	for _, child := range []string{
		"DROP FOREIGN KEY fk_sensitive_profile_permit_node",
		"DROP FOREIGN KEY fk_sensitive_browser_task_node",
		"DROP TABLE browser_profile_runtime_presence",
	} {
		if at := strings.Index(sql, child); at < 0 || at > parentDrop {
			t.Errorf("%q must precede DROP TABLE local_agent_nodes", child)
		}
	}

	// The whole point is that the model leaves no replacement state behind.
	if strings.Contains(sql, "CREATE TABLE") {
		t.Fatal("removing the device model must not create tables")
	}
	usersAt := strings.Index(sql, "ALTER TABLE users")
	if usersAt < 0 {
		t.Fatal("migration does not alter users")
	}
	end := strings.Index(sql[usersAt:], ";")
	if end < 0 {
		t.Fatal("users alteration has no terminator")
	}
	if strings.Contains(sql[usersAt:usersAt+end], "ADD") {
		t.Fatal("the users table must only lose columns here")
	}
}
