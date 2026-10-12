package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/ffmpeg"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("compose-worker", flag.ContinueOnError)
	input := flags.String("input", "", "local video to verify")
	start := flags.Int64("start", 0, "cut start in seconds")
	end := flags.Int64("end", 0, "cut end in seconds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("compose-worker accepts no positional arguments")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	workspace, lock, err := prepare(ctx, cfg)
	if err != nil {
		return err
	}
	defer lock.Release()
	if strings.TrimSpace(*input) == "" {
		return errors.New("input video is required")
	}
	registry, err := executor.NewRegistry(executor.BasicCutV1())
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	runDir, err := workspace.NewRun("verify-" + hex.EncodeToString(id))
	if err != nil {
		return err
	}
	defer runDir.Release()
	params, err := json.Marshal(map[string]int64{"start_seconds": *start, "end_seconds": *end})
	if err != nil {
		return err
	}
	runner := compose.Runner{FFmpegPath: cfg.Media.FFmpeg.FFmpegPath, FFprobePath: cfg.Media.FFmpeg.FFprobePath, Registry: registry}
	output, err := runner.Run(ctx, runDir, compose.Request{TemplateCode: "basic_cut", TemplateVersion: 1, Params: params, InputPath: *input})
	if err != nil {
		return err
	}
	fmt.Printf("compose verified: %s duration=%dms\n", output.Path, output.DurationMS)
	return nil
}

// The verifier needs only configuration, media tools, storage and one work directory.
func prepare(ctx context.Context, cfg config.Config) (*compose.Workspace, *compose.Lock, error) {
	if cfg.Credentials.ObjectStorage.AccessKey == "" || cfg.Credentials.ObjectStorage.SecretKey == "" {
		return nil, nil, errors.New("compose worker: object storage credentials are not configured")
	}
	tools := cfg.Media.FFmpeg
	if err := ffmpeg.Verify(ctx, ffmpeg.Tools{FFmpegPath: tools.FFmpegPath, FFprobePath: tools.FFprobePath, Version: tools.Version, RequiredEncoders: tools.RequiredEncoders, FFmpegSHA256: tools.FFmpegSHA256, FFprobeSHA256: tools.FFprobeSHA256}); err != nil {
		return nil, nil, fmt.Errorf("compose worker tools: %w", err)
	}
	workspace, err := compose.NewWorkspace(cfg.Compose.Worker.WorkspaceRoot)
	if err != nil {
		return nil, nil, err
	}
	lock, err := compose.AcquireLock(filepath.Join(workspace.Root(), "compose-worker.lock"))
	if err != nil {
		return nil, nil, err
	}
	return workspace, lock, nil
}
