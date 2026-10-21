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
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
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

var errComposeOutputInvalid = errors.New("compose output failed verification")

// Task error_code values stored on failures. They surface through the task
// detail API as plain strings (openapi leaves error_code free).
const (
	composeErrTemplateUnavailable = "template_unavailable"
	composeErrInputMissing        = "input_missing"
	composeErrInputInvalid        = "input_invalid"
	composeErrExecutionFailed     = "execution_failed"
	composeErrStorageError        = "storage_error"
	composeErrTaskTimeout         = "task_timeout"
	composeErrExecutionUncertain  = "execution_uncertain"
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
	RenewComposeLease(taskID, nodeID int64, attempt int, lease time.Duration) (bool, error)
	ReportComposeProgress(taskID, nodeID int64, attempt int, stage string, progress int) (bool, error)
	RecordComposeAttemptStep(taskID, nodeID int64, attempt int, step model.ComposeTaskAttemptStep) (bool, error)
	CommitComposeOutput(out model.CompositeOutput, outputKey string, nodeID int64, attempt int, step model.ComposeTaskAttemptStep) (model.ComposeOutputCommit, error)
	ReleaseComposeForRetry(taskID, nodeID int64, attempt int, errorCode, message string) (bool, error)
	FailComposeTask(taskID, nodeID int64, attempt int, errorCode, message string) (bool, error)
	Find(id int64) (model.ComposeTask, bool, error)
}

// EnsureInputPrepared asks the transfer domain to start a cloud-side input
// preparation for a material whose source is not ready, and reports whether the
// material's projection was moved to `downloading` by that request.
//
// It is injectable because the file_transfer_task boundary (spec §4.4) lives in
// another module, and it sees the task and the material because the preparation
// is requested in the task creator's name — the transfer row's actor comes from
// the task — and because which write is safe depends on what the caller saw: a
// `ready` row with a key is taken back only by the key-and-timestamp compare,
// every other state by the ordinary preparation marker.
type EnsureInputPrepared func(ctx context.Context, requestor model.ComposeTask, material model.Material) (observed bool, err error)

// ComposeInputGate is one claim's memory of its single self-heal attempt.
//
// A claim is the unit of retry budget, so it is also the unit of self-heal: the
// first time an attempt finds the source missing it may ask for one preparation,
// and everything after that waits for or judges that one request. The gate
// survives the `waiting_input` loop (the worker polls the attempt again and
// again under the same claim) and is dropped when the claim ends — a manual
// retry is a new claim and gets a fresh gate, which is exactly the "retry once"
// a user means by clicking retry.
//
// The three fields are written on the attempt's goroutine only; the worker does
// not share them between slots.
type ComposeInputGate struct {
	// PrepareRequested says this claim has asked for a preparation. At most one
	// request per claim: a request that does not produce a source ends the task
	// instead of being repeated until the timeout.
	PrepareRequested bool
	// PrepareObserved says this claim has seen evidence that the requested
	// preparation actually ran: the request's mark moved the projection, the
	// material was seen `downloading`, or a `ready` row appeared carrying a
	// different object key. A source that is still missing after the request was
	// observed is a failed self-heal, not a slow one.
	PrepareObserved bool
	// RequestedKey is the object key that was observed missing when the request
	// was made. A later `ready` row naming a different key is a result of this
	// request even though the content-addressed key is usually the same.
	RequestedKey string
}

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

// attemptStep marks when the phase currently running started, so both a
// completed step and a failed one report how long it ran.
type attemptStep struct {
	key     string
	started time.Time
}

func beginStep(key string) attemptStep { return attemptStep{key: key, started: time.Now()} }

// The metric builders keep call sites from spelling out a pointer literal per
// step; fields that are not set stay omitted from the stored entry.
func sizeMetrics(bytes int64) model.ComposeAttemptStepMetrics {
	return model.ComposeAttemptStepMetrics{SizeBytes: &bytes}
}

