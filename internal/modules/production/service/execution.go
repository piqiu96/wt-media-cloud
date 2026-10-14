package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

// ComposeAttemptOutcome tells the worker loop what to do after one attempt.
// The service owns the classification and the row writes; the loop only acts.
type ComposeAttemptOutcome string

const (
	ComposeAttemptSuccess   ComposeAttemptOutcome = "success"
	ComposeAttemptWaitInput ComposeAttemptOutcome = "wait_input"
	ComposeAttemptCancelled ComposeAttemptOutcome = "cancelled"
	ComposeAttemptRetry     ComposeAttemptOutcome = "retry"
	ComposeAttemptFailed    ComposeAttemptOutcome = "failed"
	ComposeAttemptLeaseLost ComposeAttemptOutcome = "lease_lost"
)

// Task error_code values stored on failures. They surface through the task
// detail API as plain strings (openapi leaves error_code free).
const (
	composeErrTemplateUnavailable = "template_unavailable"
	composeErrInputMissing        = "input_missing"
	composeErrInputInvalid        = "input_invalid"
	composeErrExecutionFailed     = "execution_failed"
	composeErrStorageError        = "storage_error"
	composeErrTaskTimeout         = "task_timeout"
)

// Compose stage/progress boundaries (change.md §7: stage enum, progress at
// stage boundaries, no per-frame percentages).
const (
	composeStageValidatingInput = "validating_input"
	composeStageWaitingInput    = "waiting_input"
	composeStageCutting         = "cutting"
	composeStageUploading       = "uploading"
	composeStageCommitting      = "committing"
)

// composeRunner is the slice of compose.Runner the attempt needs; tests stub
// it because the real one needs FFmpeg.
type composeRunner interface {
	Run(ctx context.Context, runDir *compose.RunDir, request compose.Request) (compose.Output, error)
}

// composeTaskLifecycle is the slice of the task repository the worker drives.
type composeTaskLifecycle interface {
	RenewComposeLease(taskID, nodeID int64, lease time.Duration) (bool, error)
	ReportComposeProgress(taskID, nodeID int64, stage string, progress int) (bool, error)
	CommitComposeOutput(out model.CompositeOutput, outputKey string, nodeID int64) (model.ComposeOutputCommit, error)
	ReleaseComposeForRetry(taskID, nodeID int64, errorCode, message string) (bool, error)
	FailComposeTask(taskID, nodeID int64, errorCode, message string) (bool, error)
	Find(id int64) (model.ComposeTask, bool, error)
}

// EnsureInputPrepared asks the transfer domain to start a cloud-side input
// preparation for a material whose source is not ready yet. It is injectable
// because the file_transfer_task boundary (spec §4.4) lives in another module,
// and it sees the task because the preparation is requested in the task
// creator's name — the transfer row's actor comes from it.
type EnsureInputPrepared func(ctx context.Context, requestor model.ComposeTask, materialID int64) error

// ComposeExecutorDeps carries the collaborators of one attempt. Everything is
// an interface or a function so the unit tests substitute memory fakes.
type ComposeExecutorDeps struct {
	Tasks               composeTaskLifecycle
	Materials           composeTaskMaterialSource
	Registry            *executor.Registry
	Store               storage.Store
	Runner              composeRunner
	GrantTTL            time.Duration
	Client              *http.Client
	FetchGuide          func(context.Context, string, string) error
	EnsureInputPrepared EnsureInputPrepared
}

