package repository

import (
	"encoding/json"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveOutputPaginationAndMaterialAggregationScope(t *testing.T) {
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
	var strategyID int64
	if err = tx.Raw("SELECT id FROM compose_strategies WHERE operator_id IS NOT NULL LIMIT 1").Row().Scan(&strategyID); err != nil {
		t.Fatal(err)
	}
	strategy, _, err := findComposeStrategy(tx, strategyID)
	if err != nil {
		t.Fatal(err)
	}
	var materials []int64
	if err = tx.Raw("SELECT id FROM materials WHERE team_id=? ORDER BY id LIMIT 2", strategy.TeamID).Scan(&materials).Error; err != nil || len(materials) != 2 {
		t.Fatalf("materials=%v err=%v", materials, err)
	}
	var foreign int64
	if err = tx.Raw("SELECT id FROM users WHERE id<>? LIMIT 1", *strategy.OperatorID).Row().Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	historicCategory := strategy.CategoryID
	if err = tx.Raw("SELECT id FROM categories WHERE id<>? LIMIT 1", strategy.CategoryID).Row().Scan(&historicCategory); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	ids := []int64{}
	key := fmt.Sprintf("output117-%d", now.UnixNano())
	for i := 0; i < 5; i++ {
		owner := *strategy.OperatorID
		if i == 4 {
			owner = foreign
		}
		material := materials[0]
		if i == 3 {
			material = materials[1]
		}
		task := model.ComposeTask{TeamID: strategy.TeamID, CategoryID: strategy.CategoryID, OperatorID: &owner, MaterialID: material, StrategyID: strategy.ID, TemplateCode: "basic_cut", TemplateVersion: strategy.TemplateVersion, ConfigSnapshot: json.RawMessage(`{"start_seconds":0,"end_seconds":1}`), PlatformsJSON: json.RawMessage(`["bilibili"]`), SettingKey: "one", SettingName: "短版", Status: model.ComposeTaskSuccess, TriggerType: "manual", CreatedBy: owner, RequestKey: fmt.Sprintf("%s-%d", key, i)}
		if i == 2 {
			task.CategoryID = historicCategory
		}
		if err = composeTaskInsertWithAudit(tx, &task, "compose_task.create", owner, now); err != nil {
			t.Fatal(err)
		}
		output := model.CompositeOutput{Name: "测试标题 · 短版", TeamID: task.TeamID, CategoryID: task.CategoryID, OperatorID: &owner, MaterialID: material, TaskID: task.ID, StrategyID: task.StrategyID, TemplateCode: task.TemplateCode, TemplateVersion: task.TemplateVersion, ObjectKey: fmt.Sprintf("%s-%d", key, i), FileSizeBytes: 12, SHA256: strings.Repeat("a", 64), Media: json.RawMessage(`{"width":1080,"height":1920,"duration_ms":1000}`), PublishStatus: "pending_publish", CreatedBy: owner, CreatedAt: now, UpdatedAt: now}
		if err = tx.Create(&output).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, output.ID)
	}
	filter := ComposeOutputFilter{TeamID: strategy.TeamID, OperatorID: *strategy.OperatorID, OutputIDs: ids, RestrictIDs: true}
	refs, err := outputObjectRefs(tx, filter)
	if err != nil || len(refs) != 4 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref.ObjectKey, key) {
			t.Fatalf("object ref leaked scope: %+v", ref)
		}
	}
	template, found, err := (ComposeOutputStore{}).OriginalTemplate("basic_cut", 1)
	if err != nil || !found || len(template.Schema) == 0 || template.ExecutorID == "" {
		t.Fatalf("template=%+v found=%v err=%v", template, found, err)
	}
	first, err := pageComposeOutputs(tx, filter, 1, 2)
	if err != nil || first.Total != 4 || len(first.Items) != 2 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := pageComposeOutputs(tx, filter, 2, 2)
	if err != nil || second.Total != 4 || len(second.Items) != 2 || second.Items[0].ID >= first.Items[1].ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if len(first.Items[0].Platforms) != 1 || first.Items[0].SettingName != "短版" || first.Items[0].StrategyName == "" {
		t.Fatalf("projection=%+v", first.Items[0])
	}
	aggregate, err := pageComposeOutputMaterials(tx, filter, 1, 1)
	if err != nil || aggregate.Total != 2 || len(aggregate.Items) != 1 {
		t.Fatalf("aggregate=%+v err=%v", aggregate, err)
	}
	all, err := pageComposeOutputMaterials(tx, filter, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[int64]int64{}
	for _, m := range all.Items {
		counts[m.MaterialID] = m.OutputCount
		if len(m.Strategies) != 1 || m.Strategies[0].ID != strategy.ID {
			t.Fatalf("strategies=%+v", m.Strategies)
		}
	}
	if counts[materials[0]] != 3 || counts[materials[1]] != 1 {
		t.Fatalf("counts=%v", counts)
	}
	filter.MaterialID = materials[1]
	filter.Platform = "bilibili"
	filtered, err := pageComposeOutputs(tx, filter, 1, 20)
	if err != nil || filtered.Total != 1 {
		t.Fatalf("filtered=%+v err=%v", filtered, err)
	}
	filter.Platform = "baidu"
	empty, err := pageComposeOutputs(tx, filter, 1, 20)
	if err != nil || empty.Total != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}
