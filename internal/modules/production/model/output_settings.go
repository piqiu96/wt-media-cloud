package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ComposeOutputSetting is an independent output group. Platforms label the output;
// they do not multiply the number of copies.
type ComposeOutputSetting struct {
	Key       string         `json:"key"`
	Name      string         `json:"name"`
	Platforms []string       `json:"platforms"`
	Copies    int            `json:"copies"`
	Params    map[string]any `json:"params"`
}

var outputSettingKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func NormalizeComposeOutputSettings(input []ComposeOutputSetting) ([]ComposeOutputSetting, error) {
	if len(input) < 1 || len(input) > 20 {
		return nil, fmt.Errorf("output_settings must contain 1-20 groups")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var result []ComposeOutputSetting
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	// Reserve supplied keys so generated keys cannot collide with later groups.
	for _, group := range result {
		if group.Key != "" {
			if !outputSettingKey.MatchString(group.Key) || keys[group.Key] {
				return nil, fmt.Errorf("output setting key is invalid or duplicated")
			}
			keys[group.Key] = true
		}
	}
	for i := range result {
		group := &result[i]
		if group.Key == "" {
			for n := i + 1; ; n++ {
				key := fmt.Sprintf("group_%d", n)
				if !keys[key] {
					group.Key = key
					keys[key] = true
					break
				}
			}
		}
		group.Name = strings.TrimSpace(group.Name)
		if n := utf8.RuneCountInString(group.Name); n < 1 || n > 120 {
			return nil, fmt.Errorf("output setting name must contain 1-120 characters")
		}
		if group.Copies < 1 || group.Copies > 20 {
			return nil, fmt.Errorf("output setting copies must be 1-20")
		}
		if len(group.Platforms) < 1 || len(group.Platforms) > 3 {
			return nil, fmt.Errorf("output setting requires 1-3 platforms")
		}
		seen := map[string]bool{}
		for _, platform := range group.Platforms {
			if (platform != "bilibili" && platform != "baidu" && platform != "xiaohongshu") || seen[platform] {
				return nil, fmt.Errorf("output setting platform is invalid or duplicated")
			}
			seen[platform] = true
		}
		if group.Params == nil {
			return nil, fmt.Errorf("output setting params must be an object")
		}
	}
	return result, nil
}

// EffectiveOutputSettings adapts historical single-config strategies and returns
// detached values so request overrides never mutate a stored strategy.
func (s ComposeStrategy) EffectiveOutputSettings() []ComposeOutputSetting {
	groups := s.OutputSettings
	if groups == nil && len(s.OutputSettingsJSON) > 0 {
		if json.Unmarshal(s.OutputSettingsJSON, &groups) != nil {
			return nil
		}
	}
	if groups == nil {
		var params map[string]any
		if json.Unmarshal(s.Config, &params) != nil || params == nil {
			return nil
		}
		groups = []ComposeOutputSetting{{Key: "default", Name: "默认成片", Platforms: []string{"bilibili", "baidu"}, Copies: 1, Params: params}}
	}
	result, err := NormalizeComposeOutputSettings(groups)
	if err != nil {
		return nil
	}
	return result
}
