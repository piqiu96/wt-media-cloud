// Package profileguard owns Cloud preflight permits for sensitive Profile tasks.
package profileguard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type SensitiveOperation string

const (
	OperationAssistedPublication       SensitiveOperation = "assisted_publication"
	OperationInteraction               SensitiveOperation = "interaction"
	OperationAuthenticatedAccountCheck SensitiveOperation = "authenticated_account_check"
	OperationCookieRead                SensitiveOperation = "cookie_read"
	OperationCookieWrite               SensitiveOperation = "cookie_write"
	OperationProfileMutation           SensitiveOperation = "profile_mutation"
	OperationProxyMutation             SensitiveOperation = "proxy_mutation"
)

type TaskStatus string

const (
	TaskAuthorized     TaskStatus = "authorized"
	TaskRunning        TaskStatus = "running"
	TaskCompleted      TaskStatus = "completed"
	TaskReviewRequired TaskStatus = "review_required"
)

type Outcome string

const (
	OutcomeGranted        Outcome = "granted"
	OutcomeWaiting        Outcome = "waiting"
	OutcomeReviewRequired Outcome = "review_required"
)

type PermitStatus string

const (
	PermitActive         PermitStatus = "active"
	PermitReleased       PermitStatus = "released"
	PermitReviewRequired PermitStatus = "review_required"
)

type FinishOutcome string

const (
	FinishCompleted       FinishOutcome = "completed"
	FinishResultUncertain FinishOutcome = "result_uncertain"
)

var (
	ErrInvalidInput            = errors.New("sensitive task guard input is invalid")
	ErrTaskNotFound            = errors.New("sensitive task authorization was not found")
	ErrTaskAssignmentMismatch  = errors.New("sensitive task is not assigned to this node and user")
	ErrRuntimeUnavailable      = errors.New("fresh matching Profile runtime is unavailable")
	ErrPermitCredentialInvalid = errors.New("sensitive permit credential is invalid")
)

type SensitiveTask struct {
	ID           string             `json:"id"`
	UserID       identity.UserID    `json:"user_id"`
	ProfileID    string             `json:"profile_id"`
	BitProfileID string             `json:"bit_profile_id"`
	NodeID       string             `json:"node_id"`
	Operation    SensitiveOperation `json:"operation"`
	Status       TaskStatus         `json:"status"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type Permit struct {
	ID             string
	TaskID         string
	UserID         identity.UserID
	ProfileID      string
	NodeID         string
	Operation      SensitiveOperation
	Status         PermitStatus
	CredentialHash string
	AcquiredAt     time.Time
	ExpiresAt      time.Time
	FinishedAt     *time.Time
}

type PreflightOutcome struct {
	Outcome          Outcome    `json:"outcome"`
	PermitID         string     `json:"permit_id,omitempty"`
	PermitCredential string     `json:"permit_credential,omitempty"`
	ProfileID        string     `json:"profile_id"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

type NodeAuthenticator interface {
	AuthenticateNode(nodeID, credential string) (runtimebinding.AgentNode, error)
}

type Store interface {
	FindAuthorizedTask(taskID string) (SensitiveTask, bool, error)
	AcquirePermit(task SensitiveTask, node runtimebinding.AgentNode, permit Permit, at time.Time, freshness time.Duration) (PreflightOutcome, error)
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

func NewService(store Store, nodes NodeAuthenticator, options ...Option) *Service {
	service := &Service{
		store: store, nodes: nodes, now: func() time.Time { return time.Now().UTC() }, newID: common.NewID,
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
	outcome, err := s.store.AcquirePermit(task, node, permit, now, s.freshness)
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
