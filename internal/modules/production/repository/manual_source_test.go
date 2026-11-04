package repository

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func manualSourceFacts() (model.CompositeOutput, model.ComposeTask) {
	owner := int64(9)
	output := model.CompositeOutput{ID: 91, TaskID: 71, TeamID: 7, CategoryID: 2, OperatorID: &owner, MaterialID: 42, StrategyID: 5, TemplateCode: "basic_cut", TemplateVersion: 1}
	task := model.ComposeTask{ID: 71, TeamID: 7, CategoryID: 2, OperatorID: &owner, MaterialID: 42, StrategyID: 5, TemplateCode: "basic_cut", TemplateVersion: 1, Status: model.ComposeTaskSuccess, SettingKey: "original", SettingName: "原始设置", PlatformsJSON: json.RawMessage(`["bilibili"]`), ConfigSnapshot: json.RawMessage(`{"start_seconds":0,"end_seconds":3}`)}
	return output, task
}

func TestManualSourceUsesExactlyOriginalSettingOnce(t *testing.T) {
	output, task := manualSourceFacts()
	setting, err := ManualSourceSetting(output, task)
	if err != nil || setting.Key != "original" || setting.Name != "原始设置" || setting.Copies != 1 || len(setting.Platforms) != 1 || setting.Params["end_seconds"] != float64(3) {
		t.Fatalf("setting=%+v err=%v", setting, err)
	}
	setting.Params["end_seconds"] = 99
	if string(task.ConfigSnapshot) != `{"start_seconds":0,"end_seconds":3}` {
		t.Fatal("source snapshot mutated")
	}
}

func TestManualSourceRejectsIncompleteOrMismatchedHistory(t *testing.T) {
	for name, mutate := range map[string]func(*model.CompositeOutput, *model.ComposeTask){
		"missing owner":       func(o *model.CompositeOutput, _ *model.ComposeTask) { o.OperatorID = nil },
		"task owner":          func(_ *model.CompositeOutput, task *model.ComposeTask) { owner := int64(10); task.OperatorID = &owner },
		"task material":       func(_ *model.CompositeOutput, task *model.ComposeTask) { task.MaterialID++ },
		"task template":       func(_ *model.CompositeOutput, task *model.ComposeTask) { task.TemplateVersion++ },
		"task not successful": func(_ *model.CompositeOutput, task *model.ComposeTask) { task.Status = model.ComposeTaskFailed },
		"missing setting":     func(_ *model.CompositeOutput, task *model.ComposeTask) { task.SettingKey = "" },
		"missing name":        func(_ *model.CompositeOutput, task *model.ComposeTask) { task.SettingName = "" },
		"missing platforms":   func(_ *model.CompositeOutput, task *model.ComposeTask) { task.PlatformsJSON = nil },
		"missing params":      func(_ *model.CompositeOutput, task *model.ComposeTask) { task.ConfigSnapshot = nil },
		"null params":         func(_ *model.CompositeOutput, task *model.ComposeTask) { task.ConfigSnapshot = json.RawMessage(`null`) },
	} {
		t.Run(name, func(t *testing.T) {
			output, task := manualSourceFacts()
			mutate(&output, &task)
			if _, err := ManualSourceSetting(output, task); !errors.Is(err, ErrManualBatchInvalid) {
				t.Fatalf("incomplete history accepted: %v", err)
			}
		})
	}
}

func TestManualSourceStrategyMayChangeGroupsButNotAssignment(t *testing.T) {
	output, _ := manualSourceFacts()
	strategy := model.ComposeStrategy{ID: output.StrategyID, TeamID: output.TeamID, CategoryID: output.CategoryID, OperatorID: output.OperatorID, Status: model.ComposeStrategyEnabled, Config: json.RawMessage(`{"end_seconds":999}`)}
	if err := ValidateManualSourceStrategy(output, strategy); err != nil {
		t.Fatal(err)
	}
	strategy.Status = model.ComposeStrategyDisabled
	if err := ValidateManualSourceStrategy(output, strategy); !errors.Is(err, ErrManualBatchInvalid) {
		t.Fatalf("disabled strategy accepted: %v", err)
	}
	strategy.Status = model.ComposeStrategyEnabled
	other := int64(10)
	strategy.OperatorID = &other
	if err := ValidateManualSourceStrategy(output, strategy); !errors.Is(err, ErrManualBatchInvalid) {
		t.Fatalf("changed assignment accepted: %v", err)
	}
}

func TestManualSourceReplayDoesNotReadCurrentSourceOrStrategy(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Now().UTC()
	command := ManualBatchCommand{SourceOutputID: 91, ActorID: 9, Name: "再制作", RequestKey: "source-request", IntentHash: "original-hash", Authorize: func(strategy model.ComposeStrategy) error {
		if strategy.ID != 3 || strategy.OperatorID == nil || *strategy.OperatorID != 9 {
			t.Fatal("replay changed immutable ownership")
		}
		return nil
	}, AuthorizeSource: func(model.CompositeOutput) error {
		t.Fatal("replay looked up source output")
		return nil
	}, ValidateSourceInput: func(model.Material) error {
		t.Fatal("replay checked mutable input")
		return nil
	}, ResolveSettings: func(model.ComposeStrategy, model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		t.Fatal("replay resolved current settings")
		return nil, nil
	}}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM compose_task_batches WHERE request_key`).WithArgs(command.RequestKey).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery(`SELECT id,name,team_id`).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "team_id", "category_id", "operator_id", "compose_strategy_id", "trigger_type", "created_by", "request_key", "request_hash", "config_snapshot_json", "created_at"}).AddRow(5, "再制作", 7, 1, 9, 3, "manual", 9, command.RequestKey, command.IntentHash, []byte(`{"source_output_id":91,"source_task_id":71,"settings":[],"acceptance":[]}`), now))
	mock.ExpectQuery(`SELECT status,COUNT`).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).AddRow("pending", 1))
	mock.ExpectQuery(`SELECT .* FROM compose_tasks WHERE batch_id=\? ORDER BY id`).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	mock.ExpectCommit()
	result, err := createManualBatch(db, command, now)
	if err != nil || !result.Replayed || result.Batch == nil || result.Batch.ID != 5 || len(result.Created) != 1 {
		t.Fatalf("replay=%+v err=%v", result, err)
	}
	assertExpectations(t, mock)
}
