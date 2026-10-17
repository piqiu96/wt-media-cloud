package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestLiveStrategyRulesRoundTripAndImmediateRun(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("set WT_MEDIA_TEST_LIVE_DB=1 for local MySQL verification")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.HomeEnvVar, root)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Databases) != 1 || cfg.Databases[0].Host != "127.0.0.1" || cfg.Databases[0].Database != "wt_media_cloud" {
		t.Fatal("live strategy test only accepts local development database")
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	var userID, teamID, templateID int64
	var gameID string
	if err := tx.Raw(`SELECT mu.user_id, mu.team_id, sc.game_id FROM material_usages mu JOIN materials m ON m.id = mu.material_id JOIN source_contents sc ON sc.id = m.source_content_id JOIN users u ON u.id = mu.user_id WHERE mu.status = 'active' AND m.status = 'available' AND u.status = 'enabled' AND sc.game_id IS NOT NULL ORDER BY mu.id LIMIT 1`).Row().Scan(&userID, &teamID, &gameID); err != nil {
		t.Skipf("no eligible local material: %v", err)
	}
	if err := tx.Raw(`SELECT id FROM compose_templates WHERE code = 'basic_cut' AND version = 1 AND status = 'active' ORDER BY id LIMIT 1`).Row().Scan(&templateID); err != nil {
		t.Skipf("no active basic cut template: %v", err)
	}
	rules := []model.ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []model.ComposeCondition{{Field: "like_count", Operator: "gte", Value: json.RawMessage(`0`)}}}}
	rulesJSON, _ := json.Marshal(rules)
	strategy := model.ComposeStrategy{TeamID: teamID, GameID: gameID, TemplateID: templateID, TemplateVersion: 1, Name: "本机事务验证", Config: json.RawMessage(`{"start_seconds":0,"end_seconds":10}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: rules, RulesJSON: rulesJSON, SingleLimit: 1, DailyLimit: 1, Status: model.ComposeStrategyEnabled, CreatedBy: userID, UpdatedBy: userID}
	stored, err := createComposeStrategyAudited(tx, strategy, "compose_strategy.create", time.Now().UTC())
	if err != nil || stored.ID == 0 || len(stored.PriorityRules) != 1 || stored.PriorityRules[0].Priority != 1 {
		t.Fatalf("strategy readback=%+v err=%v", stored, err)
	}
	next := func(_ model.ComposeStrategy, _ time.Time) (*time.Time, error) {
		t.Fatal("manual run changed the schedule")
		return nil, nil
	}
	run, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: stored.ID, Now: time.Now().UTC(), ManualKey: "local-rule-proof-001", NextRun: next})
	if err != nil || !processed || run.CreatedCount != 1 || run.TriggerType != "manual" {
		t.Fatalf("run=%+v processed=%v err=%v", run, processed, err)
	}
	var tasks int
	if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot = ?`, stored.ID, run.ScheduleSlot).Row().Scan(&tasks); err != nil || tasks != 1 {
		t.Fatalf("task readback=%d err=%v", tasks, err)
	}
	plan := model.ComposeSchedule{Mode: "interval", IntervalMinutes: 15, Timezone: "Asia/Shanghai"}
	due := time.Now().UTC().Add(-time.Minute)
	stored.ExecutionMode, stored.Schedule, stored.NextRunAt = model.ComposeExecutionScheduled, &plan, &due
	stored, err = updateComposeStrategyAudited(tx, stored, "compose_strategy.update", time.Now().UTC())
	if err != nil || stored.ExecutionMode != model.ComposeExecutionScheduled || stored.NextRunAt == nil {
		t.Fatalf("scheduled readback=%+v err=%v", stored, err)
	}
	_, processed, err = runComposeSchedule(tx, ScheduleRunOptions{StrategyID: stored.ID, Now: time.Now().UTC(), ManualKey: "local-rule-proof-002", NextRun: next})
	if err != nil || !processed {
		t.Fatalf("scheduled strategy manual run processed=%v err=%v", processed, err)
	}
	stored, _, err = findComposeStrategy(tx, stored.ID)
	if err != nil || stored.NextRunAt == nil || !stored.NextRunAt.Equal(due) {
		t.Fatalf("manual trigger changed due time=%v err=%v", stored.NextRunAt, err)
	}
	advance := func(_ model.ComposeStrategy, at time.Time) (*time.Time, error) {
		value := at.Add(15 * time.Minute)
		return &value, nil
	}
	_, processed, err = runComposeSchedule(tx, ScheduleRunOptions{StrategyID: stored.ID, Now: time.Now().UTC(), NextRun: advance})
	if err != nil || !processed {
		t.Fatalf("scheduled due run processed=%v err=%v", processed, err)
	}
	stored, _, err = findComposeStrategy(tx, stored.ID)
	if err != nil || stored.NextRunAt == nil || !stored.NextRunAt.After(time.Now()) {
		t.Fatalf("next run was not advanced=%v err=%v", stored.NextRunAt, err)
	}
	if err := tx.Exec(`UPDATE compose_strategies SET status = 'disabled' WHERE id = ?`, stored.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, processed, err = runComposeSchedule(tx, ScheduleRunOptions{StrategyID: stored.ID, Now: time.Now().UTC(), ManualKey: "local-rule-proof-003", NextRun: next})
	if err != nil || processed {
		t.Fatalf("disabled strategy processed=%v err=%v", processed, err)
	}
	var firstUsage, firstMaterial int64
	if err := tx.Raw(`SELECT mu.id, mu.material_id FROM material_usages mu JOIN materials m ON m.id = mu.material_id JOIN source_contents sc ON sc.id = m.source_content_id WHERE mu.user_id = ? AND mu.team_id = ? AND mu.status = 'active' AND m.status = 'available' AND sc.game_id = ? ORDER BY mu.id LIMIT 1`, userID, teamID, gameID).Row().Scan(&firstUsage, &firstMaterial); err != nil {
		t.Fatal(err)
	}
	blocker := strategy
	blocker.Name = "本机事务冲突占位"
	blocker, err = createComposeStrategyAudited(tx, blocker, "compose_strategy.create", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	failureStrategy := strategy
	failureStrategy.Name, failureStrategy.SingleLimit, failureStrategy.DailyLimit = "本机事务逐项失败", 2, 2
	failureStrategy, err = createComposeStrategyAudited(tx, failureStrategy, "compose_strategy.create", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	key := "local-rule-proof-failure"
	runKey := scheduleRunKey(failureStrategy.ID, time.Time{}, key)
	blockerTask := model.ComposeTask{TeamID: teamID, GameID: gameID, TriggerType: "manual", DisplayName: "占位任务", MaterialID: firstMaterial, MaterialUsageID: firstUsage, StrategyID: blocker.ID, TemplateCode: "basic_cut", TemplateVersion: 1, ConfigSnapshot: strategy.Config, Status: model.ComposeTaskPending, RequestKey: scheduledTaskKey(runKey, firstMaterial), CreatedBy: userID}
	if err := composeTaskInsertWithAudit(tx, &blockerTask, "compose_task.schedule", userID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	failureRun, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: failureStrategy.ID, Now: time.Now().UTC(), ManualKey: key, NextRun: next})
	failedItem := false
	for _, skipped := range failureRun.Skipped {
		if skipped.MaterialID == firstMaterial && skipped.Reason == "task_create_failed" {
			failedItem = true
		}
	}
	if err != nil || !processed || failureRun.CreatedCount != 2 || !failedItem {
		t.Fatalf("partial failure run=%+v processed=%v err=%v", failureRun, processed, err)
	}
	if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ?`, failureStrategy.ID).Row().Scan(&tasks); err != nil || tasks != 2 {
		t.Fatalf("partial success task readback=%d err=%v", tasks, err)
	}
}
