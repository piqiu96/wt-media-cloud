package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
)

// Task is the row the fixtures build. It is aliased here rather than exported
// from the package: callers of this module deal in the wire body (`TaskBody`),
// and the row type is the repository's business.
type Task = model.Task

// memoryStore records what it was asked to do, because most of what these tests
// are about is *which* statement the service chose and with what arguments — the
// difference between scoping an update by the actor's team and by the row's is
// invisible in the returned value and decisive in the database.
//
// Every continuation operation defaults to refusing. A double that succeeded by
// default would let a test of "this is refused" pass without the service ever
// deciding anything; refusing by default means a test that expects success has to
// say so, and one that forgets fails loudly instead of passing vacuously.
type memoryStore struct {
	tasks map[string]Task

	createResult Task
	createErr    error

	// The user-download path is recorded separately from the generic create: the
	// point of the method is the input it *does not* take, and a double that
	// shared `lastCreate` would let a caller-supplied dedupe key go unnoticed.
	lastDownload repository.CreateUserDownloadInput
	lastPrepare  repository.CreateMaterialSourcePrepareInput

	candidate      Task
	candidateFound bool
	nextErr        error

	claimOK           bool
	heartbeatOK       bool
	progressOK        bool
	completeOK        bool
	failOK            bool
	cancelOK          bool
	retryOK           bool
	retryResult       Task
	failDependentsErr error

	listFilter   repository.TaskFilter
	lastStatusID identityservice.UserID
	statusResult map[int64]string
	lastCreate   repository.CreateTaskInput
	lastProg   repository.ProgressInput
	lastDone   repository.CompletionInput
	lastFail   repository.FailureInput
	lastCancel struct {
		taskID string
		team   identityservice.TeamID
		user   identityservice.UserID
	}
	lastRetry struct {
		taskID string
		team   identityservice.TeamID
		user   identityservice.UserID
	}
	lastDependents struct {
		prepareID string
		code      string
		message   string
	}
	lastClaim struct {
		taskID string
		nodeID string
		lease  time.Duration
	}
	counts map[string]int
}

func newMemoryStore(tasks ...Task) *memoryStore {
	store := &memoryStore{tasks: make(map[string]Task, len(tasks)), counts: make(map[string]int)}
	for _, task := range tasks {
		store.tasks[task.ID] = task
	}
	return store
}

func (s *memoryStore) count(name string) int { return s.counts[name] }

