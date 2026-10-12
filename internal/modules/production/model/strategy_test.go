package model

import "testing"

func TestBasicCutTemplateAcceptsOnlySupportedParameters(t *testing.T) {
	valid := StrategyParameters{VideoRatio: "16:9"}
	if err := ValidateStrategyParameters("basic_cut", "1.0", valid, []string{"video_ratio"}); err != nil {
		t.Fatalf("valid parameters rejected: %v", err)
	}
	for _, tc := range []struct {
		name      string
		template  string
		version   string
		params    StrategyParameters
		overrides []string
	}{
		{"unknown template", "future_plugin", "1.0", valid, nil},
		{"unknown version", "basic_cut", "9.9", valid, nil},
		{"unsupported ratio", "basic_cut", "1.0", StrategyParameters{VideoRatio: "4:3"}, nil},
		{"subtitle needs assets", "basic_cut", "1.0", StrategyParameters{VideoRatio: "16:9", Subtitle: true}, nil},
		{"bgm needs assets", "basic_cut", "1.0", StrategyParameters{VideoRatio: "16:9", BGM: true}, nil},
		{"outro needs assets", "basic_cut", "1.0", StrategyParameters{VideoRatio: "16:9", Outro: true}, nil},
		{"unsupported override", "basic_cut", "1.0", valid, []string{"subtitle"}},
		{"duplicate override", "basic_cut", "1.0", valid, []string{"video_ratio", "video_ratio"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStrategyParameters(tc.template, tc.version, tc.params, tc.overrides); err == nil {
				t.Fatal("invalid parameters accepted")
			}
		})
	}
}

func TestTemplateCatalogHasOneVersionedEntry(t *testing.T) {
	templates := StrategyTemplates()
	if len(templates) != 1 || templates[0].Key != "basic_cut" || templates[0].Version != "1.0" {
		t.Fatalf("unexpected template catalog: %+v", templates)
	}
}
