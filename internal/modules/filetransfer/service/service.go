// Package service owns the transfer task lifecycle as both the session API and
// the executor API see it.
//
// Two audiences, one lifecycle. A session asks what it has queued and asks for a
// transfer to stop or to be retried; a Local Agent asks for work, reports how far
// it has got, and reports the outcome. The rules that matter — who may act on a
// task, when a task may be leased, what a success has to agree with — live here
// rather than in the handlers so that the two APIs cannot drift apart in their
// answers, and so that a caller that is not HTTP at all gets the same rules.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	Status         = model.Status
	Purpose        = model.Purpose
	AssetType      = model.AssetType
	ExecutionScope = model.ExecutionScope

	// The wire bodies are re-exported so that a handler — or a sibling module
	// queueing a transfer — needs one import rather than two. They are aliases,
	// not copies: the key set belongs to the dto package, and a second declaration
	// here would be a second place for it to drift from the frozen contract.
	TaskBody          = dto.Task
	LocalLease        = dto.LocalLease
	ClaimResult       = dto.ClaimResult
	TransferTerminal  = dto.TransferTerminal
	HeartbeatRequest  = dto.HeartbeatRequest
	ProgressRequest   = dto.ProgressRequest
	CompletionRequest = dto.CompletionRequest
)

const (
	StatusPending   = model.StatusPending
	StatusRunning   = model.StatusRunning
	StatusSuccess   = model.StatusSuccess
	StatusFailed    = model.StatusFailed
	StatusCancelled = model.StatusCancelled

	PurposeUserDownload        = model.PurposeUserDownload
	PurposeComposeInputPrepare = model.PurposeComposeInputPrepare

	ScopeCloud      = model.ExecutionCloud
	ScopeLocalAgent = model.ExecutionLocalAgent

	AssetMaterial = model.AssetMaterial
)

var (
	// ErrInvalidInput is a request that cannot be acted on at all — a malformed
	// id, a negative byte count, a status outside the frozen enum. It is kept
	// apart from ErrTaskConflict, which is a well-formed request about a task that
	// happens to be in the wrong state.
	ErrInvalidInput = errors.New("file transfer input is invalid")

	// ErrNodeUnauthenticated is an unknown or superseded credential, a missing
	// one, or a node whose bound session has died. Those are one answer to the
	// caller — this node may not act — and one remedy, re-registering, so they
	// share a sentinel instead of leaking the distinction into the handler.
	ErrNodeUnauthenticated = errors.New("local node credential is not usable")

	// ErrTaskNotFound is no such task *or* a task outside the caller's team. The
	// tenant boundary is deliberately not something a caller can probe by
	// comparing 403 against 404.
	ErrTaskNotFound = errors.New("file transfer task not found")

	// ErrTaskForbidden is another user's task within the caller's own team, which
	// the frozen session contract names as `transfer_task_forbidden`.
	ErrTaskForbidden = errors.New("file transfer task belongs to another user")

	// ErrTaskConflict is a well-formed request the task's current state refuses —
	// already terminal, not retryable, or no longer leased by this node.
	ErrTaskConflict = errors.New("file transfer task is not in a state that allows this operation")

	// ErrTaskCancelled is a task that is over, or that has been asked to stop.
	// It is separate from ErrTaskConflict because it is the one refusal an
	// executor must act on rather than retry: the bytes it is writing are no
	// longer wanted.
	ErrTaskCancelled = errors.New("file transfer task has been cancelled or finished")

	// ErrIntegrityFailed is a reported success that does not agree with the task
	// it claims to have completed, and the frozen `transfer_integrity_failed`.
	ErrIntegrityFailed = errors.New("reported transfer does not match the leased task")

	// ErrGrantUnavailable is the seam's contract: a download grant could not be
	// minted. Nothing is leased when this is returned, so the task keeps its
	// attempts and stays claimable.
	ErrGrantUnavailable = errors.New("download grant is unavailable")
)

