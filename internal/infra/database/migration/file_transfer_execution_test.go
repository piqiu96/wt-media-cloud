package migration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sqlWithoutComments removes `--` line comments so the assertions below read
// statements rather than prose.
//
// Without this, a comment *explaining* that no credential is persisted would
// contain the very token the check forbids and fail the build — and the fix
// someone would reach for is to delete the explanation. The forbidden-token
// check is about what the migration does, so it has to read only what executes.
func sqlWithoutComments(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if index := strings.Index(line, "--"); index >= 0 {
			line = line[:index]
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func readTransferExecutionMigration(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "migrations", "20260926_040_file_transfer_task_execution.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transfer execution migration: %v", err)
	}
	return sqlWithoutComments(string(content))
}

// Migration 039 is committed and possibly applied elsewhere, so the columns the
// executor needs arrive in their own migration. This pins that they arrive at
// all, on the transfer table, with the types the repository's predicates assume
// (`CHAR(64)` is what the 64-character SHA-256 check in `completeTask` compares
// against).
func TestFileTransferExecutionMigrationAddsCancellationAndIntegrityExpectation(t *testing.T) {
	sql := readTransferExecutionMigration(t)
	for _, fragment := range []string{
		"ALTER TABLE file_transfer_tasks ADD COLUMN cancel_requested_at DATETIME(6) NULL",
		"ALTER TABLE file_transfer_tasks ADD COLUMN expected_sha256 CHAR(64) NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	// The same prohibition 039 carries. A signed URL or a durable storage
	// credential here would survive every later "we only keep a grant" claim,
	// because this is the table the executor reads.
	for _, forbidden := range []string{"local_path", "signed_url", "storage_credential", "node_credential", "download_url"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("Cloud migration must not persist %q", forbidden)
		}
	}
}

// The columns belong to the transfer table, and this migration must stay purely
// additive.
//
// The `CREATE TABLE` half is the load-bearing one: if 040 redefined
// `file_transfer_tasks` it would collide with 039 on a fresh database, and on a
// database that had applied 039 it would either fail or diverge. The point of the
// split is that neither file has to know whether the other ran.
func TestFileTransferExecutionMigrationDoesNotRedefineTheTransferTable(t *testing.T) {
	sql := readTransferExecutionMigration(t)
	if regexp.MustCompile(`\bCREATE\s+TABLE\b`).MatchString(sql) {
		t.Error("this migration must only alter the table 039 created")
	}
	if regexp.MustCompile(`\b(DROP|TRUNCATE)\b`).MatchString(sql) {
		t.Error("an additive migration must not drop or truncate anything")
	}
	// Naming the table is fine; touching any other table is not — the executor's
	// cancellation and integrity columns live on the transfer task and nowhere
	// else, and an `ALTER TABLE materials` here would be the production domain
	// creeping into the executor's migration.
	for _, statement := range strings.Split(sql, ";") {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "ALTER TABLE file_transfer_tasks") {
			t.Errorf("unexpected statement in the transfer execution migration: %q", trimmed)
		}
	}
}
