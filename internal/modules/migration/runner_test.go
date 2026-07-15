package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadDirSortsSQLMigrations(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "20260714_002_second.sql"), "CREATE TABLE second (id INT);")
	writeFile(t, filepath.Join(dir, "20260714_001_first.sql"), "CREATE TABLE first (id INT);")
	writeFile(t, filepath.Join(dir, "README.md"), "ignored")

	migrations, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("LoadDir() got %d migrations, want 2", len(migrations))
	}
	if migrations[0].Version != "20260714_001_first" || migrations[1].Version != "20260714_002_second" {
		t.Fatalf("LoadDir() order = %#v", migrations)
	}
}

func TestApplyRunsOnlyPendingMigrationStatements(t *testing.T) {
	db, mock, closeDB := newMockDB(t)
	defer closeDB()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("20260714_001_first"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE second (id INT)")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE INDEX idx_second_id ON second (id)")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations")).
		WithArgs("20260714_002_second", "second").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	applied, err := Apply(context.Background(), db, []Migration{
		{Version: "20260714_001_first", Name: "first", SQL: "CREATE TABLE first (id INT);"},
		{Version: "20260714_002_second", Name: "second", SQL: "CREATE TABLE second (id INT);\nCREATE INDEX idx_second_id ON second (id);"},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(applied) != 1 || applied[0].Version != "20260714_002_second" {
		t.Fatalf("Apply() applied = %#v, want second only", applied)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return db, mock, func() {
		_ = db.Close()
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