func uploadMetrics(size int64, digest string) model.ComposeAttemptStepMetrics {
	return model.ComposeAttemptStepMetrics{SizeBytes: &size, SHA256: digest}
}

func durationMetrics(milliseconds int64) model.ComposeAttemptStepMetrics {
	return model.ComposeAttemptStepMetrics{OutputDurationMS: &milliseconds}
}

// fileMetrics reports a staged file's size, or nothing when the filesystem
// cannot tell us: every metric is a fact about the bytes this worker moved.
func fileMetrics(path string) model.ComposeAttemptStepMetrics {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 0 {
		return model.ComposeAttemptStepMetrics{}
	}
	return sizeMetrics(info.Size())
}

// RunComposeAttempt executes one claimed task inside an owned run directory:
// validate against the snapshot, gate on input readiness, run the executor,
// upload to a temporary key, verify, copy to the formal key, and commit the
// output and the success flip in one transaction. Source facts come only from
// the task row — the latest strategy config is never read here (ADR-0021).
//
// gate is the claim's self-heal memory and is reused across the waiting loop;
// nil is a fresh zero gate, which is what a single-shot caller (and every test
// that does not exercise the wait) means by calling once.
func RunComposeAttempt(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, runDir *compose.RunDir, gate *ComposeInputGate) ComposeAttemptOutcome {
	if gate == nil {
		gate = &ComposeInputGate{}
	}
	validateStep := beginStep(model.ComposeStepValidateInput)
	if !ownsProgress(deps.Tasks, task, nodeID, composeStageValidatingInput, 10) {
		return ComposeAttemptLeaseLost
	}
	plan, err := validateSnapshotTemplate(deps.Registry, task)
	if err != nil {
		return failTerminal(deps.Tasks, task, nodeID, validateStep, composeErrTemplateUnavailable, err.Error())
	}
	inputPath, outcome := prepareInput(ctx, deps, task, nodeID, beginStep(model.ComposeStepPrepareInput), runDir, gate)
	if outcome != "" {
		return outcome
	}
	guidePath := ""
	if guide, ok := plan.(executor.GuidePlan); ok {
		fetchStep := beginStep(model.ComposeStepFetchGuide)
		guidePath = filepath.Join(runDir.Path(), "guide-input")
		fetch := deps.FetchGuide
		if fetch == nil {
			fetch = compose.DownloadGuide
		}
		if err := fetch(ctx, guide.GuideURL(), guidePath); err != nil {
			if cancelRequested(deps.Tasks, task.ID) {
				return ComposeAttemptCancelled
			}
			if errors.Is(ctx.Err(), context.Canceled) {
				return ComposeAttemptLeaseLost
			}
			if errors.Is(err, compose.ErrGuideTemporary) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return transientFailureInContext(ctx, deps, task, nodeID, fetchStep, composeErrStorageError, err.Error())
			}
			return failTerminal(deps.Tasks, task, nodeID, fetchStep, composeErrInputInvalid, err.Error())
		}
		recordStepSuccess(deps.Tasks, task, nodeID, fetchStep, fileMetrics(guidePath))
	}
	if cancelRequested(deps.Tasks, task.ID) {
		return ComposeAttemptCancelled
	}
	if !ownsProgress(deps.Tasks, task, nodeID, composeStageCutting, 40) {
		return ComposeAttemptLeaseLost
	}
	cutStep := beginStep(model.ComposeStepCut)
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
			return transientFailure(deps, task, nodeID, cutStep, composeErrTaskTimeout, err.Error())
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return ComposeAttemptLeaseLost
		}
		return failTerminal(deps.Tasks, task, nodeID, cutStep, composeErrExecutionFailed, err.Error())
	}
	recordStepSuccess(deps.Tasks, task, nodeID, cutStep, durationMetrics(output.DurationMS))
	if cancelRequested(deps.Tasks, task.ID) {
		return ComposeAttemptCancelled
	}
	if !ownsProgress(deps.Tasks, task, nodeID, composeStageUploading, 80) {
		return ComposeAttemptLeaseLost
	}
	attemptPath := strconv.FormatInt(task.ID, 10) + "/" + strconv.Itoa(task.AttemptCount) + "/output.mp4"
	tempKey := "compose-staging/" + attemptPath
	formalKey := "compose-outputs/" + attemptPath
	// A failed Put may have stored some bytes before returning an error.
	// Every exit from this point attempts cleanup; the recovery scan handles
	// any deletion that fails or is skipped by a crashed worker.
	defer removeComposeObject(ctx, deps.Store, tempKey)
	uploadStep := beginStep(model.ComposeStepUpload)
	size, digest, err := uploadVerified(ctx, deps.Store, output.Path, tempKey)
	if err != nil {
		if errors.Is(err, errComposeOutputInvalid) {
			return failTerminal(deps.Tasks, task, nodeID, uploadStep, composeErrExecutionFailed, err.Error())
		}
		return transientFailureInContext(ctx, deps, task, nodeID, uploadStep, composeErrStorageError, err.Error())
	}
	recordStepSuccess(deps.Tasks, task, nodeID, uploadStep, uploadMetrics(size, digest))
	if !ownsProgress(deps.Tasks, task, nodeID, composeStageCommitting, 90) {
		return ComposeAttemptLeaseLost
	}
	commitStep := beginStep(model.ComposeStepCommit)
	if err := deps.Store.Copy(ctx, tempKey, formalKey); err != nil {
		return transientFailureInContext(ctx, deps, task, nodeID, commitStep, composeErrStorageError, "copy to formal key: "+err.Error())
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
	// The commit is the one step that cannot be appended afterwards: it closes
	// the attempt row, so the append's own guard would refuse it. It rides
	// inside the transaction instead, where the record of the step and the
	// success it reports are one fact.
	closing := finishedStep(commitStep)
	closing.Outcome = model.ComposeStepOutcomeSuccess
	if _, err := deps.Tasks.CommitComposeOutput(out, formalKey, nodeID, task.AttemptCount, closing); err != nil {
		// ErrComposeCommitLost proves the transaction rolled back. Other
		// errors may be a lost response after commit; deleting the formal
		// object then could leave a success row pointing at missing bytes.
		if errors.Is(err, repository.ErrComposeCommitLost) {
			removeComposeObject(ctx, deps.Store, formalKey)
		}
		// The attempt is left for the recovery scan, so this entry is what
		// tells the operator the run stopped here with an unknown result.
		recordStepFailure(deps.Tasks, task, nodeID, commitStep, composeErrExecutionUncertain)
		return ComposeAttemptLeaseLost
	}
	return ComposeAttemptSuccess
}

