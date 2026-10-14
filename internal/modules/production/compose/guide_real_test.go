package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

func TestGuideTemplatesProducePlayableVideo(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	ctx := context.Background()
	dir := t.TempDir()
	makeClip := func(name, color, tone, duration string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c="+color+":s=320x240:r=30", "-f", "lavfi", "-i", "sine=frequency="+tone+":sample_rate=48000", "-t", duration, "-c:v", "libx264", "-c:a", "aac", "-y", path)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("make %s: %v %s", name, err, output)
		}
		return path
	}
	source := makeClip("source.mp4", "red", "440", "3")
	guide := makeClip("guide.mp4", "blue", "880", "2")
	registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{FFmpegPath: ffmpeg, FFprobePath: ffprobe, Registry: registry}
	workspace, err := NewWorkspace(filepath.Join(dir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	frameColor := func(t *testing.T, path, when string, x, y int) string {
		t.Helper()
		command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-ss", when, "-i", path, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "png", "-")
		frame, err := command.Output()
		if err != nil {
			t.Fatalf("extract frame: %v", err)
		}
		img, err := png.Decode(bytes.NewReader(frame))
		if err != nil {
			t.Fatal(err)
		}
		r, _, b, _ := img.At(x, y).RGBA()
		if r > b*2 {
			return "red"
		}
		if b > r*2 {
			return "blue"
		}
		return "other"
	}
	for _, tc := range []struct {
		name, mode                         string
		insert, guideDuration, maxDuration int64
		expectedMS                         int64
	}{
		{"insert_middle", "guide_insert", 1, 0, 0, 5000},
		{"insert_end", "guide_insert", 8, 1, 0, 4000},
		{"insert_limit", "guide_insert", 1, 0, 4, 4000},
		{"pip_middle", "guide_pip", 1, 0, 0, 5000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runDir, err := workspace.NewRun(strings.ReplaceAll(tc.name, "_", "-"))
			if err != nil {
				t.Fatal(err)
			}
			defer runDir.Release()
			params, _ := json.Marshal(map[string]any{"guide_video_url": "https://media.example.com/guide.mp4", "insert_seconds": tc.insert, "guide_duration_seconds": tc.guideDuration, "max_duration_seconds": tc.maxDuration})
			output, err := runner.Run(ctx, runDir, Request{TemplateCode: tc.mode, TemplateVersion: 1, Params: params, InputPath: source, GuidePath: guide})
			if err != nil {
				t.Fatal(err)
			}
			if output.DurationMS < tc.expectedMS-300 || output.DurationMS > tc.expectedMS+300 {
				t.Fatalf("duration = %dms; want about %dms", output.DurationMS, tc.expectedMS)
			}
			if tc.name == "insert_middle" || tc.name == "pip_middle" {
				if color := frameColor(t, output.Path, "0.5", 20, 20); color != "red" {
					t.Fatalf("source before insert = %s", color)
				}
				if color := frameColor(t, output.Path, "1.5", 200, 160); color != "blue" {
					t.Fatalf("guide at insert = %s", color)
				}
				if tc.name == "pip_middle" {
					if color := frameColor(t, output.Path, "1.5", 20, 20); color != "red" {
						t.Fatalf("PIP background = %s", color)
					}
				}
			}
		})
	}
}

func TestGuideInsertAcceptsVideoWithoutAudio(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	dir := t.TempDir()
	clip := func(name, color string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if output, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c="+color+":s=320x240:r=30", "-t", "2", "-c:v", "libx264", "-y", path).CombinedOutput(); err != nil {
			t.Fatalf("clip: %v %s", err, output)
		}
		return path
	}
	source, guide := clip("silent-source.mp4", "red"), clip("silent-guide.mp4", "blue")
	registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := NewWorkspace(filepath.Join(dir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	runDir, err := workspace.NewRun("silent-run")
	if err != nil {
		t.Fatal(err)
	}
	defer runDir.Release()
	runner := Runner{FFmpegPath: ffmpeg, FFprobePath: ffprobe, Registry: registry}
	output, err := runner.Run(context.Background(), runDir, Request{TemplateCode: "guide_insert", TemplateVersion: 1, Params: json.RawMessage(`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":1,"guide_duration_seconds":0,"max_duration_seconds":0}`), InputPath: source, GuidePath: guide})
	if err != nil {
		t.Fatal(err)
	}
	if output.DurationMS < 3700 || output.DurationMS > 4300 {
		t.Fatalf("duration=%d", output.DurationMS)
	}
}
