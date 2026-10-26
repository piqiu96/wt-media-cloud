package executionpreflight

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newPreflightMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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

// 归属在读取时就分好：越界的 Profile 和媒体账号对当前用户等同于不存在。
func TestLoadCloudFactsScopesProfileAndAccountToTheUser(t *testing.T) {
	db, mock, closeDB := newPreflightMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, bit_main_user_id FROM users WHERE id = ?")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "bit_main_user_id"}).AddRow("enabled", "main-1"))
	mock.ExpectQuery(`SELECT user_id, COALESCE\(main_user_id, ''\), local_status\s+FROM browser_profiles WHERE id = \?`).
		WithArgs("11").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "main_user_id", "local_status"}).AddRow(8, "main-1", "active"))
	mock.ExpectQuery(`SELECT user_id, login_status FROM media_accounts WHERE id = \?`).
		WithArgs("21").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "login_status"}).AddRow(7, "normal"))

	facts, err := queryCloudFacts(db, 7, "11", "21")
	if err != nil {
		t.Fatalf("loadCloudFacts() error = %v", err)
	}
	if !facts.UserEnabled || facts.BitMainUserID != "main-1" {
		t.Fatalf("user facts = %+v", facts)
	}
	if !facts.Profile.Found || facts.Profile.OwnedByUser || !facts.Profile.AssignedToOtherUser || facts.Profile.MainUserID != "main-1" {
		t.Fatalf("profile facts = %+v", facts.Profile)
	}
	if !facts.MediaAccount.Found || facts.MediaAccount.LoginStatus != "normal" {
		t.Fatalf("media account facts = %+v", facts.MediaAccount)
	}
}

// 用户行没了就当未启用，不再继续查它的 Profile。
func TestLoadCloudFactsTreatsAMissingUserAsNotEnabled(t *testing.T) {
	db, mock, closeDB := newPreflightMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, bit_main_user_id FROM users WHERE id = ?")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "bit_main_user_id"}))

	facts, err := queryCloudFacts(db, 7, "11", "21")
	if err != nil {
		t.Fatalf("loadCloudFacts() error = %v", err)
	}
	if facts.UserEnabled || facts.BitMainUserID != "" || facts.Profile.Found || facts.MediaAccount.Found {
		t.Fatalf("facts = %+v", facts)
	}
}

func TestLoadCloudFactsSkipsTheQueriesTheActionDoesNotNeed(t *testing.T) {
	db, mock, closeDB := newPreflightMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, bit_main_user_id FROM users WHERE id = ?")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "bit_main_user_id"}).AddRow("enabled", "main-1"))

	if _, err := queryCloudFacts(db, 7, "", ""); err != nil {
		t.Fatalf("loadCloudFacts() error = %v", err)
	}
}

func TestLoadCloudFactsReportsOtherUsersMediaAccountAsAbsent(t *testing.T) {
	db, mock, closeDB := newPreflightMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, bit_main_user_id FROM users WHERE id = ?")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "bit_main_user_id"}).AddRow("enabled", "main-1"))
	mock.ExpectQuery(`SELECT user_id, login_status FROM media_accounts WHERE id = \?`).
		WithArgs("21").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "login_status"}).AddRow(8, "normal"))

	facts, err := queryCloudFacts(db, 7, "", "21")
	if err != nil {
		t.Fatalf("loadCloudFacts() error = %v", err)
	}
	if facts.MediaAccount.Found {
		t.Fatalf("another user's media account = %+v", facts.MediaAccount)
	}
}
