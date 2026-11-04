package model

import (
	"encoding/json"
	"testing"
)

func TestEffectiveOutputSettingsLegacyAndIsolation(t *testing.T) {
	s := ComposeStrategy{Config: json.RawMessage(`{"start_seconds":1,"end_seconds":9}`)}
	groups := s.EffectiveOutputSettings()
	if len(groups) != 1 || groups[0].Key != "default" || groups[0].Copies != 1 || len(groups[0].Platforms) != 2 || groups[0].Params["end_seconds"] != float64(9) {
		t.Fatalf("legacy=%+v", groups)
	}
	s.OutputSettings = groups
	copied := s.EffectiveOutputSettings()
	copied[0].Params["end_seconds"] = 99
	copied[0].Platforms[0] = "bad"
	if s.OutputSettings[0].Params["end_seconds"] != float64(9) || s.OutputSettings[0].Platforms[0] != "bilibili" {
		t.Fatal("settings share mutable state")
	}
}
func TestNormalizeOutputSettingsRejectsInvalidGroups(t *testing.T) {
	valid := ComposeOutputSetting{Key: "main", Name: "主片", Platforms: []string{"bilibili", "xiaohongshu"}, Copies: 2, Params: map[string]any{"start_seconds": 0, "end_seconds": 9}}
	cases := [][]ComposeOutputSetting{{}, {valid, valid}}
	for _, mutate := range []func(*ComposeOutputSetting){func(s *ComposeOutputSetting) { s.Platforms = nil }, func(s *ComposeOutputSetting) { s.Platforms = []string{"douyin"} }, func(s *ComposeOutputSetting) { s.Copies = 21 }, func(s *ComposeOutputSetting) { s.Copies = 0 }, func(s *ComposeOutputSetting) { s.Key = "../bad" }, func(s *ComposeOutputSetting) { s.Name = "" }} {
		v := valid
		mutate(&v)
		cases = append(cases, []ComposeOutputSetting{v})
	}
	for _, in := range cases {
		if _, err := NormalizeComposeOutputSettings(in); err == nil {
			t.Fatalf("accepted %+v", in)
		}
	}
	valid.Key = ""
	groups, err := NormalizeComposeOutputSettings([]ComposeOutputSetting{valid})
	if err != nil || groups[0].Key == "" {
		t.Fatalf("generated key=%+v %v", groups, err)
	}
	again, _ := NormalizeComposeOutputSettings(groups)
	if again[0].Key != groups[0].Key {
		t.Fatal("key changed")
	}
}
