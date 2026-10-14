package jobs

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

// CHG-20261013-083 (M4-C5 T-04): the worker loop over the lifecycle store. The
// fakes stand in for the store and for RunComposeAttempt, so each test can name
// exactly one scheduling behaviour.

type workerTasksFake struct {
	mu             sync.Mutex
	claims         []model.ComposeTask
	claimCalls     int
	renewLost      bool
	renewError     bool
	renewed        []int64
	confirmed      []int64
	released       []string
	requeued       []int64
	failed         []int64
	terminalFailed []int64
	terminalCodes  []string
	expired        []model.ComposeTask
	findRow        model.ComposeTask
	reapSeen       chan struct{}
}

func (f *workerTasksFake) ClaimNextComposeTask(int64, time.Duration) (model.ComposeTask, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCalls++
	if len(f.claims) == 0 {
		return model.ComposeTask{}, false, nil
	}
	item := f.claims[0]
	f.claims = f.claims[1:]
	return item, true, nil
}

func (f *workerTasksFake) RenewComposeLease(taskID, _ int64, _ int, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.renewed = append(f.renewed, taskID)
	if f.renewError {
		return false, errors.New("database unavailable")
	}
	return !f.renewLost, nil
}

func TestExpiredLeaseDoesNotStartAnUnconfirmedSecondExecution(t *testing.T) {
	oldNode := int64(9)
	fake := &workerTasksFake{expired: []model.ComposeTask{{ID: 77, ClaimedNodeID: &oldNode, AttemptCount: 1, MaxAttempts: 3}}}
	if err := recoverExpiredComposeLeases(fake, func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	if got := fake.requeuedIDs(); len(got) != 0 {
		t.Fatalf("unconfirmed old execution was requeued: %v", got)
	}
	if got := fake.failedIDs(); len(got) != 1 || got[0] != 77 {
		t.Fatalf("expired lease must be quarantined: %v", got)
	}
}

func TestHeartbeatDatabaseErrorStopsTheAttempt(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}, renewError: true}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptLeaseLost}, blocks: make(chan struct{})}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cancel, done := startWorker(t, ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}, workerSettings())
	deadline := time.Now().Add(300 * time.Millisecond)
	for spy.returnedCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stoppedBeforeShutdown := spy.returnedCount() == 1
	if err := stopWorker(cancel, done); err != nil {
		t.Fatal(err)
	}
	if !stoppedBeforeShutdown {
		t.Fatal("attempt kept running after lease renewal became unavailable")
	}
}

func (f *workerTasksFake) ConfirmComposeCancel(taskID, _ int64, _ int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.confirmed = append(f.confirmed, taskID)
	return true, nil
}

func (f *workerTasksFake) ReleaseComposeForRetry(taskID, _ int64, _ int, code, message string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, code+"|"+message)
	return true, nil
}

func (f *workerTasksFake) FailComposeTask(taskID, _ int64, _ int, code, message string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.terminalFailed = append(f.terminalFailed, taskID)
	f.terminalCodes = append(f.terminalCodes, code)
	return true, nil
}

func (f *workerTasksFake) Find(id int64) (model.ComposeTask, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.findRow
	row.ID = id
	return row, true, nil
}

func (f *workerTasksFake) FindExpiredComposeLeases() ([]model.ComposeTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]model.ComposeTask(nil), f.expired...), nil
}

func (f *workerTasksFake) FailExpiredComposeLease(taskID, _ int64, _ int, _, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, taskID)
	f.signalReap()
	return true, nil
}

func (f *workerTasksFake) signalReap() {
	if f.reapSeen != nil {
		select {
		case <-f.reapSeen:
		default:
			close(f.reapSeen)
		}
	}
}

func (f *workerTasksFake) claimCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claimCalls
}

func (f *workerTasksFake) confirmedIDs() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.confirmed...)
}

func (f *workerTasksFake) releasedCodes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.released...)
}

func (f *workerTasksFake) requeuedIDs() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.requeued...)
}

func (f *workerTasksFake) failedIDs() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.failed...)
}

func (f *workerTasksFake) renewedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.renewed)
}

func (f *workerTasksFake) terminalFailedIDs() ([]int64, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.terminalFailed...), append([]string(nil), f.terminalCodes...)
}

// attemptSpy records every Attempt call. outcomes repeats its last entry once
// the list is exhausted, so "always wait" is a one-element list.
type attemptSpy struct {
	mu       sync.Mutex
	calls    int
	returned int
	tasks    []int64
	paths    []string
	outcomes []productionservice.ComposeAttemptOutcome
	blocks   chan struct{}
}

