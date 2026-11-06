package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type ComposeSettingOverride struct {
	Key    string         `json:"key"`
	Params map[string]any `json:"params"`
}
type manualBatchRepository interface {
	CreateManualBatch(repository.ManualBatchCommand) (repository.ManualBatchResult, error)
}
type normalizedManualInput struct {
	repository.ManualBatchCommand
	Overrides      []ComposeSettingOverride
	LegacyOverride json.RawMessage
}

func normalizeManualInput(actorID int64, input CreateComposeTasksInput) (normalizedManualInput, error) {
	out := normalizedManualInput{}
	invalid := func(message string) (normalizedManualInput, error) {
		return out, fmt.Errorf("%w: %s", ErrComposeTaskInvalid, message)
	}
	out.ActorID = actorID
	out.SourceOutputID = input.SourceOutputID
	out.StrategyID = input.StrategyID
	out.OperatorID = input.OperatorID
	out.Name = strings.TrimSpace(input.Name)
	out.RemakeConfirmed = input.RemakeConfirmed
	if input.SourceOutputID < 0 || input.StrategyID < 0 || (out.SourceOutputID == 0 && out.StrategyID == 0) || input.OperatorID < 0 {
		return invalid("strategy_id/operator_id is invalid")
	}
	if out.SourceOutputID > 0 && (len(input.MaterialIDs) > 0 || len(input.Items) > 0 || len(input.ConfigOverride) > 0) {
		return invalid("source_output_id cannot be combined with material_ids, items or config_override")
	}
	if out.SourceOutputID > 0 && !input.RemakeConfirmed {
		return invalid("source_output_id requires explicit remake_confirmed:true")
	}
	if n := utf8.RuneCountInString(out.Name); n < 1 || n > 60 {
		return invalid("name must contain 1-60 characters")
	}
	key := strings.TrimSpace(input.RequestKey)
	if len(input.MaterialIDs) > 0 && len(input.Items) > 0 {
		return invalid("material_ids and legacy items cannot be mixed")
	}
	out.MaterialIDs = append([]int64(nil), input.MaterialIDs...)
	if len(input.Items) > 0 {
		keys := []string{}
		seen := map[string]bool{}
		for _, item := range input.Items {
			k := strings.TrimSpace(item.RequestKey)
			if len(k) < 8 || len(k) > 64 || seen[k] {
				return invalid("legacy request_key is invalid or duplicated")
			}
			seen[k] = true
			keys = append(keys, k)
			out.MaterialIDs = append(out.MaterialIDs, item.MaterialID)
		}
		if key == "" {
			raw, _ := json.Marshal(keys)
			sum := sha256.Sum256(raw)
			key = hex.EncodeToString(sum[:])
		}
	}
	if len(key) < 8 || len(key) > 64 {
		return invalid("request_key must contain 8-64 characters")
	}
	if out.SourceOutputID == 0 && (len(out.MaterialIDs) < 1 || len(out.MaterialIDs) > 50) {
		return invalid("material_ids must contain 1-50 entries")
	}
	seen := map[int64]bool{}
	for _, id := range out.MaterialIDs {
		if id <= 0 || seen[id] {
			return invalid("material IDs must be positive and unique")
		}
		seen[id] = true
	}
	if len(input.SettingOverrides) > 0 && len(input.ConfigOverride) > 0 {
		return invalid("setting_overrides and config_override cannot be mixed")
	}
	out.Overrides = input.SettingOverrides
	out.LegacyOverride = input.ConfigOverride
	if len(input.ConfigOverride) > 0 {
		var params map[string]any
		if json.Unmarshal(input.ConfigOverride, &params) != nil || params == nil {
			return invalid("config_override must be an object")
		}
		out.LegacyOverride, _ = json.Marshal(params)
	}
	intent := struct {
		ActorID                int64
		SourceOutputID         int64 `json:"source_output_id,omitempty"`
		StrategyID, OperatorID int64
		Name                   string
		MaterialIDs            []int64
		Overrides              []ComposeSettingOverride
		LegacyOverride         json.RawMessage
		RemakeConfirmed        bool
	}{actorID, input.SourceOutputID, input.StrategyID, input.OperatorID, out.Name, out.MaterialIDs, out.Overrides, out.LegacyOverride, out.RemakeConfirmed}
	raw, err := json.Marshal(intent)
	if err != nil {
		return invalid("parameters cannot be encoded")
	}
	digest := sha256.Sum256(raw)
	out.IntentHash = hex.EncodeToString(digest[:])
	namespace := sha256.Sum256([]byte(fmt.Sprintf("manual:%d:%s", actorID, key)))
	out.RequestKey = hex.EncodeToString(namespace[:])
	return out, nil
}

