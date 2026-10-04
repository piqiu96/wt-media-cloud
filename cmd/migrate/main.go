package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
)

func main() {
	paths, err := config.ResolveRuntimePaths()
	if err != nil {
		log.Fatal(err)
	}
	dir := flag.String("dir", paths.Migrations, "directory containing .sql migration files")
	createDatabase := flag.Bool("create-database", true, "create the configured database if it does not exist")
	flag.Parse()

	if err := run(*dir, *createDatabase); err != nil {
		log.Fatal(err)
	}
}

func run(dir string, createDatabase bool) error {
	closer, err := bootstrap.InitializeMigration()
	if err != nil {
		if !createDatabase {
			return err
		}
		cfg, loadErr := config.Load()
		if loadErr != nil {
			return errors.Join(err, loadErr)
		}
		if ensureErr := ensurePrimaryDatabase(cfg); ensureErr != nil {
			return errors.Join(err, ensureErr)
		}
		closer, err = bootstrap.InitializeMigration()
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	migrations, loadErr := migration.LoadDir(dir)
	applied, applyErr := migration.Apply(ctx, database.DB(), migrations)
	closeErr := closer()

	if loadErr != nil {
		return loadErr
	}
	if applyErr != nil {
		return applyErr
	}
	if closeErr != nil {
		return closeErr
	}

	fmt.Printf("migration ok: %d applied, %d total\n", len(applied), len(migrations))
	for _, item := range applied {
		fmt.Printf("applied %s %s\n", item.Version, item.Name)
	}
	return nil
}

func ensurePrimaryDatabase(cfg config.Config) error {
	var databaseConfig config.DatabaseConfig
	found := false
	for _, item := range cfg.Databases {
		if item.Name == "primary" {
			databaseConfig = item
			found = true
			break
		}
	}
	if !found {
		return errors.New("database \"primary\" is required")
	}

	location, err := time.LoadLocation(databaseConfig.Location)
	if err != nil {
		return err
	}
	dsnConfig := mysqldriver.Config{
		User:      databaseConfig.Username,
		Passwd:    databaseConfig.Password,
		Net:       "tcp",
		Addr:      joinHostPort(databaseConfig.Host, databaseConfig.Port),
		ParseTime: databaseConfig.ParseTime,
		Loc:       location,
	}
	if databaseConfig.Charset != "" {
		dsnConfig.Params = map[string]string{"charset": databaseConfig.Charset}
	}

	db, err := sql.Open("mysql", dsnConfig.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteIdentifier(databaseConfig.Database)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")
	return err
}

func joinHostPort(host string, port int) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + fmt.Sprint(port)
	}
	return host + ":" + fmt.Sprint(port)
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}