func (s *attemptSpy) attempt(ctx context.Context, task model.ComposeTask, runDir *compose.RunDir) productionservice.ComposeAttemptOutcome {
	s.mu.Lock()
	call := s.calls
	s.calls++
	s.tasks = append(s.tasks, task.ID)
	s.paths = append(s.paths, runDir.Path())
	s.mu.Unlock()
	if s.blocks != nil && call == 0 {
		select {
		case <-ctx.Done():
		case <-s.blocks:
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.returned++
	outcome := productionservice.ComposeAttemptSuccess
	if len(s.outcomes) > 0 {
		if call < len(s.outcomes) {
			outcome = s.outcomes[call]
		} else {
			outcome = s.outcomes[len(s.outcomes)-1]
		}
	}
	return outcome
}

func (s *attemptSpy) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *attemptSpy) returnedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.returned
}

func (s *attemptSpy) taskIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int64(nil), s.tasks...)
}

func claimedTask(id int64) model.ComposeTask {
	node := int64(9)
	return model.ComposeTask{
		ID: id, TeamID: 7, GameID: "delta", TriggerType: "manual", MaterialID: 42, MaterialUsageID: 9,
		StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1,
		Status: model.ComposeTaskRunning, ClaimedNodeID: &node,
		AttemptCount: 1, MaxAttempts: 3, CreatedBy: 11,
	}
}

func workerSettings() ComposeWorkerSettings {
	return ComposeWorkerSettings{
		NodeID: 9, Concurrency: 1, Lease: 50 * time.Millisecond,
		HeartbeatInterval: 2 * time.Millisecond, TaskTimeout: 2 * time.Second,
		PollInterval: 2 * time.Millisecond, RecoveryInterval: time.Hour,
	}
}

func waitFor(t *testing.T, condition func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func startWorker(t *testing.T, deps ComposeWorkerDeps, settings ComposeWorkerSettings) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunComposeWorker(ctx, deps, settings)
	}()
	t.Cleanup(cancel)
	return cancel, done
}

func stopWorker(cancel context.CancelFunc, done <-chan error) error {
	cancel()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		return context.DeadlineExceeded
	}
}

func TestComposeWorkerRunsClaimedTaskToSuccess(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}}
	// The attempt holds the slot like a real FFmpeg run just long enough for a
	// heartbeat tick, then succeeds.
	spy := &attemptSpy{blocks: make(chan struct{})}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return fake.renewedCount() >= 1 }, "the lease to be renewed while the attempt runs")
	close(spy.blocks)
	waitFor(t, func() bool { return spy.returnedCount() == 1 && fake.claimCount() >= 2 }, "the claimed task to run once")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if ids := spy.taskIDs(); len(ids) != 1 || ids[0] != 77 {
		t.Fatalf("attempts=%v", ids)
	}
	if confirmed := fake.confirmedIDs(); len(confirmed) != 0 {
		t.Fatalf("a success must not confirm a cancel: %v", confirmed)
	}
	entries, err := os.ReadDir(workspace.Root())
	if err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("run directories left behind: %d", len(entries))
	}
}

func TestComposeWorkerConfirmsACancelItTerminated(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}, findRow: model.ComposeTask{CancelRequested: true}}
	// The first attempt blocks like an FFmpeg run until the cancel watcher
	// stops it, then reports the controlled stop.
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptCancelled}, blocks: make(chan struct{})}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return len(fake.confirmedIDs()) == 1 }, "the cancel to be confirmed")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if confirmed := fake.confirmedIDs(); len(confirmed) != 1 || confirmed[0] != 77 {
		t.Fatalf("confirmed=%v", confirmed)
	}
	if released := fake.releasedCodes(); len(released) != 0 {
		t.Fatalf("a cancelled task must not be requeued: %v", released)
	}
}

// A cancel request can already sit on the row when a claim starts — it rides
// through a transient release (the flag is never cleared). The attempt then
// returns Cancelled before the watcher has polled once, so the confirmation
// must come from the row: otherwise the row stays running with the flag set,
// unclaimable, until the lease expires.
func TestComposeWorkerConfirmsACancelFlagThatRodeTheClaim(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(83)}, findRow: model.ComposeTask{CancelRequested: true}}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptCancelled}}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	settings := workerSettings()
	settings.PollInterval = time.Hour
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, settings)

	waitFor(t, func() bool { return len(fake.confirmedIDs()) == 1 }, "the ridden cancel flag to be confirmed")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if released := fake.releasedCodes(); len(released) != 0 {
		t.Fatalf("a cancelled task must not be requeued: %v", released)
	}
}

func TestComposeWorkerStopsWithoutConfirmingWhenTheLeaseIsLost(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}, renewLost: true}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptLeaseLost}, blocks: make(chan struct{})}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return spy.returnedCount() == 1 && fake.renewedCount() > 0 }, "the failed renewal to stop the attempt")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if confirmed := fake.confirmedIDs(); len(confirmed) != 0 {
		t.Fatalf("a lost lease must not confirm a cancel: %v", confirmed)
	}
}