// Store is the persistence the lifecycle needs, and nothing more.
//
// Every method takes the time it is to use rather than reading a clock of its
// own, so the module's sense of "now" is decided here, once, and a test can pin
// it.
type Store interface {
	CreateTask(repository.CreateTaskInput, time.Time) (model.Task, error)
	CreateUserDownloadTask(repository.CreateUserDownloadInput, time.Time) (model.Task, error)
	CreateMaterialSourcePrepareTask(repository.CreateMaterialSourcePrepareInput, time.Time) (model.Task, error)
	GetTask(string) (model.Task, error)
	ListTasks(repository.TaskFilter) ([]model.Task, error)
	LatestUserDownloadStatuses(identityservice.UserID, []int64) (map[int64]string, error)
	NextLocalTask(string, time.Time) (model.Task, bool, error)
	ClaimLocalTask(string, string, time.Time, time.Duration) (model.Task, bool, error)
	HeartbeatTask(string, string, time.Time, time.Duration) (bool, error)
	ReportProgress(repository.ProgressInput, time.Time) (bool, error)
	CompleteTask(repository.CompletionInput, time.Time) (bool, error)
	FailTask(repository.FailureInput, time.Time) (bool, error)
	CancelTask(string, identityservice.TeamID, identityservice.UserID, time.Time) (bool, error)
	FailDependents(string, string, string, time.Time) (int64, error)
	RetryTask(string, identityservice.TeamID, identityservice.UserID, time.Time) (model.Task, bool, error)
}

// NodeAuthenticator is the one question this module asks the runtime-binding
// domain: which node does this credential belong to?
//
// A consumer-side interface, one method wide, implemented in store_adapter.go —
// the shape profileguard already uses. Declaring it here rather than calling
// `runtimeservice.AuthenticateNodeCredential` directly keeps the dependency on
// that module to a single call site, and lets a test answer the question without
// a node table.
type NodeAuthenticator interface {
	AuthenticateNodeCredential(credential string) (runtimeservice.AgentNode, error)
}

// DownloadGrant is a short-lived, single-object GET grant: the only place a
// signed URL exists, and never a durable storage credential.
type DownloadGrant struct {
	URL       string
	ExpiresAt time.Time
}

// GrantIssuer mints the grant a lease carries.
//
// The context is there for an issuer that has to reach the object store; the
// implementation wired today signs locally and ignores it.
type GrantIssuer interface {
	PresignGet(ctx context.Context, objectKey string) (DownloadGrant, error)
}

const (
	// DefaultLease is how long a lease lasts unless overridden, and the executor
	// is told it in `lease_seconds`. It heartbeats at a third of it, so three
	// consecutive failures are absorbed before the lease lapses.
	//
	// Exported because the Cloud worker claims through the repository rather than
	// through this service, and a lease is one decision shared by both executors:
	// a second copy of the number would make the retry timing depend on which of
	// them ran the task, with nothing to notice when the two drifted apart.
	DefaultLease = 120 * time.Second

	// defaultMaxAttempts mirrors the column default in
	// `migrations/20260926_039_content_production_m4_a.sql`. The insert writes the
	// value explicitly, so the column default never applies and this const is the
	// one that decides.
	defaultMaxAttempts = 3

	// defaultListLimit bounds the session listing. The repository caps at 200 on
	// its own; this states the module's bound instead of inheriting one.
	defaultListLimit = 100

	// maxFileNameLength is the frozen `Completion.file_name` bound.
	maxFileNameLength = 255
)

type Service struct {
	store    Store
	nodes    NodeAuthenticator
	grants   GrantIssuer
	now      func() time.Time
	lease    time.Duration
	maxTries int
	listMax  int
}

type Option func(*Service)

// WithClock pins the clock, for tests.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithLease overrides the lease duration, for tests that need a short one.
func WithLease(lease time.Duration) Option { return func(s *Service) { s.lease = lease } }

// WithGrantIssuer replaces the grant issuer.
func WithGrantIssuer(issuer GrantIssuer) Option { return func(s *Service) { s.grants = issuer } }

func NewService(store Store, nodes NodeAuthenticator, opts ...Option) *Service {
	service := &Service{
		store:    store,
		nodes:    nodes,
		grants:   unavailableGrants{},
		now:      time.Now,
		lease:    DefaultLease,
		maxTries: defaultMaxAttempts,
		listMax:  defaultListLimit,
	}
	for _, opt := range opts {
		opt(service)
	}
	if service.nodes == nil {
		service.nodes = unavailableNodes{}
	}
	return service
}

// unavailableNodes fails closed: a service built without an authenticator must
// not read "I could not check" as "the caller is a node".
type unavailableNodes struct{}

func (unavailableNodes) AuthenticateNodeCredential(string) (runtimeservice.AgentNode, error) {
	return runtimeservice.AgentNode{}, ErrNodeUnauthenticated
}

