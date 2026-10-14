package executor

import (
	"encoding/json"
	"testing"
)

func TestGuideDefinitionAcceptsOnlyItsFixedParameters(t *testing.T) {
	for _, definition := range []Definition{GuideInsertV1(), GuidePIPV1()} {
		config := json.RawMessage(`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":3,"guide_duration_seconds":0,"max_duration_seconds":0}`)
		plan, err := definition.Executor.ValidateParams(config)
		if err != nil {
			t.Fatalf("%s valid config: %v", definition.Code, err)
		}
		if plan.(GuidePlan).GuideURL() != "https://media.example.com/guide.mp4" {
			t.Fatal("URL missing from plan")
		}
		for _, invalid := range []string{
			`{"guide_video_url":"http://media.example.com/guide.mp4","insert_seconds":3,"guide_duration_seconds":0,"max_duration_seconds":0}`,
			`{"guide_video_url":"https://127.0.0.1:8080/guide.mp4","insert_seconds":3,"guide_duration_seconds":0,"max_duration_seconds":0}`,
			`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":3,"guide_duration_seconds":0,"max_duration_seconds":0,"custom_filter":"overlay"}`,
			`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":-1,"guide_duration_seconds":0,"max_duration_seconds":0}`,
		} {
			if _, err := definition.Executor.ValidateParams(json.RawMessage(invalid)); err == nil {
				t.Fatalf("%s accepted %s", definition.Code, invalid)
			}
		}
	}
}
