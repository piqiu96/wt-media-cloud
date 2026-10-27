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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + proxyColumns)).
		WithArgs("proxy-1").
		WillReturnRows(sqlmock.NewRows(proxyColumnNames()).
			AddRow("proxy-1", 7, "香港-01", "static", "socks5", "127.0.0.1", 1080, nil, nil, nil, nil, `["备用","直播"]`, nil, now, "active", 3, now, "ok", nil, "", now, now))

	proxy, found, err := findByID(db, "proxy-1")
	if err != nil || !found {
		t.Fatalf("findByID() = %v, %v, want found without error", found, err)
	}
	if proxy.ID != "proxy-1" || proxy.ProxyProtocol != model.ProtocolSOCKS5 || proxy.Host != "127.0.0.1" {
		t.Fatalf("proxy = %#v", proxy)
	}
	if proxy.TeamID != 7 || proxy.Name != "香港-01" || len(proxy.Tags) != 2 {
		t.Fatalf("team/name/tags = %d %q %v", proxy.TeamID, proxy.Name, proxy.Tags)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRecordCheckResultOnlyAppliesToTheMeasuredConnection(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE proxy_configs SET last_check_at = ?, last_check_result = ?, updated_at = ? WHERE id = ? AND proxy_protocol = ? AND host = ? AND port = ?")).
		WithArgs(now, "ok", now, "proxy-1", "socks5", "10.0.0.1", 1080).
		WillReturnResult(sqlmock.NewResult(0, 0))

	applied, err := recordCheckResult(db, "proxy-1", "socks5", "10.0.0.1", 1080, "ok", now)
	if err != nil {
		t.Fatalf("recordCheckResult() error = %v", err)
	}
	if applied {
		t.Fatal("applied = true, want a late result for an edited connection to be dropped")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestListConnectionKeysIsTeamScoped(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, proxy_protocol, host, port FROM proxy_configs WHERE team_id = ?")).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "proxy_protocol", "host", "port"}).
			AddRow("proxy-1", "http", "10.0.0.1", 8080).
			AddRow("proxy-2", "socks5", "10.0.0.2", 1080))

	keys, err := listConnectionKeys(db, 7)
	if err != nil {
		t.Fatalf("listConnectionKeys() error = %v", err)
	}
	if len(keys) != 2 || keys[1].ID != "proxy-2" || keys[1].Port != 1080 {
		t.Fatalf("keys = %#v", keys)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func proxyColumnNames() []string {
	return []string{
		"id", "team_id", "name", "source_type", "proxy_protocol", "host", "port",
		"username", "password", "region", "supplier", "tags_json", "extract_url",
		"expires_at", "business_status", "max_profile_count", "last_check_at",
		"last_check_result", "observed_exit_ip", "remark", "created_at", "updated_at",
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
