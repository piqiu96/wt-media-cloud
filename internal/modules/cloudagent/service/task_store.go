package service

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	TaskStatus        = model.TaskStatus
	TaskType          = model.TaskType
	Task              = model.Task
	CreateTaskRequest = dto.CreateTaskRequest
	ClaimTaskRequest  = dto.ClaimTaskRequest
	ReportTaskRequest = dto.ReportTaskRequest
	CancelTaskRequest = dto.CancelTaskRequest
)

const (
	TaskStatusPending   = model.TaskStatusPending
	TaskStatusLeased    = model.TaskStatusLeased
	TaskStatusRunning   = model.TaskStatusRunning
	TaskStatusSucceeded = model.TaskStatusSucceeded
	TaskStatusFailed    = model.TaskStatusFailed
	TaskStatusCancelled = model.TaskStatusCancelled

	TaskTypeNoop          = model.TaskTypeNoop
	TaskTypeCookieRead    = model.TaskTypeCookieRead
	TaskTypeCookieWrite   = model.TaskTypeCookieWrite
	TaskTypeAccountCheck  = model.TaskTypeAccountCheck
	TaskTypeProfileCreate = model.TaskTypeProfileCreate
	TaskTypeProfileOpen   = model.TaskTypeProfileOpen
	TaskTypeProfileClose  = model.TaskTypeProfileClose
	TaskTypeProfileUpdate = model.TaskTypeProfileUpdate
	TaskTypeProxyCheck    = model.TaskTypeProxyCheck
	TaskTypeProxyMutation = model.TaskTypeProxyMutation

	DefaultLeaseS = 60
)

var (
	ErrTaskNotFound        = model.ErrTaskNotFound
	ErrNoPendingTask       = model.ErrNoPendingTask
	ErrTaskLeaseTaken      = model.ErrTaskLeaseTaken
	ErrInvalidTask         = model.ErrInvalidTask
	ErrTaskAgentMismatch   = model.ErrTaskAgentMismatch
	ErrTaskAlreadyTerminal = model.ErrTaskAlreadyTerminal
	ErrInvalidStatus       = model.ErrInvalidStatus
	ErrInvalidTaskType     = model.ErrInvalidTaskType
	ErrTaskNotRetryable    = model.ErrTaskNotRetryable
)

func ParseTaskStatus(value string) (TaskStatus, bool) {
	switch value {
	case TaskStatusPending.String():
		return TaskStatusPending, true
	case TaskStatusLeased.String():
		return TaskStatusLeased, true
	case TaskStatusRunning.String():
		return TaskStatusRunning, true
	case TaskStatusSucceeded.String():
		return TaskStatusSucceeded, true
	case TaskStatusFailed.String():
		return TaskStatusFailed, true
	case TaskStatusCancelled.String():
		return TaskStatusCancelled, true
	default:
		return TaskStatusPending, false
	}
}

func ValidReportStatus(status TaskStatus) bool {
	return status == TaskStatusRunning || status == TaskStatusSucceeded || status == TaskStatusFailed
}

func IsTerminal(status TaskStatus) bool {
	return status == TaskStatusSucceeded || status == TaskStatusFailed || status == TaskStatusCancelled
}

func ParseTaskType(value string) (TaskType, bool) {
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
		newID:       func() string { return id.NewID("task") },
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

// SetClock replaces the task clock for deterministic repository tests.
func (s *TaskStore) SetClock(now func() time.Time) { s.now = now }

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
