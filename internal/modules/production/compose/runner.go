package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

type Runner struct {
	FFmpegPath  string
	FFprobePath string
	Registry    *executor.Registry
}

type Request struct {
	TemplateCode    string
	TemplateVersion uint32
	Params          json.RawMessage
	InputPath       string
	Resources       map[string]string
	Defaults        json.RawMessage
	Seed            int64
}

// Output.Path remains in the run directory until the caller releases it.
type Output struct {
	Path       string
	DurationMS int64
	Width      int
	Height     int
	Timeline   json.RawMessage
}

func (r *Runner) Run(ctx context.Context, runDir *RunDir, request Request) (Output, error) {
	definition, err := r.Registry.Lookup(request.TemplateCode, request.TemplateVersion)
	if err != nil {
		return Output{}, err
	}
	plan, err := definition.Executor.ValidateParams(request.Params)
	if err != nil {
		return Output{}, err
	}
	resourcePlan, ok := plan.(executor.ResourcePlan)
	if !ok {
		return Output{}, fmt.Errorf("template executor does not implement the common render entry")
	}
	probe := func(filename string) (executor.MediaInput, error) {
		info, e := probeFile(ctx, r.FFprobePath, filename)
		if e != nil {
			return executor.MediaInput{}, e
		}
		if !info.hasVideo {
			return executor.MediaInput{}, fmt.Errorf("input has no video stream")
		}
		duration := info.durationMS
		if info.videoDurationMS > 0 {
			duration = info.videoDurationMS
		}
		return executor.MediaInput{Path: filename, Duration: float64(duration) / 1000, Width: info.width, Height: info.height, HasAudio: info.hasAudio}, nil
	}
	input, err := probe(request.InputPath)
	if err != nil {
		return Output{}, err
	}
	resources := map[string]executor.MediaInput{}
	for name, filename := range request.Resources {
		if filename == "" {
			continue
		}
		item, e := probe(filename)
		if e != nil {
			return Output{}, fmt.Errorf("resource %s: %w", name, e)
		}
		resources[name] = item
	}
	outputPath := filepath.Join(runDir.Path(), "output.mp4")
	render, err := resourcePlan.Compile(executor.RenderInput{Source: input, Resources: resources, Defaults: request.Defaults, Seed: request.Seed, OutputPath: outputPath})
	if err != nil {
		return Output{}, err
	}
	arguments, requestedMS := render.Arguments, render.DurationMS
	if err := runFFmpeg(ctx, r.FFmpegPath, arguments); err != nil {
		return Output{}, err
	}
	if _, err := os.Stat(outputPath); err != nil {
		return Output{}, fmt.Errorf("ffmpeg reported success but the output file is missing: %w", err)
	}

	output, err := probeFile(ctx, r.FFprobePath, outputPath)
	if err != nil {
		return Output{}, fmt.Errorf("output: %w", err)
	}
	if !output.hasVideo {
		return Output{}, fmt.Errorf("output has no video stream")
	}
	if render.RequireAudio && !output.hasAudio {
		return Output{}, fmt.Errorf("output has no audio stream")
	}
	if render.ToleranceMS > 0 {
		if math.Abs(float64(output.durationMS-requestedMS)) > float64(render.ToleranceMS) || output.videoDurationMS <= 0 || math.Abs(float64(output.videoDurationMS-requestedMS)) > float64(render.ToleranceMS) || render.RequireAudio && (output.audioDurationMS <= 0 || math.Abs(float64(output.audioDurationMS-requestedMS)) > float64(render.ToleranceMS)) {
			return Output{}, fmt.Errorf("output audio/video duration does not match the timeline")
		}
		if err := runFFmpeg(ctx, r.FFmpegPath, []string{"-v", "error", "-nostdin", "-xerror", "-protocol_whitelist", "file,pipe", "-i", outputPath, "-f", "null", "-"}); err != nil {
			return Output{}, err
		}
	}
	if output.durationMS*10 < requestedMS*9 {
		return Output{}, fmt.Errorf("output duration %d ms is shorter than 90%% of the requested %d ms", output.durationMS, requestedMS)
	}
	if output.durationMS*10 > requestedMS*11 {
		return Output{}, fmt.Errorf("output duration %d ms is longer than 110%% of the requested %d ms", output.durationMS, requestedMS)
	}
	return Output{Path: outputPath, DurationMS: output.durationMS, Width: output.width, Height: output.height, Timeline: render.Timeline}, nil
}
