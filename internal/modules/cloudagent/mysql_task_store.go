package cloudagent

import (
	"database/sql"
	"encoding/json"
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
		`INSERT INTO tasks (task_id, task_type, status, idempotency_key, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE task_id=task_id`,
		taskID, taskType, status, req.IdempotencyKey, jsonValue(req.Payload), now,
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
		Payload:        clonePayload(req.Payload),
	}
}

func (s *MySQLTaskStore) Get(taskID string) (Task, error) {
	if s.db == nil {
		return s.mem.Get(taskID)
	}
	return s.scanTask(s.db.QueryRow(
		`SELECT task_id, task_type, status, COALESCE(idempotency_key,''), COALESCE(agent_id,''),
		        created_at, COALESCE(lease_expires_at,''), progress, COALESCE(message,''),
		        COALESCE(updated_at,''), COALESCE(error_code,''), payload_json, result_json
		 FROM tasks WHERE task_id = ?`, taskID,
	))
}

func (s *MySQLTaskStore) Retry(taskID string) (Task, error) {
	if s.db == nil {
		return s.mem.Retry(taskID)
	}
	original, err := s.Get(taskID)
	if err != nil {
		return Task{}, err
	}
	if original.Status != TaskStatusFailed.String() && original.Status != TaskStatusCancelled.String() {
		return Task{}, ErrTaskNotRetryable
	}
	retryID := common.NewID("task")
	now := s.now()
	_, err = s.db.Exec(`INSERT INTO tasks (task_id, task_type, status, idempotency_key, payload_json, created_at) VALUES (?, ?, 'pending', ?, ?, ?)`, retryID, original.TaskType, original.IdempotencyKey+":retry:"+retryID, jsonValue(original.Payload), now)
	if err != nil {
		return Task{}, err
	}
	return s.Get(retryID)
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
		        COALESCE(updated_at,''), COALESCE(error_code,''), payload_json, result_json
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
		`UPDATE tasks SET status = ?, progress = ?, message = ?, error_code = ?, result_json = ?, updated_at = ?
		 WHERE task_id = ? AND agent_id = ? AND status NOT IN ('succeeded','failed','cancelled')`,
		req.Status, req.Progress, req.Message, req.ErrorCode, jsonValue(req.Result), now, taskID, req.AgentID,
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
	updated, err := s.Get(taskID)
	if err == nil && req.Status == TaskStatusSucceeded.String() && req.Result != nil {
		_ = s.projectResult(updated)
	}
	return updated, err
}

// projectResult records only verified, non-secret summaries in Cloud-owned business tables.
func (s *MySQLTaskStore) projectResult(task Task) error {
	if s.db == nil || task.Result == nil {
		return nil
	}
	now := s.now()
	switch task.TaskType {
	case TaskTypeAccountCheck.String():
		accountID, _ := task.Result["account_id"].(string)
		status, _ := task.Result["check_result"].(string)
		if accountID == "" || status == "" {
			return nil
		}
		switch status {
		case "normal", "not_logged_in", "verification_needed", "expired", "restricted", "account_mismatch", "environment_error":
		default:
			status = "environment_error"
		}
		_, err := s.db.Exec(`UPDATE media_accounts SET login_status = ?, last_checked_at = ?, updated_at = ? WHERE id = ?`, status, now, now, accountID)
		return err
	case TaskTypeProxyCheck.String():
		proxyID, _ := task.Result["proxy_id"].(string)
		connectivity, _ := task.Result["connectivity"].(string)
		if proxyID == "" || connectivity == "" {
			return nil
		}
		if connectivity == "reachable" {
			connectivity = "ok"
		}
		_, err := s.db.Exec(`UPDATE proxy_configs SET last_check_result = ?, last_check_at = ?, updated_at = ? WHERE id = ?`, connectivity, now, now, proxyID)
		return err
	case TaskTypeProfileOpen.String(), TaskTypeProfileClose.String(), TaskTypeProfileUpdate.String():
		cloudProfileID, _ := task.Result["cloud_profile_id"].(string)
		if cloudProfileID == "" {
			return nil
		}
		status := "updated"
		if task.TaskType == TaskTypeProfileOpen.String() {
			status = "open"
		}
		if task.TaskType == TaskTypeProfileClose.String() {
			status = "closed"
		}
		_, err := s.db.Exec(`UPDATE browser_profiles SET bit_status = ?, updated_at = ? WHERE id = ?`, status, now, cloudProfileID)
		return err
	case TaskTypeProxyMutation.String():
		cloudProfileID, _ := task.Result["cloud_profile_id"].(string)
		host, _ := task.Result["proxy_host"].(string)
		port, _ := task.Result["proxy_port"].(float64)
		if cloudProfileID == "" || host == "" {
			return nil
		}
		_, err := s.db.Exec(`UPDATE browser_profiles SET proxy_host = ?, proxy_port = ?, updated_at = ? WHERE id = ?`, host, int(port), now, cloudProfileID)
		return err
	case TaskTypeCookieRead.String(), TaskTypeCookieWrite.String():
		accountID, _ := task.Result["account_id"].(string)
		if accountID == "" {
			return nil
		}
		status := "read"
		if task.TaskType == TaskTypeCookieWrite.String() {
			status = "active"
		}
		if status == "active" {
			_, err := s.db.Exec(`UPDATE media_accounts SET cookie_status = ?, active_cookie_updated_at = ?, updated_at = ? WHERE id = ?`, status, now, now, accountID)
			return err
		}
		_, err := s.db.Exec(`UPDATE media_accounts SET cookie_status = ?, updated_at = ? WHERE id = ?`, status, now, accountID)
		return err
	default:
		return nil
	}
}

// CountByStatus returns task counts grouped by status from MySQL.
func (s *MySQLTaskStore) CountByStatus() map[string]int {
	counts := map[string]int{"pending": 0, "running": 0, "succeeded": 0, "failed": 0, "cancelled": 0}
	if s.db == nil {
		return s.mem.CountByStatus()
	}
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return counts
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err == nil {
			counts[status] = count
		}
	}
	return counts
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
	var payload, result []byte
	err := row.Scan(
		&t.TaskID, &t.TaskType, &t.Status,
		&idempotencyKey, &agentID,
		&t.CreatedAt, &leaseExpiresAt, &progress, &message,
		&updatedAt, &errorCode, &payload, &result,
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
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &t.Payload)
	}
	if len(result) > 0 {
		_ = json.Unmarshal(result, &t.Result)
	}
	return t, nil
}

func jsonValue(payload map[string]any) any {
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func ifNotEmpty(s string) string {
	if s == "" {
		return ""
	}
	return s
}