func removeComposeObject(ctx context.Context, store storage.Store, key string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	_ = store.Remove(cleanupCtx, key)
}

// validateSnapshotTemplate checks the snapshot's binding against the executor
// registry before any bytes move: an unknown template version or a params
// rejection is a business failure with a clean task row, not a half-run.
func validateSnapshotTemplate(registry *executor.Registry, task model.ComposeTask) (executor.Plan, error) {
	definition, err := registry.Lookup(task.TemplateCode, task.TemplateVersion)
	if err != nil {
		return nil, fmt.Errorf("template %s v%d is not deployed to this worker: %w", task.TemplateCode, task.TemplateVersion, err)
	}
	plan, err := definition.Executor.ValidateParams(task.ConfigSnapshot)
	if err != nil {
		return nil, fmt.Errorf("snapshot params rejected by %s v%d: %w", task.TemplateCode, task.TemplateVersion, err)
	}
	return plan, nil
}

// prepareInput gates on readiness (PRD §5.4.4: an unready source keeps the
// task waiting, it never fabricates an input), and stages the prepared source
// into the run directory. A non-empty outcome short-circuits the attempt.
//
// `ready` is a projection, and this is the function that stops trusting it: the
// object is stat'ed before a byte is promised, and a row whose object is gone is
// repaired through one preparation rather than failed on the spot. The branch
// table it implements, with `gate` carrying what this claim has already done:
//
//	ready, key, object present      -> download and run
//	ready, key, object missing      -> first time: request + mark repreparing;
//	                                   after an observed request: input_missing
//	ready, no key                   -> request an ordinary preparation, or
//	                                   input_missing if there is no page to re-resolve
//	downloading                     -> keep the (single, deduped) preparation, wait
//	failed                          -> request one retry; an observed retry that
//	                                   failed again is input_invalid
//	not_downloaded                  -> request, then wait; observed-and-still-empty
//	                                   is input_invalid
//
// A material with no `source_url` never gets a preparation task: the worker
// would have nothing to resolve and the task could only fail. It fails directly,
// with the code that names what is missing.
func prepareInput(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, step attemptStep, runDir *compose.RunDir, gate *ComposeInputGate) (string, ComposeAttemptOutcome) {
	material, found, err := deps.Materials.FindMaterial(task.MaterialID)
	if err != nil {
		return "", transientFailureInContext(ctx, deps, task, nodeID, step, composeErrStorageError, "read material: "+err.Error())
	}
	if !found {
		return "", failTerminal(deps.Tasks, task, nodeID, step, composeErrInputMissing, "material row is gone")
	}
	if gate.PrepareRequested {
		if material.VideoStatus == model.VideoDownloading {
			// The projection says a preparation is in flight, which is the
			// evidence a waiting request was looking for.
			gate.PrepareObserved = true
		}
		if material.VideoStatus == model.VideoReady && material.SourceObjectKey != "" && material.SourceObjectKey != gate.RequestedKey {
			// A different content-addressed key means these are different bytes,
			// so a newer preparation has produced them.
			gate.PrepareObserved = true
		}
	}

	switch material.VideoStatus {
	case model.VideoReady:
		if material.SourceObjectKey == "" {
			return waitOrFailForKeyless(ctx, deps, task, nodeID, step, material, gate)
		}
		if _, statErr := deps.Store.Stat(ctx, material.SourceObjectKey); statErr != nil {
			if !errors.Is(statErr, storage.ErrObjectNotFound) {
				return "", transientFailureInContext(ctx, deps, task, nodeID, step, composeErrStorageError, "source object: "+statErr.Error())
			}
			if gate.PrepareRequested {
				if gate.PrepareObserved {
					return "", failTerminal(deps.Tasks, task, nodeID, step, composeErrInputMissing, "source object is still gone after re-preparing: "+statErr.Error())
				}
				return waitForInput(deps, task, nodeID)
			}
			if strings.TrimSpace(material.SourceURL) == "" {
				return "", failTerminal(deps.Tasks, task, nodeID, step, composeErrInputMissing, "source object is gone and the material has no page address to prepare it from: "+statErr.Error())
			}
			return requestInputPreparation(ctx, deps, task, nodeID, step, material, gate, material.SourceObjectKey)
		}
		grant, err := deps.Store.PresignGet(ctx, material.SourceObjectKey, deps.GrantTTL)
		if err != nil {
			return "", transientFailureInContext(ctx, deps, task, nodeID, step, composeErrStorageError, "presign source: "+err.Error())
		}
		inputPath := filepath.Join(runDir.Path(), "source-input")
		if err := downloadFile(ctx, deps.Client, grant.URL, inputPath); err != nil {
			return "", transientFailureInContext(ctx, deps, task, nodeID, step, composeErrStorageError, "download source: "+err.Error())
		}
		recordStepSuccess(deps.Tasks, task, nodeID, step, fileMetrics(inputPath))
		return inputPath, ""

	case model.VideoDownloading:
		// Someone's preparation is running. Asking again is a no-op by dedupe and
		// covers a projection that says `downloading` with the task row gone.
		gate.PrepareObserved = true
		if deps.EnsureInputPrepared != nil {
			observed, err := deps.EnsureInputPrepared(ctx, task, material)
			if err != nil {
				return "", transientFailureInContext(ctx, deps, task, nodeID, step, composeErrStorageError, "request input preparation: "+err.Error())
			}
			if observed {
				gate.PrepareObserved = true
			}
		}
		return waitForInput(deps, task, nodeID)

	case model.VideoFailed, model.VideoNotDownloaded:
		code := composeErrInputInvalid
		if gate.PrepareRequested {
			if gate.PrepareObserved {
				return "", failTerminal(deps.Tasks, task, nodeID, step, code, "source preparation did not produce a video for material "+strconv.FormatInt(task.MaterialID, 10))
			}
			return waitForInput(deps, task, nodeID)
		}
		if strings.TrimSpace(material.SourceURL) == "" {
			return "", failTerminal(deps.Tasks, task, nodeID, step, code, "source is not prepared and the material has no page address to prepare it from")
		}
		return requestInputPreparation(ctx, deps, task, nodeID, step, material, gate, "")

	default:
		return "", failTerminal(deps.Tasks, task, nodeID, step, composeErrInputInvalid, "source status "+string(material.VideoStatus)+" cannot be prepared")
	}
}

