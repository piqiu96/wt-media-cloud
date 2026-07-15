package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/migration"
)

func main() {
	dir := flag.String("dir", "migrations", "directory containing .sql migration files")
	createDatabase := flag.Bool("create-database", true, "create the DSN database if it does not exist")
	flag.Parse()

	if err := run(*dir, *createDatabase); err != nil {
		log.Fatal(err)
	}
}

func run(dir string, createDatabase bool) error {
	dsn := os.Getenv("WT_MEDIA_MYSQL_DSN")
	if dsn == "" {
		return database.ErrMissingMySQLDSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if createDatabase {
		if err := ensureDatabase(ctx, dsn); err != nil {
			return err
		}
	}

	db, err := database.OpenMySQL(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	migrations, err := migration.LoadDir(dir)
	if err != nil {
		return err
	}
	applied, err := migration.Apply(ctx, db, migrations)
	if err != nil {
		return err
	}
	fmt.Printf("migration ok: %d applied, %d total\n", len(applied), len(migrations))
	for _, item := range applied {
		fmt.Printf("applied %s %s\n", item.Version, item.Name)
	}
	return nil
}

func ensureDatabase(ctx context.Context, dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return err
	}
	if cfg.DBName == "" {
		return errors.New("mysql dsn must include a database name")
	}
	dbName := cfg.DBName
	cfg.DBName = ""
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteIdentifier(dbName)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")
	return err
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}
