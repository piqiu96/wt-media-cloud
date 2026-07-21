package cloudagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
)

// TaskStatus represents the formal state machine for task lifecycle.
type TaskStatus int

const (
	TaskStatusPending   TaskStatus = iota // 0: awaiting claim
	TaskStatusLeased                      // 1: claimed by an agent, lease active
	TaskStatusRunning                     // 2: agent reported execution started
	TaskStatusSucceeded                   // 3: agent reported success
	TaskStatusFailed                      // 4: agent reported failure
	TaskStatusCancelled                   // 5: cancelled by operator or system
)

// String returns the JSON-safe string representation.
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
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// ParseTaskStatus converts a string to TaskStatus.
func ParseTaskStatus(s string) (TaskStatus, bool) {
	switch s {
	case "pending":
		return TaskStatusPending, true
	case "leased":
		return TaskStatusLeased, true
	case "running":
		return TaskStatusRunning, true
	case "succeeded":
		return TaskStatusSucceeded, true
	case "failed":
		return TaskStatusFailed, true
	case "cancelled":
		return TaskStatusCancelled, true
	default:
		return TaskStatusPending, false
	}
}

// ValidReportStatus returns true if status is a valid report target.
func ValidReportStatus(s TaskStatus) bool {
	return s == TaskStatusRunning || s == TaskStatusSucceeded || s == TaskStatusFailed
}

// IsTerminal returns true if the task has reached a final state.
func IsTerminal(s TaskStatus) bool {
	return s == TaskStatusSucceeded || s == TaskStatusFailed || s == TaskStatusCancelled
}

// TaskType represents the formal task type classification.
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
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

func ParseTaskType(s string) (TaskType, bool) {
	switch s {
	case "noop_task":
		return TaskTypeNoop, true
	case "cookie_read_task":
		return TaskTypeCookieRead, true
	case "cookie_write_task":
		return TaskTypeCookieWrite, true
	case "account_check_task":
		return TaskTypeAccountCheck, true
	case "profile_create_task":
		return TaskTypeProfileCreate, true
	case "profile_open_task":
		return TaskTypeProfileOpen, true
	case "profile_close_task":
		return TaskTypeProfileClose, true
	case "profile_update_task":
		return TaskTypeProfileUpdate, true
	case "proxy_check_task":
		return TaskTypeProxyCheck, true
	default:
		return TaskTypeNoop, false
	}
}

const (
	DefaultLeaseS = 60
)