func mergeManualSettings(strategy model.ComposeStrategy, definition executor.Definition, overrides []ComposeSettingOverride) ([]model.ComposeOutputSetting, error) {
	invalid := func(message string) ([]model.ComposeOutputSetting, error) {
		return nil, fmt.Errorf("%w: %s", ErrComposeTaskInvalid, message)
	}
	groups := strategy.EffectiveOutputSettings()
	if len(groups) == 0 {
		return invalid("strategy output settings are invalid")
	}
	indices := map[string]int{}
	for i, g := range groups {
		indices[g.Key] = i
	}
	seen := map[string]bool{}
	for _, override := range overrides {
		index, ok := indices[override.Key]
		if !ok || seen[override.Key] || override.Params == nil {
			return invalid("setting override key is invalid, duplicated, or params is not an object")
		}
		seen[override.Key] = true
		if err := validateTaskOverride(definition.Schema, groups[index].Params, override.Params); err != nil {
			return invalid(err.Error())
		}
		for key, value := range override.Params {
			groups[index].Params[key] = value
		}
	}
	for _, g := range groups {
		raw, err := json.Marshal(g.Params)
		if err != nil {
			return invalid("params cannot be encoded")
		}
		if _, err = definition.Executor.ValidateParams(raw); err != nil {
			return invalid(err.Error())
		}
	}
	return model.NormalizeComposeOutputSettings(groups)
}

func (s *ComposeTaskService) createManualBatch(actor identity.PublicUser, input CreateComposeTasksInput, store manualBatchRepository) (CreateComposeTasksResult, error) {
	if actor.Status != identity.UserStatusEnabled || (actor.Role != identity.RoleAdmin && actor.Role != identity.RoleSeniorOperator && actor.Role != identity.RoleOperator) {
		return CreateComposeTasksResult{}, ErrComposeTaskForbidden
	}
	normalized, err := normalizeManualInput(int64(actor.ID), input)
	if err != nil {
		return CreateComposeTasksResult{}, err
	}
	command := normalized.ManualBatchCommand
	command.ResolveTechnicalConfig = templateTechnicalConfig
	command.AuthorizeSource = func(output model.CompositeOutput) error {
		if !canReadComposeOutput(actor, output) {
			return ErrComposeTaskNotFound
		}
		return nil
	}
	command.ValidateSourceInput = func(material model.Material) error {
		if strings.TrimSpace(material.SourceURL) == "" && material.VideoStatus != model.VideoReady {
			return fmt.Errorf("%w: original material input is not ready and has no source to prepare", ErrComposeTaskInvalid)
		}
		available, _, err := newWiredService().materialSelectionAvailable(material)
		if err != nil {
			return err
		}
		if !available {
			return fmt.Errorf("%w: original material input is unavailable", ErrComposeTaskInvalid)
		}
		return nil
	}
	command.Authorize = func(strategy model.ComposeStrategy) error {
		if !canReadStrategy(actor, strategy) {
			return ErrComposeTaskNotFound
		}
		return nil
	}
	command.ResolveSettings = func(strategy model.ComposeStrategy, template model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		definition, err := deployedTemplate(template.Code, template.Version)
		if err != nil || template.ExecutorID != definition.ExecutorID || template.Status != "active" || !sameJSON(template.Schema, definition.Schema) {
			return nil, ErrStrategyTemplateUnavailable
		}
		overrides := normalized.Overrides
		if len(normalized.LegacyOverride) > 0 {
			groups := strategy.EffectiveOutputSettings()
			if len(groups) != 1 {
				return nil, fmt.Errorf("%w: config_override requires a single setting", ErrComposeTaskInvalid)
			}
			var params map[string]any
			if json.Unmarshal(normalized.LegacyOverride, &params) != nil || params == nil {
				return nil, ErrComposeTaskInvalid
			}
			overrides = []ComposeSettingOverride{{Key: groups[0].Key, Params: params}}
		}
		return mergeManualSettings(strategy, definition, overrides)
	}
	result, err := store.CreateManualBatch(command)
	if errors.Is(err, repository.ErrManualBatchConflict) {
		err = ErrComposeTaskConflict
	}
	if errors.Is(err, repository.ErrManualBatchInvalid) {
		err = fmt.Errorf("%w: %v", ErrComposeTaskInvalid, err)
	}
	if err != nil {
		return CreateComposeTasksResult{}, err
	}
	response := CreateComposeTasksResult{Batch: result.Batch, Created: result.Created, Failed: []CreateComposeTaskFailure{}, Replayed: result.Replayed}
	for _, failure := range result.Failures {
		cause := ErrComposeTaskInvalid
		if failure.Code == "remake_required" {
			cause = ErrComposeTaskConflict
		}
		if failure.Code == "forbidden" {
			cause = ErrComposeTaskForbidden
		}
		response.Failed = append(response.Failed, CreateComposeTaskFailure{MaterialID: failure.MaterialID, Err: cause, Message: failure.Message})
	}
	return response, nil
}
