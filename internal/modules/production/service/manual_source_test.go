package service

import (
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/testutil/composefixture"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

func TestSourceManualInputDerivesSelectionAndHashesSource(t *testing.T) {
	input := CreateComposeTasksInput{SourceOutputID: 91, Name: "再次制作", RequestKey: "remake-request", RemakeConfirmed: true}
	a, err := normalizeManualInput(7, input)
	if err != nil || a.SourceOutputID != 91 || a.StrategyID != 0 || len(a.MaterialIDs) != 0 {
		t.Fatalf("source normalization=%+v err=%v", a, err)
	}
	input.SourceOutputID = 92
	b, err := normalizeManualInput(7, input)
	if err != nil || a.RequestKey != b.RequestKey || a.IntentHash == b.IntentHash {
		t.Fatalf("source not included in command identity: %v", err)
	}
	for _, incompatible := range []CreateComposeTasksInput{
		{MaterialIDs: []int64{1}},
		{Items: []CreateComposeTaskItemInput{{MaterialID: 1, RequestKey: "legacy-key"}}},
		{ConfigOverride: json.RawMessage(`{"end_seconds":2}`)},
	} {
		incompatible.SourceOutputID = 91
		incompatible.RemakeConfirmed = true
		incompatible.Name = "再次制作"
		incompatible.RequestKey = "remake-request"
		if _, err := normalizeManualInput(7, incompatible); !errors.Is(err, ErrComposeTaskInvalid) {
			t.Fatalf("accepted source with alternate input: %+v err=%v", incompatible, err)
		}
	}
}

func TestSourceManualRequiresExplicitRemakeConfirmation(t *testing.T) {
	for _, wire := range []string{
		`{"source_output_id":91,"name":"再次制作","request_key":"source-request"}`,
		`{"source_output_id":91,"name":"再次制作","request_key":"source-request","remake_confirmed":false}`,
	} {
		var input CreateComposeTasksInput
		if err := json.Unmarshal([]byte(wire), &input); err != nil {
			t.Fatal(err)
		}
		if _, err := normalizeManualInput(7, input); !errors.Is(err, ErrComposeTaskInvalid) {
			t.Fatalf("unconfirmed source accepted: %s err=%v", wire, err)
		}
	}
}

func TestSourceExtensionPreservesExistingManualCommandHashes(t *testing.T) {
	got, err := normalizeManualInput(7, CreateComposeTasksInput{StrategyID: 5, Name: "original batch", MaterialIDs: []int64{42}, RequestKey: "original-key"})
	// Frozen hash of the same accepted CHG116 request, before source_output_id
	// existed. Zero source must not invalidate an already persisted command.
	if err != nil || got.IntentHash != "1f71e1e77db18bc66a171fc2f5ada0a84242f34acfb52ba49e0cee47cc91c4fc" {
		t.Fatalf("existing manual hash changed: %s err=%v", got.IntentHash, err)
	}
}

type sourceCommandStore struct {
	*taskMemoryStore
	accept func(repository.ManualBatchCommand) error
}

func (store sourceCommandStore) CreateManualBatch(command repository.ManualBatchCommand) (repository.ManualBatchResult, error) {
	if err := store.accept(command); err != nil {
		return repository.ManualBatchResult{}, err
	}
	return repository.ManualBatchResult{Batch: &model.ComposeTaskBatch{ID: 7}, Created: []model.ComposeTask{}, Failures: []repository.ManualBatchFailure{}}, nil
}

func TestSourceCommandKeepsOutputReadScopeBeforeDerivingPrivateHistory(t *testing.T) {
	for _, tt := range []struct {
		name        string
		role        identity.Role
		team, owner int64
		allowed     bool
	}{
		{"own output", identity.RoleOperator, 7, 9, true},
		{"other owner", identity.RoleOperator, 7, 10, false},
		{"other team", identity.RoleSeniorOperator, 8, 10, false},
		{"same team senior", identity.RoleSeniorOperator, 7, 10, true},
		{"administrator", identity.RoleAdmin, 8, 10, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, tasks, _, _, actor := composeTaskFixture()
			actor.Role = tt.role
			svc.tasks = sourceCommandStore{taskMemoryStore: tasks, accept: func(command repository.ManualBatchCommand) error {
				if command.SourceOutputID != 91 || command.StrategyID != 0 || len(command.MaterialIDs) != 0 {
					t.Fatal("client input replaced server-derived source identity")
				}
				return command.AuthorizeSource(model.CompositeOutput{ID: 91, TeamID: tt.team, CategoryID: 1, OperatorID: &tt.owner})
			}}
			_, err := svc.Create(actor, CreateComposeTasksInput{SourceOutputID: 91, Name: "原成片再制作", RequestKey: "source-request", RemakeConfirmed: true})
			if tt.allowed && err != nil || !tt.allowed && !errors.Is(err, ErrComposeTaskNotFound) {
				t.Fatalf("source scope allowed=%v err=%v", tt.allowed, err)
			}
		})
	}
}

