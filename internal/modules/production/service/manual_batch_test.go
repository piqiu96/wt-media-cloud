package service

import (
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"github.com/wt-media/wt-media-cloud/internal/testutil/composefixture"
	"testing"
)

func TestManualSettingOverridesAreOnlyExplicitlyEditable(t *testing.T) {
	d := composefixture.Resource()
	base := model.ComposeStrategy{Config: json.RawMessage(`{"guide_video_url":"https://media.example.com/a.mp4","insert_seconds":1,"guide_duration_seconds":0,"max_duration_seconds":0}`)}
	for _, params := range []map[string]any{{"guide_video_url": "https://media.example.com/a.mp4"}, {"unknown": 1}, {"insert_seconds": "2"}} {
		if _, err := mergeManualSettings(base, d, []ComposeSettingOverride{{Key: "default", Params: params}}); !errors.Is(err, ErrComposeTaskInvalid) {
			t.Fatalf("accepted %v err=%v", params, err)
		}
	}
	got, err := mergeManualSettings(base, d, []ComposeSettingOverride{{Key: "default", Params: map[string]any{"insert_seconds": 2}}})
	if err != nil || got[0].Params["insert_seconds"] != float64(2) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if base.EffectiveOutputSettings()[0].Params["insert_seconds"] != float64(1) {
		t.Fatal("mutated strategy")
	}
	if _, err := mergeManualSettings(base, d, []ComposeSettingOverride{{Key: "missing", Params: map[string]any{}}}); err == nil {
		t.Fatal("accepted unknown group")
	}
}

func TestManualCommandHashTracksIntentAndLegacyKeyNotContent(t *testing.T) {
	input := CreateComposeTasksInput{StrategyID: 1, Name: "批次", Items: []CreateComposeTaskItemInput{{MaterialID: 4, RequestKey: "legacy-key-1"}}}
	first, err := normalizeManualInput(7, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Items[0].MaterialID = 5
	second, err := normalizeManualInput(7, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.RequestKey != second.RequestKey || first.IntentHash == second.IntentHash {
		t.Fatal("legacy key must collide on changed intent")
	}
	input = CreateComposeTasksInput{StrategyID: 1, Name: "批次", RequestKey: "request-key-1", MaterialIDs: []int64{1, 1}}
	if _, err = normalizeManualInput(7, input); !errors.Is(err, ErrComposeTaskInvalid) {
		t.Fatal("duplicate materials accepted")
	}
}

type manualBatchCapableStore struct {
	*taskMemoryStore
	strategy model.ComposeStrategy
	template model.ComposeTemplate
	command  repository.ManualBatchCommand
	groups   []model.ComposeOutputSetting
}

func (s *manualBatchCapableStore) CreateManualBatch(command repository.ManualBatchCommand) (repository.ManualBatchResult, error) {
	s.command = command
	if err := command.Authorize(s.strategy); err != nil {
		return repository.ManualBatchResult{}, err
	}
	groups, err := command.ResolveSettings(s.strategy, s.template)
	if err != nil {
		return repository.ManualBatchResult{}, err
	}
	s.groups = groups
	id := int64(71)
	return repository.ManualBatchResult{Batch: &model.ComposeTaskBatch{ID: id}, Created: []model.ComposeTask{{ID: 1, BatchID: &id}}, Failures: []repository.ManualBatchFailure{}}, nil
}
func TestLegacyProductionCreateDispatchesToBatchCapability(t *testing.T) {
	svc, tasks, strategies, _, actor := composeTaskFixture()
	capable := &manualBatchCapableStore{taskMemoryStore: tasks, strategy: strategies.items[0], template: strategies.template}
	svc.tasks = capable
	input := CreateComposeTasksInput{StrategyID: 5, Name: "兼容批次", Items: []CreateComposeTaskItemInput{{MaterialID: 42, RequestKey: "legacy-item-key"}}, ConfigOverride: json.RawMessage(`{"end_seconds":9}`)}
	result, err := svc.Create(actor, input)
	if err != nil || result.Batch == nil || result.Created[0].BatchID == nil || tasks.creates != 0 {
		t.Fatalf("result=%+v err=%v legacywrites=%d", result, err, tasks.creates)
	}
	if capable.command.RequestKey == "" || capable.groups[0].Params["end_seconds"] != float64(9) {
		t.Fatalf("command=%+v groups=%v", capable.command, capable.groups)
	}
}
func TestManualOverridesRejectDuplicateGroupsAndLockedMetadata(t *testing.T) {
	definition := composefixture.Cut()
	strategy := model.ComposeStrategy{Config: json.RawMessage(`{"start_seconds":0,"end_seconds":9}`)}
	_, err := mergeManualSettings(strategy, definition, []ComposeSettingOverride{{Key: "default", Params: map[string]any{}}, {Key: "default", Params: map[string]any{}}})
	if !errors.Is(err, ErrComposeTaskInvalid) {
		t.Fatal("duplicate group accepted")
	}
	for _, key := range []string{"copies", "name", "platforms", "order"} {
		if _, err = mergeManualSettings(strategy, definition, []ComposeSettingOverride{{Key: "default", Params: map[string]any{key: 1}}}); !errors.Is(err, ErrComposeTaskInvalid) {
			t.Fatalf("metadata %s accepted", key)
		}
	}
	// Omitted editable metadata defaults to locked even for a recognized executor parameter.
	var schema map[string]any
	json.Unmarshal(definition.Schema, &schema)
	delete(schema["properties"].(map[string]any)["start_seconds"].(map[string]any), "editable")
	definition.Schema, _ = json.Marshal(schema)
	if _, err = mergeManualSettings(strategy, definition, []ComposeSettingOverride{{Key: "default", Params: map[string]any{"start_seconds": 1}}}); !errors.Is(err, ErrComposeTaskInvalid) {
		t.Fatal("defaultlocked accepted")
	}
}

func TestManualLegacyHashCanonicalizesJSONPropertyOrder(t *testing.T) {
	input := CreateComposeTasksInput{StrategyID: 5, Name: "同次提交", RequestKey: "canonical-key", MaterialIDs: []int64{42}, ConfigOverride: json.RawMessage(`{"start_seconds":1,"end_seconds":9}`)}
	a, err := normalizeManualInput(9, input)
	if err != nil {
		t.Fatal(err)
	}
	input.ConfigOverride = json.RawMessage(`{ "end_seconds":9, "start_seconds":1 }`)
	b, err := normalizeManualInput(9, input)
	if err != nil {
		t.Fatal(err)
	}
	if a.IntentHash != b.IntentHash {
		t.Fatal("JSON property order changed command identity")
	}
}
