// Package database owns initialized Cloud database connections.
package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/config"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	defaultDatabaseName = "primary"
	pingTimeout         = 5 * time.Second
)

var (
	mu          sync.RWMutex
	lifecycleMu sync.Mutex
	db          *gorm.DB
	namedDBs    map[string]*gorm.DB
)

// ErrMissingMySQLDSN remains for the transitional migrate command. New callers
// must initialize connections from config.DatabaseConfig instead of a raw DSN.
var ErrMissingMySQLDSN = errors.New("mysql dsn is required")

type opener func(config.DatabaseConfig) (*gorm.DB, error)

// Initialize opens every configured database and atomically publishes them.
func Initialize(configs []config.DatabaseConfig) error {
	return initialize(configs, openMySQL)
}

func initialize(configs []config.DatabaseConfig, open opener) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if err := validateDatabaseConfigs(configs); err != nil {
		return err
	}

	opened := make(map[string]*gorm.DB, len(configs))
	for _, databaseConfig := range configs {
		gormDB, err := open(databaseConfig)
		if err != nil {
			return errors.Join(
				fmt.Errorf("initialize database %q: %w", databaseConfig.Name, err),
				closeDatabases(opened),
			)
		}
		if gormDB == nil {
			return errors.Join(
				fmt.Errorf("initialize database %q: opener returned nil database", databaseConfig.Name),
				closeDatabases(opened),
			)
		}
		opened[databaseConfig.Name] = gormDB
	}

	mu.Lock()
	previous := namedDBs
	db = opened[defaultDatabaseName]
	namedDBs = opened
	mu.Unlock()

	return closeDatabases(previous)
}

// DB returns the code-defined primary database. It panics when Bootstrap has
// not initialized database resources, rather than returning a nil connection.
func DB() *gorm.DB {
	mu.RLock()
	primary := db
	mu.RUnlock()
	if primary == nil {
		panic("database: DB called before Initialize")
	}
	return primary
}

// Named returns a configured database by name.
func Named(name string) (*gorm.DB, bool) {
	mu.RLock()
	database, ok := namedDBs[name]
	mu.RUnlock()
	return database, ok
}

// Close clears the published registry and closes each unique underlying SQL
// connection once. Calling Close repeatedly is safe.
func Close() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	mu.Lock()
	databases := namedDBs
	db = nil
	namedDBs = nil
	mu.Unlock()

	return closeDatabases(databases)
}

func validateDatabaseConfigs(configs []config.DatabaseConfig) error {
	seen := make(map[string]struct{}, len(configs))
	for _, databaseConfig := range configs {
		name := strings.TrimSpace(databaseConfig.Name)
		if name == "" {
			return errors.New("database name is required")
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("duplicate database name %q", name)
		}
		seen[name] = struct{}{}
	}
	if _, ok := seen[defaultDatabaseName]; !ok {
		return fmt.Errorf("database %q is required", defaultDatabaseName)
	}
	return nil
}

func openMySQL(databaseConfig config.DatabaseConfig) (*gorm.DB, error) {
	location, err := time.LoadLocation(databaseConfig.Location)
	if err != nil {
		return nil, fmt.Errorf("load location %q: %w", databaseConfig.Location, err)
	}

	dsnConfig := mysqldriver.Config{
		User:      databaseConfig.Username,
		Passwd:    databaseConfig.Password,
		Net:       "tcp",
		Addr:      net.JoinHostPort(databaseConfig.Host, strconv.Itoa(databaseConfig.Port)),
		DBName:    databaseConfig.Database,
		ParseTime: databaseConfig.ParseTime,
		Loc:       location,
	}
	if databaseConfig.Charset != "" {
		dsnConfig.Params = map[string]string{"charset": databaseConfig.Charset}
	}

	gormDB, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{
		DSN:                       dsnConfig.FormatDSN(),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(databaseConfig.Pool.MaxIdle)
	sqlDB.SetMaxOpenConns(databaseConfig.Pool.MaxOpen)
	sqlDB.SetConnMaxLifetime(databaseConfig.Pool.MaxLifetime.Duration)

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return gormDB, nil
}

func closeDatabases(databases map[string]*gorm.DB) error {
	if len(databases) == 0 {
		return nil
	}
	seen := make(map[any]struct{}, len(databases))
	var closeErrors []error
	for _, gormDB := range databases {
		if gormDB == nil {
			continue
		}
		sqlDB, err := gormDB.DB()
		if err != nil {
			closeErrors = append(closeErrors, err)
			continue
		}
		if _, duplicate := seen[sqlDB]; duplicate {
			continue
		}
		seen[sqlDB] = struct{}{}
		if err := sqlDB.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}
