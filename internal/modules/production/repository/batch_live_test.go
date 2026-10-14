package repository

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
)

// One schedule run's own rows must read back per material with honest counts:
// a batch with a success, a failure and a pending task reads as partial
// failure, not as all-success. Seeded rows live in a rolled-back transaction.
func TestLiveComposeBatchTasksReadTheRunSlot(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("set WT_MEDIA_TEST_LIVE_DB=1 after migrations")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	// A batch is one row per material (the anti-duplicate key). Seed the
	// strategy's distinct materials, one task each, with a partial-failure
	// shape: every material succeeds except the last one, which fails.
	var donorStrategy int64
	if err := database.DB().Raw(`SELECT compose_strategy_id FROM compose_tasks GROUP BY compose_strategy_id HAVING COUNT(DISTINCT material_id) >= 2 ORDER BY compose_strategy_id DESC LIMIT 1`).Row().Scan(&donorStrategy); err != nil {
		t.Skipf("no strategy with two materials to seed a batch from: %v", err)
	}
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	slot := time.Now().UTC().Truncate(time.Microsecond)
	requestKey := fmt.Sprintf("chg095-batch-%d", time.Now().UnixNano())
	materials := []int64{}
	if err := database.DB().Raw(`SELECT DISTINCT material_id FROM compose_tasks WHERE compose_strategy_id = ? ORDER BY material_id LIMIT 2`, donorStrategy).Scan(&materials).Error; err != nil || len(materials) != 2 {
		t.Fatalf("read two materials of strategy %d: %v", donorStrategy, err)
	}
	for i, material := range materials {
		result := tx.Exec(`INSERT INTO compose_tasks (team_id, game_id, trigger_type, schedule_slot, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, request_key, created_by, created_at, updated_at) SELECT team_id, game_id, 'scheduled', ?, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, 'pending', ?, created_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_tasks WHERE id = (SELECT id FROM compose_tasks WHERE compose_strategy_id = ? AND material_id = ? ORDER BY id DESC LIMIT 1)`, slot, fmt.Sprintf("%s-%d", requestKey, i), donorStrategy, material)
		if result.Error != nil || result.RowsAffected != 1 {
			t.Fatalf("seed batch task %d (material %d): rows=%d err=%v", i, material, result.RowsAffected, result.Error)
		}
	}
	if err := tx.Exec(`UPDATE compose_tasks SET status = CASE WHEN request_key = ? THEN 'failed' ELSE 'success' END, error_message = 'batch seeded' WHERE request_key LIKE ?`, requestKey+"-1", requestKey+"%").Error; err != nil {
		t.Fatal(err)
	}

	tasks, counts, err := composeBatchTasks(tx, donorStrategy, slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != len(materials) {
		t.Fatalf("batch must read all %d seeded tasks, got %d", len(materials), len(tasks))
	}
	if counts.Success != 1 || counts.Failed != 1 || counts.Pending != 0 || counts.Running != 0 || counts.Cancelled != 0 {
		t.Fatalf("partial failure must count honestly: %+v", counts)
	}
	var failed *int
	for i := range tasks {
		if tasks[i].Status == "failed" {
			failed = &i
		}
	}
	if failed == nil || tasks[*failed].ErrorMessage != "batch seeded" {
		t.Fatalf("the failed task must carry its error message: %+v", tasks)
	}
}