func TestComposeWorkerReattemptsAWaitingTaskWithoutReclaiming(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{
		productionservice.ComposeAttemptWaitInput,
		productionservice.ComposeAttemptWaitInput,
		productionservice.ComposeAttemptSuccess,
	}}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return spy.returnedCount() == 3 }, "the waiting task to be reattempted")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if got := fake.claimCount(); got != 1 {
		t.Fatalf("waiting reattempts must not consume claims: %d", got)
	}
	if confirmed := fake.confirmedIDs(); len(confirmed) != 0 {
		t.Fatalf("confirmed=%v", confirmed)
	}
}

func TestComposeWorkerConfirmsACancelThatLandsWhileWaiting(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}, findRow: model.ComposeTask{CancelRequested: true}}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptWaitInput}}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return len(fake.confirmedIDs()) == 1 }, "the waiting cancel to be confirmed")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if released := fake.releasedCodes(); len(released) != 0 {
		t.Fatalf("a cancelled task must not be requeued: %v", released)
	}
}

func TestComposeWorkerRequeuesAWaitingTaskKilledByTimeout(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77)}}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptWaitInput}}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	settings := workerSettings()
	settings.TaskTimeout = 40 * time.Millisecond
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, settings)

	waitFor(t, func() bool { return len(fake.releasedCodes()) == 1 }, "the timed-out wait to be released")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if released := fake.releasedCodes(); len(released) != 1 || released[0][:13] != "task_timeout|" {
		t.Fatalf("released=%v", released)
	}
	if confirmed := fake.confirmedIDs(); len(confirmed) != 0 {
		t.Fatalf("a timeout is not a cancel: %v", confirmed)
	}
}

// A wait that outlives the timeout with no attempts left must terminalize
// here: a requeue would leave a pending row no claim can ever take.
func TestComposeWorkerFailsAWaitingTaskWhenTheTimeoutExhaustsTheBudget(t *testing.T) {
	fake := &workerTasksFake{}
	exhausted := claimedTask(82)
	exhausted.AttemptCount = 3
	fake.claims = []model.ComposeTask{exhausted}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{productionservice.ComposeAttemptWaitInput}}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	settings := workerSettings()
	settings.TaskTimeout = 40 * time.Millisecond
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, settings)

	waitFor(t, func() bool { terminalFailed, _ := fake.terminalFailedIDs(); return len(terminalFailed) == 1 }, "the exhausted wait to terminalize")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	ids, codes := fake.terminalFailedIDs()
	if len(ids) != 1 || ids[0] != 82 || codes[0] != "task_timeout" {
		t.Fatalf("terminal=%v codes=%v", ids, codes)
	}
	if released := fake.releasedCodes(); len(released) != 0 {
		t.Fatalf("an exhausted task must not be requeued: %v", released)
	}
}

func TestComposeWorkerReclaimsARetryOutcome(t *testing.T) {
	fake := &workerTasksFake{claims: []model.ComposeTask{claimedTask(77), claimedTask(77)}}
	spy := &attemptSpy{outcomes: []productionservice.ComposeAttemptOutcome{
		productionservice.ComposeAttemptRetry,
		productionservice.ComposeAttemptSuccess,
	}}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: spy.attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return spy.returnedCount() == 2 }, "the requeued task to be reclaimed")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if got := fake.claimCount(); got < 2 {
		t.Fatalf("a retry outcome must go back through the claim: claims=%d", got)
	}
}

func TestComposeWorkerRecoversExpiredLeasesAtStartup(t *testing.T) {
	fake := &workerTasksFake{reapSeen: make(chan struct{})}
	withinBudget, exhausted := claimedTask(80), claimedTask(81)
	exhausted.AttemptCount = 3
	fake.expired = []model.ComposeTask{withinBudget, exhausted}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: (&attemptSpy{}).attempt}
	cancel, done := startWorker(t, deps, workerSettings())

	waitFor(t, func() bool { return len(fake.failedIDs()) == 2 }, "the recovery scan to quarantine both rows")
	if err := stopWorker(cancel, done); err != nil {
		t.Fatalf("RunComposeWorker: %v", err)
	}
	if requeued := fake.requeuedIDs(); len(requeued) != 0 {
		t.Fatalf("expired rows cannot be requeued without stop proof: %v", requeued)
	}
	if failed := fake.failedIDs(); len(failed) != 2 || failed[0] != 80 || failed[1] != 81 {
		t.Fatalf("failed=%v, want both rows quarantined", failed)
	}
}

func TestComposeWorkerRefusesInvalidSettings(t *testing.T) {
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	fake := &workerTasksFake{}
	settings := workerSettings()
	settings.Concurrency = 0
	deps := ComposeWorkerDeps{Tasks: fake, Workspace: workspace, Attempt: (&attemptSpy{}).attempt}
	if err := RunComposeWorker(context.Background(), deps, settings); err == nil {
		t.Fatal("RunComposeWorker() = nil error, want a refusal for an unbounded worker pool")
	}
	if fake.claimCount() != 0 {
		t.Fatalf("a refused worker must not claim: %d", fake.claimCount())
	}
}
