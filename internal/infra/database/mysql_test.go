package database

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/config"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestInitializeRequiresPrimary(t *testing.T) {
	resetRegistry(t)

	err := initialize([]config.DatabaseConfig{{Name: "analytics"}}, func(config.DatabaseConfig) (*gorm.DB, error) {
		t.Fatal("opener must not run when primary is missing")
		return nil, nil
	})
	if err == nil {
		t.Fatal("initialize() error = nil, want missing primary error")
	}
}

func TestInitializeRejectsDuplicateNames(t *testing.T) {
	resetRegistry(t)

	err := initialize([]config.DatabaseConfig{{Name: "primary"}, {Name: "primary"}}, func(config.DatabaseConfig) (*gorm.DB, error) {
		t.Fatal("opener must not run for duplicate database names")
		return nil, nil
	})
	if err == nil {
		t.Fatal("initialize() error = nil, want duplicate name error")
	}
}

func TestInitializeRegistersDefaultAndNamedDatabases(t *testing.T) {
	resetRegistry(t)
	primary, primaryMock := newMockGORM(t)
	analytics, analyticsMock := newMockGORM(t)
	primaryMock.ExpectClose()
	analyticsMock.ExpectClose()

	err := initialize([]config.DatabaseConfig{{Name: "primary"}, {Name: "analytics"}}, func(cfg config.DatabaseConfig) (*gorm.DB, error) {
		switch cfg.Name {
		case "primary":
			return primary, nil
		case "analytics":
			return analytics, nil
		default:
			t.Fatalf("unexpected database config %q", cfg.Name)
			return nil, nil
		}
	})
	if err != nil {
		t.Fatalf("initialize() error = %v", err)
	}
	if got := DB(); got != primary {
		t.Fatalf("DB() = %p, want %p", got, primary)
	}
	if got, ok := Named("analytics"); !ok || got != analytics {
		t.Fatalf("Named(analytics) = (%p, %v), want (%p, true)", got, ok, analytics)
	}

	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertMock(t, primaryMock)
	assertMock(t, analyticsMock)
}

func TestInitializeDoesNotPublishPartialRegistry(t *testing.T) {
	resetRegistry(t)
	oldPrimary, oldMock := newMockGORM(t)
	newPrimary, newMock := newMockGORM(t)
	oldMock.ExpectClose()
	newMock.ExpectClose()
	installRegistry(oldPrimary, map[string]*gorm.DB{"primary": oldPrimary})

	err := initialize([]config.DatabaseConfig{{Name: "primary"}, {Name: "analytics"}}, func(cfg config.DatabaseConfig) (*gorm.DB, error) {
		if cfg.Name == "primary" {
			return newPrimary, nil
		}
		return nil, errors.New("analytics unavailable")
	})
	if err == nil {
		t.Fatal("initialize() error = nil, want opener failure")
	}
	if got := DB(); got != oldPrimary {
		t.Fatalf("DB() = %p, want existing primary %p after failed initialization", got, oldPrimary)
	}
	if _, ok := Named("analytics"); ok {
		t.Fatal("Named(analytics) published a partial registry")
	}
	assertMock(t, newMock)

	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertMock(t, oldMock)
}

func TestInitializeClosesOpenedDatabasesWhenLaterOpenFails(t *testing.T) {
	resetRegistry(t)
	primary, primaryMock := newMockGORM(t)
	primaryMock.ExpectClose()

	err := initialize([]config.DatabaseConfig{{Name: "primary"}, {Name: "analytics"}}, func(cfg config.DatabaseConfig) (*gorm.DB, error) {
		if cfg.Name == "primary" {
			return primary, nil
		}
		return nil, errors.New("analytics unavailable")
	})
	if err == nil {
		t.Fatal("initialize() error = nil, want opener failure")
	}
	assertMock(t, primaryMock)
}

func TestDBPanicsBeforeInitialize(t *testing.T) {
	resetRegistry(t)

	defer func() {
		if recover() == nil {
			t.Fatal("DB() did not panic before initialization")
		}
	}()
	_ = DB()
}

func TestNamedReturnsFalseForUnknownDatabase(t *testing.T) {
	resetRegistry(t)
	primary, primaryMock := newMockGORM(t)
	primaryMock.ExpectClose()
	installRegistry(primary, map[string]*gorm.DB{"primary": primary})

	if got, ok := Named("unknown"); got != nil || ok {
		t.Fatalf("Named(unknown) = (%p, %v), want (nil, false)", got, ok)
	}
	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertMock(t, primaryMock)
}

func TestCloseClosesEveryUniqueSQLConnectionOnce(t *testing.T) {
	resetRegistry(t)
	primary, primaryMock := newMockGORM(t)
	alias, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{
		Conn:                      mustSQLDB(t, primary),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	primaryMock.ExpectClose()
	installRegistry(primary, map[string]*gorm.DB{"primary": primary, "alias": alias})

	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertMock(t, primaryMock)
}

func TestCloseIsIdempotent(t *testing.T) {
	resetRegistry(t)
	primary, primaryMock := newMockGORM(t)
	primaryMock.ExpectClose()
	installRegistry(primary, map[string]*gorm.DB{"primary": primary})

	if err := Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	assertMock(t, primaryMock)
}

func newMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	gormDB, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return gormDB, mock
}

func mustSQLDB(t *testing.T, gormDB *gorm.DB) *sql.DB {
	t.Helper()
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("gormDB.DB() error = %v", err)
	}
	return sqlDB
}

func resetRegistry(t *testing.T) {
	t.Helper()
	if err := Close(); err != nil {
		t.Fatalf("reset Close() error = %v", err)
	}
	t.Cleanup(func() { _ = Close() })
}

func installRegistry(primary *gorm.DB, named map[string]*gorm.DB) {
	mu.Lock()
	db = primary
	namedDBs = named
	mu.Unlock()
}

func assertMock(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