func (s *memoryStore) CreateTask(input repository.CreateTaskInput, now time.Time) (Task, error) {
	s.counts["create"]++
	s.lastCreate = input
	if s.createErr != nil {
		return Task{}, s.createErr
	}
	if s.createResult.ID != "" {
		return s.createResult, nil
	}
	return Task{
		ID: input.ID, TeamID: input.TeamID, AssetType: input.AssetType, AssetID: input.AssetID,
		AssetTitle: input.AssetTitle, SourceObjectKey: input.SourceObjectKey, Purpose: input.Purpose,
		ExecutionScope: input.ExecutionScope, Status: model.StatusPending, RequestedBy: input.RequestedBy,
		AssignedNodeID: input.AssignedNodeID, TotalBytes: input.TotalBytes, ExpectedSHA256: input.ExpectedSHA256,
		MaxAttempts: input.MaxAttempts, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *memoryStore) CreateUserDownloadTask(input repository.CreateUserDownloadInput, now time.Time) (Task, error) {
	s.counts["createDownload"]++
	s.lastDownload = input
	if s.createErr != nil {
		return Task{}, s.createErr
	}
	if s.createResult.ID != "" {
		return s.createResult, nil
	}
	return Task{
		ID: input.ID, TeamID: input.TeamID, AssetType: model.AssetMaterial, AssetID: input.AssetID,
		AssetTitle: input.AssetTitle, SourceObjectKey: input.SourceObjectKey,
		Purpose: model.PurposeUserDownload, ExecutionScope: model.ExecutionLocalAgent,
		Status: model.StatusPending, RequestedBy: input.RequestedBy, AssignedNodeID: input.AssignedNodeID,
		TotalBytes: input.TotalBytes, ExpectedSHA256: input.ExpectedSHA256,
		MaxAttempts: input.MaxAttempts, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *memoryStore) CreateMaterialSourcePrepareTask(input repository.CreateMaterialSourcePrepareInput, now time.Time) (Task, error) {
	s.counts["createPrepare"]++
	s.lastPrepare = input
	if s.createErr != nil {
		return Task{}, s.createErr
	}
	if s.createResult.ID != "" {
		return s.createResult, nil
	}
	return Task{
		ID: input.ID, TeamID: input.TeamID, AssetType: model.AssetMaterial, AssetID: input.AssetID,
		AssetTitle: input.AssetTitle, Purpose: model.PurposeComposeInputPrepare,
		ExecutionScope: model.ExecutionCloud, Status: model.StatusPending, RequestedBy: input.RequestedBy,
		MaxAttempts: input.MaxAttempts, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *memoryStore) GetTask(taskID string) (Task, error) {
	s.counts["get"]++
	task, ok := s.tasks[taskID]
	if !ok {
		return Task{}, repository.ErrNotFound
	}
	return task, nil
}

func (s *memoryStore) ListTasks(filter repository.TaskFilter) ([]Task, error) {
	s.counts["list"]++
	s.listFilter = filter
	ordered := make([]Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		ordered = append(ordered, task)
	}
	return ordered, nil
}

func (s *memoryStore) LatestUserDownloadStatuses(userID identityservice.UserID, _ []int64) (map[int64]string, error) {
	s.counts["status"]++
	s.lastStatusID = userID
	return s.statusResult, nil
}

func (s *memoryStore) NextLocalTask(_ string, _ time.Time) (Task, bool, error) {
	s.counts["next"]++
	return s.candidate, s.candidateFound, s.nextErr
}

func (s *memoryStore) ClaimLocalTask(taskID, nodeID string, _ time.Time, lease time.Duration) (Task, bool, error) {
	s.counts["claim"]++
	s.lastClaim.taskID, s.lastClaim.nodeID, s.lastClaim.lease = taskID, nodeID, lease
	if !s.claimOK {
		return Task{}, false, nil
	}
	task := s.tasks[taskID]
	task.Status = model.StatusRunning
	task.ClaimedByNodeID = nodeID
	task.AttemptCount++
	return task, true, nil
}

func (s *memoryStore) HeartbeatTask(string, string, time.Time, time.Duration) (bool, error) {
	s.counts["heartbeat"]++
	return s.heartbeatOK, nil
}

func (s *memoryStore) ReportProgress(input repository.ProgressInput, _ time.Time) (bool, error) {
	s.counts["progress"]++
	s.lastProg = input
	return s.progressOK, nil
}

// The two terminal doubles apply their state change, because the service answers
// with the row *after* the write: a fake that only recorded the call would make
// "the body echoes what Cloud recorded" untestable, and would let a service that
// echoed the request instead of the row pass.
func (s *memoryStore) CompleteTask(input repository.CompletionInput, _ time.Time) (bool, error) {
	s.counts["complete"]++
	s.lastDone = input
	if !s.completeOK {
		return false, nil
	}
	task := s.tasks[input.TaskID]
	task.Status = model.StatusSuccess
	task.TransferredBytes = input.Bytes
	task.IntegritySHA256 = input.SHA256
	if input.FileName != "" {
		task.FileName = input.FileName
	}
	s.tasks[input.TaskID] = task
	return true, nil
}

func (s *memoryStore) FailTask(input repository.FailureInput, _ time.Time) (bool, error) {
	s.counts["fail"]++
	s.lastFail = input
	if !s.failOK {
		return false, nil
	}
	task := s.tasks[input.TaskID]
	task.Status = input.Status
	task.ErrorCode = input.ErrorCode
	task.ErrorMessage = input.ErrorMessage
	s.tasks[input.TaskID] = task
	return true, nil
}

func (s *memoryStore) CancelTask(taskID string, teamID identityservice.TeamID, requestedBy identityservice.UserID, _ time.Time) (bool, error) {
	s.counts["cancel"]++
	s.lastCancel.taskID, s.lastCancel.team, s.lastCancel.user = taskID, teamID, requestedBy
	return s.cancelOK, nil
}

// FailDependents records what it was asked to release. It returns no error unless
// a test supplies one: unlike the operations above, its result is a row count the
// service does not branch on, so the interesting readings are the arguments —
// which preparation, and under what code.
func (s *memoryStore) FailDependents(prepareTaskID, errorCode, errorMessage string, _ time.Time) (int64, error) {
	s.counts["failDependents"]++
	s.lastDependents.prepareID, s.lastDependents.code, s.lastDependents.message = prepareTaskID, errorCode, errorMessage
	if s.failDependentsErr != nil {
		return 0, s.failDependentsErr
	}
	return 0, nil
}

func (s *memoryStore) RetryTask(taskID string, teamID identityservice.TeamID, requestedBy identityservice.UserID, _ time.Time) (Task, bool, error) {
	s.counts["retry"]++
	s.lastRetry.taskID, s.lastRetry.team, s.lastRetry.user = taskID, teamID, requestedBy
	if !s.retryOK {
		return Task{}, false, nil
	}
	return s.retryResult, true, nil
}

type stubNodes struct {
	node           runtimeservice.AgentNode
	err            error
	calls          int
	lastCredential string
}

func (s *stubNodes) AuthenticateNodeCredential(credential string) (runtimeservice.AgentNode, error) {
	s.calls++
	s.lastCredential = credential
	return s.node, s.err
}

type stubGrants struct {
	grant DownloadGrant
	err   error
	calls int
	keys  []string
}

func (s *stubGrants) PresignGet(_ context.Context, objectKey string) (DownloadGrant, error) {
	s.calls++
	s.keys = append(s.keys, objectKey)
	return s.grant, s.err
}

const nodeID = "node-1"

func workingNode() NodeAuthenticator {
	return &stubNodes{node: runtimeservice.AgentNode{ID: nodeID, UserID: identityservice.UserID(9)}}
}

func testService(store *memoryStore, nodes NodeAuthenticator, options ...Option) *Service {
	return NewService(store, nodes, options...)
}

func fixedClock(at time.Time) Option { return WithClock(func() time.Time { return at }) }

func testNow() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) }

func teamOf(id int64) identityservice.TeamID { return identityservice.TeamID(id) }

func actorWith(id int64, team identityservice.TeamID) identityservice.PublicUser {
	return identityservice.PublicUser{
		ID:     identityservice.UserID(id),
		Status: identityservice.UserStatusEnabled,
		TeamID: &team,
	}
}

func testSHA(seed string) string { return strings.Repeat(seed, 64/len(seed)+1)[:64] }

func taskFixture(id string, status model.Status, user identityservice.UserID, team identityservice.TeamID) Task {
	now := testNow()
	// Only a running task is held by a node: a pending one has no claimant, and a
	// fixture that claimed one for it would make the wrong-node refusal untestable.
	claimedBy := ""
	if status == model.StatusRunning {
		claimedBy = nodeID
	}
	return Task{
		ID: id, TeamID: team, AssetType: model.AssetMaterial, AssetID: 42, AssetTitle: "演示素材",
		SourceObjectKey: "materials/42/deadbeef.mp4", Purpose: model.PurposeUserDownload,
		ExecutionScope: model.ExecutionLocalAgent, Status: status, RequestedBy: user,
		AssignedNodeID: nodeID, ClaimedByNodeID: claimedBy, TotalBytes: 1000, ExpectedSHA256: strings.Repeat("a", 64),
		MaxAttempts: 3, CreatedAt: now, UpdatedAt: now,
	}
}

// prepareFixture is a material's source preparation rather than a download: the
// same row shape with the purpose and scope the worker claims it under, and no
// assignee, because a Cloud task is leased by any worker rather than bound to the
// node a download was queued for.
func prepareFixture(id string, status model.Status, user identityservice.UserID, team identityservice.TeamID) Task {
	task := taskFixture(id, status, user, team)
	task.Purpose = model.PurposeComposeInputPrepare
	task.ExecutionScope = model.ExecutionCloud
	task.AssignedNodeID = ""
	return task
}

// ---- the session surface ----

func TestListTasksAsksTheRepositoryForTheActingUser(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusPending, 9, teamOf(7)))
	service := testService(store, workingNode(), fixedClock(testNow()))

	items, err := service.ListTasks(actorWith(9, teamOf(7)))
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if store.listFilter.RequestedBy == nil || *store.listFilter.RequestedBy != identityservice.UserID(9) {
		t.Fatalf("filter = %+v, want the acting user", store.listFilter)
	}
	if store.listFilter.Limit <= 0 {
		t.Fatalf("filter limit = %d, want the module's own bound", store.listFilter.Limit)
	}
	if len(items) != 1 || items[0].ID != "transfer-1" {
		t.Fatalf("items = %+v", items)
	}
}

// The download centre names a row with the asset's title. It is the only name a task
// has until an executor reports `file_name`, and it is written onto the row at
// creation rather than looked up on read, so a later rename cannot change what the
// operator was told they were downloading.
func TestListTasksCarriesTheAssetTitleTheTaskWasCreatedWith(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusPending, 9, teamOf(7)))
	service := testService(store, workingNode(), fixedClock(testNow()))

	items, err := service.ListTasks(actorWith(9, teamOf(7)))
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if items[0].AssetTitle != "演示素材" {
		t.Fatalf("asset_title = %q, want the title the task was created with", items[0].AssetTitle)
	}
}

