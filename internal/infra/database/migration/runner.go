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

// splitStatements splits a migration file into the statements to execute, and
// drops the comments between them.
//
// The naive `strings.Split(sqlText, ";")` this replaces is not sufficient, and
// migration 039 is the worked example: its leading comment contains a
// semicolon ("... remains metadata-only; no worker ..."), so the split cut the
// comment in two and handed the server `no\n-- worker ...\nALTER TABLE ...`.
// MySQL answered `Error 1064 ... near 'no -- worker, object-storage
// connection...'` — a syntax error that points at the SQL rather than at the
// splitter, which is why it survived review. Measured: 039 and 040 were the only
// two migrations whose comments contain a semicolon, and they were the only two
// that could not be applied; the other 38 applied cleanly.
//
// So the split has to know where the SQL is. A `;` inside a string literal, a
// quoted identifier, or a comment does not end a statement, and comment text
// never reaches the server — MySQL rejects a fragment that is only a comment as
// an empty query, so such fragments must be dropped rather than executed.
func splitStatements(sqlText string) []string {
	const (
		plain = iota
		singleQuoted
		doubleQuoted
		backtickQuoted
		lineComment
	)

	input := []rune(sqlText)
	statements := make([]string, 0, len(input)/64+1)
	var current strings.Builder
	state := plain

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch state {
		case lineComment:
			// Dropped, not accumulated: a comment is not SQL.
			if ch == '\n' {
				state = plain
			}

		case singleQuoted, doubleQuoted, backtickQuoted:
			current.WriteRune(ch)
			closing := '\''
			switch state {
			case doubleQuoted:
				closing = '"'
			case backtickQuoted:
				closing = '`'
			}
			if state != backtickQuoted && ch == '\\' && i+1 < len(input) {
				// A backslash escapes the next character, so a quote after it is
				// data rather than the end of the literal.
				i++
				current.WriteRune(input[i])
				continue
			}
			if ch == closing {
				if i+1 < len(input) && input[i+1] == closing {
					// A doubled quote is an escaped quote, not the end.
					i++
					current.WriteRune(input[i])
					continue
				}
				state = plain
			}

		default:
			switch {
			case ch == '-' && i+1 < len(input) && input[i+1] == '-' && startsLineComment(input, i+2):
				state = lineComment
				i++ // consume the second dash
			case ch == '\'':
				state = singleQuoted
				current.WriteRune(ch)
			case ch == '"':
				state = doubleQuoted
				current.WriteRune(ch)
			case ch == '`':
				state = backtickQuoted
				current.WriteRune(ch)
			case ch == ';':
				appendStatement(&statements, current.String())
				current.Reset()
			default:
				current.WriteRune(ch)
			}
		}
	}
	appendStatement(&statements, current.String())
	return statements
}

// startsLineComment reports whether the `--` ending at `at` opens a comment by
// MySQL's rule: the dashes must be followed by whitespace or end the input.
//
// `--x` is arithmetic rather than a comment, and treating it as one would delete
// real SQL silently — the failure mode this whole function exists to avoid.
func startsLineComment(input []rune, at int) bool {
	if at >= len(input) {
		return true
	}
	switch input[at] {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func appendStatement(statements *[]string, raw string) {
	if statement := strings.TrimSpace(raw); statement != "" {
		*statements = append(*statements, statement)
	}
}
