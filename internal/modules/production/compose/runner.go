package compose

import (
	"context"
	"encoding/json"
	"fmt"
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
	GuidePath       string
}

// Output.Path remains in the run directory until the caller releases it.
type Output struct {
	Path       string
	DurationMS int64
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
	startMS, endMS := plan.SegmentMS()

	input, err := probeFile(ctx, r.FFprobePath, request.InputPath)
	if err != nil {
		return Output{}, fmt.Errorf("input %s: %w", request.InputPath, err)
	}
	if !input.hasVideo {
		return Output{}, fmt.Errorf("input %s has no video stream", request.InputPath)
	}
	if _, guide := plan.(executor.GuidePlan); !guide && input.durationMS < endMS {
		return Output{}, fmt.Errorf("input %s is %d ms long, shorter than the requested window ending at %d ms", request.InputPath, input.durationMS, endMS)
	}

	outputPath := filepath.Join(runDir.Path(), "output.mp4")
	arguments := plan.FFmpegArguments(request.InputPath, outputPath)
	requestedMS := endMS - startMS
	if guidePlan, guide := plan.(executor.GuidePlan); guide {
		if request.GuidePath == "" {
			return Output{}, fmt.Errorf("guide video is required")
		}
		guideInput, err := probeFile(ctx, r.FFprobePath, request.GuidePath)
		if err != nil {
			return Output{}, fmt.Errorf("guide input: %w", err)
		}
		if !guideInput.hasVideo {
			return Output{}, fmt.Errorf("guide input has no video stream")
		}
		requestedMS, err = guidePlan.OutputDurationMS(input.durationMS, guideInput.durationMS)
		if err != nil {
			return Output{}, err
		}
		arguments, err = guidePlan.FFmpegGuideArguments(request.InputPath, request.GuidePath, outputPath, input.durationMS, guideInput.durationMS, input.width, input.height, input.hasAudio, guideInput.hasAudio)
		if err != nil {
			return Output{}, err
		}
	}
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
	if output.durationMS*10 < requestedMS*9 {
		return Output{}, fmt.Errorf("output duration %d ms is shorter than 90%% of the requested %d ms", output.durationMS, requestedMS)
	}
	if output.durationMS*10 > requestedMS*11 {
		return Output{}, fmt.Errorf("output duration %d ms is longer than 110%% of the requested %d ms", output.durationMS, requestedMS)
	}
	return Output{Path: outputPath, DurationMS: output.durationMS}, nil
}