// waitOrFailForKeyless handles the `ready` row that names no object. It
// promises a video nobody can fetch, so it is treated as unprepared: the same
// preparation marker (broadened to take back exactly this shape) moves it
// along, and an observed request that still left it keyless is input_invalid —
// the row is malformed rather than merely stale.
func waitOrFailForKeyless(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, step attemptStep, material model.Material, gate *ComposeInputGate) (string, ComposeAttemptOutcome) {
	if gate.PrepareRequested {
		if gate.PrepareObserved {
			return "", failTerminal(deps.Tasks, task, nodeID, step, composeErrInputInvalid, "material is ready without an object key after re-preparing")
		}
		return waitForInput(deps, task, nodeID)
	}
	if strings.TrimSpace(material.SourceURL) == "" {
		return "", failTerminal(deps.Tasks, task, nodeID, step, composeErrInputMissing, "material is ready without an object key and has no page address to prepare it from")
	}
	return requestInputPreparation(ctx, deps, task, nodeID, step, material, gate, "")
}

// requestInputPreparation asks for the one preparation this claim may request
// and records it on the gate. `observed` is the transfer-side mark's own answer:
// true means the projection moved because of this request, which is stronger
// evidence than any later poll.
func requestInputPreparation(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, step attemptStep, material model.Material, gate *ComposeInputGate, requestedKey string) (string, ComposeAttemptOutcome) {
	if deps.EnsureInputPrepared != nil {
		observed, err := deps.EnsureInputPrepared(ctx, task, material)
		if err != nil {
			return "", transientFailureInContext(ctx, deps, task, nodeID, step, composeErrStorageError, "request input preparation: "+err.Error())
		}
		gate.PrepareObserved = observed
	}
	gate.PrepareRequested = true
	gate.RequestedKey = requestedKey
	return waitForInput(deps, task, nodeID)
}

