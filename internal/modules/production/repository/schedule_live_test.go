package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestLiveComposeScheduleDueAndRepeatScan(t *testing.T) {
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
	if _, err := (ComposeScheduleStore{}).DueStrategyIDs(time.Now().UTC()); err != nil {
		t.Fatalf("due strategy scan: %v", err)
	}
	var sourceID int64
	err = database.DB().Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN users u ON u.id = s.operator_id AND u.status = 'enabled' AND u.role IN ('operator', 'senior_operator') AND u.team_id = s.team_id JOIN user_game_scopes g ON g.user_id = u.id AND g.game_id = s.game_id WHERE s.status = 'enabled' AND t.status = 'active' AND EXISTS (SELECT 1 FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.team_id = s.team_id AND sc.game_id = s.game_id AND m.status = 'available') ORDER BY s.id LIMIT 1`).Row().Scan(&sourceID)
	if err != nil {
		t.Skipf("no active strategy with an available operator: %v", err)
	}
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC().Truncate(time.Microsecond)
	due := now.Add(-time.Minute)
	schedule := model.ComposeSchedule{Mode: "interval", IntervalMinutes: 30, Timezone: "Asia/Shanghai"}
	scheduleJSON, _ := json.Marshal(schedule)
	// The live database already holds real run history, so the run assertions
	// need a strategy whose history this test owns. The fixture copies the
	// team, operator, game and template of an eligible strategy and adds a
	// match-all rule; the surrounding transaction removes it again.
	rules, err := json.Marshal([]model.ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []model.ComposeCondition{{Field: "like_count", Operator: "gte", Value: json.RawMessage(`0`)}}}})
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("CHG105-live-schedule-%d", time.Now().UnixNano())
	insert := tx.Exec(`INSERT INTO compose_strategies (team_id, game_id, operator_id, compose_template_id, compose_template_version, name, config_json, execution_mode, schedule_json, priority_rules_json, single_limit, daily_limit, next_run_at, status, created_by, updated_by, created_at, updated_at) SELECT team_id, game_id, operator_id, compose_template_id, compose_template_version, ?, config_json, 'scheduled', ?, ?, 1, 2, ?, 'enabled', created_by, updated_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies WHERE id = ?`, name, scheduleJSON, rules, due, sourceID)
	if insert.Error != nil || insert.RowsAffected != 1 {
		t.Fatalf("create fixture strategy from %d: rows=%d err=%v", sourceID, insert.RowsAffected, insert.Error)
	}
	var strategyID int64
	if err := tx.Raw(`SELECT id FROM compose_strategies WHERE name = ?`, name).Row().Scan(&strategyID); err != nil {
		t.Fatal(err)
	}
	calculate := func(_ model.ComposeStrategy, at time.Time) (*time.Time, error) {
		value := at.Add(30 * time.Minute)
		return &value, nil
	}
	options := ScheduleRunOptions{StrategyID: strategyID, Now: now, NextRun: calculate}
	first, processed, err := runComposeSchedule(tx, options)
	t.Logf("strategy=%d candidates=%d created=%d skipped=%v", strategyID, first.CandidateCount, first.CreatedCount, first.Skipped)
	if err != nil || !processed || first.TriggerType != "scheduled" || first.CreatedCount > 2 {
		t.Fatalf("first=%+v processed=%v err=%v", first, processed, err)
	}
	runs, err := listComposeScheduleRuns(tx, strategyID, 20)
	if err != nil || len(runs) != 1 || runs[0].RunKey != first.RunKey || runs[0].CandidateCount != first.CandidateCount || len(runs[0].Skipped) != len(first.Skipped) {
		t.Fatalf("schedule run readback=%+v err=%v", runs, err)
	}
	var taskCount int
	if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot = ?`, strategyID, first.ScheduleSlot).Row().Scan(&taskCount); err != nil || taskCount != first.CreatedCount {
		t.Fatalf("task count=%d want=%d err=%v", taskCount, first.CreatedCount, err)
	}
	if first.CreatedCount > 0 {
		var materialID int64
		var usageID sql.NullInt64
		if err := tx.Raw(`SELECT material_id, material_usage_id FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot = ? LIMIT 1`, strategyID, first.ScheduleSlot).Row().Scan(&materialID, &usageID); err != nil {
			t.Fatal(err)
		}
		if usageID.Valid {
			t.Fatalf("scheduled task must not reference a personal usage, got %d", usageID.Int64)
		}
		candidate := scheduleCandidate{MaterialID: materialID, GameID: sql.NullString{String: "", Valid: false}, Status: "available"}
		if err := tx.Raw(`SELECT sc.game_id FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.id = ?`, materialID).Row().Scan(&candidate.GameID); err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(`UPDATE compose_tasks SET status = 'failed' WHERE compose_strategy_id = ? AND schedule_slot = ?`, strategyID, first.ScheduleSlot).Error; err != nil {
			t.Fatal(err)
		}
		strategy, err := scanComposeStrategy(tx.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id = ?`, strategyID).Row())
		if err != nil {
			t.Fatal(err)
		}
		// The limit checks sit behind the "already run" check, so they have to be
		// asked about a pair with no task row at all — the id is deliberately one
		// that cannot exist, which is exactly that shape.
		fresh := candidate
		fresh.MaterialID = -999
		for _, check := range []struct {
			created, daily int
			want           string
		}{{0, strategy.DailyLimit, "daily_limit"}, {strategy.SingleLimit, 0, "per_run_limit"}} {
			reason, err := composeCandidateSkipReason(tx, strategy, fresh, check.created, check.daily)
			if err != nil || reason != check.want {
				t.Fatalf("limit skip=%q want=%q err=%v", reason, check.want, err)
			}
		}
		// Any task row for the pair skips the candidate, whatever its status:
		// the scheduler and the create guard apply one rule, and running the
		// pair again is the retry action on that task.
		for _, status := range []string{"success", "failed", "cancelled", "pending"} {
			if err := tx.Exec(`UPDATE compose_tasks SET status = ? WHERE compose_strategy_id = ? AND schedule_slot = ?`, status, strategyID, first.ScheduleSlot).Error; err != nil {
				t.Fatal(err)
			}
			reason, err := composeCandidateSkipReason(tx, strategy, candidate, 0, 0)
			if err != nil || reason != "previously_run" {
				t.Fatalf("%s material skip=%q err=%v", status, reason, err)
			}
		}
	}
	second, processed, err := runComposeSchedule(tx, options)
	if err != nil || processed || second.CreatedCount != 0 {
		t.Fatalf("repeat=%+v processed=%v err=%v", second, processed, err)
	}
	manual := ScheduleRunOptions{StrategyID: strategyID, Now: now.Add(time.Minute), ManualKey: "test-immediate-001", NextRun: calculate}
	immediate, processed, err := runComposeSchedule(tx, manual)
	if err != nil || !processed || immediate.TriggerType != "manual" {
		t.Fatalf("immediate=%+v processed=%v err=%v", immediate, processed, err)
	}
	repeated, processed, err := runComposeSchedule(tx, manual)
	if err != nil || !processed || repeated.RunKey != immediate.RunKey {
		t.Fatalf("immediate repeat=%+v processed=%v err=%v", repeated, processed, err)
	}
	var nextRun time.Time
	if err := tx.Raw(`SELECT next_run_at FROM compose_strategies WHERE id = ?`, strategyID).Row().Scan(&nextRun); err != nil || !nextRun.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("next run=%v err=%v", nextRun, err)
	}
}

// The quotas and the operator gate are part of the same run path, so they are
// measured on real rows: a strategy with single_limit=1 and daily_limit=1
// creates one task on its first immediate run, and the second immediate run
// records daily_limit for every remaining pair instead of creating another.
// A strategy with no operator records operator_unavailable and no task.
func TestLiveComposeScheduleQuotaAndOperatorGate(t *testing.T) {
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
	var sourceID int64
	err = database.DB().Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN users u ON u.id = s.operator_id AND u.status = 'enabled' AND u.role IN ('operator', 'senior_operator') AND u.team_id = s.team_id JOIN user_game_scopes g ON g.user_id = u.id AND g.game_id = s.game_id WHERE s.status = 'enabled' AND t.status = 'active' AND EXISTS (SELECT 1 FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.team_id = s.team_id AND sc.game_id = s.game_id AND m.status = 'available') ORDER BY s.id LIMIT 1`).Row().Scan(&sourceID)
	if err != nil {
		t.Skipf("no active strategy with an available operator and a team material: %v", err)
	}
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC().Truncate(time.Microsecond)
	rules, err := json.Marshal([]model.ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []model.ComposeCondition{{Field: "like_count", Operator: "gte", Value: json.RawMessage(`0`)}}}})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(operator string, singleLimit, dailyLimit int) int64 {
		name := fmt.Sprintf("CHG105-live-%s-%d", operator, time.Now().UnixNano())
		result := tx.Exec(`INSERT INTO compose_strategies (team_id, game_id, operator_id, compose_template_id, compose_template_version, name, config_json, execution_mode, schedule_json, priority_rules_json, single_limit, daily_limit, next_run_at, status, created_by, updated_by, created_at, updated_at) SELECT team_id, game_id, `+operator+`, compose_template_id, compose_template_version, ?, config_json, 'manual', NULL, ?, ?, ?, NULL, 'enabled', created_by, updated_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies WHERE id = ?`, name, rules, singleLimit, dailyLimit, sourceID)
		if result.Error != nil || result.RowsAffected != 1 {
			t.Fatalf("create %s fixture: rows=%d err=%v", operator, result.RowsAffected, result.Error)
		}
		var id int64
		if err := tx.Raw(`SELECT id FROM compose_strategies WHERE name = ?`, name).Row().Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	noAdvance := func(_ model.ComposeStrategy, _ time.Time) (*time.Time, error) {
		t.Fatal("an immediate run must not advance the schedule")
		return nil, nil
	}
	quotaID := insert("operator_id", 1, 1)
	first, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: quotaID, Now: now, ManualKey: "live-quota-001", NextRun: noAdvance})
	if err != nil || !processed || first.CreatedCount != 1 {
		t.Fatalf("quota first run=%+v processed=%v err=%v", first, processed, err)
	}
	second, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: quotaID, Now: now, ManualKey: "live-quota-002", NextRun: noAdvance})
	if err != nil || !processed || second.CreatedCount != 0 {
		t.Fatalf("quota second run=%+v processed=%v err=%v", second, processed, err)
	}
	dailyLimited := 0
	for _, skipped := range second.Skipped {
		if skipped.Reason == "daily_limit" {
			dailyLimited++
		}
	}
	if dailyLimited == 0 {
		t.Fatalf("second immediate run must record daily_limit, got %v", second.Skipped)
	}
	missingID := insert("NULL", 1, 1)
	missing, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: missingID, Now: now, ManualKey: "live-no-operator-001", NextRun: noAdvance})
	if err != nil || !processed || missing.CreatedCount != 0 || len(missing.Skipped) != 1 || missing.Skipped[0].Reason != "operator_unavailable" {
		t.Fatalf("missing operator run=%+v processed=%v err=%v", missing, processed, err)
	}
	var missingTasks int
	if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ?`, missingID).Row().Scan(&missingTasks); err != nil || missingTasks != 0 {
		t.Fatalf("missing operator created %d tasks: %v", missingTasks, err)
	}
	t.Logf("quota strategy=%d first=%d second=%d daily_limited=%d; missing operator strategy=%d skipped=%v", quotaID, first.CreatedCount, second.CreatedCount, dailyLimited, missingID, missing.Skipped)
}
