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
	content, err := osReadFileAsString("../../../../migrations/20261029_078_category_model.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, required := range []string{"CREATE TABLE categories", "type VARCHAR", "RENAME TABLE media_account_games TO media_account_categories", "category_id BIGINT"} {
		if !containsString(content, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestFindByIDUsesGORM(t *testing.T) {
	db, mock, closeDB := newAccountMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ma.id, ma.user_id, ma.team_id, ma.platform")).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "team_id", "platform", "platform_account_id", "platform_name", "platform_avatar_url", "browser_profile_id", "remark", "identification_status", "duplicate_of_account_id", "enabled", "platform_login_status", "original_cookie", "active_cookie", "cookie_status", "cookie_updated_at", "last_check_at", "last_check_items", "created_at", "updated_at", "user_name", "last_check_status", "last_check_message", "last_check_identity", "check_operation_id", "check_profile_id", "check_started_at", "before_cookie", "cookie_update"}).
			AddRow("account-1", 1, nil, "douyin", "platform-1", "account", nil, nil, nil, "identified", nil, true, "normal", nil, nil, nil, nil, nil, nil, now, now, "运营张三", "never", nil, nil, nil, nil, nil, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT media_account_id, category_id FROM media_account_categories")).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"media_account_id", "category_id"}).AddRow("account-1", int64(1)))

	record, found, err := findByID(db, "account-1")
	if err != nil || !found {
		t.Fatalf("findByID() = %v, %v, want found without error", found, err)
	}
	if record.ID != "account-1" || record.Platform != model.PlatformDouyin || len(record.CategoryIDs) != 1 {
		t.Fatalf("record = %#v", record)
	}
	// 使用人随账号一起读出；它来自 join，不参与写入。
	if record.UserName != "运营张三" {
		t.Fatalf("user_name = %q, want 运营张三", record.UserName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestCreateReadsAutoIncrementIDFromTheSameTransaction(t *testing.T) {
	db, mock, closeDB := newAccountMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO media_accounts")).WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT LAST_INSERT_ID()")).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(41)))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM media_account_categories WHERE media_account_id = ?")).WithArgs("41").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	got, err := create(db, model.AccountRecord{Account: model.Account{
		UserID: 1, Platform: model.PlatformBilibili, IdentificationStatus: model.IdentificationPending,
		Enabled: true, PlatformLoginStatus: model.PlatformLoginUnknown, Tags: []string{}, CreatedAt: now, UpdatedAt: now,
	}})
	if err != nil || got != "41" {
		t.Fatalf("create() = %q, %v, want transaction-local auto increment ID 41", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestFindReturnsAnEmptyCategorySliceWhenTheAccountHasNoRelations(t *testing.T) {
	db, mock, closeDB := newAccountMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ma.id, ma.user_id, ma.team_id, ma.platform")).WithArgs("account-empty").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "team_id", "platform", "platform_account_id", "platform_name", "platform_avatar_url", "browser_profile_id", "remark", "identification_status", "duplicate_of_account_id", "enabled", "platform_login_status", "original_cookie", "active_cookie", "cookie_status", "cookie_updated_at", "last_check_at", "last_check_items", "created_at", "updated_at", "user_name", "last_check_status", "last_check_message", "last_check_identity", "check_operation_id", "check_profile_id", "check_started_at", "before_cookie", "cookie_update"}).
			AddRow("account-empty", 1, nil, "bilibili", nil, "未分类账号", nil, nil, nil, "pending_identification", nil, true, "unknown", nil, nil, nil, nil, nil, nil, now, now, "运营张三", "never", nil, nil, nil, nil, nil, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT media_account_id, category_id FROM media_account_categories")).WithArgs("account-empty").
		WillReturnRows(sqlmock.NewRows([]string{"media_account_id", "category_id"}))

	record, found, err := findByID(db, "account-empty")
	if err != nil || !found {
		t.Fatalf("findByID() = %v, %v, want found without error", found, err)
	}
	if record.CategoryIDs == nil || len(record.CategoryIDs) != 0 {
		t.Fatalf("category_ids = %#v, want a non-nil empty slice", record.CategoryIDs)
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