func TestSourceCommandUsesOriginalTemplateSchemaForOverrides(t *testing.T) {
	definition := composefixture.Resource()
	original := model.ComposeStrategy{OutputSettings: []model.ComposeOutputSetting{{Key: "old-group", Name: "原设置", Copies: 1, Platforms: []string{"baidu"}, Params: map[string]any{"guide_video_url": "https://media.example.com/original.mp4", "insert_seconds": 1, "guide_duration_seconds": 0, "max_duration_seconds": 0}}}}
	for _, tt := range []struct {
		name    string
		key     string
		params  map[string]any
		allowed bool
	}{
		{"editable", "old-group", map[string]any{"insert_seconds": 2}, true},
		{"fixed original URL", "old-group", map[string]any{"guide_video_url": "https://media.example.com/replacement.mp4"}, false},
		{"alternate group", "current-group", map[string]any{"insert_seconds": 2}, false},
		{"copies", "old-group", map[string]any{"copies": 20}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, tasks, _, _, actor := composeTaskFixture()
			svc.tasks = sourceCommandStore{taskMemoryStore: tasks, accept: func(command repository.ManualBatchCommand) error {
				groups, err := command.ResolveSettings(original, model.ComposeTemplate{Code: definition.Code, Version: definition.Version, Schema: definition.Schema, ExecutorID: definition.ExecutorID, Status: "active"})
				if err == nil && (len(groups) != 1 || groups[0].Copies != 1 || groups[0].Params["guide_video_url"] != "https://media.example.com/original.mp4") {
					t.Fatalf("source changed locked values: %+v", groups)
				}
				return err
			}}
			_, err := svc.Create(actor, CreateComposeTasksInput{SourceOutputID: 91, Name: "原成片再制作", RequestKey: "source-request", RemakeConfirmed: true, SettingOverrides: []ComposeSettingOverride{{Key: tt.key, Params: tt.params}}})
			if tt.allowed && err != nil || !tt.allowed && !errors.Is(err, ErrComposeTaskInvalid) {
				t.Fatalf("original schema allowed=%v err=%v", tt.allowed, err)
			}
		})
	}
}

func TestSourceCommandRequiresAnInputTheWorkerCanActuallyRead(t *testing.T) {
	for _, tt := range []struct {
		name     string
		material model.Material
		allowed  bool
	}{
		{"preparable source", model.Material{Status: model.MaterialAvailable, SourceURL: "https://www.douyin.com/video/123", VideoStatus: model.VideoNotDownloaded}, true},
		{"unprepared orphan key", model.Material{Status: model.MaterialAvailable, SourceObjectKey: "private/key", VideoStatus: model.VideoFailed}, false},
		{"no input", model.Material{Status: model.MaterialAvailable, VideoStatus: model.VideoNotDownloaded}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, tasks, _, _, actor := composeTaskFixture()
			svc.tasks = sourceCommandStore{taskMemoryStore: tasks, accept: func(command repository.ManualBatchCommand) error {
				return command.ValidateSourceInput(tt.material)
			}}
			_, err := svc.Create(actor, CreateComposeTasksInput{SourceOutputID: 91, Name: "原成片再制作", RequestKey: "source-request", RemakeConfirmed: true})
			if tt.allowed && err != nil || !tt.allowed && !errors.Is(err, ErrComposeTaskInvalid) {
				t.Fatalf("input availability allowed=%v err=%v", tt.allowed, err)
			}
		})
	}
}
