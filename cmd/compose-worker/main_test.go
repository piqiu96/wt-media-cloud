package main

import (
	"context"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func TestPrepareRejectsMissingStorageCredentials(t *testing.T) {
	_, _, err := prepare(context.Background(), config.Config{})
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("prepare error = %v", err)
	}
}

func TestPrepareRejectsMissingFFmpegTools(t *testing.T) {
	cfg := config.Config{}
	cfg.Credentials.ObjectStorage.AccessKey = "test"
	cfg.Credentials.ObjectStorage.SecretKey = "test"
	cfg.Media.FFmpeg.Version = "8.1.2"
	_, _, err := prepare(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "ffprobe") {
		t.Fatalf("prepare error = %v", err)
	}
}
