package compose

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os/exec"
	"time"
)

// ExtractThumbnail reads the first decoded frame of the verified final video.
// It never substitutes a source-material image or a sample asset.
func ExtractThumbnail(ctx context.Context, binary, video string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, binary, "-v", "error", "-nostdin", "-xerror", "-protocol_whitelist", "file,pipe", "-i", video, "-frames:v", "1", "-an", "-threads", "1", "-f", "image2pipe", "-c:v", "png", "pipe:1").Output()
	if err != nil {
		return nil, fmt.Errorf("extract output thumbnail: %w", err)
	}
	if len(data) == 0 || len(data) > 16<<20 {
		return nil, fmt.Errorf("invalid thumbnail size")
	}
	if _, err = png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("invalid output thumbnail: %w", err)
	}
	return data, nil
}