func waitForInput(deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64) (string, ComposeAttemptOutcome) {
	_, _ = deps.Tasks.ReportComposeProgress(task.ID, nodeID, task.AttemptCount, composeStageWaitingInput, 20)
	return "", ComposeAttemptWaitInput
}

// uploadVerified writes the output to the temporary key and proves the store
// accepted those exact bytes before the caller commits anything formal.
//
// The proof is the upload's Content-MD5 (`storage.Put` sends it and the store
// verifies it): a successful upload already means "the stored bytes hash to the
// digest computed over the bytes read", with no second transfer. The `Stat` is
// the independent second reading — a store that accepted the checksum but
// recorded a different length is a storage fault, and a formal copy must not be
// built on top of it. There is deliberately no readback: it would move the
// whole output again to learn what the checksum proved at upload time.
func uploadVerified(ctx context.Context, store storage.Store, path, tempKey string) (int64, string, error) {
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
		return 0, "", fmt.Errorf("%w: compose output is empty", errComposeOutputInvalid)
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
		return 0, "", fmt.Errorf("%w: temporary object size mismatch: local %d, remote %d", errComposeOutputInvalid, info.Size(), remote.Size)
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

func ownsProgress(tasks composeTaskLifecycle, task model.ComposeTask, nodeID int64, stage string, progress int) bool {
	ok, _ := tasks.ReportComposeProgress(task.ID, nodeID, task.AttemptCount, stage, progress)
	return ok
}

func cancelRequested(tasks composeTaskLifecycle, taskID int64) bool {
	item, found, err := tasks.Find(taskID)
	return err == nil && found && item.CancelRequested
}

// finishedStep stamps a marker with the finish time and the elapsed duration —
// the part of a step every path shares.
func finishedStep(step attemptStep) model.ComposeTaskAttemptStep {
	finished := time.Now()
	return model.ComposeTaskAttemptStep{
		Step:       step.key,
		StartedAt:  step.started,
		FinishedAt: finished,
		DurationMS: finished.Sub(step.started).Milliseconds(),
	}
}

// recordStep appends one finished step to the attempt this worker still owns.
// The write is evidence, never a control signal: the result is ignored, and a
// lost race can only drop an entry, never duplicate or misattribute one. The
// server owns the label and the failure category, so only the phase's own
// facts are passed in.
func recordStep(tasks composeTaskLifecycle, task model.ComposeTask, nodeID int64, step attemptStep, outcome, code string, metrics model.ComposeAttemptStepMetrics) {
	if step.started.IsZero() {
		return
	}
	entry := finishedStep(step)
	entry.Outcome, entry.ErrorCode, entry.Metrics = outcome, code, metrics
	_, _ = tasks.RecordComposeAttemptStep(task.ID, nodeID, task.AttemptCount, entry)
}

func recordStepSuccess(tasks composeTaskLifecycle, task model.ComposeTask, nodeID int64, step attemptStep, metrics model.ComposeAttemptStepMetrics) {
	recordStep(tasks, task, nodeID, step, model.ComposeStepOutcomeSuccess, "", metrics)
}

func recordStepFailure(tasks composeTaskLifecycle, task model.ComposeTask, nodeID int64, step attemptStep, code string) {
	recordStep(tasks, task, nodeID, step, model.ComposeStepOutcomeFailed, code, model.ComposeAttemptStepMetrics{})
}

// failTerminal records the failing step, marks the task failed and reports the
// outcome; the task write is best-effort — a lost lease leaves the row to the
// recovery scan, which is the honest owner of that situation.
func failTerminal(tasks composeTaskLifecycle, task model.ComposeTask, nodeID int64, step attemptStep, code, message string) ComposeAttemptOutcome {
	recordStepFailure(tasks, task, nodeID, step, code)
	_, _ = tasks.FailComposeTask(task.ID, nodeID, task.AttemptCount, code, message)
	return ComposeAttemptFailed
}

// transientFailure requeues within the attempt budget, or terminal-fails with
// the exhaustion note once the budget is spent — a task must never end up
// pending forever with no attempts left. The failing step is recorded on
// whichever branch ends the attempt, and the requeue closes the attempt row,
// so it is written first: afterwards the append's own guard refuses it.
func transientFailure(deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, step attemptStep, code, message string) ComposeAttemptOutcome {
	if task.AttemptCount < task.MaxAttempts {
		recordStepFailure(deps.Tasks, task, nodeID, step, code)
		if ok, _ := deps.Tasks.ReleaseComposeForRetry(task.ID, nodeID, task.AttemptCount, code, message); ok {
			return ComposeAttemptRetry
		}
		return ComposeAttemptLeaseLost
	}
	return failTerminal(deps.Tasks, task, nodeID, step, code, message+fmt.Sprintf(" (attempts exhausted after %d)", task.AttemptCount))
}

func transientFailureInContext(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, step attemptStep, code, message string) ComposeAttemptOutcome {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ComposeAttemptLeaseLost
	}
	return transientFailure(deps, task, nodeID, step, code, message)
}