// unavailableGrants is the default issuer until the object store is wired.
// Returning a sentinel rather than an empty URL is what keeps the claim path
// honest: there is no state in which a lease is issued carrying a grant this
// module could not mint.
type unavailableGrants struct{}

func (unavailableGrants) PresignGet(context.Context, string) (DownloadGrant, error) {
	return DownloadGrant{}, ErrGrantUnavailable
}

// CreateTaskInput is what an asset domain supplies to queue a transfer.
//
// The id, the asset type and the attempt bound are absent on purpose: they are
// this module's to decide, and a caller that could set them could also set an
// unbounded retry budget or a task id that collides with a live one.
type CreateTaskInput struct {
	TeamID          identityservice.TeamID
	AssetID         int64
	AssetTitle      string
	SourceObjectKey string
	Purpose         model.Purpose
	ExecutionScope  model.ExecutionScope
	RequestedBy     identityservice.UserID
	AssignedNodeID  string
	TotalBytes      int64
	ExpectedSHA256  string
	DedupeKey       string
}

// CreateTask queues a transfer and returns the task that now exists.
//
// It returns the row rather than an error when the dedupe key is already taken:
// a repeat click is not a failure, and the surviving task is the answer the
// caller asked for. Whether a caller should answer 200 or 201 for that is the
// caller's business, which is why creation is reported as a value here and not
// as a status.
func (s *Service) CreateTask(input CreateTaskInput) (dto.Task, error) {
	if input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 || strings.TrimSpace(input.DedupeKey) == "" {
		return dto.Task{}, ErrInvalidInput
	}
	task, err := s.store.CreateTask(repository.CreateTaskInput{
		ID:              id.NewID("transfer"),
		TeamID:          input.TeamID,
		AssetType:       model.AssetMaterial,
		AssetID:         input.AssetID,
		AssetTitle:      strings.TrimSpace(input.AssetTitle),
		SourceObjectKey: strings.TrimSpace(input.SourceObjectKey),
		Purpose:         input.Purpose,
		ExecutionScope:  input.ExecutionScope,
		RequestedBy:     input.RequestedBy,
		AssignedNodeID:  strings.TrimSpace(input.AssignedNodeID),
		DedupeKey:       input.DedupeKey,
		TotalBytes:      input.TotalBytes,
		ExpectedSHA256:  strings.ToLower(strings.TrimSpace(input.ExpectedSHA256)),
		MaxAttempts:     s.maxTries,
	}, s.now().UTC())
	if err != nil {
		return dto.Task{}, err
	}
	return taskBody(task), nil
}

// CreateUserDownload queues a user download, and answers with the task that now
// exists — which for a repeated click is the one already outstanding.
//
// The idempotency decision is the repository's, because it needs the count and
// the insert in one transaction; what belongs here is the fact that this method
// takes no dedupe key and no generation. A caller cannot ask for a duplicate.
func (s *Service) CreateUserDownload(input CreateUserDownloadInput) (dto.Task, error) {
	if input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 {
		return dto.Task{}, ErrInvalidInput
	}
	task, err := s.store.CreateUserDownloadTask(repository.CreateUserDownloadInput{
		ID:               id.NewID("transfer"),
		TeamID:           input.TeamID,
		AssetID:          input.AssetID,
		AssetTitle:       input.AssetTitle,
		GameName:         input.GameName,
		PublishedAt:      input.PublishedAt,
		SourceObjectKey:  input.SourceObjectKey,
		RequestedBy:      input.RequestedBy,
		AssignedNodeID:   input.AssignedNodeID,
		TotalBytes:       input.TotalBytes,
		ExpectedSHA256:   input.ExpectedSHA256,
		DependencyTaskID: input.DependencyTaskID,
		MaxAttempts:      s.maxTries,
	}, s.now().UTC())
	if err != nil {
		return dto.Task{}, err
	}
	return taskBody(task), nil
}

// CreateUserDownloadInput carries the material facts a download needs and no
// identity of its own: who is downloading and which machine it goes to are
// fields, so a caller cannot omit them and have a task created for somebody.
type CreateUserDownloadInput struct {
	TeamID     identityservice.TeamID
	AssetID    int64
	AssetTitle string
	// GameName is what the executor files the download under, when the material
	// has a game. Empty is a legal value: the executor falls back to a fixed
	// placeholder.
	GameName string
	// PublishedAt is when the material was published, carried to the executor for
	// naming. Nil is a legal value: the executor omits that segment.
	PublishedAt     *time.Time
	SourceObjectKey string
	RequestedBy     identityservice.UserID
	AssignedNodeID  string
	TotalBytes      int64
	ExpectedSHA256  string
	// DependencyTaskID is the Cloud preparation this download waits for, when the
	// video was not ready at the moment the user asked for it. With it set, the
	// object key, size and hash above are empty — the preparation produces them —
	// and the task is not claimable until it has.
	DependencyTaskID string
}

