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
			AddRow("proxy-1", 7, "香港-01", "static", "socks5", "127.0.0.1", 1080, nil, nil, nil, nil, `["备用","直播"]`, nil, now, true, 3, now, "ok", "ok", "", now, now))

	proxy, found, err := findByID(db, "proxy-1")
	if err != nil || !found {
		t.Fatalf("findByID() = %v, %v, want found without error", found, err)
	}
	if proxy.ID != "proxy-1" || proxy.Protocol != model.ProtocolSOCKS5 || proxy.Host != "127.0.0.1" {
		t.Fatalf("proxy = %#v", proxy)
	}
	if proxy.TeamID != 7 || proxy.Name != "香港-01" || len(proxy.Tags) != 2 {
		t.Fatalf("team/name/tags = %d %q %v", proxy.TeamID, proxy.Name, proxy.Tags)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestCreateUsesDatabaseGeneratedNumericID(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	proxy := model.Proxy{TeamID: 7, Name: "新代理", Source: model.ProxySourceStatic, Protocol: model.ProtocolHTTP,
		Host: "127.0.0.1", Port: 8080, Enabled: true, MaxProfiles: 3, CreatedAt: now, UpdatedAt: now}
	mock.ExpectExec("INSERT INTO proxies \\(team_id, name, source").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT LAST_INSERT_ID()")).
		WillReturnRows(sqlmock.NewRows([]string{"LAST_INSERT_ID()"}).AddRow(42))
	if err := create(db, &proxy); err != nil {
		t.Fatal(err)
	}
	if proxy.ID != "42" {
		t.Fatalf("generated ID = %q, want 42", proxy.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListFiltersAdminTeamTextAndNumericIDOnServer(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	mock.ExpectQuery("SELECT .* FROM proxies WHERE EXISTS ").
		WithArgs("%运!_营%", "%运!_营%", "%42%", "%42%", "%42%", "%42%", "%42%", 200, 0).
		WillReturnRows(sqlmock.NewRows(proxyColumnNames()))
	items, err := list(db, ProxyFilter{TeamSearch: "运_营", Search: "42", Limit: 200})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecordCheckResultOnlyAppliesToTheMeasuredConnection(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE proxies SET last_check_at = ?, last_check_result = ?, last_check_message = ?, updated_at = ? WHERE id = ? AND protocol = ? AND host = ? AND port = ? AND BINARY COALESCE(username, '') = BINARY ? AND BINARY COALESCE(password, '') = BINARY ?")).
		WithArgs(now, "ok", "", now, "proxy-1", "socks5", "10.0.0.1", 1080, "user", "Secret").
		WillReturnResult(sqlmock.NewResult(0, 0))

	applied, err := recordCheckResult(db, "proxy-1", "socks5", "10.0.0.1", 1080, "user", "Secret", "ok", "", now)
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, protocol, host, port FROM proxies WHERE team_id = ?")).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "protocol", "host", "port"}).
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
		"id", "team_id", "name", "source", "protocol", "host", "port",
		"username", "password", "region", "supplier", "tags", "extract_url",
		"expires_at", "enabled", "max_profiles", "last_check_at",
		"last_check_result", "last_check_message", "remark", "created_at", "updated_at",
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
