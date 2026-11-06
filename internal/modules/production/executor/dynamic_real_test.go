package executor

import (
	"context"
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"math"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestDynamicRealRender(t *testing.T) {
	filename := os.Getenv("WT_MEDIA_DYNAMIC_INPUTS")
	if filename == "" {
		t.Skip("explicit local video verification")
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Source, Intro, Ad, Ending, Output string
		Params, Defaults                  json.RawMessage
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	inspect := func(path string) MediaInput {
		info, e := media.ProbeFile(ctx, ffprobe, path)
		if e != nil {
			t.Fatal(e)
		}
		duration := info.VideoDuration
		if duration <= 0 {
			duration = info.Duration
		}
		return MediaInput{Path: path, Duration: duration, Width: info.Width, Height: info.Height, HasAudio: info.HasAudio}
	}
	plan, err := DynamicPIPEndingV1().Executor.ValidateParams(fixture.Params)
	if err != nil {
		t.Fatal(err)
	}
	result, err := plan.(ResourcePlan).Compile(RenderInput{Source: inspect(fixture.Source), Resources: map[string]MediaInput{"intro": inspect(fixture.Intro), "ad": inspect(fixture.Ad), "ending": inspect(fixture.Ending)}, Defaults: fixture.Defaults, Seed: 20261106, OutputPath: fixture.Output})
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.CommandContext(ctx, ffmpeg, result.Arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("render: %v: %.4000s", err, output)
	}
	info, err := media.ProbeFile(ctx, ffprobe, fixture.Output)
	if err != nil {
		t.Fatal(err)
	}
	expected := float64(result.DurationMS) / 1000
	if !info.HasAudio || math.Abs(info.VideoDuration-expected) > .1 || math.Abs(info.AudioDuration-expected) > .1 {
		t.Fatalf("invalid output %+v expected %g", info, expected)
	}
	if output, err = exec.CommandContext(ctx, ffmpeg, "-v", "error", "-xerror", "-i", fixture.Output, "-f", "null", "-").CombinedOutput(); err != nil {
		t.Fatalf("decode: %v %.2000s", err, output)
	}
	if err = os.WriteFile(fixture.Output+".timeline.json", result.Timeline, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("output=%s duration=%g video=%g audio=%g", fixture.Output, info.Duration, info.VideoDuration, info.AudioDuration)
}
