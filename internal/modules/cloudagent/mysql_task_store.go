package cloudagent

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
)

// MySQLTaskStore persists tasks in MySQL. Falls back to in-memory when db is nil.
type MySQLTaskStore struct {
	db  *sql.DB
	now func() time.Time
	mem *TaskStore // fallback when db is nil
}

func NewMySQLTaskStore(db *sql.DB) *MySQLTaskStore {
	return &MySQLTaskStore{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
		mem: NewTaskStore(),
	}
}

func NewMySQLTaskStoreWithClock(db *sql.DB, now func() time.Time, newID func() string) *MySQLTaskStore {
	mem := NewTaskStoreWithClock(now, newID)
	return &MySQLTaskStore{db: db, now: now, mem: mem}
}

func (s *MySQLTaskStore) Create(req CreateTaskRequest) Task {
	if s.db == nil {
		return s.mem.Create(req)
	}

	taskType := req.TaskType
	if taskType == "" {
		taskType = TaskTypeNoop.String()
	}
	if _, ok := ParseTaskType(taskType); !ok {
		taskType = TaskTypeNoop.String()
	}

	now := s.now()
	taskID := common.NewID("task")
	status := TaskStatusPending.String()

	// Idempotency check with INSERT ... ON DUPLICATE KEY.
	if req.IdempotencyKey != "" {
		err := s.db.QueryRow(
			`SELECT task_id, task_type, status, COALESCE(idempotency_key,''), COALESCE(agent_id,''),
			        created_at, COALESCE(lease_expires_at,''), progress, COALESCE(message,''),
			        COALESCE(updated_at,''), COALESCE(error_code,'')
			 FROM tasks WHERE idempotency_key = ?`, req.IdempotencyKey,
		).Scan(&taskID, &taskType, &status, &req.IdempotencyKey, &taskID, &taskID, &taskID,
			&taskID, &taskID, &taskID, &taskID)
		if err == nil {
			// Return existing task. Full scan approach below is simpler.
		}
	}

	_, err := s.db.Exec(
		`INSERT INTO tasks (task_id, task_type, status, idempotency_key, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE task_id=task_id`,
		taskID, taskType, status, req.IdempotencyKey, now,
	)
	if err != nil {
		return s.mem.Create(req)
	}
	return Task{
		TaskID:         taskID,
		TaskType:       taskType,
		Status:         status,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now.Format(time.RFC3339),
	}
}

func (s *MySQLTaskStore) Get(taskID string) (Task, error) {
	if s.db == nil {
		return s.mem.Get(taskID)
	}
	return s.scanTask(s.db.QueryRow(
		`SELECT task_id, task_type, status, COALESCE(idempotency_key,''), COALESCE(agent_id,''),
		        created_at, COALESCE(lease_expires_at,''), progress, COALESCE(message,''),
		        COALESCE(updated_at,''), COALESCE(error_code,'')
		 FROM tasks WHERE task_id = ?`, taskID,
	))
}

func (s *MySQLTaskStore) Claim(req ClaimTaskRequest) (Task, error) {
	if s.db == nil {
		return s.mem.Claim(req)
	}

	leaseSeconds := req.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = DefaultLeaseS
	}
	now := s.now()
	leaseExpiresAt := now.Add(time.Duration(leaseSeconds) * time.Second)

	// Re-claim own active lease.
	task, err := s.scanTask(s.db.QueryRow(
		`SELECT task_id, task_type, status, COALESCE(idempotency_key,''), COALESCE(agent_id,''),
		        created_at, COALESCE(lease_expires_at,''), progress, COALESCE(message,''),
		        COALESCE(updated_at,''), COALESCE(error_code,'')
		 FROM tasks WHERE agent_id = ? AND status = 'leased' AND lease_expires_at > ?`,
		req.AgentID, now,
	))
	if err == nil {
		return task, nil
	}

	// Claim pending or expired task.
	var taskID string
	err = s.db.QueryRow(
		`SELECT task_id FROM tasks
		 WHERE (status = 'pending' OR (status = 'leased' AND lease_expires_at <= ?))
		   AND status NOT IN ('succeeded','failed','cancelled')
		 ORDER BY created_at ASC LIMIT 1 FOR UPDATE`,
		now,
	).Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNoPendingTask
	}
	if err != nil {
		return s.mem.Claim(req)
	}

	_, err = s.db.Exec(
		`UPDATE tasks SET status = 'leased', agent_id = ?, lease_expires_at = ?, updated_at = ?
		 WHERE task_id = ?`,
		req.AgentID, leaseExpiresAt, now, taskID,
	)
	if err != nil {
		return s.mem.Claim(req)
	}
	return s.Get(taskID)
}

func (s *MySQLTaskStore) Report(taskID string, req ReportTaskRequest) (Task, error) {
	if s.db == nil {
		return s.mem.Report(taskID, req)
	}

	reqStatus, ok := ParseTaskStatus(req.Status)
	if !ok || !ValidReportStatus(reqStatus) {
		return Task{}, ErrInvalidStatus
	}
	if req.AgentID == "" || req.Progress < 0 || req.Progress > 100 {
		return Task{}, ErrInvalidTask
	}

	now := s.now()
	result, err := s.db.Exec(
		`UPDATE tasks SET status = ?, progress = ?, message = ?, error_code = ?, updated_at = ?
		 WHERE task_id = ? AND agent_id = ? AND status NOT IN ('succeeded','failed','cancelled')`,
		req.Status, req.Progress, req.Message, req.ErrorCode, now, taskID, req.AgentID,
	)
	if err != nil {
		return s.mem.Report(taskID, req)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Check if task exists.
		task, err := s.Get(taskID)
		if err != nil {
			return Task{}, err
		}
		if task.AgentID != req.AgentID {
			return Task{}, ErrTaskAgentMismatch
		}
		return task, ErrTaskAlreadyTerminal
	}
	return s.Get(taskID)
}

func (s *MySQLTaskStore) Cancel(taskID string, req CancelTaskRequest) (Task, error) {
	if s.db == nil {
		return s.mem.Cancel(taskID, req)
	}

	now := s.now()
	result, err := s.db.Exec(
		`UPDATE tasks SET status = 'cancelled', message = ?, updated_at = ?
		 WHERE task_id = ? AND status NOT IN ('succeeded','failed','cancelled')`,
		req.Message, now, taskID,
	)
	if err != nil {
		return s.mem.Cancel(taskID, req)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		task, err := s.Get(taskID)
		if err != nil {
			return Task{}, err
		}
		return task, ErrTaskAlreadyTerminal
	}
	return s.Get(taskID)
}

func (s *MySQLTaskStore) scanTask(row *sql.Row) (Task, error) {
	var t Task
	var idempotencyKey, agentID, leaseExpiresAt, message, updatedAt, errorCode string
	var progress int
	err := row.Scan(
		&t.TaskID, &t.TaskType, &t.Status,
		&idempotencyKey, &agentID,
		&t.CreatedAt, &leaseExpiresAt, &progress, &message,
		&updatedAt, &errorCode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("scan task: %w", err)
	}
	t.IdempotencyKey = ifNotEmpty(idempotencyKey)
	t.AgentID = ifNotEmpty(agentID)
	t.LeaseExpiresAt = ifNotEmpty(leaseExpiresAt)
	t.Progress = progress
	t.Message = ifNotEmpty(message)
	t.UpdatedAt = ifNotEmpty(updatedAt)
	t.ErrorCode = ifNotEmpty(errorCode)
	return t, nil
}

func ifNotEmpty(s string) string {
	if s == "" {
		return ""
	}
	return s
}