// An empty optional field is null on the wire, not an empty string: the frozen
// schema makes all four nullable, and `"file_name": ""` would claim the transfer
// produced a file whose name is empty.
func TestListTasksNullsTheOptionalFieldsThatHaveNoValueYet(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	service := testService(store, workingNode(), fixedClock(testNow()))

	items, err := service.ListTasks(actorWith(9, teamOf(7)))
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	body := items[0]
	if body.ChecksumSHA256 != nil || body.FileName != nil || body.ErrorCode != nil || body.ErrorMessage != nil {
		t.Fatalf("body = %+v, want every unset optional to be nil", body)
	}
	if body.EstimatedRemainingSeconds != nil {
		t.Fatalf("eta = %v, want nil for a task that has not reported one", *body.EstimatedRemainingSeconds)
	}
	if body.TotalBytes != 1000 || body.CompletedBytes != 0 {
		t.Fatalf("body = %+v", body)
	}
}

func TestListTasksRefusesAnActorWithNoIdentity(t *testing.T) {
	store := newMemoryStore()
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.ListTasks(identityservice.PublicUser{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if store.count("list") != 0 {
		t.Fatalf("list called %d times, want none for a request with no actor", store.count("list"))
	}
}

// The derived download lifecycle is a pass-through to the store, scoped to the
// given user: the module does not decide what the raw statuses mean (the
// production module maps them), it only refuses an identity-less caller.
func TestLatestUserDownloadStatusesPassesTheUserThroughAndRefusesNobody(t *testing.T) {
	store := newMemoryStore()
	store.statusResult = map[int64]string{42: "success"}
	service := testService(store, workingNode(), fixedClock(testNow()))

	got, err := service.LatestUserDownloadStatuses(identityservice.UserID(9), []int64{42})
	if err != nil {
		t.Fatalf("LatestUserDownloadStatuses() error = %v", err)
	}
	if got[42] != "success" || store.lastStatusID != identityservice.UserID(9) {
		t.Fatalf("got = %v (asked as %v), want the store's raw answer for user 9", got, store.lastStatusID)
	}

	if _, err := service.LatestUserDownloadStatuses(0, []int64{42}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("LatestUserDownloadStatuses(0) error = %v, want ErrInvalidInput", err)
	}
}

// The two refusals a wrong actor gets are different, and the difference is the
// tenant boundary: another user in the same team is named as forbidden, while a
// task in another team is reported as absent so that 403-versus-404 cannot be
// used to probe whether a task id exists.
func TestCancelTaskKeepsAnotherUsersTaskApartFromAnotherTeams(t *testing.T) {
	store := newMemoryStore(
		taskFixture("mine", model.StatusPending, 9, teamOf(7)),
		taskFixture("colleague", model.StatusPending, 11, teamOf(7)),
		taskFixture("other-team", model.StatusPending, 12, teamOf(8)),
	)
	store.cancelOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.CancelTask(actorWith(9, teamOf(7)), "colleague"); !errors.Is(err, ErrTaskForbidden) {
		t.Fatalf("colleague error = %v, want forbidden", err)
	}
	if _, err := service.CancelTask(actorWith(9, teamOf(7)), "other-team"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("other team error = %v, want not found", err)
	}
	if _, err := service.CancelTask(actorWith(9, teamOf(8)), "mine"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("actor whose team changed error = %v, want not found", err)
	}
	if store.count("cancel") != 0 {
		t.Fatalf("cancel called %d times, want none for a refused actor", store.count("cancel"))
	}
}

// The answer is the refreshed row, not the request. A running transfer is not
// terminal when the user asks it to stop — the executor confirms that — so the
// body a client renders must be re-read rather than assembled from the intent.
func TestCancelTaskAnswersWithTheRefreshedTaskNotTheRequest(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.cancelOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	body, err := service.CancelTask(actorWith(9, teamOf(7)), "transfer-1")
	if err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
	if body.Status != string(model.StatusRunning) {
		t.Fatalf("status = %q, want the row's own value", body.Status)
	}
	// Two reads: one to decide whether this actor may act, one to answer with the
	// row that now exists. The second is what makes the body honest, so it is
	// asserted rather than assumed.
	if store.count("get") != 2 {
		t.Fatalf("get called %d times, want the row read to scope and re-read to answer", store.count("get"))
	}
	if store.lastCancel.team != teamOf(7) || store.lastCancel.user != 9 {
		t.Fatalf("cancel scoped by %+v, want the acting user", store.lastCancel)
	}
}

func TestCancelTaskRefusesATaskThatIsAlreadyTerminal(t *testing.T) {
	for _, status := range []model.Status{model.StatusSuccess, model.StatusFailed, model.StatusCancelled} {
		store := newMemoryStore(taskFixture("transfer-1", status, 9, teamOf(7)))
		store.cancelOK = true
		service := testService(store, workingNode(), fixedClock(testNow()))

		if _, err := service.CancelTask(actorWith(9, teamOf(7)), "transfer-1"); !errors.Is(err, ErrTaskConflict) {
			t.Fatalf("%s: error = %v, want conflict", status, err)
		}
		if store.count("cancel") != 0 {
			t.Fatalf("%s: cancel called %d times, want none", status, store.count("cancel"))
		}
	}
}

// Cancelling a preparation has to end the downloads waiting on it, because
// nothing else will.
//
// A waiter points at the preparation through `dependency_task_id` and is not
// leasable while it does, so it is satisfied by a hand-over and ended by a
// failure — and a cancelled preparation does neither. It is terminal, so no
// hand-over is coming; the cancellation write stops at the preparation's own row,
// so no failure is recorded either. Every waiter would sit `pending` for good,
// pointing at a task that is over: not leasable, so no node picks it up, and not
// terminal, so the download centre offers it no action at all. The reconciliation
// sweep is the safety net written for exactly this, and it is a job this build has
// not registered.
func TestCancellingAPreparationReleasesTheDownloadsWaitingOnIt(t *testing.T) {
	store := newMemoryStore(prepareFixture("prepare-1", model.StatusPending, 9, teamOf(7)))
	store.cancelOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.CancelTask(actorWith(9, teamOf(7)), "prepare-1"); err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
	if store.count("failDependents") != 1 {
		t.Fatalf("failDependents called %d times, want the waiters released", store.count("failDependents"))
	}
	if store.lastDependents.prepareID != "prepare-1" {
		t.Fatalf("released the dependents of %q, want the cancelled preparation", store.lastDependents.prepareID)
	}
	// The waiters inherit the cancellation's own code, so the two rows read the
	// same cause; a generic code here would put "failed" on a download the user
	// themselves stopped.
	if store.lastDependents.code != "cancelled_by_user" {
		t.Fatalf("code = %q, want the code the cancelled row carries", store.lastDependents.code)
	}
}

// The release is scoped to preparations, and a download has no waiters.
//
// This is what keeps the call above from becoming a statement that means "the
// task I just cancelled". Cancelling a download is the ordinary case, and a
// blanket release would be the sort of write that matches no rows today and
// something else tomorrow.
func TestCancellingADownloadLeavesDependentsAlone(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusPending, 9, teamOf(7)))
	store.cancelOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.CancelTask(actorWith(9, teamOf(7)), "transfer-1"); err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
	if store.count("failDependents") != 0 {
		t.Fatalf("failDependents called %d times, want none for a download", store.count("failDependents"))
	}
}

