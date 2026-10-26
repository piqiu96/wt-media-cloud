// Package model owns Cloud-Agent domain and contract types.
package model

import "errors"

var (
	ErrTaskNotFound        = errors.New("task not found")
	ErrNoPendingTask       = errors.New("no pending task")
	ErrTaskLeaseTaken      = errors.New("task lease taken")
	ErrInvalidTask         = errors.New("invalid task")
	ErrTaskAgentMismatch   = errors.New("task agent mismatch")
	ErrTaskAlreadyTerminal = errors.New("task already in terminal state")
	ErrInvalidStatus       = errors.New("invalid task status")
	ErrInvalidTaskType     = errors.New("invalid task type")
	ErrUnsupportedTaskType = errors.New("unsupported task type")
	ErrTaskNotRetryable    = errors.New("task is not retryable")
	ErrStaleAttempt        = errors.New("task attempt is stale")
	ErrResultUnknownRetry  = errors.New("retrying an unknown result needs confirmation")
)

type Compatibility struct {
	API                          string   `json:"api"`
	MajorVersion                 string   `json:"major_version"`
	ContractRevision             string   `json:"contract_revision"`
	MinimumAgentContractRevision string   `json:"minimum_agent_contract_revision"`
	CompatibleAgentMajorVersions []string `json:"compatible_agent_major_versions"`
	Status                       string   `json:"status"`
}

type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusLeased
	TaskStatusRunning
	TaskStatusSucceeded
	TaskStatusFailed
	TaskStatusCancelled
)

func (s TaskStatus) String() string {
	switch s {
	case TaskStatusPending:
		return "pending"
	case TaskStatusLeased:
		return "leased"
	case TaskStatusRunning:
		return "running"
	case TaskStatusSucceeded:
		return "succeeded"
	case TaskStatusFailed:
		return "failed"
	case TaskStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

type TaskType int

const (
	TaskTypeNoop          TaskType = iota // 0: built-in verification task
	TaskTypeCookieRead                    // 1: read cookies from BitBrowser Profile
	TaskTypeCookieWrite                   // 2: write cookies to BitBrowser Profile
	TaskTypeAccountCheck                  // 3: check platform login status
	TaskTypeProfileCreate                 // 4: create BitBrowser Profile
	TaskTypeProfileOpen                   // 5: open BitBrowser Profile
	TaskTypeProfileClose                  // 6: close BitBrowser Profile
	TaskTypeProfileUpdate                 // 7: update BitBrowser Profile
	TaskTypeProxyCheck                    // 8: verify proxy through Agent
	TaskTypeProxyMutation                 // 9: assign proxy to BitBrowser Profile
)

// TaskTypeSpec 是任务类型的静默超时与外部副作用；副作用决定中断后能否重试。
type TaskTypeSpec struct {
	TimeoutSeconds     int
	ExternalSideEffect bool
}

func (t TaskType) Spec() TaskTypeSpec {
	switch t {
	case TaskTypeProfileOpen, TaskTypeProfileClose:
		return TaskTypeSpec{TimeoutSeconds: 120, ExternalSideEffect: false}
	case TaskTypeCookieWrite, TaskTypeProfileCreate, TaskTypeProfileUpdate, TaskTypeProxyMutation:
		return TaskTypeSpec{TimeoutSeconds: 300, ExternalSideEffect: true}
	default:
		return TaskTypeSpec{TimeoutSeconds: 300, ExternalSideEffect: false}
	}
}

// ParseTaskTypeValue 只接受 `*_task` 拼写；空值或未知类型返回 false，不降级为 noop。
func ParseTaskTypeValue(value string) (TaskType, bool) {
	switch value {
	case TaskTypeNoop.String():
		return TaskTypeNoop, true
	case TaskTypeCookieRead.String():
		return TaskTypeCookieRead, true
	case TaskTypeCookieWrite.String():
		return TaskTypeCookieWrite, true
	case TaskTypeAccountCheck.String():
		return TaskTypeAccountCheck, true
	case TaskTypeProfileCreate.String():
		return TaskTypeProfileCreate, true
	case TaskTypeProfileOpen.String():
		return TaskTypeProfileOpen, true
	case TaskTypeProfileClose.String():
		return TaskTypeProfileClose, true
	case TaskTypeProfileUpdate.String():
		return TaskTypeProfileUpdate, true
	case TaskTypeProxyCheck.String():
		return TaskTypeProxyCheck, true
	case TaskTypeProxyMutation.String():
		return TaskTypeProxyMutation, true
	default:
		return TaskTypeNoop, false
	}
}

func (t TaskType) String() string {
	switch t {
	case TaskTypeNoop:
		return "noop_task"
	case TaskTypeCookieRead:
		return "cookie_read_task"
	case TaskTypeCookieWrite:
		return "cookie_write_task"
	case TaskTypeAccountCheck:
		return "account_check_task"
	case TaskTypeProfileCreate:
		return "profile_create_task"
	case TaskTypeProfileOpen:
		return "profile_open_task"
	case TaskTypeProfileClose:
		return "profile_close_task"
	case TaskTypeProfileUpdate:
		return "profile_update_task"
	case TaskTypeProxyCheck:
		return "proxy_check_task"
	case TaskTypeProxyMutation:
		return "proxy_mutation_task"
	default:
		return "unknown"
	}
}

type Task struct {
	TaskID                  string         `json:"task_id"`
	TaskType                string         `json:"task_type"`
	Status                  string         `json:"status"`
	UserID                  int64          `json:"user_id,omitempty"`
	Attempt                 int            `json:"attempt"`
	AttemptID               string         `json:"attempt_id,omitempty"`
	IdempotencyKey          string         `json:"idempotency_key,omitempty"`
	AgentID                 string         `json:"agent_id,omitempty"`
	CreatedAt               string         `json:"created_at"`
	StartedAt               string         `json:"started_at,omitempty"`
	LastReportAt            string         `json:"last_report_at,omitempty"`
	FinishedAt              string         `json:"finished_at,omitempty"`
	LeaseExpiresAt          string         `json:"lease_expires_at,omitempty"`
	Progress                int            `json:"progress,omitempty"`
	Message                 string         `json:"message,omitempty"`
	UpdatedAt               string         `json:"updated_at,omitempty"`
	ErrorCode               string         `json:"error_code,omitempty"`
	ErrorMessage            string         `json:"error_message,omitempty"`
	ExecutionTimeoutSeconds int            `json:"execution_timeout_seconds,omitempty"`
	ExternalSideEffect      bool           `json:"external_side_effect"`
	Retryable               bool           `json:"retryable"`
	RecoveryAction          string         `json:"recovery_action,omitempty"`
	SettledBy               string         `json:"settled_by,omitempty"`
	Payload                 map[string]any `json:"payload,omitempty"`
	Result                  map[string]any `json:"result,omitempty"`
}
