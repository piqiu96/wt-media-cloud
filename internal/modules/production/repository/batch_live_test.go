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

	// A batch is one row per material (the anti-duplicate key). The local
	// database may hold no strategy with two tasks at all, so the fixture is
	// created here: copy an eligible strategy, then seed two materials with a
	// partial-failure shape — one succeeds, one fails.
	var sourceID int64
	err = database.DB().Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN users u ON u.id = s.operator_id AND u.status = 'enabled' AND u.role IN ('operator', 'senior_operator') AND u.team_id = s.team_id JOIN user_game_scopes g ON g.user_id = u.id AND g.game_id = s.game_id WHERE s.status = 'enabled' AND t.status = 'active' ORDER BY s.id LIMIT 1`).Row().Scan(&sourceID)
	if err != nil {
		t.Skipf("no active strategy with an available operator: %v", err)
	}
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	name := fmt.Sprintf("CHG105-live-batch-%d", time.Now().UnixNano())
	result := tx.Exec(`INSERT INTO compose_strategies (team_id, game_id, operator_id, compose_template_id, compose_template_version, name, config_json, execution_mode, schedule_json, priority_rules_json, single_limit, daily_limit, next_run_at, status, created_by, updated_by, created_at, updated_at) SELECT team_id, game_id, operator_id, compose_template_id, compose_template_version, ?, config_json, 'manual', NULL, priority_rules_json, 2, 2, NULL, 'enabled', created_by, updated_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies WHERE id = ?`, name, sourceID)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("create batch fixture from %d: rows=%d err=%v", sourceID, result.RowsAffected, result.Error)
	}
	var donorStrategy int64
	if err := tx.Raw(`SELECT id FROM compose_strategies WHERE name = ?`, name).Row().Scan(&donorStrategy); err != nil {
		t.Fatal(err)
	}
	slot := time.Now().UTC().Truncate(time.Microsecond)
	requestKey := fmt.Sprintf("chg095-batch-%d", time.Now().UnixNano())
	materials := []int64{}
	if err := tx.Raw(`SELECT m.id FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id JOIN compose_strategies s ON s.team_id = m.team_id AND s.game_id = sc.game_id WHERE s.id = ? ORDER BY m.id LIMIT 2`, donorStrategy).Scan(&materials).Error; err != nil || len(materials) != 2 {
		t.Fatalf("read two materials of strategy %d: %v", donorStrategy, err)
	}
	for i, material := range materials {
		result := tx.Exec(`INSERT INTO compose_tasks (team_id, game_id, operator_id, trigger_type, schedule_slot, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, request_key, created_by, created_at, updated_at) SELECT s.team_id, s.game_id, s.operator_id, 'scheduled', ?, ?, NULL, s.id, t.code, s.compose_template_version, s.config_json, 'pending', ?, s.created_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version WHERE s.id = ?`, slot, material, fmt.Sprintf("%s-%d", requestKey, i), donorStrategy)
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
