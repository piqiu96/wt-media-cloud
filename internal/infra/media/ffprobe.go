package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type Info struct {
	Duration           float64
	Width, Height      int
	HasAudio           bool
	Format, VideoCodec string
}

// Probe runs the configured binary without a shell. It is shared by Cloud
// file intake and slice preparation; duration is optional for still images.
func ProbeFile(ctx context.Context, binary, filename string) (Info, error) {
	data, err := exec.CommandContext(ctx, binary, "-v", "error", "-protocol_whitelist", "file,pipe", "-show_format", "-show_streams", "-of", "json", filename).Output()
	if err != nil {
		return Info{}, fmt.Errorf("unreadable media: %w", err)
	}
	var report struct {
		Format struct {
			Duration string `json:"duration"`
			Name     string `json:"format_name"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return Info{}, err
	}
	var info Info
	info.Duration, _ = strconv.ParseFloat(report.Format.Duration, 64)
	info.Format = report.Format.Name
	for _, stream := range report.Streams {
		if stream.CodecType == "video" && info.Width == 0 {
			info.Width, info.Height, info.VideoCodec = stream.Width, stream.Height, stream.CodecName
		}
		if stream.CodecType == "audio" {
			info.HasAudio = true
		}
	}
	if info.Width <= 0 || info.Height <= 0 {
		return Info{}, fmt.Errorf("media has no usable image/video stream")
	}
	return info, nil
}
