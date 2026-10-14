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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/ffmpeg"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/jobs"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// The compose worker binary has two modes. Without -input it is the M4-C5
// worker: it claims pending compose tasks and drives each through the
// execution state machine until the row reaches a state no other process
// depends on it for. With -input it stays the M4-C4 verification shell, which
// runs one local cut through the same runner and storage path.
func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("compose-worker", flag.ContinueOnError)
	input := flags.String("input", "", "local video to verify (skips the claim loop)")
	start := flags.Int64("start", 0, "cut start in seconds")
	end := flags.Int64("end", 0, "cut end in seconds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("compose-worker accepts no positional arguments")
	}
	if strings.TrimSpace(*input) != "" {
		return runVerify(ctx, *input, *start, *end)
	}
	return runWorker(ctx)
}

func runVerify(ctx context.Context, input string, start, end int64) error {
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
	registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
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
	params, err := json.Marshal(map[string]int64{"start_seconds": start, "end_seconds": end})
	if err != nil {
		return err
	}
	runner := compose.Runner{FFmpegPath: cfg.Media.FFmpeg.FFmpegPath, FFprobePath: cfg.Media.FFmpeg.FFprobePath, Registry: registry}
	output, err := runner.Run(ctx, runDir, compose.Request{TemplateCode: "basic_cut", TemplateVersion: 1, Params: params, InputPath: input})
	if err != nil {
		return err
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		return err
	}
	defer storage.Close()
	if err := compose.VerifyUpload(ctx, storage.Get(), cfg.Storage.ObjectStorage.PresignTTL.Duration, output.Path, filepath.Base(runDir.Path())); err != nil {
		return err
	}
	if err := runDir.Release(); err != nil {
		return fmt.Errorf("clean compose run directory: %w", err)
	}
	fmt.Printf("compose verified: duration=%dms; object storage readback sha256 matched and temporary key removed\n", output.DurationMS)
	return nil
}

// runWorker brings up everything one claim needs before claiming anything:
// tools verified, storage reachable, database open, workspace and the single
// lock held. A step that fails here leaves the process dead — a worker that
// cannot serve the tasks it claims would spend attempts on its own missing
// environment.
func runWorker(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("compose worker config: %w", err)
	}
	if err := verifyWorkerEnvironment(ctx, cfg); err != nil {
		return err
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		return fmt.Errorf("compose worker object storage: %w", err)
	}
	defer storage.Close()
	if err := database.Initialize(cfg.Databases); err != nil {
		return fmt.Errorf("compose worker database: %w", err)
	}
	defer database.Close()
	if err := repository.CheckComposeCapacity(cfg.Compose.Worker.GlobalConcurrency); err != nil {
		return err
	}

	workspace, lock, err := prepare(ctx, cfg)
	if err != nil {
		return err
	}
	defer lock.Release()

	registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
	if err != nil {
		return err
	}
	runner := compose.Runner{FFmpegPath: cfg.Media.FFmpeg.FFmpegPath, FFprobePath: cfg.Media.FFmpeg.FFprobePath, Registry: registry}
	executorDeps := productionservice.ComposeExecutorDeps{
		Tasks:     repository.ComposeTaskStore{},
		Materials: materialSource{},
		Registry:  registry,
		Store:     storage.Get(),
		Runner:    &runner,
		GrantTTL:  cfg.Storage.ObjectStorage.PresignTTL.Duration,
		Client:    &http.Client{},
		// The preparation is requested in the task creator's name so the
		// transfer row's actor reads as the person whose compose run needs
		// the video, not as the worker process.
		EnsureInputPrepared: func(ctx context.Context, task model.ComposeTask, materialID int64) error {
			material, found, err := repository.FindMaterialUnscoped(materialID)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("material %d is gone", materialID)
			}
			_, err = transferservice.EnsureMaterialSourcePrepare(transferservice.EnsureMaterialSourcePrepareInput{
				TeamID:      material.TeamID,
				AssetID:     material.ID,
				AssetTitle:  material.Title,
				RequestedBy: identityservice.UserID(task.CreatedBy),
			})
			return err
		},
	}
	settings := jobs.ComposeWorkerSettings{
		NodeID:            cfg.Compose.Worker.NodeID,
		Concurrency:       cfg.Compose.Worker.Concurrency,
		WorkspaceRoot:     cfg.Compose.Worker.WorkspaceRoot,
		MinFreeBytes:      cfg.Compose.Worker.MinFreeBytes,
		Lease:             cfg.Compose.Worker.Lease.Duration,
		HeartbeatInterval: cfg.Compose.Worker.HeartbeatInterval.Duration,
		TaskTimeout:       cfg.Compose.Worker.TaskTimeout.Duration,
		PollInterval:      cfg.Compose.Worker.PollInterval.Duration,
		RecoveryInterval:  cfg.Compose.Worker.RecoveryInterval.Duration,
	}
	deps := jobs.ComposeWorkerDeps{
		Tasks:     repository.ComposeTaskStore{},
		Workspace: workspace,
		Attempt: func(ctx context.Context, task model.ComposeTask, runDir *compose.RunDir) productionservice.ComposeAttemptOutcome {
			return productionservice.RunComposeAttempt(ctx, executorDeps, task, settings.NodeID, runDir)
		},
		Log: func(format string, args ...any) { logger.Job().Infof(format, args...) },
	}
	if err := jobs.RunComposeWorker(ctx, deps, settings); err != nil {
		return err
	}
	logger.Job().Infof("compose worker stopped node_id=%d", settings.NodeID)
	return nil
}

// verifyWorkerEnvironment refuses to start when the worker's own environment
// is incomplete — no storage credentials, or an ffmpeg pair the config names
// that is not usable on this host. A worker that cannot serve the tasks it
// claims would spend attempts on that missing environment.
func verifyWorkerEnvironment(ctx context.Context, cfg config.Config) error {
	if cfg.Credentials.ObjectStorage.AccessKey == "" || cfg.Credentials.ObjectStorage.SecretKey == "" {
		return errors.New("compose worker: object storage credentials are not configured")
	}
	tools := cfg.Media.FFmpeg
	if err := ffmpeg.Verify(ctx, ffmpeg.Tools{FFmpegPath: tools.FFmpegPath, FFprobePath: tools.FFprobePath, Version: tools.Version, RequiredEncoders: tools.RequiredEncoders, FFmpegSHA256: tools.FFmpegSHA256, FFprobeSHA256: tools.FFprobeSHA256}); err != nil {
		return fmt.Errorf("compose worker tools: %w", err)
	}
	return nil
}

// materialSource is the worker's whole read path into materials: one lookup,
// the same unscoped read the production service's own store adapter uses.
type materialSource struct{}

func (materialSource) FindMaterial(id int64) (model.Material, bool, error) {
	return repository.FindMaterialUnscoped(id)
}

// prepare assembles the workspace and takes the single-instance lock shared
// with the verification mode. The lock is what makes a lapsed lease mean "the
// previous process is gone" for the recovery scan: no second worker can be
// running its FFmpeg on this host.
func prepare(ctx context.Context, cfg config.Config) (*compose.Workspace, *compose.Lock, error) {
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
