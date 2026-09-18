package repository

import (
	"os"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestProfileBindingMigrationKeepsScansAndProfilesSeparate(t *testing.T) {
	content, err := readFileAsString("../../../../migrations/20260714_003_browser_profiles.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, required := range []string{"CREATE TABLE profile_sync_scans", "CREATE TABLE browser_profiles", "bit_profile_id"} {
		if !containsString(content, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestFindBindingUsesGORMAndMapsIdentity(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, bit_main_user_id, bit_account_status")).
		WithArgs(identitymodel.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "bit_main_user_id", "bit_account_status", "bit_account_bound_at", "bit_account_last_verified_at"}).
			AddRow(identitymodel.UserID(1), "main-user-1", "bound", now, now))

	binding, found, err := findBinding(db, 1)
	if err != nil || !found {
		t.Fatalf("findBinding() = %v, %v, want found without error", found, err)
	}
	if binding.MainUserID != "main-user-1" || binding.Status != model.BitAccountBound {
		t.Fatalf("binding = %#v", binding)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func newProfileMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	db, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db, mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		_ = sqlDB.Close()
	}
}

func readFileAsString(path string) (string, error) {
	content, err := osReadFile(path)
	return string(content), err
}

func containsString(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
