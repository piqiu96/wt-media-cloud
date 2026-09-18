package repository

import (
	"os"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestMediaAccountMigrationKeepsAccountAndGroupTablesSeparate(t *testing.T) {
	content, err := osReadFileAsString("../../../../migrations/20260903_025_media_account_games.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, required := range []string{"CREATE TABLE media_account_games", "media_account_id", "game_id"} {
		if !containsString(content, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestFindByIDUsesGORM(t *testing.T) {
	db, mock, closeDB := newAccountMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, team_id, platform")).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "team_id", "platform", "platform_account_id", "name", "avatar_url", "browser_profile_id", "remark", "identification_status", "duplicate_of_account_id", "business_status", "login_status", "original_cookie", "active_cookie", "cookie_status", "active_cookie_updated_at", "last_checked_at", "check_items", "created_at", "updated_at"}).
			AddRow("account-1", 1, nil, "douyin", "platform-1", "account", nil, nil, nil, "identified", nil, "enabled", "normal", nil, nil, nil, nil, nil, nil, now, now))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT media_account_id, game_id FROM media_account_games")).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"media_account_id", "game_id"}).AddRow("account-1", "game-a"))

	record, found, err := findByID(db, "account-1")
	if err != nil || !found {
		t.Fatalf("findByID() = %v, %v, want found without error", found, err)
	}
	if record.ID != "account-1" || record.Platform != model.PlatformDouyin || len(record.GameIDs) != 1 {
		t.Fatalf("record = %#v", record)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func newAccountMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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

func osReadFileAsString(path string) (string, error) {
	content, err := os.ReadFile(path)
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
