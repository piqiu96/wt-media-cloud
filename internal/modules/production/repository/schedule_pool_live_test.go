package repository

import (
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// CHG-20261026-105: scheduled production reads the strategy team's material
// pool. Neither the creator nor the operator has to "join" the material, and
// no personal usage row may be created on their behalf.
func TestLiveComposeScheduleReadsTheTeamPoolWithoutPersonalUsages(t *testing.T) {
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
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC().Truncate(time.Microsecond)

	var teamID, operatorID, creatorID int64
	var categoryID sharedidentity.CategoryID
	if err := tx.Raw(`SELECT u.id, u.team_id, g.category_id FROM users u JOIN user_category_scopes g ON g.user_id = u.id WHERE u.status = 'enabled' AND u.role IN ('operator', 'senior_operator') AND u.team_id IS NOT NULL ORDER BY u.id, g.category_id LIMIT 1`).Row().Scan(&operatorID, &teamID, &categoryID); err != nil {
		t.Skipf("no enabled operator with a category scope: %v", err)
	}
	if err := tx.Raw(`SELECT id FROM users WHERE role = 'admin' AND status = 'enabled' ORDER BY id LIMIT 1`).Row().Scan(&creatorID); err != nil {
		t.Skipf("no enabled administrator: %v", err)
	}
	var templateID int64
	if err := tx.Raw(`SELECT id FROM compose_templates WHERE code = 'basic_cut' AND version = 1 AND status = 'active' ORDER BY id LIMIT 1`).Row().Scan(&templateID); err != nil {
		t.Skipf("no active basic cut template: %v", err)
	}
	// A pool material whose like_count is unique inside the category, so the rule
	// below can select exactly this one; and no usage for either person.
	var materialID, likeCount int64
	if err := tx.Raw(`SELECT m.id, sc.like_count FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.team_id = ? AND sc.category_id = ? AND m.status = 'available' AND NOT EXISTS (SELECT 1 FROM material_usages mu WHERE mu.material_id = m.id AND mu.user_id IN (?, ?)) AND (SELECT COUNT(*) FROM materials m2 JOIN source_contents sc2 ON sc2.id = m2.source_content_id WHERE m2.team_id = ? AND sc2.category_id = ? AND sc2.like_count = sc.like_count) = 1 ORDER BY m.id LIMIT 1`, teamID, categoryID, creatorID, operatorID, teamID, categoryID).Row().Scan(&materialID, &likeCount); err != nil {
		t.Skipf("no team material without creator/operator usage: %v", err)
	}
	// A different material with a real usage lets the same transaction exercise
	// the manual "my materials" path, which must keep writing the usage id.
	var usageID, usageMaterialID, usageUserID int64
	var usageCategory sharedidentity.CategoryID
	if err := tx.Raw(`SELECT mu.id, mu.material_id, mu.user_id, sc.category_id FROM material_usages mu JOIN materials m ON m.id = mu.material_id JOIN source_contents sc ON sc.id = m.source_content_id WHERE mu.team_id = ? AND mu.status = 'active' AND m.status = 'available' AND mu.material_id <> ? ORDER BY mu.id LIMIT 1`, teamID, materialID).Row().Scan(&usageID, &usageMaterialID, &usageUserID, &usageCategory); err != nil {
		t.Skipf("team has no other active usage to exercise the manual path: %v", err)
	}

	rules := []model.ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []model.ComposeCondition{{Field: "like_count", Operator: "eq", Value: json.RawMessage(strconv.FormatInt(likeCount, 10))}}}}
	rulesJSON, _ := json.Marshal(rules)
	stored, err := createComposeStrategyAudited(tx, model.ComposeStrategy{
		TeamID: teamID, CategoryID: categoryID, OperatorID: &operatorID, TemplateID: templateID, TemplateVersion: 1,
		Name: "团队素材池验证", Config: json.RawMessage(`{"start_seconds":0,"end_seconds":10}`),
		ExecutionMode: model.ComposeExecutionManual, PriorityRules: rules, RulesJSON: rulesJSON,
		SingleLimit: 1, DailyLimit: 1, Status: model.ComposeStrategyEnabled,
		CreatedBy: creatorID, UpdatedBy: creatorID,
	}, "compose_strategy.create", now)
	if err != nil {
		t.Fatal(err)
	}

	candidates, err := listComposeScheduleCandidates(tx, stored)
	if err != nil {
		t.Fatal(err)
	}
	var teamMaterials int
	if err := tx.Raw(`SELECT COUNT(*) FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.team_id = ?`, teamID).Row().Scan(&teamMaterials); err != nil {
		t.Fatal(err)
	}
	if len(candidates) != teamMaterials || len(candidates) == 0 {
		t.Fatalf("candidates=%d want every one of the team's %d materials", len(candidates), teamMaterials)
	}
	pickedInPool := false
	for _, candidate := range candidates {
		if candidate.MaterialID == materialID {
			pickedInPool = true
		}
	}
	if !pickedInPool {
		t.Fatalf("material %d has no creator/operator usage and must still be a candidate", materialID)
	}
	var foreignMaterialID int64
	if err := tx.Raw(`SELECT m.id FROM materials m WHERE m.team_id <> ? ORDER BY m.id LIMIT 1`, teamID).Row().Scan(&foreignMaterialID); err == nil {
		for _, candidate := range candidates {
			if candidate.MaterialID == foreignMaterialID {
				t.Fatalf("cross-team material %d leaked into the candidate pool", foreignMaterialID)
			}
		}
	}
	var joinedUsages int
	if err := tx.Raw(`SELECT COUNT(*) FROM material_usages WHERE material_id = ? AND user_id IN (?, ?)`, materialID, creatorID, operatorID).Row().Scan(&joinedUsages); err != nil {
		t.Fatal(err)
	}
	if joinedUsages != 0 {
		t.Fatalf("fixture precondition broken: material %d already has %d usage rows", materialID, joinedUsages)
	}

	noAdvance := func(_ model.ComposeStrategy, _ time.Time) (*time.Time, error) { return nil, nil }
	run, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: stored.ID, Now: now, ManualKey: "t03-team-pool-001", NextRun: noAdvance})
	if err != nil || !processed || run.CreatedCount != 1 {
		t.Fatalf("run=%+v processed=%v err=%v", run, processed, err)
	}
	t.Logf("strategy=%d team=%d material=%d candidates=%d created=%d skipped=%v", stored.ID, teamID, materialID, len(candidates), run.CreatedCount, run.Skipped)
	var taskMaterial, taskCreator, taskOperator int64
	var taskUsage sql.NullInt64
	if err := tx.Raw(`SELECT material_id, created_by, operator_id, material_usage_id FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot = ?`, stored.ID, run.ScheduleSlot).Row().Scan(&taskMaterial, &taskCreator, &taskOperator, &taskUsage); err != nil {
		t.Fatal(err)
	}
	if taskMaterial != materialID || taskCreator != creatorID || taskOperator != operatorID || taskUsage.Valid {
		t.Fatalf("task material=%d created_by=%d operator=%d usage=%v", taskMaterial, taskCreator, taskOperator, taskUsage)
	}
	if err := tx.Raw(`SELECT COUNT(*) FROM material_usages WHERE material_id = ? AND user_id IN (?, ?)`, materialID, creatorID, operatorID).Row().Scan(&joinedUsages); err != nil {
		t.Fatal(err)
	}
	if joinedUsages != 0 {
		t.Fatalf("scheduling created %d personal usage rows for material %d", joinedUsages, materialID)
	}

	var crossCategory int
	if err := tx.Raw(`SELECT COUNT(*) FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.team_id = ? AND (sc.category_id IS NULL OR sc.category_id <> ?)`, teamID, categoryID).Row().Scan(&crossCategory); err != nil {
		t.Fatal(err)
	}
	if crossCategory > 0 {
		otherCategorySkipped := false
		for _, skipped := range run.Skipped {
			if skipped.Reason == "other_category" {
				otherCategorySkipped = true
			}
		}
		if !otherCategorySkipped {
			t.Fatalf("team has %d out-of-category materials but the run recorded no other_category skip: %v", crossCategory, run.Skipped)
		}
	} else if reason, err := composeCandidateSkipReason(tx, stored, scheduleCandidate{MaterialID: -999, CategoryID: sql.NullInt64{Int64: int64(categoryID) + 1000, Valid: true}, Status: "available"}, 0, 0); err != nil || reason != "other_category" {
		t.Fatalf("other_category skip=%q err=%v", reason, err)
	}
	if reason, err := composeCandidateSkipReason(tx, stored, scheduleCandidate{MaterialID: -999, CategoryID: sql.NullInt64{Int64: int64(categoryID), Valid: true}, Status: "paused"}, 0, 0); err != nil || reason != "material_unavailable" {
		t.Fatalf("material_unavailable skip=%q err=%v", reason, err)
	}

	repeat, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: stored.ID, Now: now, ManualKey: "t03-team-pool-002", NextRun: noAdvance})
	if err != nil || !processed || repeat.CreatedCount != 0 {
		t.Fatalf("repeat run=%+v processed=%v err=%v", repeat, processed, err)
	}
	previouslyRun := false
	for _, skipped := range repeat.Skipped {
		if skipped.MaterialID == materialID && skipped.Reason == "previously_run" {
			previouslyRun = true
		}
	}
	if !previouslyRun {
		t.Fatalf("second run must skip the produced pair as previously_run: %v", repeat.Skipped)
	}

	// The creator is only the audit origin. Disabling that person must not stop
	// an otherwise valid strategy from producing.
	rulesJSON, _ = json.Marshal(rules)
	disabledCreatorStrategy, err := createComposeStrategyAudited(tx, model.ComposeStrategy{
		TeamID: teamID, CategoryID: categoryID, OperatorID: &operatorID, TemplateID: templateID, TemplateVersion: 1,
		Name: "禁用创建者验证", Config: json.RawMessage(`{"start_seconds":0,"end_seconds":10}`),
		ExecutionMode: model.ComposeExecutionManual, PriorityRules: rules, RulesJSON: rulesJSON,
		SingleLimit: 1, DailyLimit: 1, Status: model.ComposeStrategyEnabled,
		CreatedBy: creatorID, UpdatedBy: creatorID,
	}, "compose_strategy.create", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`UPDATE users SET status = 'disabled' WHERE id = ?`, creatorID).Error; err != nil {
		t.Fatal(err)
	}
	disabledRun, processed, err := runComposeSchedule(tx, ScheduleRunOptions{StrategyID: disabledCreatorStrategy.ID, Now: now, ManualKey: "t03-creator-disabled-001", NextRun: noAdvance})
	if err != nil || !processed || disabledRun.CreatedCount != 1 {
		t.Fatalf("disabled creator run=%+v processed=%v err=%v", disabledRun, processed, err)
	}
	for _, skipped := range disabledRun.Skipped {
		if skipped.Reason == "creator_disabled" {
			t.Fatalf("a disabled creator must not block the run: %v", disabledRun.Skipped)
		}
	}
	var disabledTaskCreator int64
	if err := tx.Raw(`SELECT created_by FROM compose_tasks WHERE compose_strategy_id = ?`, disabledCreatorStrategy.ID).Row().Scan(&disabledTaskCreator); err != nil {
		t.Fatal(err)
	}
	if disabledTaskCreator != creatorID {
		t.Fatalf("task created_by=%d want the disabled creator %d", disabledTaskCreator, creatorID)
	}
	t.Logf("disabled creator strategy=%d created=%d task_created_by=%d", disabledCreatorStrategy.ID, disabledRun.CreatedCount, disabledTaskCreator)

	manual, written, err := createForUsage(tx, model.ComposeTask{
		TeamID: teamID, CategoryID: usageCategory, OperatorID: &operatorID, TriggerType: "manual",
		MaterialID: usageMaterialID, StrategyID: stored.ID, TemplateCode: "basic_cut", TemplateVersion: 1,
		ConfigSnapshot: json.RawMessage(`{}`), Status: model.ComposeTaskPending,
		RequestKey: "t03-manual-usage-001", CreatedBy: usageUserID,
	}, "compose_task.create", now)
	if err != nil || !written || manual.MaterialUsageID == nil || *manual.MaterialUsageID != usageID {
		t.Fatalf("manual task=%+v written=%v err=%v; want usage %d", manual, written, err, usageID)
	}
	t.Logf("manual task=%d usage=%d material=%d", manual.ID, usageID, usageMaterialID)
}