// EnsureMaterialSourcePrepareInput is what queueing a preparation needs. It has no
// user-facing knobs on purpose: the size, the hash, the object key and the
// lifetime are all decided by the execution, not by the caller.
type EnsureMaterialSourcePrepareInput struct {
	TeamID      identityservice.TeamID
	AssetID     int64
	AssetTitle  string
	RequestedBy identityservice.UserID
}

// EnsureMaterialSourcePrepare queues the Cloud task that fetches a material's
// source video, reusing the outstanding one if there is one.
//
// "Reusing" is the store's dedupe, not a lookup performed here: two clicks in the
// same moment both see no outstanding task and both insert, and the unique index
// decides. A read-then-write in this function could not make that promise, and the
// cost of being wrong is the same video fetched twice.
//
// The caller decides whether a preparation is needed at all — that is a fact about
// the material (`video_status`), and this module may not read the production
// tables.
func (s *Service) EnsureMaterialSourcePrepare(input EnsureMaterialSourcePrepareInput) (dto.Task, error) {
	if input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 {
		return dto.Task{}, ErrInvalidInput
	}
	task, err := s.store.CreateMaterialSourcePrepareTask(repository.CreateMaterialSourcePrepareInput{
		ID:          id.NewID("transfer"),
		TeamID:      input.TeamID,
		AssetID:     input.AssetID,
		AssetTitle:  strings.TrimSpace(input.AssetTitle),
		RequestedBy: input.RequestedBy,
		MaxAttempts: s.maxTries,
	}, s.now().UTC())
	if err != nil {
		return dto.Task{}, err
	}
	return taskBody(task), nil
}

// ListTasks returns the acting user's tasks, newest first.
//
// The filter is built from the actor alone: there is no parameter for a team or
// a user to widen it with, so the listing cannot be asked for someone else's
// work even by a caller that tries.
func (s *Service) ListTasks(actor identityservice.PublicUser) ([]dto.Task, error) {
	if actor.ID <= 0 {
		return nil, ErrInvalidInput
	}
	requestedBy := actor.ID
	tasks, err := s.store.ListTasks(repository.TaskFilter{RequestedBy: &requestedBy, Limit: s.listMax})
	if err != nil {
		return nil, err
	}
	items := make([]dto.Task, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, taskBody(task))
	}
	return items, nil
}

// LatestUserDownloadStatuses answers each material's newest user_download status
// for one user, as a raw task status keyed by asset_id. The production module
// maps that to a display state; this module stays out of what a badge means.
func (s *Service) LatestUserDownloadStatuses(userID identityservice.UserID, materialIDs []int64) (map[int64]string, error) {
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.store.LatestUserDownloadStatuses(userID, materialIDs)
}

// CancelTask asks for a transfer to stop and answers with the refreshed task.
//
// The answer is not always `cancelled`. A pending task has no executor and
// becomes terminal here; a running one only records the request, and stays
// `running` until the executor confirms it (see `cancelTask` in the repository
// for why), so the refreshed body is the only honest answer to "what happened".
func (s *Service) CancelTask(actor identityservice.PublicUser, taskID string) (dto.Task, error) {
	task, team, err := s.scopedTask(actor, taskID)
	if err != nil {
		return dto.Task{}, err
	}
	if task.Status != model.StatusPending && task.Status != model.StatusRunning {
		return dto.Task{}, ErrTaskConflict
	}
	cancelled, err := s.store.CancelTask(task.ID, team, actor.ID, s.now().UTC())
	if err != nil {
		return dto.Task{}, err
	}
	if !cancelled {
		// The task left `pending`/`running` between the read and the write. The
		// same statement's `status IN (...)` clause is what reports it, and the
		// outcome is the same as having seen it terminal a moment earlier.
		return dto.Task{}, ErrTaskConflict
	}
	if task.Purpose == model.PurposeComposeInputPrepare {
		// A cancelled preparation ends the downloads waiting on it. They are
		// `pending` with `dependency_task_id` set, which makes them un-leasable, and
		// the only two ways out of that state are a hand-over — which a cancelled
		// preparation cannot produce — and a failure, which nothing else here writes.
		// Left alone they wait for a preparation that is over, and the download centre
		// shows them no action at all, because a pending row is cancellable and
		// nothing else.
		//
		// Both cancellation phases are covered by doing it after the write rather than
		// inside one of them. A running preparation's executor releases the waiters
		// itself when it notices the request, and `failDependents` matches only rows
		// still `pending` with this pointer, so that second release matches nothing —
		// and if the executor never comes back, this one has already done it.
		//
		// The failure is returned rather than swallowed: the cancellation landed, but
		// the waiters are still stuck, and a caller told "cancelled" would have no
		// reason to look again. `CreateDownload` reports a failed projection write the
		// same way, for the same reason — the module may not claim a state it could
		// not leave consistent.
		if _, err := s.store.FailDependents(task.ID, codeCancelledByUser, cancelledPreparationMessage, s.now().UTC()); err != nil {
			return dto.Task{}, err
		}
	}
	return s.refreshedTask(task.ID)
}

