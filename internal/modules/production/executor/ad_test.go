package executor

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func adParams() json.RawMessage {
	return json.RawMessage(`{"ad_video_url":"https://example.com/ad.mp4","start_seconds":2,"end_seconds":0,"max_duration_seconds":60,"ad_start_min_seconds":10,"ad_start_max_seconds":10,"ad_pip_width":0.92,"transition_seconds":0.2}`)
}
func TestAdOnlyTimelineAndResources(t *testing.T) {
	plan, err := AdPIPEndingV1().Executor.ValidateParams(adParams())
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := plan.(DynamicPlan).Timeline(100, 0, 5, 6, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline.Segments) != 4 || timeline.Transitions != 3 || math.Abs(timeline.Duration-60) > .001 || math.Abs(timeline.AdStart-10) > .001 {
		t.Fatalf("timeline %+v", timeline)
	}
	if timeline.Segments[2].SourceStart != 2+timeline.Insert || timeline.Segments[0].SourceStart != 2 {
		t.Fatal("ad background consumed source progression")
	}
	if _, err = plan.(DynamicPlan).Timeline(12, 0, 5, 6, 42); err == nil {
		t.Fatal("insufficient background accepted")
	}
	var fields map[string]any
	_ = json.Unmarshal(adParams(), &fields)
	fields["intro_video_url"] = "https://example.com/intro.mp4"
	raw, _ := json.Marshal(fields)
	if _, err = AdPIPEndingV1().Executor.ValidateParams(raw); err == nil {
		t.Fatal("intro parameter accepted")
	}
}
func TestAdOnlyCompileSupportsSilentAdvertisement(t *testing.T) {
	defaults := json.RawMessage(`{"width":1280,"height":720,"fps":30,"crf":20,"preset":"medium","blur":20,"brightness":-0.08,"shadow_blur":8,"shadow_opacity":0.35,"shadow_offset":8,"tail_seconds":6,"tail_fade":0.35,"ending_index_key":"ending/index.json","filter_threads":1}`)
	plan, err := AdPIPEndingV1().Executor.ValidateParams(adParams())
	if err != nil {
		t.Fatal(err)
	}
	resources, err := plan.Resources(defaults)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 || resources[0].Name != "ad" || resources[1].Name != "ending" {
		t.Fatalf("resources %+v", resources)
	}
	source := MediaInput{Path: "source.mp4", Duration: 100, Width: 1280, Height: 720, HasAudio: true}
	ad := MediaInput{Path: "ad.mp4", Duration: 5, Width: 720, Height: 1280}
	tail := MediaInput{Path: "tail.mp4", Duration: 6, Width: 1280, Height: 720, HasAudio: true}
	result, err := plan.Compile(RenderInput{Source: source, Resources: map[string]MediaInput{"ad": ad, "ending": tail}, Defaults: defaults, Seed: 42, OutputPath: "output.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(result.Arguments, " ")
	if strings.Contains(args, "intro") || !strings.Contains(args, "anullsrc") || !strings.Contains(args, "[1:v]") || !strings.Contains(args, "[2:a]") {
		t.Fatalf("invalid ad-only render %s", args)
	}
	if result.DurationMS != 60000 {
		t.Fatalf("duration %d", result.DurationMS)
	}
}
