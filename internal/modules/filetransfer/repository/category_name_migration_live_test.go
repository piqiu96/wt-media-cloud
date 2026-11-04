package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
)

// The session-local table shadows the production table: historical migrations
// run unchanged, while neither schema nor rows in the development table change.
func TestLiveCategoryNameMigrationPreservesTransferRowsAndPublishedAt(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires local MySQL")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	db, err := database.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `CREATE TEMPORARY TABLE file_transfer_tasks (id BIGINT PRIMARY KEY, asset_title VARCHAR(255) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), `DROP TEMPORARY TABLE IF EXISTS file_transfer_tasks`)
	apply := func(name string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = conn.ExecContext(ctx, string(raw)); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	apply("20260930_043_file_transfer_game_name.sql")
	apply("20261001_044_file_transfer_publish_time.sql")
	published := time.Date(2026, 10, 1, 9, 30, 12, 123456000, time.FixedZone("CST", 8*60*60))
	if _, err = conn.ExecContext(ctx, `INSERT INTO file_transfer_tasks(id,asset_title,game_name,published_at) VALUES (1,'旧任务','原类目',?),(2,'空历史',NULL,NULL),(3,'空类目','',?)`, published, published); err != nil {
		t.Fatal(err)
	}
	apply("20261105_093_file_transfer_category_name.sql")
	rows, err := conn.QueryContext(ctx, `SELECT id,asset_title,category_name,published_at FROM file_transfer_tasks ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var id int
		var title string
		var category sql.NullString
		var at sql.NullTime
		if err = rows.Scan(&id, &title, &category, &at); err != nil {
			t.Fatal(err)
		}
		count++
		switch id {
		case 1:
			if title != "旧任务" || !category.Valid || category.String != "原类目" || !at.Valid || !at.Time.Equal(published) {
				t.Fatalf("historic row changed: %d %q %+v %+v", id, title, category, at)
			}
		case 2:
			if title != "空历史" || category.Valid || at.Valid {
				t.Fatalf("unknown historical values were fabricated: %q %+v %+v", title, category, at)
			}
		case 3:
			if title != "空类目" || !category.Valid || category.String != "" || !at.Valid || !at.Time.Equal(published) {
				t.Fatalf("empty historical values changed: %q %+v %+v", title, category, at)
			}
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if count != 3 {
		t.Fatalf("row count=%d", count)
	}
	if _, err = conn.ExecContext(ctx, `INSERT INTO file_transfer_tasks(id,asset_title,category_name,published_at) VALUES (4,'新任务','新类目',?)`, published); err != nil {
		t.Fatalf("current task writer cannot use migrated column: %v", err)
	}
	if _, err = conn.QueryContext(ctx, `SELECT game_name FROM file_transfer_tasks`); err == nil {
		t.Fatal("obsolete column still exists")
	}
}