const (
	// codeCancelledByUser is the code the cancellation paths write, and the one a
	// released waiter inherits so that both rows name the same cause.
	codeCancelledByUser = "cancelled_by_user"

	// cancelledPreparationMessage is what a released waiter's row reads. It is
	// phrased like the sweep's release for a preparation that failed, because from
	// the waiter's side the two are the same event: the thing it was waiting for
	// ended without producing a file.
	cancelledPreparationMessage = "the preparation this download waited for was cancelled"
)

// RetryTask requeues a failed task within its attempt bound, and answers with
// the requeued body.
//
// Both guards are checked here *and* in the update: here so the caller gets
// `transfer_task_conflict` instead of a silent no-op, and in the update so a race
// between the two cannot requeue a task that has since succeeded.
//
// The third guard is the dependency pointer, and it is the same "no silent no-op"
// rule: a failed download that still points at its preparation is waiting on
// something that already ended, so the requeue would leave it `pending` and
// un-leasable — exactly the row it is now. See `retryTask` for the full argument;
// the remedy for these rows is a new download rather than a retry.
func (s *Service) RetryTask(actor identityservice.PublicUser, taskID string) (dto.Task, error) {
	task, team, err := s.scopedTask(actor, taskID)
	if err != nil {
		return dto.Task{}, err
	}
	if task.Status != model.StatusFailed || task.DependencyTaskID != "" || task.AttemptCount >= task.MaxAttempts {
		return dto.Task{}, ErrTaskConflict
	}
	requeued, retried, err := s.store.RetryTask(task.ID, team, actor.ID, s.now().UTC())
	if err != nil {
		return dto.Task{}, err
	}
	if !retried {
		return dto.Task{}, ErrTaskConflict
	}
	return taskBody(requeued), nil
}

// ClaimTask leases one task for the node a credential identifies, or answers
// with no task at all.
//
// The order is candidate, grant, claim — not claim, grant. A lease is a promise
// that a download is available, so the grant has to exist before the row says
// `running`: minting it afterwards would, whenever minting failed, leave a task
// recorded as running with nobody holding anything, and with an attempt already
// spent. Reading the candidate first costs one extra query and, when a competing
// poll wins the claim below, one wasted local signature. See `NextLocalTask`.
func (s *Service) ClaimTask(ctx context.Context, credential string) (dto.ClaimResult, error) {
	node, err := s.authenticate(credential)
	if err != nil {
		return dto.ClaimResult{}, err
	}
	now := s.now().UTC()
	candidate, found, err := s.store.NextLocalTask(node.ID, now)
	if err != nil {
		return dto.ClaimResult{}, err
	}
	if !found {
		return dto.ClaimResult{}, nil
	}
	grant, err := s.grants.PresignGet(ctx, candidate.SourceObjectKey)
	if err != nil {
		return dto.ClaimResult{}, err
	}
	if strings.TrimSpace(grant.URL) == "" {
		// A grant with no url is not a grant, and a lease carrying it would be
		// unusable in a way no executor could diagnose. Refusing here keeps that
		// failure on the side that can fix it, and leases nothing.
		return dto.ClaimResult{}, ErrGrantUnavailable
	}
	leased, claimed, err := s.store.ClaimLocalTask(candidate.ID, node.ID, now, s.lease)
	if err != nil {
		return dto.ClaimResult{}, err
	}
	if !claimed {
		// Another poll took it, or it stopped being leasable. Saying "no task" is
		// accurate, and the executor's next poll finds out what is next.
		return dto.ClaimResult{}, nil
	}
	return dto.ClaimResult{Task: leaseBody(leased, grant, s.lease)}, nil
}

