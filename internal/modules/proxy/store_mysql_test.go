package proxy

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLStoreCreateUsesOnePlaceholderPerProxyColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		db.Close()
	})

	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	proxy := ProxyConfig{
		ID: "proxy-1", SourceType: ProxySourceStatic, ProxyProtocol: ProtocolSOCKS5,
		Host: "127.0.0.1", Port: 19086, BusinessStatus: BizActive,
		MaxProfileCount: 2, Remark: "acceptance fixture", CreatedAt: now, UpdatedAt: now,
	}
	expected := `INSERT INTO proxy_configs (` + proxyColumns + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	mock.ExpectExec(regexp.QuoteMeta(expected)).
		WithArgs(proxy.ID, proxy.SourceType, proxy.ProxyProtocol, proxy.Host, proxy.Port,
			nil, nil, nil, nil, nil, nil, proxy.BusinessStatus, proxy.MaxProfileCount,
			nil, nil, nil, proxy.Remark, proxy.CreatedAt, proxy.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := NewMySQLStore(db).Create(proxy); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}
