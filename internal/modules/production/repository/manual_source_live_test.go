package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	identity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestLiveManualSourceKeepsOriginalSnapshotAndReplaysBeforeCurrentChecks(t *testing.T) {
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
	key := fmt.Sprintf("source117-%d", now.UnixNano())
	var donor int64
	err = tx.Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id=s.compose_template_id AND t.version=s.compose_template_version JOIN users u ON u.id=s.operator_id AND u.status='enabled' AND u.role IN ('operator','senior_operator') JOIN user_category_scopes cs ON cs.user_id=u.id AND cs.category_id=s.category_id WHERE s.status='enabled' AND t.code='basic_cut' AND t.version=1 AND t.status='active' LIMIT 1`).Row().Scan(&donor)
	if err != nil {
		t.Fatalf("needs eligible local basic_cut v1 strategy: %v", err)
	}
	strategy, err := scanComposeStrategy(tx.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id=?`, donor).Row())
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(`INSERT INTO compose_strategies(team_id,category_id,operator_id,compose_template_id,compose_template_version,name,config_json,execution_mode,priority_rules_json,single_limit,daily_limit,status,created_by,updated_by,created_at,updated_at) SELECT team_id,category_id,operator_id,compose_template_id,compose_template_version,?,config_json,'manual',priority_rules_json,1,0,'enabled',created_by,updated_by,?,? FROM compose_strategies WHERE id=?`, key, now, now, donor).Error; err != nil {
		t.Fatal(err)
	}
	if err = tx.Raw(`SELECT id FROM compose_strategies WHERE name=?`, key).Row().Scan(&strategy.ID); err != nil {
		t.Fatal(err)
	}
	var materialID int64
	if err = tx.Raw(`SELECT m.id FROM materials m JOIN source_contents sc ON sc.id=m.source_content_id WHERE m.team_id=? AND sc.category_id=? AND m.status='available' AND COALESCE(sc.source_url,'')<>'' ORDER BY m.id LIMIT 1`, strategy.TeamID, strategy.CategoryID).Row().Scan(&materialID); err != nil {
		t.Fatalf("needs accessible original material source: %v", err)
	}
	usage, _, err := createOrRestoreUsage(tx, CreateUsageInput{TeamID: identity.TeamID(strategy.TeamID), MaterialID: materialID, UserID: identity.UserID(*strategy.OperatorID)}, now)
	if err != nil {
		t.Fatal(err)
	}
	original := model.ComposeTask{TeamID: strategy.TeamID, CategoryID: strategy.CategoryID, OperatorID: strategy.OperatorID, MaterialID: materialID, MaterialUsageID: &usage.ID, StrategyID: strategy.ID, TemplateCode: "basic_cut", TemplateVersion: 1, ConfigSnapshot: json.RawMessage(`{"start_seconds":1,"end_seconds":3}`), PlatformsJSON: json.RawMessage(`["xiaohongshu"]`), SettingKey: "historic", SettingName: "原始版本", CopyIndex: 4, Status: model.ComposeTaskSuccess, TriggerType: "manual", CreatedBy: *strategy.OperatorID, RequestKey: key + "-original"}
	if err = composeTaskInsertWithAudit(tx, &original, "compose_task.create", original.CreatedBy, now); err != nil {
		t.Fatal(err)
	}
	output := model.CompositeOutput{TeamID: strategy.TeamID, CategoryID: strategy.CategoryID, OperatorID: strategy.OperatorID, MaterialID: materialID, MaterialUsageID: &usage.ID, TaskID: original.ID, StrategyID: strategy.ID, TemplateCode: original.TemplateCode, TemplateVersion: original.TemplateVersion, ObjectKey: "qa/absent-source-output/" + key, FileSizeBytes: 1, SHA256: strings.Repeat("a", 64), Media: json.RawMessage(`{"duration_ms":2000}`), PublishStatus: "pending_publish", CreatedBy: original.CreatedBy, CreatedAt: now, UpdatedAt: now}
	if err = tx.Create(&output).Error; err != nil {
		t.Fatal(err)
	}
	// The current strategy no longer contains the original group and now requests
	// 40 outputs with unrelated parameters. A source command must still make one.
	current := []model.ComposeOutputSetting{{Key: "new-one", Name: "当前设置一", Copies: 20, Platforms: []string{"baidu"}, Params: map[string]any{"start_seconds": 0, "end_seconds": 99}}, {Key: "new-two", Name: "当前设置二", Copies: 20, Platforms: []string{"bilibili"}, Params: map[string]any{"start_seconds": 0, "end_seconds": 199}}}
	raw, _ := json.Marshal(current)
	if err = tx.Exec(`UPDATE compose_strategies SET output_settings_json=?,config_json=JSON_OBJECT('start_seconds',0,'end_seconds',999) WHERE id=?`, raw, strategy.ID).Error; err != nil {
		t.Fatal(err)
	}
	inputChecks := 0
	command := ManualBatchCommand{SourceOutputID: output.ID, ActorID: original.CreatedBy, Name: "原成片再次制作", RequestKey: key + "-remake", IntentHash: key + "-intent", RemakeConfirmed: true, Authorize: func(s model.ComposeStrategy) error {
		if s.ID != strategy.ID || s.OperatorID == nil || *s.OperatorID != original.CreatedBy {
			return errors.New("unexpected scope")
		}
		return nil
	}, AuthorizeSource: func(o model.CompositeOutput) error {
		if o.ID != output.ID {
			return errors.New("unexpected source")
		}
		return nil
	}, ValidateSourceInput: func(m model.Material) error {
		inputChecks++
		if m.ID != materialID || m.SourceURL == "" {
			return errors.New("original input unavailable")
		}
		return nil
	}, ResolveSettings: func(s model.ComposeStrategy, template model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		if template.Code != original.TemplateCode || template.Version != original.TemplateVersion {
			return nil, errors.New("replaced original template version")
		}
		groups := s.EffectiveOutputSettings()
		for _, group := range groups {
			params, _ := json.Marshal(group.Params)
			definition, err := executor.DeployedDefinition(template.Code, template.Version)
			if err != nil {
				return nil, err
			}
			if _, err = definition.Executor.ValidateParams(params); err != nil {
				return nil, err
			}
		}
		return groups, nil
	}}
	result, err := createManualBatch(tx, command, now)
	if err != nil || result.Batch == nil || len(result.Created) != 1 || inputChecks != 1 {
		t.Fatalf("source result=%+v input checks=%d err=%v", result, inputChecks, err)
	}
	created := result.Created[0]
	if created.MaterialID != materialID || created.StrategyID != strategy.ID || *created.OperatorID != original.CreatedBy || created.SettingKey != "historic" || created.SettingName != "原始版本" || created.CopyIndex != 1 || string(created.PlatformsJSON) != `["xiaohongshu"]` || created.TemplateVersion != 1 {
		t.Fatalf("original task facts drifted: %+v", created)
	}
	var params map[string]any
	json.Unmarshal(created.ConfigSnapshot, &params)
	if params["start_seconds"] != float64(1) || params["end_seconds"] != float64(3) {
		t.Fatalf("original final parameters drifted: %s", created.ConfigSnapshot)
	}
	var snapshot manualBatchSnapshot
	if err = json.Unmarshal(result.Batch.ConfigSnapshot, &snapshot); err != nil || snapshot.SourceOutputID != output.ID || snapshot.SourceTaskID != original.ID || len(snapshot.Settings) != 1 || snapshot.Settings[0].Copies != 1 {
		t.Fatalf("source trace not retained: %+v err=%v", snapshot, err)
	}
	// Optional client echoes cannot substitute a strategy/owner, and the command
	// key cannot be reused with a different source intent.
	for _, ownerEcho := range []bool{false, true} {
		tampered := command
		tampered.RequestKey += fmt.Sprint("-tamper-", ownerEcho)
		if ownerEcho {
			tampered.OperatorID = original.CreatedBy + 1
		} else {
			tampered.StrategyID = strategy.ID + 1
		}
		if _, err = createManualBatch(tx, tampered, now); !errors.Is(err, ErrManualBatchInvalid) {
			t.Fatalf("tampered echo accepted: %v", err)
		}
	}
	changed := command
	changed.SourceOutputID++
	changed.IntentHash += "-other-source"
	if _, err = createManualBatch(tx, changed, now); !errors.Is(err, ErrManualBatchConflict) {
		t.Fatalf("changed source intent accepted: %v", err)
	}
	if err = tx.Exec(`UPDATE material_usages SET status='removed',removed_at=? WHERE id=?`, now, usage.ID).Error; err != nil {
		t.Fatal(err)
	}
	missingUsage := command
	missingUsage.RequestKey += "-explicit-join"
	unjoined, err := createManualBatch(tx, missingUsage, now)
	if err != nil || unjoined.Batch != nil || len(unjoined.Failures) != 1 || unjoined.Failures[0].Code != "usage_unavailable" {
		t.Fatalf("missing usage silently joined: %+v err=%v", unjoined, err)
	}
	if _, _, err = createOrRestoreUsage(tx, CreateUsageInput{TeamID: identity.TeamID(strategy.TeamID), MaterialID: materialID, UserID: identity.UserID(original.CreatedBy)}, now); err != nil {
		t.Fatal(err)
	}
	joined, err := createManualBatch(tx, missingUsage, now)
	if err != nil || joined.Batch == nil || len(joined.Created) != 1 {
		t.Fatalf("explicitly restored owner usage rejected: %+v err=%v", joined, err)
	}
	if err = tx.Exec(`UPDATE compose_strategies SET operator_id=NULL WHERE id=?`, strategy.ID).Error; err != nil {
		t.Fatal(err)
	}
	changedOwner := command
	changedOwner.RequestKey += "-changed-owner"
	if _, err = createManualBatch(tx, changedOwner, now); err == nil {
		t.Fatal("changed current strategy owner accepted")
	}
	if err = tx.Exec(`UPDATE compose_strategies SET operator_id=? WHERE id=?`, original.CreatedBy, strategy.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`UPDATE compose_tasks SET template_version=999 WHERE id=?`, `UPDATE composite_outputs SET template_version=999 WHERE id=?`} {
		id := original.ID
		if strings.Contains(query, "composite_outputs") {
			id = output.ID
		}
		if err = tx.Exec(query, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	missingTemplate := command
	missingTemplate.RequestKey += "-missing-template"
	if _, err = createManualBatch(tx, missingTemplate, now); !errors.Is(err, ErrManualBatchInvalid) {
		t.Fatalf("undeployed original template silently replaced: %v", err)
	}
	if err = tx.Exec(`UPDATE compose_tasks SET template_version=1 WHERE id=?`, original.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(`UPDATE composite_outputs SET template_version=1 WHERE id=?`, output.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(`UPDATE compose_tasks SET platforms_json=NULL WHERE id=?`, original.ID).Error; err != nil {
		t.Fatal(err)
	}
	missingHistory := command
	missingHistory.RequestKey += "-missing-history"
	if _, err = createManualBatch(tx, missingHistory, now); !errors.Is(err, ErrManualBatchInvalid) {
		t.Fatalf("missing history silently filled: %v", err)
	}
	// All mutable eligibility is now invalid, including the original task
	// snapshot. Only the accepted command's stored identity should be consulted.
	if err = tx.Exec(`UPDATE compose_strategies SET status='disabled' WHERE id=?`, strategy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(`UPDATE material_usages SET status='removed',removed_at=? WHERE id=?`, now, usage.ID).Error; err != nil {
		t.Fatal(err)
	}
	checksBeforeReplay := inputChecks
	replay, err := createManualBatch(tx, command, now)
	if err != nil || !replay.Replayed || replay.Batch == nil || replay.Batch.ID != result.Batch.ID || len(replay.Created) != 1 || replay.Created[0].ID != created.ID || inputChecks != checksBeforeReplay {
		t.Fatalf("replay consulted mutable source state: %+v checks=%d err=%v", replay, inputChecks, err)
	}
	t.Log("one original group/copy survives current strategy changes; absent source output object is irrelevant; echoes/history guarded; stored replay precedes mutable checks")
}