// HeartbeatTask renews the lease a node holds and carries how far it has got.
func (s *Service) HeartbeatTask(credential, taskID string, completedBytes int64) error {
	node, err := s.authenticate(credential)
	if err != nil {
		return err
	}
	if strings.TrimSpace(taskID) == "" || completedBytes < 0 {
		return ErrInvalidInput
	}
	renewed, err := s.store.HeartbeatTask(taskID, node.ID, s.now().UTC(), s.lease)
	if err != nil {
		return err
	}
	if !renewed {
		return s.refusalReason(taskID)
	}
	return nil
}

// ReportProgress records how far a transfer has got, and derives the estimate of
// what is left.
//
// The frozen `Progress` body carries no total and no ETA — only the bytes done
// and the current rate — while the denominator is on the row, so the arithmetic
// happens here, where both are known, rather than being left to a UI that would
// have to infer the total from the largest byte count it had seen. The rate is
// the executor's own measurement: Cloud records what the machine doing the work
// reports instead of recomputing it from its own clock, which would disagree.
//
// The read and the write are two statements, so the total could in principle
// move between them. It cannot in practice: `total_bytes` is fixed when the task
// is created and is only ever filled in from zero.
func (s *Service) ReportProgress(credential, taskID string, completedBytes, bytesPerSecond int64) error {
	node, err := s.authenticate(credential)
	if err != nil {
		return err
	}
	if strings.TrimSpace(taskID) == "" || completedBytes < 0 || bytesPerSecond < 0 {
		return ErrInvalidInput
	}
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return missingOr(err)
	}
	recorded, err := s.store.ReportProgress(repository.ProgressInput{
		TaskID:           task.ID,
		NodeID:           node.ID,
		TransferredBytes: completedBytes,
		TotalBytes:       task.TotalBytes,
		SpeedBytesPerSec: bytesPerSecond,
		ETASeconds:       remainingSeconds(task.TotalBytes, completedBytes, bytesPerSecond),
	}, s.now().UTC())
	if err != nil {
		return err
	}
	if !recorded {
		return s.refusalReason(task.ID)
	}
	return nil
}

// CompleteTask records an executor's terminal report and echoes back what Cloud
// recorded.
//
// A success is checked against the task before it is written: the byte count
// must equal the declared total, and the hash must equal the hash the task was
// created with. The repository's completion statement enforces the same two
// things, so a race that slipped past this check still cannot mark unverified
// bytes as a success — the check exists so that the answer is a 422 naming the
// rule, rather than a silent refusal the executor cannot act on.
//
// A failure or cancellation is written through `FailTask`, not through the
// completion statement: that one sets `status = 'success'`, and the two terminal
// paths also differ in what they require of the caller (`sha256` against
// `error_code`).
func (s *Service) CompleteTask(credential, taskID string, body dto.CompletionRequest) (dto.TransferTerminal, error) {
	node, err := s.authenticate(credential)
	if err != nil {
		return dto.TransferTerminal{}, err
	}
	if strings.TrimSpace(taskID) == "" {
		return dto.TransferTerminal{}, ErrInvalidInput
	}
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return dto.TransferTerminal{}, missingOr(err)
	}
	if task.Status != model.StatusRunning || task.ClaimedByNodeID != node.ID {
		return dto.TransferTerminal{}, ErrTaskConflict
	}
	now := s.now().UTC()
	switch model.Status(body.Status) {
	case model.StatusSuccess:
		if err := s.recordSuccess(task, node.ID, body, now); err != nil {
			return dto.TransferTerminal{}, err
		}
	case model.StatusFailed, model.StatusCancelled:
		errorCode := strings.TrimSpace(body.ErrorCode)
		if errorCode == "" || len(errorCode) > 128 || len(body.ErrorMessage) > 500 {
			return dto.TransferTerminal{}, ErrInvalidInput
		}
		failed, err := s.store.FailTask(repository.FailureInput{
			TaskID:       task.ID,
			NodeID:       node.ID,
			Status:       model.Status(body.Status),
			ErrorCode:    errorCode,
			ErrorMessage: body.ErrorMessage,
		}, now)
		if err != nil {
			return dto.TransferTerminal{}, err
		}
		if !failed {
			return dto.TransferTerminal{}, ErrTaskConflict
		}
	default:
		return dto.TransferTerminal{}, ErrInvalidInput
	}
	refreshed, err := s.store.GetTask(task.ID)
	if err != nil {
		return dto.TransferTerminal{}, missingOr(err)
	}
	return dto.TransferTerminal{
		TaskID:         refreshed.ID,
		Status:         string(refreshed.Status),
		CompletedBytes: refreshed.TransferredBytes,
		FileName:       refreshed.FileName,
	}, nil
}

