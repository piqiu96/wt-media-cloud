package service

import (
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"testing"
)

func TestDynamicTaskOverridesKeepStrategyBounds(t *testing.T) {
	params := map[string]any{"intro_video_url": "https://example.com/intro.mp4", "ad_video_url": "https://example.com/ad.mp4", "start_seconds": 0, "end_seconds": 0, "max_duration_seconds": 180, "ad_start_min_seconds": 10, "ad_start_max_seconds": 15, "intro_pip_width": .9, "ad_pip_width": .92, "transition_seconds": .2}
	raw, _ := json.Marshal(params)
	strategy := model.ComposeStrategy{Config: raw}
	for _, tc := range []struct {
		name     string
		override map[string]any
		valid    bool
	}{
		{"shorten", map[string]any{"max_duration_seconds": 120}, true},
		{"raise cap", map[string]any{"max_duration_seconds": 200}, false},
		{"replace intro", map[string]any{"intro_video_url": "https://example.com/other.mp4"}, false},
		{"move ad", map[string]any{"ad_start_min_seconds": 8}, false},
		{"select source", map[string]any{"start_seconds": 5, "end_seconds": 80}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mergeManualSettings(strategy, executor.DynamicPIPEndingV1(), []ComposeSettingOverride{{Key: "default", Params: tc.override}})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
