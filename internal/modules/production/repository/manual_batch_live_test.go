package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestLiveManualBatchExpansionOwnerUsageReplayAndRemake(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires migrated local MySQL")
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
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	now := time.Now().UTC().Truncate(time.Microsecond)
	var donor int64
	err = tx.Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id=s.compose_template_id AND t.version=s.compose_template_version JOIN users u ON u.id=s.operator_id AND u.status='enabled' JOIN user_category_scopes cs ON cs.user_id=u.id AND cs.category_id=s.category_id WHERE s.status='enabled' AND t.code='basic_cut' AND t.status='active' LIMIT 1`).Row().Scan(&donor)
	if err != nil {
		t.Fatalf("needs eligible local basic_cut fixture: %v", err)
	}
	donorStrategy, err := scanComposeStrategy(tx.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id=?`, donor).Row())
	if err != nil {
		t.Fatal(err)
	}
	var actor int64
	if err = tx.Raw(`SELECT id FROM users WHERE id<>? AND status='enabled' LIMIT 1`, *donorStrategy.OperatorID).Row().Scan(&actor); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("CHG116-manual-%d", now.UnixNano())
	if err = tx.Exec(`INSERT INTO compose_strategies(team_id,category_id,operator_id,compose_template_id,compose_template_version,name,config_json,execution_mode,priority_rules_json,single_limit,daily_limit,status,created_by,updated_by,created_at,updated_at) SELECT team_id,category_id,operator_id,compose_template_id,compose_template_version,?,config_json,'manual',priority_rules_json,1,0,'enabled',created_by,updated_by,?,? FROM compose_strategies WHERE id=?`, name, now, now, donor).Error; err != nil {
		t.Fatal(err)
	}
	var strategyID int64
	if err = tx.Raw(`SELECT id FROM compose_strategies WHERE name=?`, name).Row().Scan(&strategyID); err != nil {
		t.Fatal(err)
	}
	groups := []model.ComposeOutputSetting{{Key: "one", Name: "短版", Platforms: []string{"bilibili", "baidu"}, Copies: 2, Params: map[string]any{"start_seconds": 0, "end_seconds": 2}}, {Key: "two", Name: "长版", Platforms: []string{"xiaohongshu"}, Copies: 3, Params: map[string]any{"start_seconds": 0, "end_seconds": 4}}}
	raw, _ := json.Marshal(groups)
	if err = tx.Exec(`UPDATE compose_strategies SET output_settings_json=? WHERE id=?`, raw, strategyID).Error; err != nil {
		t.Fatal(err)
	}
	var materialIDs []int64
	err = tx.Raw(`SELECT m.id FROM materials m JOIN source_contents sc ON sc.id=m.source_content_id WHERE m.team_id=? AND sc.category_id=? AND m.status='available' AND ((m.video_status='ready' AND COALESCE(m.source_object_key,'')<>'') OR COALESCE(sc.source_url,'')<>'') ORDER BY m.id LIMIT 2`, donorStrategy.TeamID, donorStrategy.CategoryID).Scan(&materialIDs).Error
	if err != nil || len(materialIDs) != 2 {
		t.Fatalf("needs two eligible material fixtures: %v %v", materialIDs, err)
	}
	for _, materialID := range materialIDs {
		if _, _, err = createOrRestoreUsage(tx, CreateUsageInput{TeamID: sharedidentity.TeamID(donorStrategy.TeamID), MaterialID: materialID, UserID: sharedidentity.UserID(*donorStrategy.OperatorID)}, now); err != nil {
			t.Fatal(err)
		}
	}
	command := ManualBatchCommand{ActorID: actor, StrategyID: strategyID, OperatorID: *donorStrategy.OperatorID, Name: name, RequestKey: name, IntentHash: "intent-hash", MaterialIDs: materialIDs, Authorize: func(model.ComposeStrategy) error { return nil }, ResolveSettings: func(s model.ComposeStrategy, _ model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		return s.EffectiveOutputSettings(), nil
	}}
	result, err := createManualBatch(tx, command, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Batch == nil || len(result.Created) != 10 || len(result.Failures) != 0 {
		t.Fatalf("result=%+v", result)
	}
	batchID := result.Batch.ID
	// Pagination totals describe the filtered set while batch counts stay global.
	firstPage, err := listComposeTaskPage(tx, ComposeTaskFilter{BatchID: batchID}, 1, 3)
	if err != nil || firstPage.Total != 10 || len(firstPage.Items) != 3 {
		t.Fatalf("firstpage=%+v err=%v", firstPage, err)
	}
	secondPage, err := listComposeTaskPage(tx, ComposeTaskFilter{BatchID: batchID}, 2, 3)
	if err != nil || secondPage.Total != 10 || len(secondPage.Items) != 3 || secondPage.Items[0].ID >= firstPage.Items[2].ID {
		t.Fatalf("secondpage=%+v err=%v", secondPage, err)
	}
	if err = tx.Exec(`UPDATE compose_tasks SET status='failed',error_code='execution_uncertain' WHERE id IN (?,?)`, result.Created[0].ID, result.Created[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	failedPage, err := listComposeTaskPage(tx, ComposeTaskFilter{BatchID: batchID, Status: model.ComposeTaskFailed}, 1, 1)
	if err != nil || failedPage.Total != 2 || len(failedPage.Items) != 1 {
		t.Fatalf("failedpage=%+v err=%v", failedPage, err)
	}
	batchPage, err := listComposeBatchPage(tx, ComposeTaskFilter{StrategyID: strategyID, Status: model.ComposeTaskFailed}, 1, 1)
	if err != nil || batchPage.Total != 1 || len(batchPage.Items) != 1 || batchPage.Items[0].Counts.Total() != 10 || batchPage.Items[0].Counts.Failed != 2 {
		t.Fatalf("batchpage=%+v err=%v", batchPage, err)
	}
	retrySkipped, cancelSkipped, err := batchActionSkips(tx, batchID)
	if err != nil || len(retrySkipped) != 2 || retrySkipped[0].Count != 2 || len(cancelSkipped) != 1 || cancelSkipped[0].Count != 2 {
		t.Fatalf("actions retry=%+v cancel=%+v err=%v", retrySkipped, cancelSkipped, err)
	}
	missingBatchPage, err := listComposeBatchPage(tx, ComposeTaskFilter{StrategyID: strategyID, BatchID: 9223372036854000}, 1, 10)
	if err != nil || missingBatchPage.Total != 0 || len(missingBatchPage.Items) != 0 {
		t.Fatalf("missing batchfilter=%+v err=%v", missingBatchPage, err)
	}

	for _, task := range result.Created {
		if task.BatchID == nil || *task.BatchID != batchID || task.CreatedBy != actor || task.OperatorID == nil || *task.OperatorID != *donorStrategy.OperatorID || task.MaterialUsageID == nil {
			t.Fatalf("task ownership=%+v", task)
		}
		var usageOwner int64
		if err = tx.Raw(`SELECT user_id FROM material_usages WHERE id=?`, *task.MaterialUsageID).Row().Scan(&usageOwner); err != nil || usageOwner != *donorStrategy.OperatorID {
			t.Fatalf("usageowner=%d err=%v", usageOwner, err)
		}
	}
	// A replay reads its stored command before invalid/disabled current settings.
	if err = tx.Exec(`UPDATE compose_strategies SET status='disabled',output_settings_json=JSON_ARRAY() WHERE id=?`, strategyID).Error; err != nil {
		t.Fatal(err)
	}
	replay, err := createManualBatch(tx, command, now)
	if err != nil || !replay.Replayed || replay.Batch.ID != batchID || len(replay.Created) != 10 {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	changed := command
	changed.IntentHash = "different"
	if _, err = createManualBatch(tx, changed, now); err != ErrManualBatchConflict {
		t.Fatalf("changed body err=%v", err)
	}
	if err = tx.Exec(`UPDATE compose_strategies SET status='enabled',output_settings_json=? WHERE id=?`, raw, strategyID).Error; err != nil {
		t.Fatal(err)
	}
	command.RequestKey += "-remake"
	command.IntentHash = "new-command"
	denied, err := createManualBatch(tx, command, now)
	if err != nil || denied.Batch != nil || len(denied.Failures) != 2 {
		t.Fatalf("unconfirmed=%+v err=%v", denied, err)
	}
	command.RemakeConfirmed = true
	remake, err := createManualBatch(tx, command, now)
	if err != nil || remake.Batch == nil || remake.Batch.ID == batchID || len(remake.Created) != 10 {
		t.Fatalf("remake=%+v err=%v", remake, err)
	}
	// Partial acceptance freezes the rejection results together with the batch.
	partialCommand := command
	partialCommand.RequestKey = fmt.Sprintf("partial-%d", now.UnixNano())
	partialCommand.MaterialIDs = []int64{materialIDs[0], 9223372036854000}
	partial, err := createManualBatch(tx, partialCommand, now)
	if err != nil || partial.Batch == nil || len(partial.Created) != 5 || len(partial.Failures) != 1 {
		t.Fatalf("partial=%+v err=%v", partial, err)
	}
	partialReplay, err := createManualBatch(tx, partialCommand, now)
	if err != nil || !partialReplay.Replayed || partialReplay.Batch.ID != partial.Batch.ID || len(partialReplay.Created) != 5 || len(partialReplay.Failures) != 1 || partialReplay.Failures[0] != partial.Failures[0] {
		t.Fatalf("partial replay=%+v err=%v", partialReplay, err)
	}
	// A task foreign-key failure after batch insertion rolls the whole command back.
	rollbackCommand := command
	rollbackCommand.RequestKey += "-rollback"
	rollbackCommand.ActorID = 9223372036854000
	if _, err = createManualBatch(tx, rollbackCommand, now); err == nil {
		t.Fatal("expected task creator foreign-key failure")
	}
	var leaked int
	if err = tx.Raw(`SELECT COUNT(*) FROM compose_task_batches WHERE request_key=?`, rollbackCommand.RequestKey).Row().Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("rollback left %d batches err=%v", leaked, err)
	}
	// Strict team/category guards apply even to a caller authorized as administrator.
	var outside int64
	if err = tx.Raw(`SELECT m.id FROM materials m JOIN source_contents sc ON sc.id=m.source_content_id WHERE m.team_id<>? OR sc.category_id<>? LIMIT 1`, donorStrategy.TeamID, donorStrategy.CategoryID).Row().Scan(&outside); err != nil {
		t.Fatal(err)
	}
	command.RequestKey += "-outside"
	command.MaterialIDs = []int64{outside}
	rejected, err := createManualBatch(tx, command, now)
	if err != nil || rejected.Batch != nil || len(rejected.Failures) != 1 || rejected.Failures[0].Code != "forbidden" {
		t.Fatalf("outside=%+v err=%v", rejected, err)
	}
}
