package repository

import (
	"os"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestLiveComposeStrategyReadAfterScheduleMigration(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("set WT_MEDIA_TEST_LIVE_DB=1 after applying migrations to verify real MySQL readback")
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
	items, err := (StrategyStore{}).List(StrategyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		stored, found, err := (StrategyStore{}).Find(item.ID)
		if err != nil || !found || stored.ID != item.ID {
			t.Fatalf("strategy %d readback: found=%v err=%v", item.ID, found, err)
		}
	}
	if len(items) == 0 {
		t.Skip("no existing strategy to verify transactional schedule readback")
	}
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	item := items[0]
	now := time.Now().UTC().Truncate(time.Microsecond)
	next := now.Add(15 * time.Minute)
	item.Schedule = &model.ComposeSchedule{Mode: "interval", IntervalMinutes: 15, Timezone: "Asia/Shanghai"}
	item.ExecutionMode = model.ComposeExecutionScheduled
	item.NextRunAt, item.UpdatedBy = &next, item.CreatedBy
	stored, err := updateComposeStrategyAudited(tx, item, "compose_strategy.schedule", now)
	if err != nil || stored.Schedule == nil || stored.ExecutionMode != model.ComposeExecutionScheduled || stored.NextRunAt == nil || !stored.NextRunAt.Equal(next) {
		t.Fatalf("transactional update/readback: stored=%+v err=%v", stored, err)
	}
	var persistedID int64
	if err := tx.Raw("SELECT id FROM compose_strategies WHERE id = ? AND schedule_json IS NOT NULL", item.ID).Row().Scan(&persistedID); err != nil || persistedID != item.ID {
		t.Fatalf("database schedule row: %v", err)
	}
}