// RunComposeAttempt executes one claimed task inside an owned run directory:
// validate against the snapshot, gate on input readiness, run the executor,
// upload to a temporary key, verify, copy to the formal key, and commit the
// output and the success flip in one transaction. Source facts come only from
// the task row — the latest strategy config is never read here (ADR-0021).
func RunComposeAttempt(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, runDir *compose.RunDir) ComposeAttemptOutcome {
	if !ownsProgress(deps.Tasks, task.ID, nodeID, composeStageValidatingInput, 10) {
		return ComposeAttemptLeaseLost
	}
	if err := validateSnapshotTemplate(deps.Registry, task); err != nil {
		return failTerminal(deps.Tasks, task.ID, nodeID, composeErrTemplateUnavailable, err.Error())
	}
	inputPath, outcome := prepareInput(ctx, deps, task, nodeID, runDir)
	if outcome != "" {
		return outcome
	}
	guidePath := ""
	definition, _ := deps.Registry.Lookup(task.TemplateCode, task.TemplateVersion)
	plan, _ := definition.Executor.ValidateParams(task.ConfigSnapshot)
	if guide, ok := plan.(executor.GuidePlan); ok {
		guidePath = filepath.Join(runDir.Path(), "guide-input")
		fetch := deps.FetchGuide
		if fetch == nil {
			fetch = compose.DownloadGuide
		}
		if err := fetch(ctx, guide.GuideURL(), guidePath); err != nil {
			return failTerminal(deps.Tasks, task.ID, nodeID, composeErrInputInvalid, err.Error())
		}
	}
	if cancelRequested(deps.Tasks, task.ID) {
		return ComposeAttemptCancelled
	}
	if !ownsProgress(deps.Tasks, task.ID, nodeID, composeStageCutting, 40) {
		return ComposeAttemptLeaseLost
	}
	output, err := deps.Runner.Run(ctx, runDir, compose.Request{TemplateCode: task.TemplateCode, TemplateVersion: task.TemplateVersion, Params: task.ConfigSnapshot, InputPath: inputPath, GuidePath: guidePath})
	if err != nil {
		// A run the worker terminated speaks with its context, not with
		// FFmpeg's last words: a cancel request is a controlled stop, a spent
		// deadline is transient (the plan was fine, the machine was slow), and
		// anything else is FFmpeg or probe failing on the input or the plan —
		// spec §5.5 business failure, never auto-retried.
		if cancelRequested(deps.Tasks, task.ID) {
			return ComposeAttemptCancelled
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return transientFailure(deps, task, nodeID, composeErrTaskTimeout, err.Error())
		}
		return failTerminal(deps.Tasks, task.ID, nodeID, composeErrExecutionFailed, err.Error())
	}
	if cancelRequested(deps.Tasks, task.ID) {
		return ComposeAttemptCancelled
	}
	if !ownsProgress(deps.Tasks, task.ID, nodeID, composeStageUploading, 80) {
		return ComposeAttemptLeaseLost
	}
	tempKey := "compose-staging/" + strconv.FormatInt(task.ID, 10) + "/output.mp4"
	formalKey := "compose-outputs/" + strconv.FormatInt(task.ID, 10) + "/output.mp4"
	size, digest, err := uploadVerified(ctx, deps.Store, deps.Client, output.Path, tempKey)
	if err != nil {
		return transientFailure(deps, task, nodeID, composeErrStorageError, err.Error())
	}
	if !ownsProgress(deps.Tasks, task.ID, nodeID, composeStageCommitting, 90) {
		return ComposeAttemptLeaseLost
	}
	if err := deps.Store.Copy(ctx, tempKey, formalKey); err != nil {
		return transientFailure(deps, task, nodeID, composeErrStorageError, "copy to formal key: "+err.Error())
	}
	media, _ := json.Marshal(map[string]int64{"duration_ms": output.DurationMS})
	out := model.CompositeOutput{
		TeamID: task.TeamID, GameID: task.GameID,
		MaterialID: task.MaterialID, MaterialUsageID: task.MaterialUsageID,
		TaskID: task.ID, StrategyID: task.StrategyID,
		TemplateCode: task.TemplateCode, TemplateVersion: task.TemplateVersion,
		ObjectKey: formalKey, FileSizeBytes: size, SHA256: digest, Media: media,
		PublishStatus: "pending_publish", CreatedBy: task.CreatedBy,
	}
	if _, err := deps.Tasks.CommitComposeOutput(out, formalKey, nodeID); err != nil {
		// The row is no longer ours (cancelled, reaped, finished): the formal
		// object is an orphan now and must not survive its dead task.
		_ = deps.Store.Remove(context.WithoutCancel(ctx), formalKey)
		_ = deps.Store.Remove(context.WithoutCancel(ctx), tempKey)
		return ComposeAttemptLeaseLost
	}
	_ = deps.Store.Remove(context.WithoutCancel(ctx), tempKey)
	return ComposeAttemptSuccess
}

// validateSnapshotTemplate checks the snapshot's binding against the executor
// registry before any bytes move: an unknown template version or a params
// rejection is a business failure with a clean task row, not a half-run.
func validateSnapshotTemplate(registry *executor.Registry, task model.ComposeTask) error {
	definition, err := registry.Lookup(task.TemplateCode, task.TemplateVersion)
	if err != nil {
		return fmt.Errorf("template %s v%d is not deployed to this worker: %w", task.TemplateCode, task.TemplateVersion, err)
	}
	if _, err := definition.Executor.ValidateParams(task.ConfigSnapshot); err != nil {
		return fmt.Errorf("snapshot params rejected by %s v%d: %w", task.TemplateCode, task.TemplateVersion, err)
	}
	return nil
}

