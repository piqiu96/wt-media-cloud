package jobs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

// The compose worker's loop (spec §4.3, §5). The service layer owns what one
// attempt means; this file owns when attempts happen: claiming, the lease
// heartbeat, the cancel-request watcher that terminates FFmpeg, the per-task
// timeout on the same termination path, and the recovery scan over lapsed
// leases. It never writes a compose row through anything but the lifecycle
// store, and it never classifies an attempt itself — it only acts on the
// outcome the service reports.

// An expired lease does not prove the old FFmpeg process has stopped. The
// task is held for explicit disposition rather than claimed a second time.
const composeErrExecutionUncertain = "execution_uncertain"

// composeErrTaskTimeoutCode is the code for a wait or run killed by the
// per-task timeout; the attempt itself classifies FFmpeg deaths, but a wait
// that outlives the timeout never reaches the attempt again.
const composeErrTaskTimeoutCode = "task_timeout"

// composeWorkerTasks is the lifecycle store surface the loop drives. One
// interface so a test answers the whole loop without a database.
type composeWorkerTasks interface {
	ClaimNextComposeTask(nodeID int64, lease time.Duration) (model.ComposeTask, bool, error)
	RenewComposeLease(taskID, nodeID int64, attempt int, lease time.Duration) (bool, error)
	ConfirmComposeCancel(taskID, nodeID int64, attempt int) (bool, error)
	ReleaseComposeForRetry(taskID, nodeID int64, attempt int, errorCode, message string) (bool, error)
	Find(id int64) (model.ComposeTask, bool, error)
	FailComposeTask(taskID, nodeID int64, attempt int, errorCode, message string) (bool, error)
	FindExpiredComposeLeases() ([]model.ComposeTask, error)
	FailExpiredComposeLease(taskID, previousNodeID int64, attempt int, errorCode, message string) (bool, error)
}

// ComposeWorkerSettings is the validated slice of config the loop runs on.
type ComposeWorkerSettings struct {
	NodeID            int64
	Concurrency       int
	WorkspaceRoot     string
	MinFreeBytes      int64
	Lease             time.Duration
	HeartbeatInterval time.Duration
	TaskTimeout       time.Duration
	PollInterval      time.Duration
	RecoveryInterval  time.Duration
}

// ComposeWorkerDeps carries the loop's collaborators. Attempt is the service's
// RunComposeAttempt over the real store and runner; the loop creates and
// releases each attempt's run directory so cleanup happens even when the
// attempt ends in any branch.
type ComposeWorkerDeps struct {
	Tasks     composeWorkerTasks
	Workspace *compose.Workspace
	Attempt   func(ctx context.Context, task model.ComposeTask, runDir *compose.RunDir, gate *productionservice.ComposeInputGate) productionservice.ComposeAttemptOutcome
	// CleanStaging retries precise per-attempt object deletion after a known
	// terminal outcome. It is optional for isolated loop tests.
	CleanStaging func(context.Context) error
	// DiskFree reports the free bytes on the volume holding dir. Real wiring
	// uses the host filesystem; tests answer with a fake.
	DiskFree func(dir string) (int64, error)
	// Log receives one line per notable worker event. Real wiring passes the
	// job logger; tests leave it nil and lose nothing they assert on.
	Log func(format string, args ...any)
}

// RunComposeWorker runs the loop until the context ends. The recovery scan
// runs first and then on its interval, so a restarted worker reaps what its
// predecessor left before claiming anything itself.
func RunComposeWorker(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings) error {
	if err := validateComposeSettings(settings); err != nil {
		return err
	}
	if deps.Log == nil {
		deps.Log = func(string, ...any) {}
	}
	if deps.DiskFree == nil {
		deps.DiskFree = freeDiskBytes
	}
	if err := recoverExpiredComposeLeases(deps.Tasks, deps.Log); err != nil {
		return fmt.Errorf("compose worker startup recovery: %w", err)
	}
	if deps.CleanStaging != nil {
		if err := deps.CleanStaging(ctx); err != nil {
			deps.Log("compose staging startup cleanup: %v", err)
		}
	}
	loopCtx, stop := context.WithCancel(ctx)
	defer stop()
	var recovery sync.WaitGroup
	recovery.Add(1)
	go func() {
		defer recovery.Done()
		ticker := time.NewTicker(settings.RecoveryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				if err := recoverExpiredComposeLeases(deps.Tasks, deps.Log); err != nil {
					deps.Log("compose worker recovery scan failed: %v", err)
				}
				if deps.CleanStaging != nil {
					if err := deps.CleanStaging(loopCtx); err != nil {
						deps.Log("compose staging cleanup failed: %v", err)
					}
				}
			}
		}
	}()
	var slots sync.WaitGroup
	for i := 0; i < settings.Concurrency; i++ {
		slots.Add(1)
		go func() {
			defer slots.Done()
			runComposeSlot(loopCtx, deps, settings)
		}()
	}
	slots.Wait()
	stop()
	recovery.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return nil
}

