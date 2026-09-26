package migration

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func readExecutorFactsMigration(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "migrations", "20260926_041_file_transfer_task_executor_facts.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transfer executor facts migration: %v", err)
	}
	return sqlWithoutComments(string(content))
}

var addedColumnPattern = regexp.MustCompile(`ADD COLUMN (\w+)`)

// The task row has to be self-contained for an executor that runs later, on
// another machine: `filetransfer` may not import `production`, so everything the
// lease needs must already be on the task.
//
// The column set is asserted as an exact set rather than as a list of
// `contains` claims. A whitelist is what makes "no persisted address or secret
// is added" a real statement: `signed_url`, `download_url`, `local_path` and
// `storage_credential` all fail it by name, and so would a column nobody has
// thought of yet. Listing forbidden names would only catch the ones enumerated.
func TestExecutorFactsMigrationAddsExactlyTheColumnsAnExecutorNeeds(t *testing.T) {
	sql := readExecutorFactsMigration(t)

	added := addedColumnPattern.FindAllStringSubmatch(sql, -1)
	if len(added) == 0 {
		t.Fatal("no ADD COLUMN found; this check would pass vacuously")
	}
	names := make([]string, 0, len(added))
	for _, match := range added {
		names = append(names, match[1])
	}
	want := []string{"asset_title", "source_object_key", "file_name"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("added columns = %v, want exactly %v", names, want)
	}

	for _, fragment := range []string{
		"ALTER TABLE file_transfer_tasks ADD COLUMN asset_title VARCHAR(255) NULL",
		"ALTER TABLE file_transfer_tasks ADD COLUMN source_object_key VARCHAR(512) NULL",
		"ALTER TABLE file_transfer_tasks ADD COLUMN file_name VARCHAR(255) NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	// The transferred name is a name, not a path: the executor reports it and the
	// UI renders it, so a path separator would put a location the Cloud must not
	// hold — a user's home directory — into a business table.
	if regexp.MustCompile(`(?i)ADD COLUMN\s+\w*(path|url|uri|credential|secret|token)\w*`).MatchString(sql) {
		t.Error("an executor fact must not be an address or a secret")
	}
}

// Same prohibition the other transfer migrations carry, asserted against
// executable statements rather than prose so the header comment explaining it
// cannot trip the check.
func TestExecutorFactsMigrationDoesNotPersistAnAddressOrSecret(t *testing.T) {
	sql := readExecutorFactsMigration(t)
	for _, forbidden := range []string{"local_path", "signed_url", "download_url", "storage_credential", "node_credential", "presign"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("Cloud must not persist %q", forbidden)
		}
	}
}

// Purely additive, and confined to the transfer table. 039 created it and 040
// added to it; a `CREATE TABLE` or a `DROP` here would either collide with them
// on a fresh database or destroy an executor's state on an existing one.
func TestExecutorFactsMigrationStaysAdditiveAndOnTheTransferTable(t *testing.T) {
	sql := readExecutorFactsMigration(t)
	if regexp.MustCompile(`\bCREATE\s+TABLE\b`).MatchString(sql) {
		t.Error("this migration must only alter the table 039 created")
	}
	if regexp.MustCompile(`\b(DROP|TRUNCATE|DELETE|UPDATE|MODIFY)\b`).MatchString(sql) {
		t.Error("an additive migration must not drop, rewrite, or modify existing rows or columns")
	}
	for _, statement := range strings.Split(sql, ";") {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "ALTER TABLE file_transfer_tasks") {
			t.Errorf("unexpected statement in the executor facts migration: %q", trimmed)
		}
	}
}