// The two writes are separate statements and only the first one is the request.
// A release that does not land leaves the waiters exactly as stuck as not calling
// it at all, so it is reported rather than swallowed — the same choice
// `CreateDownload` makes when the projection write fails after the task exists.
func TestCancellingAPreparationReportsAFailedWaiterRelease(t *testing.T) {
	store := newMemoryStore(prepareFixture("prepare-1", model.StatusPending, 9, teamOf(7)))
	store.cancelOK = true
	store.failDependentsErr = errors.New("the dependent release did not land")
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.CancelTask(actorWith(9, teamOf(7)), "prepare-1"); !errors.Is(err, store.failDependentsErr) {
		t.Fatalf("CancelTask() error = %v, want the release failure", err)
	}
}

// The update carries the actor's team, not the row's. Handing the statement the
// value it just read would make its `team_id = ?` a restatement of the row
// rather than a constraint, and the statement's own scope would stop being one.
func TestRetryTaskScopesTheUpdateToTheActorsTeam(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusFailed, 9, teamOf(7)))
	store.retryOK = true
	store.retryResult = taskFixture("transfer-1", model.StatusPending, 9, teamOf(7))
	service := testService(store, workingNode(), fixedClock(testNow()))

	body, err := service.RetryTask(actorWith(9, teamOf(7)), "transfer-1")
	if err != nil {
		t.Fatalf("RetryTask() error = %v", err)
	}
	if store.lastRetry.team != teamOf(7) || store.lastRetry.user != 9 {
		t.Fatalf("retry scoped by %+v, want the acting user", store.lastRetry)
	}
	if body.Status != string(model.StatusPending) {
		t.Fatalf("status = %q, want the requeued body", body.Status)
	}
	if store.count("get") != 1 {
		t.Fatalf("get called %d times, want one read and no more", store.count("get"))
	}
}

// A failed download whose preparation never delivered cannot be retried, and
// saying so is the whole point of the guard.
//
// Requeueing it would write a row that is `pending` with its `dependency_task_id`
// still set — the state it was just in. `leaseable` refuses a local task whose
// dependency is set, and the dependency is terminal, so nothing will ever hand it
// facts or fail it again: the retry would land on the database and change nothing
// the user could observe, which is what "I pressed retry and nothing happened"
// was. The remedy for these rows is a new download, which builds a new
// preparation, so the answer here is a conflict rather than a silent no-op.
func TestRetryTaskRefusesADownloadWhosePreparationNeverDelivered(t *testing.T) {
	task := taskFixture("transfer-1", model.StatusFailed, 9, teamOf(7))
	task.DependencyTaskID = "prepare-1"
	store := newMemoryStore(task)
	store.retryOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.RetryTask(actorWith(9, teamOf(7)), "transfer-1"); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("RetryTask() error = %v, want conflict", err)
	}
	if store.count("retry") != 0 {
		t.Fatalf("retry called %d times, want none while the dependency pointer is set", store.count("retry"))
	}
}

// The other half of the same guard: once the preparation has handed the facts
// over, the pointer is gone and the retry is the ordinary one — the download that
// failed on its own account, after it had something to download.
func TestRetryTaskStillRequeuesADownloadWhoseFactsArrived(t *testing.T) {
	task := taskFixture("transfer-1", model.StatusFailed, 9, teamOf(7))
	store := newMemoryStore(task)
	store.retryOK = true
	store.retryResult = taskFixture("transfer-1", model.StatusPending, 9, teamOf(7))
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.RetryTask(actorWith(9, teamOf(7)), "transfer-1"); err != nil {
		t.Fatalf("RetryTask() error = %v", err)
	}
	if store.count("retry") != 1 {
		t.Fatalf("retry called %d times, want the requeue", store.count("retry"))
	}
}

func TestRetryTaskRequiresAFailedTaskWithinItsBound(t *testing.T) {
	exhausted := taskFixture("exhausted", model.StatusFailed, 9, teamOf(7))
	exhausted.AttemptCount = exhausted.MaxAttempts

	for _, testCase := range []struct {
		name   string
		task   Task
		refuse bool
	}{
		{name: "failed with an attempt left", task: taskFixture("fresh", model.StatusFailed, 9, teamOf(7))},
		{name: "already succeeded", task: taskFixture("done", model.StatusSuccess, 9, teamOf(7)), refuse: true},
		{name: "still running", task: taskFixture("running", model.StatusRunning, 9, teamOf(7)), refuse: true},
		{name: "every attempt used", task: exhausted, refuse: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newMemoryStore(testCase.task)
			store.retryOK = true
			store.retryResult = testCase.task
			service := testService(store, workingNode(), fixedClock(testNow()))

			_, err := service.RetryTask(actorWith(9, teamOf(7)), testCase.task.ID)
			if testCase.refuse {
				if !errors.Is(err, ErrTaskConflict) {
					t.Fatalf("error = %v, want conflict", err)
				}
				if store.count("retry") != 0 {
					t.Fatalf("retry called %d times, want none", store.count("retry"))
				}
				return
			}
			if err != nil {
				t.Fatalf("RetryTask() error = %v", err)
			}
			if store.count("retry") != 1 {
				t.Fatalf("retry called %d times, want one", store.count("retry"))
			}
		})
	}
}

// ---- the executor surface ----

