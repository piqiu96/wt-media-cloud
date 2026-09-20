// Package profileguard owns Cloud preflight permits for sensitive Profile tasks.
package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
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
	ErrInvalidInput            = model.ErrInvalidInput
	ErrTaskNotFound            = model.ErrTaskNotFound
	ErrTaskAssignmentMismatch  = model.ErrTaskAssignmentMismatch
	ErrRuntimeUnavailable      = model.ErrRuntimeUnavailable
	ErrPermitCredentialInvalid = model.ErrPermitCredentialInvalid
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

type NodeAuthenticator interface {
	AuthenticateNode(nodeID, credential string) (runtimeservice.AgentNode, error)
}

type Store interface {
	FindAuthorizedTask(taskID string) (SensitiveTask, bool, error)
	AcquirePermit(task SensitiveTask, nodeID string, permit Permit, at time.Time, freshness time.Duration) (PreflightOutcome, error)
	RenewPermit(permitID, nodeID, credentialHash string, at, expiresAt time.Time) (time.Time, error)
	FinishPermit(permitID, nodeID, credentialHash string, outcome FinishOutcome, at time.Time) error
}

type Service struct {
	store      Store
	nodes      NodeAuthenticator
	now        func() time.Time
	newID      func(string) string
	newSecret  func() string
	permitTTL  time.Duration
	freshness  time.Duration
	maxRenewal time.Duration
}

type Option func(*Service)

func WithClock(now func() time.Time) Option            { return func(s *Service) { s.now = now } }
func WithIDGenerator(newID func(string) string) Option { return func(s *Service) { s.newID = newID } }
func WithSecretGenerator(newSecret func() string) Option {
	return func(s *Service) { s.newSecret = newSecret }
}

func newService(store Store, nodes NodeAuthenticator, options ...Option) *Service {
	service := &Service{
		store: store, nodes: nodes, now: func() time.Time { return time.Now().UTC() }, newID: id.NewID,
		newSecret: randomSecret, permitTTL: 2 * time.Minute, freshness: 90 * time.Second, maxRenewal: 2 * time.Minute,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) Preflight(nodeID, nodeCredential, taskID string) (PreflightOutcome, error) {
	if strings.TrimSpace(taskID) == "" {
		return PreflightOutcome{}, ErrInvalidInput
	}
	node, err := s.nodes.AuthenticateNode(nodeID, nodeCredential)
	if err != nil {
		return PreflightOutcome{}, err
	}
	task, found, err := s.store.FindAuthorizedTask(taskID)
	if err != nil {
		return PreflightOutcome{}, err
	}
	if !found || task.Status != TaskAuthorized {
		return PreflightOutcome{}, ErrTaskNotFound
	}
	if task.NodeID != node.ID || task.UserID != node.UserID || !validOperation(task.Operation) {
		return PreflightOutcome{}, ErrTaskAssignmentMismatch
	}
	now := s.now()
	secret := s.newSecret()
	permit := Permit{
		ID: s.newID("profile-permit"), TaskID: task.ID, UserID: task.UserID, ProfileID: task.ProfileID,
		NodeID: node.ID, Operation: task.Operation, Status: PermitActive, CredentialHash: secretHash(secret),
		AcquiredAt: now, ExpiresAt: now.Add(s.permitTTL),
	}
	outcome, err := s.store.AcquirePermit(task, node.ID, permit, now, s.freshness)
	if err != nil {
		return PreflightOutcome{}, err
	}
	if outcome.Outcome == OutcomeGranted {
		outcome.PermitCredential = secret
	}
	return outcome, nil
}

func (s *Service) Renew(nodeID, nodeCredential, permitID, permitCredential string, extension time.Duration) (time.Time, error) {
	node, err := s.nodes.AuthenticateNode(nodeID, nodeCredential)
	if err != nil {
		return time.Time{}, err
	}
	if strings.TrimSpace(permitID) == "" || strings.TrimSpace(permitCredential) == "" {
		return time.Time{}, ErrInvalidInput
	}
	if extension <= 0 || extension > s.maxRenewal {
		extension = s.maxRenewal
	}
	now := s.now()
	return s.store.RenewPermit(permitID, node.ID, secretHash(permitCredential), now, now.Add(extension))
}

func (s *Service) Finish(nodeID, nodeCredential, permitID, permitCredential string, outcome FinishOutcome) error {
	node, err := s.nodes.AuthenticateNode(nodeID, nodeCredential)
	if err != nil {
		return err
	}
	if strings.TrimSpace(permitID) == "" || strings.TrimSpace(permitCredential) == "" ||
		(outcome != FinishCompleted && outcome != FinishResultUncertain) {
		return ErrInvalidInput
	}
	return s.store.FinishPermit(permitID, node.ID, secretHash(permitCredential), outcome, s.now())
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

func secretHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomSecret() string {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
