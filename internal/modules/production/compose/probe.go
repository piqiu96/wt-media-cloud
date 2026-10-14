package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// videoProbe is what the pipeline needs to know about one file: whether it has
// a video stream and how long it declares itself to be.
type videoProbe struct {
	durationMS int64
	hasVideo   bool
	hasAudio   bool
	width      int
	height     int
}

// probeFile asks ffprobe about one file with fixed arguments. The report is
// JSON, but only two facts of it are read: a missing or unusable duration is an
// error, because both the input check and the output check depend on it.
func probeFile(ctx context.Context, ffprobePath, path string) (videoProbe, error) {
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, ffprobePath, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return videoProbe{}, fmt.Errorf("probe: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var report struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return videoProbe{}, fmt.Errorf("probe: unreadable report: %w", err)
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(report.Format.Duration), 64)
	if err != nil {
		return videoProbe{}, fmt.Errorf("probe: no usable duration: %w", err)
	}
	if seconds <= 0 {
		return videoProbe{}, errors.New("probe: no usable duration")
	}
	probe := videoProbe{durationMS: int64(seconds*1000 + 0.5)}
	for _, stream := range report.Streams {
		if stream.CodecType == "video" {
			probe.hasVideo = true
			probe.width, probe.height = stream.Width, stream.Height
		} else if stream.CodecType == "audio" {
			probe.hasAudio = true
		}
	}
	return probe, nil
}

// runFFmpeg executes the executor's fixed command. stderr is captured because
// it is the only thing that says why an encode failed; it is reported with the
// error and never silently dropped.
func runFFmpeg(ctx context.Context, path string, arguments []string) error {
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, path, arguments...)
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("ffmpeg encode: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