// The order is the point: nothing is leased until a grant exists. A lease
// without one would leave the row `running` with no executor holding anything
// and an attempt already spent, so this asserts the claim statement was never
// reached — not merely that an error came back.
func TestClaimTaskLeasesNothingWhenTheGrantCannotBeMinted(t *testing.T) {
	store := newMemoryStore()
	store.candidate = taskFixture("transfer-1", model.StatusPending, 9, teamOf(7))
	store.candidateFound = true
	store.claimOK = true
	grants := &stubGrants{err: ErrGrantUnavailable}
	service := testService(store, workingNode(), fixedClock(testNow()), WithGrantIssuer(grants))

	if _, err := service.ClaimTask(context.Background(), "secret"); !errors.Is(err, ErrGrantUnavailable) {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if store.count("claim") != 0 {
		t.Fatalf("claim called %d times, want none when there is no grant", store.count("claim"))
	}
	if grants.calls != 1 || grants.keys[0] != "materials/42/deadbeef.mp4" {
		t.Fatalf("grant minted for %+v, want the task's own object", grants.keys)
	}
}

// A grant whose url is empty is not a grant. Accepting it would lease a task the
// executor cannot download and cannot diagnose.
func TestClaimTaskRefusesAGrantWithoutAURL(t *testing.T) {
	store := newMemoryStore()
	store.candidate = taskFixture("transfer-1", model.StatusPending, 9, teamOf(7))
	store.candidateFound = true
	store.claimOK = true
	service := testService(store, workingNode(), fixedClock(testNow()), WithGrantIssuer(&stubGrants{}))

	if _, err := service.ClaimTask(context.Background(), "secret"); !errors.Is(err, ErrGrantUnavailable) {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if store.count("claim") != 0 {
		t.Fatalf("claim called %d times, want none", store.count("claim"))
	}
}

func TestClaimTaskAnswersAnExplicitNullWhenThereIsNothingToDo(t *testing.T) {
	store := newMemoryStore()
	store.candidateFound = false
	grants := &stubGrants{grant: DownloadGrant{URL: "https://example.invalid/x", ExpiresAt: testNow().Add(time.Minute)}}
	service := testService(store, workingNode(), fixedClock(testNow()), WithGrantIssuer(grants))

	result, err := service.ClaimTask(context.Background(), "secret")
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Task != nil {
		t.Fatalf("task = %+v, want nil", result.Task)
	}
	if grants.calls != 0 {
		t.Fatalf("grant minted %d times, want none with no candidate", grants.calls)
	}
}

// Losing the race is not an error: another poll took the task, and "nothing for
// you" is the accurate answer.
func TestClaimTaskAnswersNullWhenTheClaimLosesItsRace(t *testing.T) {
	store := newMemoryStore()
	store.candidate = taskFixture("transfer-1", model.StatusPending, 9, teamOf(7))
	store.candidateFound = true
	store.claimOK = false
	grants := &stubGrants{grant: DownloadGrant{URL: "https://example.invalid/x", ExpiresAt: testNow().Add(time.Minute)}}
	service := testService(store, workingNode(), fixedClock(testNow()), WithGrantIssuer(grants))

	result, err := service.ClaimTask(context.Background(), "secret")
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Task != nil {
		t.Fatalf("task = %+v, want nil", result.Task)
	}
}

// The lease has to carry everything the executor needs to work offline of Cloud:
// the name to write, the size and hash to check, and the lease length it is to
// heartbeat against. The attempt count comes from the claimed row, so it is the
// attempt this lease *is*.
func TestClaimTaskCarriesTheFactsTheExecutorNeeds(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusPending, 9, teamOf(7)))
	store.candidate = store.tasks["transfer-1"]
	store.candidateFound = true
	store.claimOK = true
	expires := testNow().Add(15 * time.Minute)
	grants := &stubGrants{grant: DownloadGrant{URL: "https://example.invalid/get?sig=x", ExpiresAt: expires}}
	service := testService(store, workingNode(), fixedClock(testNow()), WithGrantIssuer(grants), WithLease(90*time.Second))

	result, err := service.ClaimTask(context.Background(), "secret")
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Task == nil {
		t.Fatal("task = nil, want a lease")
	}
	lease := *result.Task
	if lease.TaskID != "transfer-1" || lease.Title != "演示素材" || lease.AssetID != 42 {
		t.Fatalf("lease = %+v", lease)
	}
	if lease.TotalBytes != 1000 || lease.ExpectedSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("lease = %+v, want the declared size and hash", lease)
	}
	if lease.LeaseSeconds != 90 {
		t.Fatalf("lease seconds = %d, want the configured lease", lease.LeaseSeconds)
	}
	if lease.AttemptCount != 1 || lease.MaxAttempts != 3 {
		t.Fatalf("lease = %+v, want the count this lease is", lease)
	}
	if lease.DownloadURL != "https://example.invalid/get?sig=x" || !lease.DownloadURLExpiresAt.Equal(expires) {
		t.Fatalf("lease grant = %+v", lease)
	}
	if store.lastClaim.nodeID != nodeID || store.lastClaim.lease != 90*time.Second {
		t.Fatalf("claim = %+v, want the node's own id and the configured lease", store.lastClaim)
	}
}

// Only the two credential sentinels become "this node may not act". A fault of
// ours must stay a fault of ours, or a database outage would answer every
// executor with 401 and send the fleet to re-register for nothing.
func TestClaimTaskKeepsAFailureOfOursApartFromARejectedCaller(t *testing.T) {
	broken := errors.New("node table is unreachable")
	for _, testCase := range []struct {
		name string
		err  error
		want error
	}{
		{name: "unknown credential", err: runtimeservice.ErrNodeCredentialInvalid, want: ErrNodeUnauthenticated},
		{name: "dead session", err: runtimeservice.ErrBoundSessionInvalid, want: ErrNodeUnauthenticated},
		{name: "our own fault", err: broken, want: broken},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newMemoryStore()
			store.candidateFound = true
			service := testService(store, &stubNodes{err: testCase.err}, fixedClock(testNow()))

			_, err := service.ClaimTask(context.Background(), "secret")
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
			if store.count("next") != 0 {
				t.Fatalf("candidate read %d times, want none before the caller is identified", store.count("next"))
			}
		})
	}
}

