// Package profileguard 为敏感 Profile 任务发放互斥许可。
//
// 许可只是同一 Profile 上的互斥锁，不是凭证：调用方由 Agent Token 认证，许可
// 按 Token 用户归属，没有节点身份，也没有第二个需要出示的秘密。
package service

import (
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/executionpreflight"
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
	LoadCloudFacts(userID int64, profileID, mediaAccountID string) (executionpreflight.CloudFacts, error)
	AcquirePermit(task SensitiveTask, permit Permit, at time.Time) (PreflightOutcome, error)
	RenewPermit(permitID string, userID identityservice.UserID, at, expiresAt time.Time) (time.Time, error)
	FinishPermit(permitID string, userID identityservice.UserID, outcome FinishOutcome, at time.Time) error
}

type PreflightInput struct {
	AgentFacts          executionpreflight.AgentFacts
	DesktopSessionValid bool
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

// Preflight 先走统一准入检查，再取 Profile 互斥锁。别人的任务返回 not-found，
// 端点不能用来探测任务 id。
func (s *Service) Preflight(userID identityservice.UserID, taskID string, input PreflightInput) (PreflightOutcome, error) {
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
	action, ok := executionpreflight.ActionForSensitiveOperation(string(task.Operation))
	if !ok {
		return PreflightOutcome{}, ErrTaskAssignmentMismatch
	}
	cloudFacts, err := s.store.LoadCloudFacts(int64(userID), task.ProfileID, "")
	if err != nil {
		return PreflightOutcome{}, err
	}
	cloudFacts.DesktopSessionValid = input.DesktopSessionValid
	if failure := executionpreflight.Evaluate(executionpreflight.Request{
		Mode:             executionpreflight.ModeNewOperation,
		Action:           action,
		UserID:           int64(userID),
		ProfileID:        task.ProfileID,
		Cloud:            cloudFacts,
		Agent:            input.AgentFacts,
		Now:              s.now(),
		ObservedFreshFor: executionpreflight.ObservedFreshFor(),
	}); failure != nil {
		return PreflightOutcome{}, failure
	}
	now := s.now()
	permit := Permit{
		ID: s.newID("profile-permit"), TaskID: task.ID, UserID: task.UserID, ProfileID: task.ProfileID,
		Operation: task.Operation, Status: PermitActive,
		AcquiredAt: now, ExpiresAt: now.Add(s.permitTTL),
	}
	return s.store.AcquirePermit(task, permit, now)
}

// Renew 只延长同一用户持有的许可；知道 permit id 本身不足以跨用户使用。
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
