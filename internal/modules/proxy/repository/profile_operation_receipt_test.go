package repository

import (
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
)

func TestReserveProxyReceiptCapturesConnectionUsedByBit(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	mock.ExpectExec("UPDATE browser_profiles SET proxy_operation").
		WithArgs(sqlmock.AnyArg(), "p1").WillReturnResult(sqlmock.NewResult(0, 1))
	target := ProfileProxyTarget{ProfileID: "p1"}
	proxy := model.Proxy{Protocol: model.ProtocolSOCKS5, Host: "10.9.0.1", Port: 1080}
	if err := reserveReceipt(db, &target, "apply", "1", &proxy); err != nil {
		t.Fatal(err)
	}
	if target.Operation.ProxyProtocol != "socks5" || target.Operation.ProxyHost != proxy.Host || target.Operation.ProxyPort != proxy.Port {
		t.Fatalf("receipt did not capture target connection: %+v", target.Operation)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProxyReceiptFencesOldResponseAndAction(t *testing.T) {
	for _, input := range []struct{ id, action, proxy string }{{"old", "apply", "1"}, {"op1", "unbind", "1"}, {"op1", "apply", "2"}, {"", "apply", "1"}} {
		db, mock, closeDB := newProxyMockGORM(t)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT proxy_operation, reserved_proxy_id").WithArgs("p1").WillReturnRows(sqlmock.NewRows([]string{"proxy_operation", "reserved_proxy_id"}).AddRow(`{"operation_id":"op1","action":"apply","proxy_id":"1","outcome":"pending"}`, "1"))
		mock.ExpectRollback()
		err := resolveProfileProxyOperationReceipt(db, "p1", input.id, input.action, input.proxy, "success", time.Now())
		if !errors.Is(err, ErrProxyOperationStale) {
			t.Fatalf("expected stale, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		closeDB()
	}
}

func TestProxyReceiptReplaysSettlementWithoutRewriting(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT proxy_operation, reserved_proxy_id").WithArgs("p1").WillReturnRows(sqlmock.NewRows([]string{"proxy_operation", "reserved_proxy_id"}).AddRow(`{"operation_id":"op1","action":"apply","proxy_id":"1","outcome":"success"}`, nil))
	mock.ExpectCommit()
	if err := resolveProfileProxyOperationReceipt(db, "p1", "op1", "apply", "1", "success", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProxyReceiptSettlesAndClearsOnlyTheCurrentReservation(t *testing.T) {
	for _, outcome := range []string{"success", "failed"} {
		db, mock, closeDB := newProxyMockGORM(t)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT proxy_operation, reserved_proxy_id").WithArgs("p1").WillReturnRows(sqlmock.NewRows([]string{"proxy_operation", "reserved_proxy_id"}).AddRow(`{"operation_id":"op1","action":"apply","proxy_id":"1","outcome":"pending"}`, "1"))
		if outcome == "success" {
			mock.ExpectExec("UPDATE browser_profiles SET proxy_id").WithArgs("1", "p1").WillReturnResult(sqlmock.NewResult(0, 1))
		}
		mock.ExpectExec("UPDATE browser_profiles SET reserved_proxy_id = NULL").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "p1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		if err := resolveProfileProxyOperationReceipt(db, "p1", "op1", "apply", "1", outcome, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		closeDB()
	}
}

func TestProxyReceiptSuccessProjectsAcknowledgedConnection(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT proxy_operation, reserved_proxy_id").WithArgs("p1").WillReturnRows(
		sqlmock.NewRows([]string{"proxy_operation", "reserved_proxy_id"}).AddRow(
			`{"operation_id":"op1","action":"apply","proxy_id":"1","outcome":"pending","proxy_protocol":"socks5","proxy_host":"10.9.0.1","proxy_port":1080}`,
			"1",
		),
	)
	mock.ExpectExec("UPDATE browser_profiles SET proxy_id = \\?, bit_proxy_type = \\?, bit_proxy_host = \\?, bit_proxy_port = \\? WHERE id = \\?").
		WithArgs("1", "socks5", "10.9.0.1", 1080, "p1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE browser_profiles SET reserved_proxy_id = NULL").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "p1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := resolveProfileProxyOperationReceipt(db, "p1", "op1", "apply", "1", "success", now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProxyReceiptUnbindProjectsDirectConnection(t *testing.T) {
	db, mock, closeDB := newProxyMockGORM(t)
	defer closeDB()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT proxy_operation, reserved_proxy_id").WithArgs("p1").WillReturnRows(
		sqlmock.NewRows([]string{"proxy_operation", "reserved_proxy_id"}).AddRow(
			`{"operation_id":"op1","action":"unbind","proxy_id":"1","outcome":"pending"}`,
			"1",
		),
	)
	mock.ExpectExec("UPDATE browser_profiles SET proxy_id = NULL, bit_proxy_type = 'noproxy', bit_proxy_host = NULL, bit_proxy_port = 0 WHERE id = \\?").
		WithArgs("p1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE browser_profiles SET reserved_proxy_id = NULL").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "p1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := resolveProfileProxyOperationReceipt(db, "p1", "op1", "unbind", "1", "success", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