// "The task is over" and "your lease lapsed" call for different actions, and the
// executor is the party that has to take one. Both are 409 to the wire; only the
// first means the bytes being written are no longer wanted.
func TestHeartbeatDistinguishesAFinishedTaskFromALapsedLease(t *testing.T) {
	cancelled := taskFixture("cancelled", model.StatusCancelled, 9, teamOf(7))
	requested := taskFixture("requested", model.StatusRunning, 9, teamOf(7))
	at := testNow()
	requested.CancelRequestedAt = &at
	released := taskFixture("released", model.StatusPending, 9, teamOf(7))

	for _, testCase := range []struct {
		name string
		task Task
		want error
	}{
		{name: "already terminal", task: cancelled, want: ErrTaskCancelled},
		{name: "asked to stop", task: requested, want: ErrTaskCancelled},
		{name: "lease lapsed", task: released, want: ErrTaskConflict},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newMemoryStore(testCase.task)
			store.heartbeatOK = false
			service := testService(store, workingNode(), fixedClock(testNow()))

			err := service.HeartbeatTask("secret", testCase.task.ID, 100)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestHeartbeatTaskRequiresANode(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.heartbeatOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if err := service.HeartbeatTask("secret", "transfer-1", 100); err != nil {
		t.Fatalf("HeartbeatTask() error = %v", err)
	}
	if err := service.HeartbeatTask("secret", "transfer-1", -1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative progress error = %v", err)
	}
	if store.count("heartbeat") != 1 {
		t.Fatalf("heartbeat called %d times, want only the valid one", store.count("heartbeat"))
	}
}

// The frozen `Progress` body has no total and no estimate, so both are derived
// here from the row. If the derivation moved to a client, every reader would
// have to guess the denominator from the largest byte count it had seen.
func TestReportProgressDerivesTheEstimateTheBodyCannotCarry(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.progressOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if err := service.ReportProgress("secret", "transfer-1", 250, 50); err != nil {
		t.Fatalf("ReportProgress() error = %v", err)
	}
	if store.lastProg.TotalBytes != 1000 {
		t.Fatalf("total = %d, want the row's declared size", store.lastProg.TotalBytes)
	}
	if store.lastProg.ETASeconds != 15 {
		t.Fatalf("eta = %d, want (1000-250)/50", store.lastProg.ETASeconds)
	}
	if store.lastProg.TransferredBytes != 250 || store.lastProg.SpeedBytesPerSec != 50 || store.lastProg.NodeID != nodeID {
		t.Fatalf("progress = %+v", store.lastProg)
	}
}

// An estimate nobody can give answers zero, which is the same value the
// repository's columns treat as absent — so a reader has one rule for "unknown"
// rather than three.
func TestReportProgressLeavesTheEstimateEmptyWhenItCannotBeKnown(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		total     int64
		completed int64
		speed     int64
	}{
		{name: "no size known", total: 0, completed: 100, speed: 50},
		{name: "no rate yet", total: 1000, completed: 100, speed: 0},
		{name: "already at the total", total: 1000, completed: 1000, speed: 50},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			task := taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7))
			task.TotalBytes = testCase.total
			store := newMemoryStore(task)
			store.progressOK = true
			service := testService(store, workingNode(), fixedClock(testNow()))

			if err := service.ReportProgress("secret", "transfer-1", testCase.completed, testCase.speed); err != nil {
				t.Fatalf("ReportProgress() error = %v", err)
			}
			if store.lastProg.ETASeconds != 0 {
				t.Fatalf("eta = %d, want zero for an estimate that cannot be made", store.lastProg.ETASeconds)
			}
		})
	}
}

func TestReportProgressRefusesANegativeReading(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.progressOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if err := service.ReportProgress("secret", "transfer-1", -1, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v", err)
	}
	if err := service.ReportProgress("secret", "transfer-1", 10, -1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative rate error = %v", err)
	}
	if store.count("progress") != 0 {
		t.Fatalf("progress called %d times, want none", store.count("progress"))
	}
}

// A success that disagrees with the task is the one failure the executor cannot
// discover on its own, and the answer names the rule so it can act: the bytes it
// produced are not the bytes the task declared.
func TestCompleteTaskRefusesASuccessThatDisagreesWithTheTask(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		completed int64
		sha       string
		want      error
	}{
		{name: "short", completed: 999, sha: strings.Repeat("a", 64), want: ErrIntegrityFailed},
		{name: "wrong hash", completed: 1000, sha: strings.Repeat("b", 64), want: ErrIntegrityFailed},
		{name: "not a hash at all", completed: 1000, sha: "deadbeef", want: ErrInvalidInput},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
			store.completeOK = true
			service := testService(store, workingNode(), fixedClock(testNow()))

			_, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{
				Status: string(model.StatusSuccess), CompletedBytes: testCase.completed, SHA256: testCase.sha,
			})
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
			if store.count("complete") != 0 {
				t.Fatalf("complete called %d times, want none for a mismatch", store.count("complete"))
			}
		})
	}
}

// The frozen pattern accepts either case, so the comparison would too on most
// collations; normalising on the way in makes the stored hash and the repository
// predicate one rule rather than two that happen to agree.
func TestCompleteTaskNormalisesTheChecksumItStores(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.completeOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	upper := strings.ToUpper(strings.Repeat("a", 64))
	if _, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{
		Status: string(model.StatusSuccess), CompletedBytes: 1000, SHA256: upper, FileName: "演示素材-42.mp4",
	}); err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if store.lastDone.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("stored sha = %q, want the lowercased form", store.lastDone.SHA256)
	}
	if store.lastDone.FileName != "演示素材-42.mp4" || store.lastDone.Bytes != 1000 {
		t.Fatalf("completion = %+v", store.lastDone)
	}
}

// A file name is a name, not a path: the frozen pattern refuses separators, and a
// name long enough to be truncated somewhere else is refused rather than stored.
func TestCompleteTaskRefusesANameThatCouldBecomeAPath(t *testing.T) {
	for _, name := range []string{"a/b.mp4", `a\b.mp4`, strings.Repeat("x", 256)} {
		store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
		store.completeOK = true
		service := testService(store, workingNode(), fixedClock(testNow()))

		_, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{
			Status: string(model.StatusSuccess), CompletedBytes: 1000, SHA256: strings.Repeat("a", 64), FileName: name,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("name %q error = %v, want invalid input", name, err)
		}
	}
	// An executor that named no file is not an error: the key is optional in the
	// frozen contract, and a task whose file name was already recorded keeps it.
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.completeOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))
	if _, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{
		Status: string(model.StatusSuccess), CompletedBytes: 1000, SHA256: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("unnamed file error = %v", err)
	}
}

// A failure and a cancellation go through the failure statement, not the
// completion one: they require an error code and must not be able to write
// `success` even by mistake.
func TestCompleteTaskWritesFailureThroughTheFailurePath(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.failOK = true
	store.completeOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	terminal, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{
		Status: string(model.StatusCancelled), ErrorCode: "cancelled_by_user", ErrorMessage: "cancelled by user",
	})
	if err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if store.count("complete") != 0 {
		t.Fatalf("completion statement used %d times, want the failure path", store.count("complete"))
	}
	if store.lastFail.Status != model.StatusCancelled || store.lastFail.ErrorCode != "cancelled_by_user" {
		t.Fatalf("failure = %+v", store.lastFail)
	}
	if terminal.TaskID != "transfer-1" || terminal.Status != string(model.StatusCancelled) {
		t.Fatalf("terminal = %+v", terminal)
	}
}

func TestCompleteTaskRequiresAnErrorCodeForAFailure(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.failOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	_, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{Status: string(model.StatusFailed)})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
	if store.count("fail") != 0 {
		t.Fatalf("failure statement used %d times, want none", store.count("fail"))
	}
}

func TestCompleteTaskRefusesATaskAnotherNodeHolds(t *testing.T) {
	held := taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7))
	held.ClaimedByNodeID = "node-2"
	released := taskFixture("released", model.StatusPending, 9, teamOf(7))

	for _, task := range []Task{held, released} {
		store := newMemoryStore(task)
		store.completeOK = true
		store.failOK = true
		service := testService(store, workingNode(), fixedClock(testNow()))

		_, err := service.CompleteTask("secret", task.ID, CompletionRequest{
			Status: string(model.StatusSuccess), CompletedBytes: 1000, SHA256: strings.Repeat("a", 64),
		})
		if !errors.Is(err, ErrTaskConflict) {
			t.Fatalf("%s error = %v, want conflict", task.ID, err)
		}
		if store.count("complete") != 0 {
			t.Fatalf("%s: complete called %d times, want none", task.ID, store.count("complete"))
		}
	}
}

