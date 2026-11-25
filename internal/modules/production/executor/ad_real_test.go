package executor

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/media"
)

func TestAdRealRender(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	generate := func(name string, duration string, audio bool) MediaInput {
		path := filepath.Join(dir, name+".mp4")
		args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=10"}
		if audio {
			args = append(args, "-f", "lavfi", "-i", "sine=frequency=660:sample_rate=48000")
		}
		args = append(args, "-t", duration, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", path)
		if out, e := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); e != nil {
			t.Fatalf("fixture %v %s", e, out)
		}
		info, e := media.ProbeFile(ctx, ffprobe, path)
		if e != nil {
			t.Fatal(e)
		}
		return MediaInput{Path: path, Duration: info.Duration, Width: info.Width, Height: info.Height, HasAudio: info.HasAudio}
	}
	source, tail := generate("source", "16", true), generate("tail", "6", true)
	defaults := json.RawMessage(`{"width":320,"height":180,"fps":10,"crf":28,"preset":"ultrafast","blur":2,"brightness":-0.08,"shadow_blur":2,"shadow_opacity":0.35,"shadow_offset":2,"tail_seconds":6,"tail_fade":0.35,"ending_index_key":"ending/index.json","filter_threads":1}`)
	for _, audible := range []bool{true, false} {
		ad := generate("ad", "2", audible)
		for _, transition := range []string{"0", "0.2"} {
			raw := strings.ReplaceAll(string(adParams()), "\"transition_seconds\":0.2", "\"transition_seconds\":"+transition)
			raw = strings.ReplaceAll(raw, "\"ad_start_min_seconds\":10", "\"ad_start_min_seconds\":3")
			raw = strings.ReplaceAll(raw, "\"ad_start_max_seconds\":10", "\"ad_start_max_seconds\":3")
			plan, e := AdPIPEndingV1().Executor.ValidateParams(json.RawMessage(raw))
			if e != nil {
				t.Fatal(e)
			}
			output := filepath.Join(dir, "output.mp4")
			result, e := plan.Compile(RenderInput{Source: source, Resources: map[string]MediaInput{"ad": ad, "ending": tail}, Defaults: defaults, Seed: 42, OutputPath: output})
			if e != nil {
				t.Fatal(e)
			}
			if out, e := exec.CommandContext(ctx, ffmpeg, result.Arguments...).CombinedOutput(); e != nil {
				t.Fatalf("render audio=%v transition=%s %v %s", audible, transition, e, out)
			}
			info, e := media.ProbeFile(ctx, ffprobe, output)
			if e != nil {
				t.Fatal(e)
			}
			expected := float64(result.DurationMS) / 1000
			if !info.HasAudio || info.VideoDuration < expected-.11 || info.VideoDuration > expected+.11 {
				t.Fatalf("output %+v expected %g", info, expected)
			}
			if out, e := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-xerror", "-i", output, "-f", "null", "-").CombinedOutput(); e != nil {
				t.Fatalf("decode %v %s", e, out)
			}
		}
	}
}
