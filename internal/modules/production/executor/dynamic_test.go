package executor

import (
	"encoding/json"
	"math"
	"testing"
)

func TestDynamicTimelineIncludesCopiedBackgroundAndOverlaps(t *testing.T) {
	params := json.RawMessage(`{"intro_video_url":"https://example.com/i.mp4","ad_video_url":"https://example.com/a.mp4","start_seconds":0,"end_seconds":110,"max_duration_seconds":180,"ad_start_min_seconds":10.1,"ad_start_max_seconds":10.1,"intro_pip_width":0.9,"ad_pip_width":0.92,"transition_seconds":0.2}`)
	p, err := DynamicPIPEndingV1().Executor.ValidateParams(params)
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := p.(DynamicPlan).Timeline(220, 239.0/30, 1592.0/30, 6, 42)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(timeline.Duration-176.2333333) > 0.001 {
		t.Fatalf("duration %g", timeline.Duration)
	}
	if math.Abs(timeline.AdStart-10.1) > 0.001 || math.Abs(timeline.Insert-76.0/30) > 0.001 {
		t.Fatalf("ad timeline %+v", timeline)
	}
	if timeline.Segments[1].SourceStart != 0 || timeline.Segments[3].SourceStart != timeline.Insert {
		t.Fatal("copied background consumed source content")
	}
	if len(timeline.Segments) != 5 || timeline.Transitions != 4 {
		t.Fatal("wrong segment/transition count")
	}
}
func TestDynamicRejectsInsufficientBackgroundAndInvalidInsertion(t *testing.T) {
	raw := json.RawMessage(`{"intro_video_url":"https://example.com/i.mp4","ad_video_url":"https://example.com/a.mp4","start_seconds":0,"end_seconds":0,"max_duration_seconds":60,"ad_start_min_seconds":10,"ad_start_max_seconds":15,"intro_pip_width":0.9,"ad_pip_width":0.92,"transition_seconds":0.2}`)
	p, err := DynamicPIPEndingV1().Executor.ValidateParams(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.(DynamicPlan).Timeline(20, 8, 30, 6, 1); err == nil {
		t.Fatal("short background accepted")
	}
	if _, err = p.(DynamicPlan).Timeline(100, 20, 3, 6, 1); err == nil {
		t.Fatal("intro making ad insertion invalid accepted")
	}
	a, err := p.(DynamicPlan).Timeline(100, 8, 3, 6, 91)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.(DynamicPlan).Timeline(100, 8, 3, 6, 91)
	if err != nil || a.AdStart != b.AdStart || a.Duration > 60.001 {
		t.Fatal("seed/budget is not stable")
	}
}

func TestDynamicBoundarySegmentsNeedOnlyTheirActualTransition(t *testing.T) {
	raw := json.RawMessage(`{"intro_video_url":"https://example.com/i.mp4","ad_video_url":"https://example.com/a.mp4","start_seconds":0,"end_seconds":0,"max_duration_seconds":60,"ad_start_min_seconds":10,"ad_start_max_seconds":10,"intro_pip_width":0.9,"ad_pip_width":0.92,"transition_seconds":1}`)
	p, err := DynamicPIPEndingV1().Executor.ValidateParams(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.(DynamicPlan).Timeline(100, 1.5, 4, 6, 1); err != nil {
		t.Fatalf("one outgoing transition fits the opening: %v", err)
	}
	if _, err = p.(DynamicPlan).Timeline(100, .9, 4, 6, 1); err == nil {
		t.Fatal("opening shorter than its outgoing transition accepted")
	}
}
