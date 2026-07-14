package cloudagent

import (
	"errors"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
)

const (
	TaskTypeNoop  = "noop_task"
	TaskPending   = "pending"
	TaskLeased    = "leased"
	TaskRunning   = "running"
	TaskSucceeded = "succeeded"
	TaskFailed    = "failed"
	DefaultLeaseS = 60
)

var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrNoPendingTask     = errors.New("no pending task")
	ErrTaskLeaseTaken    = errors.New("task lease taken")
	ErrInvalidTask       = errors.New("invalid task")
	ErrTaskAgentMismatch = errors.New("task agent mismatch")
)

type CreateTaskRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type ClaimTaskRequest struct {
	AgentID      string `json:"agent_id"`
	LeaseSeconds int    `json:"lease_seconds"`
}

type Task struct {
	TaskID         string `json:"task_id"`
	TaskType       string `json:"task_type"`
	Status         string `json:"status"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	CreatedAt      string `json:"created_at"`
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
	Progress       int    `json:"progress,omitempty"`
	Message        string `json:"message,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

type ReportTaskRequest struct {
	AgentID  string `json:"agent_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

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

func (s *TaskStore) CreateNoop(req CreateTaskRequest) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.IdempotencyKey != "" {
		if id, ok := s.idempotency[req.IdempotencyKey]; ok {
			return s.tasks[id]
		}
	}
	task := Task{
		TaskID:         s.newID(),
		TaskType:       TaskTypeNoop,
		Status:         TaskPending,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      s.now().Format(time.RFC3339),
	}
	s.tasks[task.TaskID] = task
	if req.IdempotencyKey != "" {
		s.idempotency[req.IdempotencyKey] = task.TaskID
	}
	return task
}

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
	for _, task := range s.tasks {
		if task.Status == TaskLeased && task.AgentID == req.AgentID && !leaseExpired(task, now) {
			return task, nil
		}
	}
	for _, task := range s.tasks {
		if task.Status == TaskPending || leaseExpired(task, now) {
			task.Status = TaskLeased
			task.AgentID = req.AgentID
			task.LeaseExpiresAt = now.Add(time.Duration(leaseSeconds) * time.Second).Format(time.RFC3339)
			s.tasks[task.TaskID] = task
			return task, nil
		}
	}
	return Task{}, ErrNoPendingTask
}

func (s *TaskStore) Get(taskID string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	return task, nil
}

func (s *TaskStore) Report(taskID string, req ReportTaskRequest) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.AgentID == "" || !validTaskReportStatus(req.Status) || req.Progress < 0 || req.Progress > 100 {
		return Task{}, ErrInvalidTask
	}
	task, ok := s.tasks[taskID]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	if task.AgentID != req.AgentID {
		return Task{}, ErrTaskAgentMismatch
	}
	task.Status = req.Status
	task.Progress = req.Progress
	task.Message = req.Message
	task.UpdatedAt = s.now().Format(time.RFC3339)
	s.tasks[task.TaskID] = task
	return task, nil
}

func leaseExpired(task Task, now time.Time) bool {
	if task.Status != TaskLeased || task.LeaseExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, task.LeaseExpiresAt)
	if err != nil {
		return true
	}
	return !expiresAt.After(now)
}

func validTaskReportStatus(status string) bool {
	return status == TaskRunning || status == TaskSucceeded || status == TaskFailed
}