var (
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

// CreateTaskRequest is used to create a new task.
type CreateTaskRequest struct {
	TaskType       string         `json:"task_type"`
	IdempotencyKey string         `json:"idempotency_key"`
	Payload        map[string]any `json:"payload,omitempty"`
}

// ClaimTaskRequest is used by an agent to claim a pending task.
type ClaimTaskRequest struct {
	AgentID      string `json:"agent_id"`
	LeaseSeconds int    `json:"lease_seconds"`
}

// Task is the formal universal task model.
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

// ReportTaskRequest is used by an agent to report task progress or result.
type ReportTaskRequest struct {
	AgentID   string         `json:"agent_id"`
	Status    string         `json:"status"`
	Progress  int            `json:"progress"`
	Message   string         `json:"message"`
	ErrorCode string         `json:"error_code,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
}

// CancelTaskRequest is used to cancel a pending or running task.
type CancelTaskRequest struct {
	Message string `json:"message"`
}

// TaskStore provides thread-safe in-memory task storage.
// Persistence will be added in M1-R2.
type TaskStore struct {
	mu          sync.Mutex
	now         func() time.Time
	newID       func() string
	tasks       map[string]Task
	idempotency map[string]string
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		now:         func() time.Time { return time.Now().UTC() },
		newID:       func() string { return common.NewID("task") },
		tasks:       make(map[string]Task),
		idempotency: make(map[string]string),
	}
}

func NewTaskStoreWithClock(now func() time.Time, newID func() string) *TaskStore {
	store := NewTaskStore()
	store.now = now
	store.newID = newID
	return store
}

// Create adds a new task. TaskType defaults to noop_task if empty.
func (s *TaskStore) Create(req CreateTaskRequest) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskType := req.TaskType
	if taskType == "" {
		taskType = TaskTypeNoop.String()
	}
	if _, ok := ParseTaskType(taskType); !ok {
		taskType = TaskTypeNoop.String()
	}

	if req.IdempotencyKey != "" {
		if id, ok := s.idempotency[req.IdempotencyKey]; ok {
			return s.tasks[id]
		}
	}
	task := Task{
		TaskID:         s.newID(),
		TaskType:       taskType,
		Status:         TaskStatusPending.String(),
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      s.now().Format(time.RFC3339),
		Payload:        clonePayload(req.Payload),
	}
	s.tasks[task.TaskID] = task
	if req.IdempotencyKey != "" {
		s.idempotency[req.IdempotencyKey] = task.TaskID
	}
	return task
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	var copied map[string]any
	if json.Unmarshal(raw, &copied) != nil {
		return nil
	}
	return copied
}

// Claim attempts to claim a pending task for the given agent.
func (s *TaskStore) Claim(req ClaimTaskRequest) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.AgentID == "" {
		return Task{}, ErrInvalidTask
	}
	leaseSeconds := req.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = DefaultLeaseS
	}

	now := s.now()
	// Re-claim agent's own active lease.
	for _, task := range s.tasks {
		if task.Status == TaskStatusLeased.String() && task.AgentID == req.AgentID && !leaseExpired(task, now) {
			return task, nil
		}
	}
	// Claim a pending or expired task.
	for _, task := range s.tasks {
		st, ok := ParseTaskStatus(task.Status)
		if !ok || IsTerminal(st) {
			continue
		}
		if task.Status == TaskStatusPending.String() || (task.Status == TaskStatusLeased.String() && leaseExpired(task, now)) || task.Status == TaskStatusCancelled.String() {
			task.Status = TaskStatusLeased.String()
			task.AgentID = req.AgentID
			task.LeaseExpiresAt = now.Add(time.Duration(leaseSeconds) * time.Second).Format(time.RFC3339)
			s.tasks[task.TaskID] = task
			return task, nil
		}
	}
	return Task{}, ErrNoPendingTask
}

// Get retrieves a task by ID.
func (s *TaskStore) Get(taskID string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	return task, nil
}

func (s *TaskStore) Retry(taskID string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	original, ok := s.tasks[taskID]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	if original.Status != TaskStatusFailed.String() && original.Status != TaskStatusCancelled.String() {
		return Task{}, ErrTaskNotRetryable
	}
	retryID := s.newID()
	retry := Task{TaskID: retryID, TaskType: original.TaskType, Status: TaskStatusPending.String(), IdempotencyKey: original.IdempotencyKey + ":retry:" + retryID, CreatedAt: s.now().Format(time.RFC3339), Payload: clonePayload(original.Payload)}
	s.tasks[retryID] = retry
	return retry, nil
}

// Report updates task status from an agent. Validates state transitions.
func (s *TaskStore) Report(taskID string, req ReportTaskRequest) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.AgentID == "" || req.Progress < 0 || req.Progress > 100 {
		return Task{}, ErrInvalidTask
	}
	reqStatus, ok := ParseTaskStatus(req.Status)
	if !ok {
		return Task{}, ErrInvalidStatus
	}
	if !ValidReportStatus(reqStatus) {
		return Task{}, ErrInvalidStatus
	}

	task, ok := s.tasks[taskID]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	if task.AgentID != req.AgentID {
		return Task{}, ErrTaskAgentMismatch
	}
	currStatus, _ := ParseTaskStatus(task.Status)
	if IsTerminal(currStatus) {
		return task, ErrTaskAlreadyTerminal
	}

	task.Status = req.Status
	task.Progress = req.Progress
	task.Message = req.Message
	task.UpdatedAt = s.now().Format(time.RFC3339)
	if req.ErrorCode != "" {
		task.ErrorCode = req.ErrorCode
	}
	if req.Result != nil {
		task.Result = clonePayload(req.Result)
	}
	s.tasks[task.TaskID] = task
	return task, nil
}

// Cancel marks a task as cancelled. Only non-terminal tasks can be cancelled.
// CountByStatus returns task counts grouped by status.
func (s *TaskStore) CountByStatus() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{}
	for _, task := range s.tasks {
		counts[task.Status]++
	}
	return counts
}

func (s *TaskStore) Cancel(taskID string, req CancelTaskRequest) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	currStatus, _ := ParseTaskStatus(task.Status)
	if IsTerminal(currStatus) {
		return task, ErrTaskAlreadyTerminal
	}

	task.Status = TaskStatusCancelled.String()
	task.Message = req.Message
	task.UpdatedAt = s.now().Format(time.RFC3339)
	s.tasks[task.TaskID] = task
	return task, nil
}

func leaseExpired(task Task, now time.Time) bool {
	if task.LeaseExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, task.LeaseExpiresAt)
	if err != nil {
		return true
	}
	return !expiresAt.After(now)
}
