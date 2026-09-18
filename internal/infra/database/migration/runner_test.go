package migration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestApplyRefreshesVersionsRecordedByEarlierMigration(t *testing.T) {
	db, mock, closeDB := newMockGORM(t)
	defer closeDB()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations")).
		WithArgs("000_bootstrap_existing", "existing").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).
			AddRow("000_bootstrap_existing").
			AddRow("20260714_001_identity"))

	applied, err := Apply(context.Background(), db, []Migration{
		{Version: "000_bootstrap_existing", Name: "existing", SQL: "INSERT IGNORE INTO schema_migrations (version) VALUES ('20260714_001_identity');"},
		{Version: "20260714_001_identity", Name: "identity", SQL: "CREATE TABLE users (id INT);"},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(applied) != 1 || applied[0].Version != "000_bootstrap_existing" {
		t.Fatalf("Apply() applied = %#v, want bootstrap only", applied)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestBootstrapRecordsHistoricalVersionOnlyWhenItsLegacyTableExists(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "migrations", "000_bootstrap_existing.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bootstrap migration: %v", err)
	}
	sql := string(content)

	for version, table := range map[string]string{
		"20260714_001_identity":                "users",
		"20260714_002_media_accounts":          "media_accounts",
		"20260714_003_browser_profiles":        "browser_profiles",
		"20260714_004_agent_runtime":           "local_agent_binding_tickets",
		"20260714_005_sensitive_profile_locks": "sensitive_browser_tasks",
		"20260715_006_tasks":                   "tasks",
		"20260715_007_agent_registry":          "agent_nodes",
		"20260716_008_proxy_configs":           "proxy_configs",
	} {
		versionAt := strings.Index(sql, "'"+version+"'")
		if versionAt < 0 {
			t.Fatalf("bootstrap missing version %s", version)
		}
		nextStatement := strings.Index(sql[versionAt:], ";")
		if nextStatement < 0 {
			t.Fatalf("bootstrap statement for %s has no terminator", version)
		}
		statement := sql[versionAt : versionAt+nextStatement]
		if !strings.Contains(statement, "information_schema.tables") || !strings.Contains(statement, "table_name = '"+table+"'") {
			t.Errorf("bootstrap statement for %s is not guarded by legacy table %s: %s", version, table, statement)
		}
	}
	for version, column := range map[string]string{
		"20260721_009_task_payload":         "payload_json",
		"20260721_010_profile_proxy_fields": "proxy_type",
		"20260721_011_task_result":          "result_json",
	} {
		versionAt := strings.Index(sql, "'"+version+"'")
		if versionAt < 0 {
			t.Fatalf("bootstrap missing version %s", version)
		}
		nextStatement := strings.Index(sql[versionAt:], ";")
		statement := sql[versionAt : versionAt+nextStatement]
		if !strings.Contains(statement, "information_schema.columns") || !strings.Contains(statement, "column_name = '"+column+"'") {
			t.Errorf("bootstrap statement for %s is not guarded by column %s: %s", version, column, statement)
		}
	}
}

func TestMigrationRunnerUsesGORMTransaction(t *testing.T) {
	db, mock, closeDB := newMockGORM(t)
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).
			AddRow("20260714_001_first").
			AddRow("20260714_002_second"))

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

func TestMigrationRunnerRollsBackOnSQLExecFailure(t *testing.T) {
	db, mock, closeDB := newMockGORM(t)
	defer closeDB()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE broken (id INT)")).
		WillReturnError(errors.New("statement failed"))
	mock.ExpectRollback()

	_, err := Apply(context.Background(), db, []Migration{{
		Version: "20260714_003_broken",
		Name:    "broken",
		SQL:     "CREATE TABLE broken (id INT);",
	}})
	if err == nil {
		t.Fatal("Apply() error = nil, want statement failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func newMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	gormDB, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return gormDB, mock, func() {
		_ = sqlDB.Close()
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