// ---- creation, and the seams' default posture ----

func TestCreateTaskOwnsTheIdTheAssetTypeAndTheAttemptBound(t *testing.T) {
	store := newMemoryStore()
	service := testService(store, workingNode(), fixedClock(testNow()))

	body, err := service.CreateTask(CreateTaskInput{
		TeamID: teamOf(7), AssetID: 42, AssetTitle: " 演示素材 ", SourceObjectKey: "materials/42/deadbeef.mp4",
		Purpose: model.PurposeUserDownload, ExecutionScope: model.ExecutionLocalAgent, RequestedBy: 9,
		AssignedNodeID: nodeID, TotalBytes: 1000, ExpectedSHA256: strings.ToUpper(strings.Repeat("a", 64)),
		DedupeKey: strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if store.lastCreate.ID == "" {
		t.Fatal("id = empty, want one generated here")
	}
	if store.lastCreate.AssetType != model.AssetMaterial {
		t.Fatalf("asset type = %q, want material", store.lastCreate.AssetType)
	}
	if store.lastCreate.MaxAttempts != 3 {
		t.Fatalf("max attempts = %d, want the three the schema declares", store.lastCreate.MaxAttempts)
	}
	if store.lastCreate.ExpectedSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("expected sha = %q, want the lowercased form", store.lastCreate.ExpectedSHA256)
	}
	if store.lastCreate.AssetTitle != "演示素材" || store.lastCreate.SourceObjectKey != "materials/42/deadbeef.mp4" {
		t.Fatalf("input = %+v", store.lastCreate)
	}
	if body.Status != string(model.StatusPending) {
		t.Fatalf("status = %q, want the row's own value", body.Status)
	}
}

// The attempt bound is stated twice — in the migration and in this module — and
// a test that compared the constant against itself would follow the constant
// wherever it moved. So the schema is the oracle: this is a statement about both
// files, and it fails if either one changes without the other.
func TestTheAttemptBoundMatchesTheSchemaDefault(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "20260926_039_content_production_m4_a.sql"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	matches := regexp.MustCompile(`max_attempts\s+INT UNSIGNED NOT NULL DEFAULT (\d+)`).FindStringSubmatch(string(content))
	if len(matches) != 2 {
		t.Fatalf("migration declares max_attempts %d times as an integer with a default, want exactly one: %v", len(matches)-1, matches)
	}
	if matches[1] != strconv.Itoa(defaultMaxAttempts) {
		t.Fatalf("module bound = %d, schema default = %s", defaultMaxAttempts, matches[1])
	}
}

func TestCreateTaskRefusesInputWithoutItsOwnScope(t *testing.T) {
	store := newMemoryStore()
	service := testService(store, workingNode(), fixedClock(testNow()))

	// The four facts that make a task addressable are this module's to check. The
	// purpose-dependent ones — the object to download, its size and its hash — are
	// the repository's, and an input our own code built without them is a fault of
	// ours rather than a malformed request, which is why it is not refused here.
	key := strings.Repeat("c", 64)
	for _, input := range []CreateTaskInput{
		{AssetID: 42, RequestedBy: 9, DedupeKey: key},
		{TeamID: teamOf(7), RequestedBy: 9, DedupeKey: key},
		{TeamID: teamOf(7), AssetID: 42, DedupeKey: key},
		{TeamID: teamOf(7), AssetID: 42, RequestedBy: 9},
	} {
		if _, err := service.CreateTask(input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("input %+v error = %v, want invalid input", input, err)
		}
	}
	if store.count("create") != 0 {
		t.Fatalf("create called %d times, want none", store.count("create"))
	}
}

// The module's own default must refuse rather than lease. An unconfigured
// issuer that returned an empty url with no error would be caught by the lease
// check; one that returned nil error and nil grant is the case this pins.
func TestTheDefaultGrantIssuerRefusesToMint(t *testing.T) {
	store := newMemoryStore()
	store.candidate = taskFixture("transfer-1", model.StatusPending, 9, teamOf(7))
	store.candidateFound = true
	store.claimOK = true
	service := testService(store, workingNode(), fixedClock(testNow()))

	if _, err := service.ClaimTask(context.Background(), "secret"); !errors.Is(err, ErrGrantUnavailable) {
		t.Fatalf("ClaimTask() error = %v, want the unconfigured issuer's refusal", err)
	}
	if store.count("claim") != 0 {
		t.Fatalf("claim called %d times, want none", store.count("claim"))
	}
}

// A service built without an authenticator must not read "I could not check" as
// "the caller is a node".
func TestAServiceWithoutANodeAuthenticatorRefusesEveryExecutorCall(t *testing.T) {
	store := newMemoryStore(taskFixture("transfer-1", model.StatusRunning, 9, teamOf(7)))
	store.candidateFound = true
	store.claimOK = true
	store.heartbeatOK = true
	store.progressOK = true
	store.completeOK = true
	store.failOK = true
	service := NewService(store, nil, fixedClock(testNow()))

	if _, err := service.ClaimTask(context.Background(), "secret"); !errors.Is(err, ErrNodeUnauthenticated) {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if err := service.HeartbeatTask("secret", "transfer-1", 1); !errors.Is(err, ErrNodeUnauthenticated) {
		t.Fatalf("HeartbeatTask() error = %v", err)
	}
	if err := service.ReportProgress("secret", "transfer-1", 1, 1); !errors.Is(err, ErrNodeUnauthenticated) {
		t.Fatalf("ReportProgress() error = %v", err)
	}
	if _, err := service.CompleteTask("secret", "transfer-1", CompletionRequest{Status: string(model.StatusSuccess), CompletedBytes: 1000, SHA256: strings.Repeat("a", 64)}); !errors.Is(err, ErrNodeUnauthenticated) {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if len(store.counts) != 0 {
		t.Fatalf("store was touched %v, want nothing before the caller is identified", store.counts)
	}
}

// An empty credential never reaches the authenticator: there is nothing to look
// up, and the answer is not in doubt.
func TestAnEmptyCredentialIsRefusedWithoutAsking(t *testing.T) {
	store := newMemoryStore()
	nodes := &stubNodes{node: runtimeservice.AgentNode{ID: nodeID}}
	service := testService(store, nodes, fixedClock(testNow()))

	if _, err := service.ClaimTask(context.Background(), "   "); !errors.Is(err, ErrNodeUnauthenticated) {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if nodes.calls != 0 {
		t.Fatalf("authenticator asked %d times, want none for a blank credential", nodes.calls)
	}
}

// The service's job here is narrow and worth pinning: it fills the purpose and
// the scope the caller is not allowed to choose, and it takes no dedupe key.
// The idempotency decision belongs to the repository, which can count and insert
// in one transaction; a service that let a caller pass a key would move that
// decision to whatever called it.
func TestCreateUserDownloadFixesThePurposeTheScopeAndTheAttemptBound(t *testing.T) {
	store := newMemoryStore()
	svc := testService(store, workingNode())

	task, err := svc.CreateUserDownload(CreateUserDownloadInput{
		TeamID:          7,
		AssetID:         42,
		AssetTitle:      "示例视频",
		SourceObjectKey: "materials/42/abc.mp4",
		RequestedBy:     9,
		AssignedNodeID:  "node-1",
		TotalBytes:      1024,
		ExpectedSHA256:  strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatalf("CreateUserDownload() error = %v", err)
	}
	if store.counts["createDownload"] != 1 || store.counts["create"] != 0 {
		t.Fatalf("the download path must not go through the generic create: %v", store.counts)
	}
	input := store.lastDownload
	if input.TeamID != 7 || input.AssetID != 42 || input.RequestedBy != 9 || input.AssignedNodeID != "node-1" {
		t.Fatalf("download input = %+v", input)
	}
	if input.TotalBytes != 1024 || input.ExpectedSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("the completeness facts must travel with the task: %+v", input)
	}
	if input.MaxAttempts != 3 {
		t.Fatalf("attempt bound = %d, want the schema default 3", input.MaxAttempts)
	}
	// The wire body, not the row: the two are built by different functions and only
	// one of them is on this path, so the strings the client will read are asserted
	// as strings.
	if task.Purpose != string(model.PurposeUserDownload) || task.ExecutionScope != string(model.ExecutionLocalAgent) {
		t.Fatalf("task purpose/scope = %s/%s", task.Purpose, task.ExecutionScope)
	}
	if task.AssetType != string(model.AssetMaterial) || task.ID == "" || task.Status != string(model.StatusPending) {
		t.Fatalf("task = %+v", task)
	}
}

func TestCreateUserDownloadRefusesAnIncompleteIdentity(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		input CreateUserDownloadInput
	}{
		{"no team", CreateUserDownloadInput{AssetID: 42, RequestedBy: 9, AssignedNodeID: "node-1"}},
		{"no material", CreateUserDownloadInput{TeamID: 7, RequestedBy: 9, AssignedNodeID: "node-1"}},
		{"no user", CreateUserDownloadInput{TeamID: 7, AssetID: 42, AssignedNodeID: "node-1"}},
	} {
		store := newMemoryStore()
		if _, err := testService(store, workingNode()).CreateUserDownload(testCase.input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: error = %v, want ErrInvalidInput", testCase.name, err)
		}
		if store.counts["createDownload"] != 0 {
			t.Fatalf("%s: a refused download still reached the store", testCase.name)
		}
	}
}

// A download that waits for a preparation has no facts of its own: the object key,
// the size and the hash do not exist until the preparation produces them. The point
// of this test is that the dependency survives the service layer at all — dropping
// it here would leave a task whose facts are empty and which the store, correctly,
// refuses to create, so the failure would look like a validation bug at the far end
// of the call.
func TestCreateUserDownloadCarriesThePreparationItWaitsFor(t *testing.T) {
	store := newMemoryStore()
	svc := testService(store, workingNode())

	task, err := svc.CreateUserDownload(CreateUserDownloadInput{
		TeamID:           7,
		AssetID:          42,
		AssetTitle:       "示例视频",
		RequestedBy:      9,
		AssignedNodeID:   "node-1",
		DependencyTaskID: "transfer-prepare-1",
	})
	if err != nil {
		t.Fatalf("CreateUserDownload() error = %v", err)
	}
	if store.lastDownload.DependencyTaskID != "transfer-prepare-1" {
		t.Fatalf("dependency = %q, want the preparation the caller named", store.lastDownload.DependencyTaskID)
	}
	if store.lastDownload.SourceObjectKey != "" || store.lastDownload.TotalBytes != 0 || store.lastDownload.ExpectedSHA256 != "" {
		t.Fatalf("a waiting download must carry no facts of its own: %+v", store.lastDownload)
	}
	if task.Status != string(model.StatusPending) {
		t.Fatalf("status = %s, want pending", task.Status)
	}
}

// The preparation is a Cloud task on a material, and every one of those three is
// fixed here rather than passed in: the caller could get any of them wrong, and two
// of them wrong at once is a task no worker would ever claim.
func TestEnsureMaterialSourcePrepareQueuesACloudPreparation(t *testing.T) {
	store := newMemoryStore()
	svc := testService(store, workingNode())

	task, err := svc.EnsureMaterialSourcePrepare(EnsureMaterialSourcePrepareInput{
		TeamID:      7,
		AssetID:     42,
		AssetTitle:  "  示例视频  ",
		RequestedBy: 9,
	})
	if err != nil {
		t.Fatalf("EnsureMaterialSourcePrepare() error = %v", err)
	}
	if store.counts["createPrepare"] != 1 || store.counts["create"] != 0 || store.counts["createDownload"] != 0 {
		t.Fatalf("the preparation must not go through another create path: %v", store.counts)
	}
	input := store.lastPrepare
	if input.TeamID != 7 || input.AssetID != 42 || input.RequestedBy != 9 {
		t.Fatalf("prepare input = %+v", input)
	}
	if input.AssetTitle != "示例视频" {
		t.Fatalf("title = %q, want the trimmed title", input.AssetTitle)
	}
	if input.MaxAttempts != 3 || input.ID == "" {
		t.Fatalf("prepare input = %+v", input)
	}
	if task.Purpose != string(model.PurposeComposeInputPrepare) || task.ExecutionScope != string(model.ExecutionCloud) {
		t.Fatalf("task purpose/scope = %s/%s, want a Cloud preparation", task.Purpose, task.ExecutionScope)
	}
	// There is deliberately no assertion that the task carries no node: a Cloud task
	// is claimed by whichever worker polls first, and `CreateMaterialSourcePrepareInput`
	// has no node field, so this path cannot name one to begin with. A test of it
	// could not fail and is not written.
}

func TestEnsureMaterialSourcePrepareRefusesAnIncompleteIdentity(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		input EnsureMaterialSourcePrepareInput
	}{
		{"no team", EnsureMaterialSourcePrepareInput{AssetID: 42, RequestedBy: 9}},
		{"no material", EnsureMaterialSourcePrepareInput{TeamID: 7, RequestedBy: 9}},
		{"no user", EnsureMaterialSourcePrepareInput{TeamID: 7, AssetID: 42}},
	} {
		store := newMemoryStore()
		if _, err := testService(store, workingNode()).EnsureMaterialSourcePrepare(testCase.input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: error = %v, want ErrInvalidInput", testCase.name, err)
		}
		if store.counts["createPrepare"] != 0 {
			t.Fatalf("%s: a refused preparation still reached the store", testCase.name)
		}
	}
}
