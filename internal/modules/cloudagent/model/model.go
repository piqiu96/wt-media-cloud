// Package model owns Cloud-Agent domain and contract types.
package model

import "errors"

var (
	ErrAgentNotFound       = errors.New("agent not found")
	ErrInvalidAgent        = errors.New("invalid agent")
	ErrIncompatibleAgent   = errors.New("incompatible agent contract")
	ErrTaskNotFound        = errors.New("task not found")
	ErrNoPendingTask       = errors.New("no pending task")
	ErrTaskLeaseTaken      = errors.New("task lease taken")
	ErrInvalidTask         = errors.New("invalid task")
	ErrTaskAgentMismatch   = errors.New("task agent mismatch")
	ErrTaskAlreadyTerminal = errors.New("task already in terminal state")
	ErrInvalidStatus       = errors.New("invalid task status")
	ErrInvalidTaskType     = errors.New("invalid task type")
	ErrTaskNotRetryable    = errors.New("task is not retryable")
)

type Compatibility struct {
	API                          string   `json:"api"`
	MajorVersion                 string   `json:"major_version"`
	ContractRevision             string   `json:"contract_revision"`
	MinimumAgentContractRevision string   `json:"minimum_agent_contract_revision"`
	CompatibleAgentMajorVersions []string `json:"compatible_agent_major_versions"`
	Status                       string   `json:"status"`
}

type AgentNode struct {
	AgentID              string   `json:"agent_id"`
	Mode                 string   `json:"mode"`
	Version              string   `json:"version"`
	ContractMajorVersion string   `json:"contract_major_version"`
	ContractRevision     string   `json:"contract_revision"`
	Status               string   `json:"status"`
	RegisteredAt         string   `json:"registered_at"`
	LastHeartbeatAt      string   `json:"last_heartbeat_at"`
	Capabilities         []string `json:"capabilities"`
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
	TaskID         string         `json:"task_id"`
	TaskType       string         `json:"task_type"`
	Status         string         `json:"status"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	CreatedAt      string         `json:"created_at"`
	LeaseExpiresAt string         `json:"lease_expires_at,omitempty"`
	Progress       int            `json:"progress,omitempty"`
	Message        string         `json:"message,omitempty"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
	ErrorCode      string         `json:"error_code,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
}
