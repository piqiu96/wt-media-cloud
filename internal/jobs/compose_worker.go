package jobs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
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

// composeErrLeaseExpired is the error_code the recovery scan writes: the
// previous worker is gone, so whatever it was doing did not finish here.
const composeErrLeaseExpired = "lease_expired"

// composeErrTaskTimeoutCode is the code for a wait or run killed by the
// per-task timeout; the attempt itself classifies FFmpeg deaths, but a wait
// that outlives the timeout never reaches the attempt again.
const composeErrTaskTimeoutCode = "task_timeout"

// composeWorkerTasks is the lifecycle store surface the loop drives. One
// interface so a test answers the whole loop without a database.
type composeWorkerTasks interface {
	ClaimNextComposeTask(nodeID int64, lease time.Duration) (model.ComposeTask, bool, error)
	RenewComposeLease(taskID, nodeID int64, lease time.Duration) (bool, error)
	ConfirmComposeCancel(taskID, nodeID int64) (bool, error)
	ReleaseComposeForRetry(taskID, nodeID int64, errorCode, message string) (bool, error)
	Find(id int64) (model.ComposeTask, bool, error)
	FailComposeTask(taskID, nodeID int64, errorCode, message string) (bool, error)
	FindExpiredComposeLeases() ([]model.ComposeTask, error)
	RequeueExpiredComposeLease(taskID, previousNodeID int64, errorCode, message string) (bool, error)
	FailExpiredComposeLease(taskID, previousNodeID int64, errorCode, message string) (bool, error)
}

// ComposeWorkerSettings is the validated slice of config the loop runs on.
type ComposeWorkerSettings struct {
	NodeID            int64
	Concurrency       int
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
	Attempt   func(ctx context.Context, task model.ComposeTask, runDir *compose.RunDir) productionservice.ComposeAttemptOutcome
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
	if err := recoverExpiredComposeLeases(deps.Tasks, deps.Log); err != nil {
		return fmt.Errorf("compose worker startup recovery: %w", err)
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
	}
	return nil
}

// runComposeSlot claims one task at a time until the context ends. An idle
// slot waits PollInterval; a finished task pauses the slot the same interval
// before the next claim, which is also the backoff that keeps a task cycling
// through transient retries from spinning on the queue.
func runComposeSlot(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings) {
	for ctx.Err() == nil {
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

	var guards sync.WaitGroup
	cancelSeen := &composeFlag{}
	guards.Add(2)
	go func() {
		defer guards.Done()
		watchComposeHeartbeat(attemptCtx, deps, settings, task.ID, stopAttempt)
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
		switch outcome := deps.Attempt(attemptCtx, task, runDir); outcome {
		case productionservice.ComposeAttemptSuccess, productionservice.ComposeAttemptFailed, productionservice.ComposeAttemptLeaseLost:
			confirmIfCancelSeen(deps, settings, task, cancelSeen)
		case productionservice.ComposeAttemptCancelled:
			confirmIfCancelSeen(deps, settings, task, cancelSeen)
		case productionservice.ComposeAttemptRetry:
			// The row is pending again; the slot's pause before the next
			// claim is the retry backoff.
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
func watchComposeHeartbeat(ctx context.Context, deps ComposeWorkerDeps, settings ComposeWorkerSettings, taskID int64, stopAttempt context.CancelFunc) {
	ticker := time.NewTicker(settings.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		ok, err := deps.Tasks.RenewComposeLease(taskID, settings.NodeID, settings.Lease)
		if err != nil {
			deps.Log("compose worker lease renewal for task %d failed: %v", taskID, err)
			continue
		}
		if !ok {
			deps.Log("compose worker lost task %d to another owner; stopping the attempt", taskID)
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
		if _, err := deps.Tasks.FailComposeTask(task.ID, settings.NodeID, composeErrTaskTimeoutCode, message); err != nil {
			deps.Log("compose worker terminal failure of timed-out task %d failed: %v", task.ID, err)
		}
		return
	}
	if _, err := deps.Tasks.ReleaseComposeForRetry(task.ID, settings.NodeID, composeErrTaskTimeoutCode, message); err != nil {
		deps.Log("compose worker release of timed-out task %d failed: %v", task.ID, err)
	}
}

func confirmIfCancelSeen(deps ComposeWorkerDeps, settings ComposeWorkerSettings, task model.ComposeTask, cancelSeen *composeFlag) {
	if !cancelSeen.get() {
		return
	}
	if ok, err := deps.Tasks.ConfirmComposeCancel(task.ID, settings.NodeID); err != nil || !ok {
		deps.Log("compose worker cancel confirmation for task %d did not land (ok=%v err=%v)", task.ID, ok, err)
	}
}

// recoverExpiredComposeLeases reaps every lapsed running row: within budget it
// requeues, past budget it terminalizes. The compose.Lock guarantees a single
// worker per host, so a lapsed lease means the owning process is gone — there
// is no live FFmpeg the scan could be racing (spec §5.3), which is what makes
// an unconditional requeue honest here.
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
		if task.AttemptCount >= task.MaxAttempts {
			message := fmt.Sprintf("worker lease expired; attempts exhausted after %d", task.AttemptCount)
			if ok, err := tasks.FailExpiredComposeLease(task.ID, node, composeErrLeaseExpired, message); err != nil || !ok {
				log("compose worker could not fail expired task %d: ok=%v err=%v", task.ID, ok, err)
			}
			continue
		}
		if ok, err := tasks.RequeueExpiredComposeLease(task.ID, node, composeErrLeaseExpired, "worker lease expired; requeued for retry"); err != nil || !ok {
			log("compose worker could not requeue expired task %d: ok=%v err=%v", task.ID, ok, err)
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
