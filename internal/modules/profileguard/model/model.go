// Package model owns Profile Guard domain types.
package model

import (
	"errors"
	"time"

	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

var (
	ErrInvalidInput            = errors.New("sensitive task guard input is invalid")
	ErrTaskNotFound            = errors.New("sensitive task authorization was not found")
	ErrTaskAssignmentMismatch  = errors.New("sensitive task is not assigned to this node and user")
	ErrRuntimeUnavailable      = errors.New("fresh matching Profile runtime is unavailable")
	ErrPermitCredentialInvalid = errors.New("sensitive permit credential is invalid")
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

type SensitiveTask struct {
	ID           string               `json:"id"`
	UserID       identitymodel.UserID `json:"user_id"`
	ProfileID    string               `json:"profile_id"`
	BitProfileID string               `json:"bit_profile_id"`
	NodeID       string               `json:"node_id"`
	Operation    SensitiveOperation   `json:"operation"`
	Status       TaskStatus           `json:"status"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

type Permit struct {
	ID             string
	TaskID         string
	UserID         identitymodel.UserID
	ProfileID      string
	NodeID         string
	Operation      SensitiveOperation
	Status         PermitStatus
	CredentialHash string
	AcquiredAt     time.Time
	ExpiresAt      time.Time
	FinishedAt     *time.Time
}