// validateComposeSettings repeats the config layer's refusals (spec §5.2) so
// the loop cannot be started misconfigured by a caller that bypassed Load.
func validateComposeSettings(settings ComposeWorkerSettings) error {
	switch {
	case settings.NodeID <= 0:
		return errors.New("compose worker node id must be a positive number")
	case settings.Concurrency <= 0:
		return errors.New("compose worker concurrency must be greater than zero")
	case settings.Lease <= 0:
		return errors.New("compose worker lease must be greater than zero")
	case settings.HeartbeatInterval <= 0 || settings.HeartbeatInterval >= settings.Lease:
		return errors.New("compose worker heartbeat interval must be positive and shorter than lease")
	case settings.TaskTimeout <= 0:
		return errors.New("compose worker task timeout must be greater than zero")
	case settings.PollInterval <= 0:
		return errors.New("compose worker poll interval must be greater than zero")
	case settings.RecoveryInterval <= 0:
		return errors.New("compose worker recovery interval must be greater than zero")
	case strings.TrimSpace(settings.WorkspaceRoot) == "":
		return errors.New("compose worker workspace root is required")
	case settings.MinFreeBytes <= 0:
		return errors.New("compose worker minimum free bytes must be greater than zero")
	}
	return nil
}

// freeDiskBytes reports the space available to unprivileged writes on the
// volume holding dir — the workspace volume the compose runs consume.
func freeDiskBytes(dir string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// runComposeSlot claims one task at a time until the context ends. An idle
// slot waits PollInterval; a finished task pauses the slot the same interval
// before the next claim. Retry eligibility itself is persisted as
// compose_tasks.next_attempt_at and checked by every claimant. Before claiming, the
// slot checks the workspace volume's free space: below the configured floor —
// or unreadable — it pauses claiming until capacity recovers, since a run it
// could not fit would only spend an attempt to die mid-cut.
func runComposeSlot(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings) {
	paused := false
	for ctx.Err() == nil {
		free, err := deps.DiskFree(settings.WorkspaceRoot)
		if err != nil || free < settings.MinFreeBytes {
			if !paused {
				paused = true
				if err != nil {
					deps.Log("compose worker cannot read free disk space for %s; pausing claims: %v", settings.WorkspaceRoot, err)
				} else {
					deps.Log("compose worker free disk space %d bytes is below the minimum %d; pausing claims", free, settings.MinFreeBytes)
				}
			}
			sleepCompose(ctx, settings.PollInterval)
			continue
		}
		if paused {
			paused = false
			deps.Log("compose worker free disk space recovered to %d bytes; resuming claims", free)
		}
		task, found, err := deps.Tasks.ClaimNextComposeTask(settings.NodeID, settings.Lease)
		if err != nil {
			deps.Log("compose worker claim failed: %v", err)
			sleepCompose(ctx, settings.PollInterval)
			continue
		}
		if !found {
			sleepCompose(ctx, settings.PollInterval)
			continue
		}
		runComposeTask(ctx, deps, settings, task)
		sleepCompose(ctx, settings.PollInterval)
	}
}

// runComposeTask drives one claimed task to a state the slot can walk away
// from. Waiting for input is the one state it stays with: the lease heartbeat
// keeps the row ours while the attempt re-checks the gate every PollInterval,
// and neither costs the task an attempt — attempts are spent at claims.
func runComposeTask(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings, task model.ComposeTask) {
	runDir, err := deps.Workspace.NewRun("task-" + strconv.FormatInt(task.ID, 10) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		deps.Log("compose worker run directory for task %d: %v", task.ID, err)
		return
	}
	defer func() {
		if err := runDir.Release(); err != nil {
			deps.Log("compose worker cleanup for task %d: %v", task.ID, err)
		}
	}()

	attemptCtx, stopAttempt := context.WithTimeout(ctx, settings.TaskTimeout)
	defer stopAttempt()

	// One gate per claim, reused by every waiting poll of this claim: the
	// self-heal budget is "once per claim", and a manual retry is a new claim.
	gate := &productionservice.ComposeInputGate{}

	var guards sync.WaitGroup
	cancelSeen := &composeFlag{}
	guards.Add(2)
	go func() {
		defer guards.Done()
		watchComposeHeartbeat(attemptCtx, deps, settings, task, stopAttempt)
	}()
	go func() {
		defer guards.Done()
		watchComposeCancel(attemptCtx, deps, settings, task.ID, cancelSeen, stopAttempt)
	}()

	for {
		if attemptCtx.Err() != nil {
			settleTerminatedWait(deps, settings, task, cancelSeen)
			break
		}
		switch outcome := deps.Attempt(attemptCtx, task, runDir, gate); outcome {
		case productionservice.ComposeAttemptSuccess, productionservice.ComposeAttemptFailed, productionservice.ComposeAttemptLeaseLost:
			confirmIfCancelSeen(deps, settings, task, cancelSeen)
		case productionservice.ComposeAttemptCancelled:
			// The flag can already be on the row when this attempt started —
			// it rode through a transient release and the watcher has not
			// polled yet. The row, not the watcher, is what says a stop is
			// still being asked for.
			if !cancelSeen.get() {
				if row, found, err := deps.Tasks.Find(task.ID); err == nil && found && row.CancelRequested {
					cancelSeen.set()
				}
			}
			confirmIfCancelSeen(deps, settings, task, cancelSeen)
		case productionservice.ComposeAttemptRetry:
			// The row is pending with a durable next_attempt_at; this slot
			// can keep polling without bypassing the shared retry deadline.
		case productionservice.ComposeAttemptWaitInput:
			sleepCompose(attemptCtx, settings.PollInterval)
			continue
		default:
			deps.Log("compose worker task %d returned unknown outcome %q", task.ID, outcome)
		}
		break
	}
	stopAttempt()
	guards.Wait()
}

// watchComposeHeartbeat renews the lease on its interval. A renewal refused
// (not errored — a database hiccup is retried on the next tick) means the row
// is no longer ours, so the attempt's context is cancelled and FFmpeg stops.
func watchComposeHeartbeat(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings, task model.ComposeTask, stopAttempt context.CancelFunc) {
	ticker := time.NewTicker(settings.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		ok, err := deps.Tasks.RenewComposeLease(task.ID, settings.NodeID, task.AttemptCount, settings.Lease)
		if err != nil {
			deps.Log("compose worker lease renewal for task %d failed; stopping the attempt: %v", task.ID, err)
			stopAttempt()
			return
		}
		if !ok {
			deps.Log("compose worker lost task %d to another owner; stopping the attempt", task.ID)
			stopAttempt()
			return
		}
	}
}

// watchComposeCancel polls the task row's cancel flag and terminates the
// attempt when the API has requested a stop — the controlled-termination
// half of the running-cancel protocol (spec §5.4).
func watchComposeCancel(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings, taskID int64, cancelSeen *composeFlag, stopAttempt context.CancelFunc) {
	ticker := time.NewTicker(settings.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		row, found, err := deps.Tasks.Find(taskID)
		if err != nil || !found {
			continue
		}
		if row.CancelRequested {
			cancelSeen.set()
			stopAttempt()
			return
		}
	}
}

// settleTerminatedWait is the exit for an attempt loop broken by the context
// rather than by an outcome: a cancel request is confirmed, anything else (the
// deadline) is a transient release — the task goes back to pending with its
// attempt accounting intact. With no attempts left the release would produce a
// pending row no claim can ever take, so that one terminalizes here.
func settleTerminatedWait(deps ComposeWorkerDeps, settings ComposeWorkerSettings, task model.ComposeTask, cancelSeen *composeFlag) {
	if cancelSeen.get() {
		confirmIfCancelSeen(deps, settings, task, cancelSeen)
		return
	}
	message := "input was not ready when the task timeout elapsed"
	if task.AttemptCount >= task.MaxAttempts {
		message += fmt.Sprintf(" (attempts exhausted after %d)", task.AttemptCount)
		if _, err := deps.Tasks.FailComposeTask(task.ID, settings.NodeID, task.AttemptCount, composeErrTaskTimeoutCode, message); err != nil {
			deps.Log("compose worker terminal failure of timed-out task %d failed: %v", task.ID, err)
		}
		return
	}
	if _, err := deps.Tasks.ReleaseComposeForRetry(task.ID, settings.NodeID, task.AttemptCount, composeErrTaskTimeoutCode, message); err != nil {
		deps.Log("compose worker release of timed-out task %d failed: %v", task.ID, err)
	}
}

func confirmIfCancelSeen(deps ComposeWorkerDeps, settings ComposeWorkerSettings, task model.ComposeTask, cancelSeen *composeFlag) {
	if !cancelSeen.get() {
		return
	}
	if ok, err := deps.Tasks.ConfirmComposeCancel(task.ID, settings.NodeID, task.AttemptCount); err != nil || !ok {
		deps.Log("compose worker cancel confirmation for task %d did not land (ok=%v err=%v)", task.ID, ok, err)
	}
}

// recoverExpiredComposeLeases quarantines lapsed running rows. A lease lapse
// does not prove the old execution stopped, especially across worker hosts.
func recoverExpiredComposeLeases(tasks composeWorkerTasks, log func(string, ...any)) error {
	expired, err := tasks.FindExpiredComposeLeases()
	if err != nil {
		return err
	}
	for _, task := range expired {
		node := int64(0)
		if task.ClaimedNodeID != nil {
			node = *task.ClaimedNodeID
		}
		if ok, err := tasks.FailExpiredComposeLease(task.ID, node, task.AttemptCount, composeErrExecutionUncertain, "worker lease expired; previous execution must be checked before retry"); err != nil || !ok {
			log("compose worker could not quarantine expired task %d: ok=%v err=%v", task.ID, ok, err)
		}
	}
	return nil
}

func sleepCompose(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// composeFlag is the cancel watcher's one-bit report to the attempt loop.
type composeFlag struct {
	mu    sync.Mutex
	value bool
}

func (f *composeFlag) set() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value = true
}

func (f *composeFlag) get() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.value
}