func (s *Service) recordSuccess(task model.Task, nodeID string, body dto.CompletionRequest, now time.Time) error {
	// The stored hash is lowercased so the comparison is one rule rather than one
	// per collation: the frozen pattern accepts either case, and MySQL's default
	// comparison would have accepted them, but the repository's predicate and this
	// check are two statements of the same rule and should not disagree.
	checksum := strings.ToLower(strings.TrimSpace(body.SHA256))
	fileName := strings.TrimSpace(body.FileName)
	if body.CompletedBytes < 0 || !isHexSHA256(checksum) {
		return ErrInvalidInput
	}
	if fileName != "" && !validCompletedFileName(fileName) {
		// Not a path, and not a name that would become one when an executor joined
		// it to a directory. The frozen contract states this as a pattern; refusing
		// it here is the same rule, enforceable before the row is written. An empty
		// name is the "no file reported yet" case and is not an error.
		return ErrInvalidInput
	}
	if task.TotalBytes > 0 && body.CompletedBytes != task.TotalBytes {
		return ErrIntegrityFailed
	}
	if task.ExpectedSHA256 != "" && checksum != strings.ToLower(task.ExpectedSHA256) {
		return ErrIntegrityFailed
	}
	completed, err := s.store.CompleteTask(repository.CompletionInput{
		TaskID:   task.ID,
		NodeID:   nodeID,
		Bytes:    body.CompletedBytes,
		SHA256:   checksum,
		FileName: fileName,
	}, now)
	if err != nil {
		return err
	}
	if !completed {
		return ErrTaskConflict
	}
	return nil
}

// authenticate answers "which node is calling", and folds the several ways that
// can fail into one refusal.
//
// Only the two credential sentinels are folded. Anything else the runtime-binding
// domain reports — a database that is down, for instance — is passed through, so
// it stays what it is: a fault of ours, not a rejected caller.
func (s *Service) authenticate(credential string) (runtimeservice.AgentNode, error) {
	credential = strings.TrimSpace(credential)
	if credential == "" {
		// There is nothing to look up, and the answer is not in doubt. Saying so
		// here keeps an unauthenticated request from costing a hash and a query
		// apiece.
		return runtimeservice.AgentNode{}, ErrNodeUnauthenticated
	}
	node, err := s.nodes.AuthenticateNodeCredential(credential)
	if err != nil {
		if errors.Is(err, runtimeservice.ErrNodeCredentialInvalid) || errors.Is(err, runtimeservice.ErrBoundSessionInvalid) {
			return runtimeservice.AgentNode{}, ErrNodeUnauthenticated
		}
		return runtimeservice.AgentNode{}, err
	}
	return node, nil
}

// scopedTask reads a task and decides whether this actor may act on it, also
// answering with the team the update statements are to be scoped by.
//
// The team returned is the actor's, not the row's. Handing the row's own value
// to a statement whose `WHERE` already names the team would make that clause a
// restatement of the row rather than a constraint; the actor's value makes it a
// real one, and a team that changed between the read and the write is then
// refused by the same statement rather than being papered over.
func (s *Service) scopedTask(actor identityservice.PublicUser, taskID string) (model.Task, identityservice.TeamID, error) {
	if strings.TrimSpace(taskID) == "" || actor.ID <= 0 {
		return model.Task{}, 0, ErrInvalidInput
	}
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return model.Task{}, 0, missingOr(err)
	}
	if actor.TeamID == nil || *actor.TeamID != task.TeamID {
		return model.Task{}, 0, ErrTaskNotFound
	}
	if task.RequestedBy != actor.ID {
		return model.Task{}, 0, ErrTaskForbidden
	}
	return task, *actor.TeamID, nil
}

// refreshedTask re-reads a task after a write, so the answer describes the row
// that now exists rather than the write that was attempted.
func (s *Service) refreshedTask(taskID string) (dto.Task, error) {
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return dto.Task{}, missingOr(err)
	}
	return taskBody(task), nil
}

