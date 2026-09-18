// Package migration owns database schema migration loading and execution.
package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type Migration struct {
	Version string
	Name    string
	SQL     string
}

type AppliedMigration struct {
	Version string
	Name    string
}

func LoadDir(dir string) ([]Migration, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	migrations := make([]Migration, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		version := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		migrations = append(migrations, Migration{
			Version: version,
			Name:    migrationName(version),
			SQL:     string(content),
		})
	}
	return migrations, nil
}

func Apply(ctx context.Context, db *gorm.DB, migrations []Migration) ([]AppliedMigration, error) {
	if err := db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    applied_at DATETIME(6) NOT NULL,
    PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`).Error; err != nil {
		return nil, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	appliedVersions, err := appliedVersionSet(ctx, db)
	if err != nil {
		return nil, err
	}

	applied := make([]AppliedMigration, 0, len(migrations))
	for _, migration := range migrations {
		if appliedVersions[migration.Version] {
			continue
		}
		if err := applyOne(ctx, db, migration); err != nil {
			return applied, err
		}
		applied = append(applied, AppliedMigration{Version: migration.Version, Name: migration.Name})
		appliedVersions, err = appliedVersionSet(ctx, db)
		if err != nil {
			return applied, fmt.Errorf("refresh applied migrations after %s: %w", migration.Version, err)
		}
	}
	return applied, nil
}

func appliedVersionSet(ctx context.Context, db *gorm.DB) (map[string]bool, error) {
	rows, err := db.WithContext(ctx).Raw("SELECT version FROM schema_migrations").Rows()
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		result[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return result, nil
}

func applyOne(ctx context.Context, db *gorm.DB, migration Migration) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, statement := range splitStatements(migration.SQL) {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("apply migration %s: %w", migration.Version, err)
			}
		}
		if err := tx.Exec(
			"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, UTC_TIMESTAMP(6))",
			migration.Version,
			migration.Name,
		).Error; err != nil {
			return fmt.Errorf("record migration %s: %w", migration.Version, err)
		}
		return nil
	})
}

func migrationName(version string) string {
	parts := strings.SplitN(version, "_", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return version
}

func splitStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}
	return statements
}