// prepareInput gates on readiness (PRD §5.4.4: an unready source keeps the
// task waiting, it never fabricates an input), and stages the prepared source
// into the run directory. A non-empty outcome short-circuits the attempt.
func prepareInput(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, runDir *compose.RunDir) (string, ComposeAttemptOutcome) {
	material, found, err := deps.Materials.FindMaterial(task.MaterialID)
	if err != nil {
		return "", transientFailure(deps, task, nodeID, composeErrStorageError, "read material: "+err.Error())
	}
	if !found {
		return "", failTerminal(deps.Tasks, task.ID, nodeID, composeErrInputMissing, "material row is gone")
	}
	if material.VideoStatus == model.VideoFailed {
		return "", failTerminal(deps.Tasks, task.ID, nodeID, composeErrInputInvalid, "source preparation failed for material "+strconv.FormatInt(task.MaterialID, 10))
	}
	if material.VideoStatus != model.VideoReady || material.SourceObjectKey == "" {
		if deps.EnsureInputPrepared != nil {
			if err := deps.EnsureInputPrepared(ctx, task, task.MaterialID); err != nil {
				return "", transientFailure(deps, task, nodeID, composeErrStorageError, "request input preparation: "+err.Error())
			}
		}
		_, _ = deps.Tasks.ReportComposeProgress(task.ID, nodeID, composeStageWaitingInput, 20)
		return "", ComposeAttemptWaitInput
	}
	if _, err := deps.Store.Stat(ctx, material.SourceObjectKey); err != nil {
		return "", transientFailure(deps, task, nodeID, composeErrStorageError, "source object: "+err.Error())
	}
	grant, err := deps.Store.PresignGet(ctx, material.SourceObjectKey, deps.GrantTTL)
	if err != nil {
		return "", transientFailure(deps, task, nodeID, composeErrStorageError, "presign source: "+err.Error())
	}
	inputPath := filepath.Join(runDir.Path(), "source-input")
	if err := downloadFile(ctx, deps.Client, grant.URL, inputPath); err != nil {
		return "", transientFailure(deps, task, nodeID, composeErrStorageError, "download source: "+err.Error())
	}
	return inputPath, ""
}

// uploadVerified writes the output to the temporary key, then proves the
// stored bytes equal the local file (size and sha256 through a presigned
// readback) before the caller commits anything formal.
func uploadVerified(ctx context.Context, store storage.Store, client *http.Client, path, tempKey string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("open compose output: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", err
	}
	if info.Size() <= 0 {
		return 0, "", errors.New("compose output is empty")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, "", fmt.Errorf("hash compose output: %w", err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, "", err
	}
	if err := store.Put(ctx, tempKey, file, info.Size(), "video/mp4"); err != nil {
		return 0, "", fmt.Errorf("upload temporary object: %w", err)
	}
	remote, err := store.Stat(ctx, tempKey)
	if err != nil {
		return 0, "", fmt.Errorf("stat temporary object: %w", err)
	}
	if remote.Size != info.Size() {
		return 0, "", fmt.Errorf("temporary object size mismatch: local %d, remote %d", info.Size(), remote.Size)
	}
	grant, err := store.PresignGet(ctx, tempKey, 10*time.Minute)
	if err != nil {
		return 0, "", fmt.Errorf("presign temporary object: %w", err)
	}
	readHash := sha256.New()
	if err := downloadInto(ctx, client, grant.URL, readHash); err != nil {
		return 0, "", fmt.Errorf("readback temporary object: %w", err)
	}
	if hex.EncodeToString(readHash.Sum(nil)) != digest {
		return 0, "", errors.New("readback sha256 mismatch")
	}
	return info.Size(), digest, nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	return downloadInto(ctx, client, url, file)
}

func downloadInto(ctx context.Context, client *http.Client, url string, sink io.Writer) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, response.StatusCode)
	}
	_, err = io.Copy(sink, response.Body)
	return err
}

func ownsProgress(tasks composeTaskLifecycle, taskID, nodeID int64, stage string, progress int) bool {
	ok, _ := tasks.ReportComposeProgress(taskID, nodeID, stage, progress)
	return ok
}

func cancelRequested(tasks composeTaskLifecycle, taskID int64) bool {
	item, found, err := tasks.Find(taskID)
	return err == nil && found && item.CancelRequested
}

// failTerminal marks the task failed and reports the outcome; the write is
// best-effort — a lost lease leaves the row to the recovery scan, which is
// the honest owner of that situation.
func failTerminal(tasks composeTaskLifecycle, taskID, nodeID int64, code, message string) ComposeAttemptOutcome {
	_, _ = tasks.FailComposeTask(taskID, nodeID, code, message)
	return ComposeAttemptFailed
}

// transientFailure requeues within the attempt budget, or terminal-fails with
// the exhaustion note once the budget is spent — a task must never end up
// pending forever with no attempts left.
func transientFailure(deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, code, message string) ComposeAttemptOutcome {
	if task.AttemptCount < task.MaxAttempts {
		if ok, _ := deps.Tasks.ReleaseComposeForRetry(task.ID, nodeID, code, message); ok {
			return ComposeAttemptRetry
		}
		return ComposeAttemptLeaseLost
	}
	return failTerminal(deps.Tasks, task.ID, nodeID, code, message+fmt.Sprintf(" (attempts exhausted after %d)", task.AttemptCount))
}
