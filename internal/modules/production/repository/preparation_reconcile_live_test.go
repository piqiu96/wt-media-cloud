package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Session-local tables exercise MySQL predicates without changing runtime data.
func TestLiveCancelledPreparationProjectionProtectsReadyAndActiveMaterials(t *testing.T) {
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
	sqlDB, err := database.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, ddl := range []string{
		`CREATE TEMPORARY TABLE materials (id BIGINT PRIMARY KEY, team_id BIGINT, video_status VARCHAR(32), video_error VARCHAR(255), updated_at DATETIME(6))`,
		`CREATE TEMPORARY TABLE file_transfer_tasks (team_id BIGINT, asset_id BIGINT, asset_type VARCHAR(32), purpose VARCHAR(32), status VARCHAR(32))`,
	} {
		if _, err = conn.ExecContext(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	defer conn.ExecContext(context.Background(), "DROP TEMPORARY TABLE IF EXISTS file_transfer_tasks, materials")
	for id := 1; id <= 7; id++ {
		status := "downloading"
		if id == 2 {
			status = "ready"
		}
		if _, err = conn.ExecContext(ctx, `INSERT INTO materials VALUES (?,1,?,'old',UTC_TIMESTAMP(6))`, id, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.ExecContext(ctx, `INSERT INTO file_transfer_tasks VALUES
(1,1,'material','compose_input_prepare','cancelled'),
(1,2,'material','compose_input_prepare','cancelled'),
(1,3,'material','compose_input_prepare','cancelled'),(1,3,'material','compose_input_prepare','pending'),
(1,4,'material','compose_input_prepare','cancelled'),(1,4,'material','compose_input_prepare','running'),
(1,5,'material','compose_input_prepare','failed'),
(1,6,'material','user_download','cancelled'),
(2,7,'material','compose_input_prepare','cancelled')`); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	if count, err := reconcileCancelledMaterialPreparations(db, time.Now()); err != nil || count != 1 {
		t.Fatalf("restore count=%d error=%v", count, err)
	}
	rows, err := conn.QueryContext(ctx, "SELECT id,video_status,video_error FROM materials ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int
		var status, message string
		if err := rows.Scan(&id, &status, &message); err != nil {
			t.Fatal(err)
		}
		want := "downloading"
		if id == 1 {
			want = "not_downloaded"
		}
		if id == 2 {
			want = "ready"
		}
		if status != want || (id == 1 && message != "") {
			t.Fatalf("material %d status=%s message=%q", id, status, message)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if count, err := reconcileCancelledMaterialPreparations(db, time.Now()); err != nil || count != 0 {
		t.Fatalf("repeat count=%d error=%v", count, err)
	}
}
