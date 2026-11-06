package service

import (
	"encoding/json"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

// The code/version mapping owns validation; deployment config supplies values.
// No caller switches on a concrete template name.
func templateDefaults(def executor.Definition) (map[string]any, json.RawMessage, error) {
	validator, required := def.Executor.(executor.DefaultsValidator)
	if !required {
		return nil, nil, nil
	}
	values, ok := config.Get().Compose.Templates[def.Code]
	if !ok {
		return nil, nil, fmt.Errorf("render defaults missing for %s", def.Code)
	}
	raw, err := json.Marshal(values.Render)
	if err == nil {
		err = validator.ValidateDefaults(raw)
	}
	return values.Params, raw, err
}
func templateTechnicalConfig(code string, version uint32) (json.RawMessage, error) {
	def, err := executor.DeployedDefinition(code, version)
	if err != nil {
		return nil, err
	}
	_, raw, err := templateDefaults(def)
	return raw, err
}

func validateTaskOverride(schema json.RawMessage, base, extra map[string]any) error {
	var s struct {
		Properties map[string]struct {
			Editable            bool `json:"editable"`
			MaximumFromStrategy bool `json:"maximum_from_strategy"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return err
	}
	for key, value := range extra {
		field, ok := s.Properties[key]
		if !ok || !field.Editable {
			return fmt.Errorf("parameter is fixed or unknown: %s", key)
		}
		if field.MaximumFromStrategy {
			b, bok := numericValue(base[key])
			v, vok := numericValue(value)
			if !bok || !vok || v > b {
				return fmt.Errorf("parameter %s exceeds strategy limit", key)
			}
		}
	}
	return nil
}
func numericValue(value any) (float64, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, false
	}
	var out float64
	err = json.Unmarshal(raw, &out)
	return out, err == nil
}
