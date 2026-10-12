package main

import (
	"context"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func TestWorkerEnvironmentRejectsMissingStorageCredentials(t *testing.T) {
	cfg := config.Config{}
	cfg.Compose.Worker.WorkspaceRoot = t.TempDir()
	err := verifyWorkerEnvironment(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("prepare error = %v", err)
	}
}

func TestWorkerEnvironmentRejectsMissingFFmpegTools(t *testing.T) {
	cfg := config.Config{}
	cfg.Compose.Worker.WorkspaceRoot = t.TempDir()
	cfg.Credentials.ObjectStorage.AccessKey = "test"
	cfg.Credentials.ObjectStorage.SecretKey = "test"
	cfg.Media.FFmpeg.Version = "8.1.2"
	err := verifyWorkerEnvironment(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "ffprobe") {
		t.Fatalf("prepare error = %v", err)
	}
}
