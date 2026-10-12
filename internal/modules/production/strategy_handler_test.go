package production

import (
	"strings"
	"testing"

	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

func TestStrategyRequestRejectsUnstoredScheduleAndUnknownParameters(t *testing.T) {
	for _, body := range []string{
		`{"team_id":2,"game_id":"game-a","schedule":{"enabled":true}}`,
		`{"team_id":2,"game_id":"game-a","parameters":{"video_ratio":"16:9","unknown":true}}`,
		`{"team_id":2} {"team_id":3}`,
	} {
		var input productionservice.StrategyInput
		if err := decodeStrategyRequest([]byte(body), &input); err == nil {
			t.Fatalf("accepted a request with unstored fields: %s", body)
		}
	}
	var input productionservice.StrategyInput
	if err := decodeStrategyRequest([]byte(`{"team_id":2,"game_id":"game-a","name":"基础策略","content_type":"highlight","template_key":"basic_cut","template_version":"1.0","parameters":{"video_ratio":"16:9"},"allowed_overrides":["video_ratio"],"status":"active"}`), &input); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if !strings.EqualFold(input.TemplateKey, "basic_cut") {
		t.Fatalf("template = %q", input.TemplateKey)
	}
}
