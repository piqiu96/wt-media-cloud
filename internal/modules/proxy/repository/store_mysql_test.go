package repository

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestFindByIDUsesGORM(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, source_type, proxy_protocol, host, port")).
		WithArgs("proxy-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "source_type", "proxy_protocol", "host", "port", "username", "password", "region", "supplier", "extract_url", "expires_at", "business_status", "max_profile_count", "last_check_at", "last_check_result", "observed_exit_ip", "remark", "created_at", "updated_at"}).
			AddRow("proxy-1", "static", "socks5", "127.0.0.1", 1080, nil, nil, nil, nil, nil, now, "active", 3, now, "ok", nil, "", now, now))

	proxy, found, err := findByID(db, "proxy-1")
	if err != nil || !found {
		t.Fatalf("findByID() = %v, %v, want found without error", found, err)
	}
	if proxy.ID != "proxy-1" || proxy.ProxyProtocol != model.ProtocolSOCKS5 || proxy.Host != "127.0.0.1" {
		t.Fatalf("proxy = %#v", proxy)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func newProxyMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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