// refusalReason explains a continuation call the repository refused.
//
// All four continuation statements answer with a bare row count, so the reason
// has to be read back. The distinction is the executor's, and it is not cosmetic:
// a task that is over — cancelled, or finished by someone else — means stop and
// write a terminal cancelled, while a lease that merely lapsed means the work is
// still wanted and the executor should re-claim it.
func (s *Service) refusalReason(taskID string) error {
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return missingOr(err)
	}
	if task.CancelRequestedAt != nil || (task.Status != model.StatusPending && task.Status != model.StatusRunning) {
		return ErrTaskCancelled
	}
	return ErrTaskConflict
}

func missingOr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrTaskNotFound
	}
	return err
}

// taskBody maps a task row onto the frozen session body.
//
// The four `*string` fields are null rather than empty when there is nothing to
// say. The frozen schema makes all four nullable, and an empty string would tell
// a reader that the transfer produced a file named "" — a different claim from
// "there is no file name yet".
func taskBody(task model.Task) dto.Task {
	return dto.Task{
		ID:                        task.ID,
		AssetType:                 string(task.AssetType),
		AssetID:                   task.AssetID,
		AssetTitle:                task.AssetTitle,
		Purpose:                   string(task.Purpose),
		ExecutionScope:            string(task.ExecutionScope),
		Status:                    string(task.Status),
		TotalBytes:                task.TotalBytes,
		CompletedBytes:            task.TransferredBytes,
		BytesPerSecond:            task.SpeedBytesPerSec,
		EstimatedRemainingSeconds: task.ETASeconds,
		AttemptCount:              task.AttemptCount,
		MaxAttempts:               task.MaxAttempts,
		ChecksumSHA256:            optional(task.IntegritySHA256),
		FileName:                  optional(task.FileName),
		ErrorCode:                 optional(task.ErrorCode),
		ErrorMessage:              optional(task.ErrorMessage),
		CreatedAt:                 task.CreatedAt,
		UpdatedAt:                 task.UpdatedAt,
	}
}

func leaseBody(task model.Task, grant DownloadGrant, lease time.Duration) *dto.LocalLease {
	return &dto.LocalLease{
		TaskID:               task.ID,
		AssetType:            string(task.AssetType),
		AssetID:              task.AssetID,
		Title:                task.AssetTitle,
		GameName:             task.GameName,
		PublishedAt:          task.PublishedAt,
		TotalBytes:           task.TotalBytes,
		ExpectedSHA256:       task.ExpectedSHA256,
		MaxAttempts:          task.MaxAttempts,
		AttemptCount:         task.AttemptCount,
		LeaseSeconds:         int(lease / time.Second),
		DownloadURL:          grant.URL,
		DownloadURLExpiresAt: grant.ExpiresAt,
	}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// remainingSeconds is the estimate, or zero when there is not one to give.
//
// A zero rate and an unknown total are both "not known yet", and a transfer that
// has already reached its total has nothing left. All three answer zero, which is
// the same value the repository's columns treat as absent, so a reader has one
// rule: `total_bytes == 0` means the size is unknown and no percentage can be
// shown.
func remainingSeconds(total, completed, speed int64) int64 {
	if total <= 0 || speed <= 0 || completed >= total {
		return 0
	}
	return (total - completed) / speed
}

// validCompletedFileName applies the frozen `^[^/\\]+(?:/[^/\\]+)?$` pattern the
// contract declares for `Completion.file_name`: at most one relative directory
// component (the download date) plus a bare file name, neither segment empty, no
// backslash, and within the 255-byte cap. An absolute path, two separators, or a
// trailing slash all fail, exactly as the pattern fails them.
//
// The runtime and the pattern must not disagree: this check used to reject any
// `/` at all, which refused the one-subdirectory name the pattern admits, and
// every date-filed download came back as a `400` the executor could not act on.
func validCompletedFileName(name string) bool {
	if name == "" || len(name) > maxFileNameLength {
		return false
	}
	if strings.ContainsRune(name, '\\') {
		return false
	}
	slash := strings.IndexByte(name, '/')
	if slash < 0 {
		return true
	}
	return slash > 0 && slash < len(name)-1 && strings.IndexByte(name[slash+1:], '/') < 0
}

// isHexSHA256 applies the frozen `^[A-Fa-f0-9]{64}$` pattern.
func isHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9', ch >= 'a' && ch <= 'f':
		default:
			return false
		}
	}
	return true
}
