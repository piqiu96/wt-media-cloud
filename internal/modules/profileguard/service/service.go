// Package profileguard owns Cloud preflight permits for sensitive Profile tasks.
//
// A permit is a mutual-exclusion lock, not a credential. It guarantees that one
// sensitive operation at a time runs against a Profile; the caller is
// authenticated by its Agent Token (strict for a new preflight, report-safe for
// renew/finish), and the permit is scoped to that token's user. There is no
// node identity and no second secret to present.
package service

import (
	"strings"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	SensitiveOperation = model.SensitiveOperation
	TaskStatus         = model.TaskStatus
	Outcome            = model.Outcome
	PermitStatus       = model.PermitStatus
	FinishOutcome      = model.FinishOutcome
	SensitiveTask      = model.SensitiveTask
	Permit             = model.Permit
	PreflightOutcome   = dto.PreflightOutcome
)

var (
	ErrInvalidInput           = model.ErrInvalidInput
	ErrTaskNotFound           = model.ErrTaskNotFound
	ErrTaskAssignmentMismatch = model.ErrTaskAssignmentMismatch
	ErrRuntimeUnavailable     = model.ErrRuntimeUnavailable
	ErrPermitInvalid          = model.ErrPermitInvalid
)

const (
	OperationAssistedPublication       = model.OperationAssistedPublication
	OperationInteraction               = model.OperationInteraction
	OperationAuthenticatedAccountCheck = model.OperationAuthenticatedAccountCheck
	OperationCookieRead                = model.OperationCookieRead
	OperationCookieWrite               = model.OperationCookieWrite
	OperationProfileMutation           = model.OperationProfileMutation
	OperationProxyMutation             = model.OperationProxyMutation

	TaskAuthorized     = model.TaskAuthorized
	TaskRunning        = model.TaskRunning
	TaskCompleted      = model.TaskCompleted
	TaskReviewRequired = model.TaskReviewRequired

	OutcomeGranted        = model.OutcomeGranted
	OutcomeWaiting        = model.OutcomeWaiting
	OutcomeReviewRequired = model.OutcomeReviewRequired

	PermitActive         = model.PermitActive
	PermitReleased       = model.PermitReleased
	PermitReviewRequired = model.PermitReviewRequired

	FinishCompleted       = model.FinishCompleted
	FinishResultUncertain = model.FinishResultUncertain
)

type Store interface {
	FindAuthorizedTask(taskID string) (SensitiveTask, bool, error)
	AcquirePermit(task SensitiveTask, permit Permit, at time.Time) (PreflightOutcome, error)
	RenewPermit(permitID string, userID identityservice.UserID, at, expiresAt time.Time) (time.Time, error)
	FinishPermit(permitID string, userID identityservice.UserID, outcome FinishOutcome, at time.Time) error
}

type Service struct {
	store      Store
	now        func() time.Time
	newID      func(string) string
	permitTTL  time.Duration
	maxRenewal time.Duration
}

type Option func(*Service)

func WithClock(now func() time.Time) Option            { return func(s *Service) { s.now = now } }
func WithIDGenerator(newID func(string) string) Option { return func(s *Service) { s.newID = newID } }

func newService(store Store, options ...Option) *Service {
	service := &Service{
		store: store, now: func() time.Time { return time.Now().UTC() }, newID: id.NewID,
		permitTTL: 2 * time.Minute, maxRenewal: 2 * time.Minute,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

// Preflight takes the per-Profile lock for a task belonging to the token's user.
// A task belonging to somebody else answers not-found, so the endpoint cannot be
// used to probe which task ids exist.
func (s *Service) Preflight(userID identityservice.UserID, taskID string) (PreflightOutcome, error) {
	if userID <= 0 || strings.TrimSpace(taskID) == "" {
		return PreflightOutcome{}, ErrInvalidInput
	}
	task, found, err := s.store.FindAuthorizedTask(taskID)
	if err != nil {
		return PreflightOutcome{}, err
	}
	if !found || task.Status != TaskAuthorized || task.UserID != userID {
		return PreflightOutcome{}, ErrTaskNotFound
	}
	if !validOperation(task.Operation) {
		return PreflightOutcome{}, ErrTaskAssignmentMismatch
	}
	now := s.now()
	permit := Permit{
		ID: s.newID("profile-permit"), TaskID: task.ID, UserID: task.UserID, ProfileID: task.ProfileID,
		Operation: task.Operation, Status: PermitActive,
		AcquiredAt: now, ExpiresAt: now.Add(s.permitTTL),
	}
	return s.store.AcquirePermit(task, permit, now)
}

// Renew extends a permit the same user holds. The user check is what keeps a
// permit id from acting as a cross-user bearer: knowing the id is not enough.
func (s *Service) Renew(userID identityservice.UserID, permitID string, extension time.Duration) (time.Time, error) {
	if userID <= 0 || strings.TrimSpace(permitID) == "" {
		return time.Time{}, ErrInvalidInput
	}
	if extension <= 0 || extension > s.maxRenewal {
		extension = s.maxRenewal
	}
	now := s.now()
	return s.store.RenewPermit(permitID, userID, now, now.Add(extension))
}

func (s *Service) Finish(userID identityservice.UserID, permitID string, outcome FinishOutcome) error {
	if userID <= 0 || strings.TrimSpace(permitID) == "" ||
		(outcome != FinishCompleted && outcome != FinishResultUncertain) {
		return ErrInvalidInput
	}
	return s.store.FinishPermit(permitID, userID, outcome, s.now())
}

func validOperation(operation SensitiveOperation) bool {
	switch operation {
	case OperationAssistedPublication, OperationInteraction, OperationAuthenticatedAccountCheck,
		OperationCookieRead, OperationCookieWrite, OperationProfileMutation, OperationProxyMutation:
		return true
	default:
		return false
	}
}
