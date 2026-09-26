package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The exact shape migration 039 shipped with, reduced: a header comment whose
// prose contains a semicolon, followed by the statements it describes.
//
// This is not hypothetical. It is what made 039 unappliable, and the server's
// answer — `Error 1064 ... near 'no -- worker, object-storage connection...'` —
// points at the SQL rather than at the comment the splitter cut in half, which
// is why it survived review and unit tests in the first place.
func TestSplitStatementsIgnoresSemicolonsInsideComments(t *testing.T) {
	sql := "-- M4-A production facts. Source materialization remains metadata-only; no\n" +
		"-- worker, object-storage connection, local path, or signed URL is introduced.\n" +
		"ALTER TABLE materials ADD COLUMN video_status VARCHAR(32) NOT NULL DEFAULT 'not_downloaded';\n" +
		"ALTER TABLE materials ADD COLUMN source_object_key VARCHAR(512) NULL;\n"

	statements := splitStatements(sql)
	want := []string{
		"ALTER TABLE materials ADD COLUMN video_status VARCHAR(32) NOT NULL DEFAULT 'not_downloaded'",
		"ALTER TABLE materials ADD COLUMN source_object_key VARCHAR(512) NULL",
	}
	if len(statements) != len(want) {
		t.Fatalf("splitStatements() = %#v, want %d statements", statements, len(want))
	}
	for i := range want {
		if statements[i] != want[i] {
			t.Errorf("statement %d = %q, want %q", i, statements[i], want[i])
		}
	}
}

// A semicolon inside a string literal is data, not a terminator. The naive split
// this replaces would cut the value in half and send two broken statements.
func TestSplitStatementsKeepsSemicolonsInsideStringLiterals(t *testing.T) {
	sql := "INSERT INTO notes (body) VALUES ('one; two');\nINSERT INTO notes (body) VALUES ('three');"

	statements := splitStatements(sql)
	if len(statements) != 2 {
		t.Fatalf("splitStatements() = %#v, want 2 statements", statements)
	}
	if !strings.Contains(statements[0], "'one; two'") {
		t.Errorf("statement 0 = %q, want the literal kept whole", statements[0])
	}
	if statements[1] != "INSERT INTO notes (body) VALUES ('three')" {
		t.Errorf("statement 1 = %q", statements[1])
	}
}

// A doubled quote is an escaped quote inside the literal, not its end, so a
// semicolon after it is still part of the value.
func TestSplitStatementsHandlesEscapedQuotes(t *testing.T) {
	sql := "INSERT INTO notes (body) VALUES ('it''s; here');"

	statements := splitStatements(sql)
	if len(statements) != 1 {
		t.Fatalf("splitStatements() = %#v, want 1 statement", statements)
	}
	if !strings.Contains(statements[0], "'it''s; here'") {
		t.Errorf("statement = %q, want the escaped literal kept whole", statements[0])
	}
}

// MySQL rejects an empty query, so a fragment that is nothing but comment text
// must not be handed to the server at all.
func TestSplitStatementsDropsCommentOnlyFragments(t *testing.T) {
	for name, sql := range map[string]string{
		"leading comment with semicolon": "-- a comment; with a semicolon\n",
		"bare comment":                   "-- nothing else here\n",
		"comment then blank lines":       "-- leading\n\n\n",
		"empty file":                     "",
		"only whitespace and separators": " \n;\n\t;\n",
	} {
		if statements := splitStatements(sql); len(statements) != 0 {
			t.Errorf("%s: splitStatements() = %#v, want no statements", name, statements)
		}
	}
}

// The behaviour the pre-existing runner tests depend on, kept as a regression
// guard while the splitter learns about comments.
func TestSplitStatementsStillSplitsOrdinaryStatements(t *testing.T) {
	sql := "CREATE TABLE second (id INT);\nCREATE INDEX idx_second_id ON second (id);"

	statements := splitStatements(sql)
	if len(statements) != 2 {
		t.Fatalf("splitStatements() = %#v, want 2 statements", statements)
	}
	if statements[0] != "CREATE TABLE second (id INT)" {
		t.Errorf("statement 0 = %q", statements[0])
	}
	if statements[1] != "CREATE INDEX idx_second_id ON second (id)" {
		t.Errorf("statement 1 = %q", statements[1])
	}
}

// A trailing comment after a statement's terminator must not glue itself onto
// the next statement, and must not become a statement of its own.
func TestSplitStatementsHandlesTrailingComments(t *testing.T) {
	sql := "CREATE TABLE a (id INT); -- why a exists\nCREATE TABLE b (id INT);\n-- trailing note\n"

	statements := splitStatements(sql)
	if len(statements) != 2 {
		t.Fatalf("splitStatements() = %#v, want 2 statements", statements)
	}
	for i, statement := range statements {
		if strings.Contains(statement, "--") {
			t.Errorf("statement %d leaked comment text: %q", i, statement)
		}
	}
}

// The sweep that would have caught 039 before it was committed.
//
// Every migration in the repository must split into statements that carry no
// comment text at all. This reads the real files rather than fixtures, because
// the defect was in a real file that no fixture resembled.
func TestEveryRepositoryMigrationSplitsIntoStatementsWithoutCommentText(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "migrations")
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no migrations found; this sweep would pass vacuously")
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		name := filepath.Base(path)
		for i, statement := range splitStatements(string(content)) {
			if strings.TrimSpace(statement) == "" {
				t.Errorf("%s statement %d is blank", name, i)
				continue
			}
			// A leading `--` is the reported failure: the comment's tail became
			// a statement. Checking every line catches a fragment that starts
			// mid-comment.
			for _, line := range strings.Split(statement, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "--") {
					t.Errorf("%s statement %d carries comment text: %q", name, i, statement)
					break
				}
			}
		}
	}
}
