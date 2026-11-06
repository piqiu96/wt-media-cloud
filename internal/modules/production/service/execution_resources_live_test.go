package service

import (
	"context"
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveDynamicPoolUsesCurrentBucket(t *testing.T) {
	if os.Getenv("WT_MEDIA_LIVE_DYNAMIC_POOL") != "1" {
		t.Skip("explicit configured storage read")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.HomeEnvVar, root)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var defaults executor.DynamicDefaults
	raw, err := json.Marshal(cfg.Compose.Templates["dynamic_pip_ending"].Render)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &defaults); err != nil {
		t.Fatal(err)
	}
	got, err := choosePoolResource(ctx, storage.Get(), defaults.EndingIndexKey, 20261106)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.Location, "s3://"+cfg.Storage.ObjectStorage.Bucket+"/") {
		t.Fatal("selected an old-bucket clip")
	}
	info, err := storage.Get().Stat(ctx, got.Location)
	if err != nil || info.Size != got.Size {
		t.Fatalf("selected clip missing: %v", err)
	}
	t.Logf("current-bucket index and selected clip metadata verified; bytes=%d", got.Size)
}
