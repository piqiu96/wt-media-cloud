package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
)

func CreateTask(req CreateTaskInput) model.Task {
	return createTask(database.DB(), req)
}

func createTask(db *gorm.DB, req CreateTaskInput) model.Task {
	taskType := req.TaskType
	if taskType == "" {
		taskType = model.TaskTypeNoop.String()
	}
	if _, ok := parseTaskType(taskType); !ok {
		taskType = model.TaskTypeNoop.String()
	}
	if req.IdempotencyKey != "" {
		var existingID string
		if err := db.Raw(`SELECT task_id FROM tasks WHERE idempotency_key = ?`, req.IdempotencyKey).Row().Scan(&existingID); err == nil {
			if existing, getErr := getTask(db, existingID); getErr == nil {
				return existing
			}
		}
	}

	now := time.Now().UTC()
	taskID := id.NewID("task")
	if err := db.Exec(`INSERT INTO tasks (task_id, task_type, status, idempotency_key, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE task_id=task_id`,
		taskID, taskType, model.TaskStatusPending.String(), req.IdempotencyKey, jsonValue(req.Payload), now,
	).Error; err != nil {
		return model.Task{}
	}
	return model.Task{
		TaskID: taskID, TaskType: taskType, Status: model.TaskStatusPending.String(),
		IdempotencyKey: req.IdempotencyKey, CreatedAt: now.Format(time.RFC3339), Payload: clonePayload(req.Payload),
	}
}

func GetTask(taskID string) (model.Task, error) {
	return getTask(database.DB(), taskID)
}

func getTask(db *gorm.DB, taskID string) (model.Task, error) {
	var task model.Task
	var idempotencyKey, agentID, leaseExpiresAt, message, updatedAt, errorCode string
	var payload, result []byte
	err := db.Raw(`SELECT task_id, task_type, status, COALESCE(idempotency_key,''), COALESCE(agent_id,''),
		created_at, COALESCE(lease_expires_at,''), progress, COALESCE(message,''),
		COALESCE(updated_at,''), COALESCE(error_code,''), payload_json, result_json
		FROM tasks WHERE task_id = ?`, taskID).Row().Scan(
		&task.TaskID, &task.TaskType, &task.Status, &idempotencyKey, &agentID,
		&task.CreatedAt, &leaseExpiresAt, &task.Progress, &message, &updatedAt, &errorCode, &payload, &result,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrTaskNotFound
	}
	if err != nil {
		return model.Task{}, fmt.Errorf("scan task: %w", err)
	}
	task.IdempotencyKey = ifNotEmpty(idempotencyKey)
	task.AgentID = ifNotEmpty(agentID)
	task.LeaseExpiresAt = ifNotEmpty(leaseExpiresAt)
	task.Message = ifNotEmpty(message)
	task.UpdatedAt = ifNotEmpty(updatedAt)
	task.ErrorCode = ifNotEmpty(errorCode)
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &task.Payload)
	}
	if len(result) > 0 {
		_ = json.Unmarshal(result, &task.Result)
	}
	return task, nil
}

func RetryTask(taskID string) (model.Task, error) {
	return retryTask(database.DB(), taskID)
}

func retryTask(db *gorm.DB, taskID string) (model.Task, error) {
	original, err := getTask(db, taskID)
	if err != nil {
		return model.Task{}, err
	}
	if original.Status != model.TaskStatusFailed.String() && original.Status != model.TaskStatusCancelled.String() {
		return model.Task{}, model.ErrTaskNotRetryable
	}
	retryID := id.NewID("task")
	if err := db.Exec(`INSERT INTO tasks (task_id, task_type, status, idempotency_key, payload_json, created_at)
		VALUES (?, ?, 'pending', ?, ?, ?)`,
		retryID, original.TaskType, original.IdempotencyKey+":retry:"+retryID, jsonValue(original.Payload), time.Now().UTC(),
	).Error; err != nil {
		return model.Task{}, err
	}
	return getTask(db, retryID)
}

func ClaimTask(req ClaimTaskInput) (model.Task, error) {
	return claimTask(database.DB(), req)
}

func claimTask(db *gorm.DB, req ClaimTaskInput) (model.Task, error) {
	leaseSeconds := req.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}
	now := time.Now().UTC()
	leaseExpiresAt := now.Add(time.Duration(leaseSeconds) * time.Second)

	var taskID string
	err := db.Raw(`SELECT task_id FROM tasks
		WHERE (status = 'pending' OR (status = 'leased' AND lease_expires_at <= ?))
		AND status NOT IN ('succeeded','failed','cancelled')
		ORDER BY created_at ASC LIMIT 1 FOR UPDATE`, now).Row().Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrNoPendingTask
	}
	if err != nil {
		return model.Task{}, err
	}
	if err := db.Exec(`UPDATE tasks SET status = 'leased', agent_id = ?, lease_expires_at = ?, updated_at = ?
		WHERE task_id = ?`, req.AgentID, leaseExpiresAt, now, taskID).Error; err != nil {
		return model.Task{}, err
	}
	return getTask(db, taskID)
}

func ReportTask(taskID string, req ReportTaskInput) (model.Task, error) {
	return reportTask(database.DB(), taskID, req)
}

func reportTask(db *gorm.DB, taskID string, req ReportTaskInput) (model.Task, error) {
	reqStatus, ok := parseTaskStatus(req.Status)
	if !ok || !validReportStatus(reqStatus) {
		return model.Task{}, model.ErrInvalidStatus
	}
	if req.AgentID == "" || req.Progress < 0 || req.Progress > 100 {
		return model.Task{}, model.ErrInvalidTask
	}
	result := db.Exec(`UPDATE tasks SET status = ?, progress = ?, message = ?, error_code = ?, result_json = ?, updated_at = ?
		WHERE task_id = ? AND agent_id = ? AND status NOT IN ('succeeded','failed','cancelled')`,
		req.Status, req.Progress, req.Message, req.ErrorCode, jsonValue(req.Result), time.Now().UTC(), taskID, req.AgentID,
	)
	if result.Error != nil {
		return model.Task{}, result.Error
	}
	if result.RowsAffected == 0 {
		task, err := getTask(db, taskID)
		if err != nil {
			return model.Task{}, err
		}
		if task.AgentID != req.AgentID {
			return model.Task{}, model.ErrTaskAgentMismatch
		}
		return task, model.ErrTaskAlreadyTerminal
	}
	updated, err := getTask(db, taskID)
	if err == nil && req.Status == model.TaskStatusSucceeded.String() && req.Result != nil {
		_ = projectResult(db, updated)
	}
	return updated, err
}

func CancelTask(taskID string, req CancelTaskInput) (model.Task, error) {
	return cancelTask(database.DB(), taskID, req)
}

func cancelTask(db *gorm.DB, taskID string, req CancelTaskInput) (model.Task, error) {
	result := db.Exec(`UPDATE tasks SET status = 'cancelled', message = ?, updated_at = ?
		WHERE task_id = ? AND status NOT IN ('succeeded','failed','cancelled')`, req.Message, time.Now().UTC(), taskID)
	if result.Error != nil {
		return model.Task{}, result.Error
	}
	if result.RowsAffected == 0 {
		task, err := getTask(db, taskID)
		if err != nil {
			return model.Task{}, err
		}
		return task, model.ErrTaskAlreadyTerminal
	}
	return getTask(db, taskID)
}

func parseTaskType(value string) (model.TaskType, bool) {
	switch value {
	case model.TaskTypeNoop.String():
		return model.TaskTypeNoop, true
	case model.TaskTypeCookieRead.String():
		return model.TaskTypeCookieRead, true
	case model.TaskTypeCookieWrite.String():
		return model.TaskTypeCookieWrite, true
	case model.TaskTypeAccountCheck.String():
		return model.TaskTypeAccountCheck, true
	case model.TaskTypeProfileCreate.String():
		return model.TaskTypeProfileCreate, true
	case model.TaskTypeProfileOpen.String():
		return model.TaskTypeProfileOpen, true
	case model.TaskTypeProfileClose.String():
		return model.TaskTypeProfileClose, true
	case model.TaskTypeProfileUpdate.String():
		return model.TaskTypeProfileUpdate, true
	case model.TaskTypeProxyCheck.String():
		return model.TaskTypeProxyCheck, true
	case model.TaskTypeProxyMutation.String():
		return model.TaskTypeProxyMutation, true
	default:
		return model.TaskTypeNoop, false
	}
}

func parseTaskStatus(value string) (model.TaskStatus, bool) {
	switch value {
	case model.TaskStatusPending.String():
		return model.TaskStatusPending, true
	case model.TaskStatusLeased.String():
		return model.TaskStatusLeased, true
	case model.TaskStatusRunning.String():
		return model.TaskStatusRunning, true
	case model.TaskStatusSucceeded.String():
		return model.TaskStatusSucceeded, true
	case model.TaskStatusFailed.String():
		return model.TaskStatusFailed, true
	case model.TaskStatusCancelled.String():
		return model.TaskStatusCancelled, true
	default:
		return model.TaskStatusPending, false
	}
}

func validReportStatus(status model.TaskStatus) bool {
	return status == model.TaskStatusRunning || status == model.TaskStatusSucceeded || status == model.TaskStatusFailed
}

func projectResult(db *gorm.DB, task model.Task) error {
	if task.Result == nil {
		return nil
	}
	now := time.Now().UTC()
	switch task.TaskType {
	case model.TaskTypeAccountCheck.String():
		accountID, _ := task.Result["account_id"].(string)
		loginStatus, _ := task.Result["login_status"].(string)
		if accountID == "" || loginStatus == "" {
			return nil
		}
		switch loginStatus {
		case "logged_in":
			loginStatus = "active"
		case "not_logged_in":
			loginStatus = "expired"
		default:
			loginStatus = "environment_error"
		}
		return db.Exec(`UPDATE media_accounts SET login_status = ?, last_checked_at = ?, updated_at = ? WHERE id = ?`, loginStatus, now, now, accountID).Error
	case model.TaskTypeProxyCheck.String():
		proxyID, _ := task.Result["proxy_id"].(string)
		connectivity, _ := task.Result["connectivity"].(string)
		if proxyID == "" || connectivity == "" {
			return nil
		}
		if connectivity == "reachable" {
			connectivity = "ok"
		}
		return db.Exec(`UPDATE proxy_configs SET last_check_result = ?, last_check_at = ?, updated_at = ? WHERE id = ?`, connectivity, now, now, proxyID).Error
	case model.TaskTypeProfileOpen.String(), model.TaskTypeProfileClose.String(), model.TaskTypeProfileUpdate.String():
		cloudProfileID, _ := task.Result["cloud_profile_id"].(string)
		if cloudProfileID == "" {
			return nil
		}
		status := "updated"
		if task.TaskType == model.TaskTypeProfileOpen.String() {
			status = "open"
		}
		if task.TaskType == model.TaskTypeProfileClose.String() {
			status = "closed"
		}
		return db.Exec(`UPDATE browser_profiles SET bit_status = ?, updated_at = ? WHERE id = ?`, status, now, cloudProfileID).Error
	case model.TaskTypeProxyMutation.String():
		cloudProfileID, _ := task.Result["cloud_profile_id"].(string)
		host, _ := task.Result["proxy_host"].(string)
		port, _ := task.Result["proxy_port"].(float64)
		if cloudProfileID == "" || host == "" {
			return nil
		}
		return db.Exec(`UPDATE browser_profiles SET proxy_host = ?, proxy_port = ?, updated_at = ? WHERE id = ?`, host, int(port), now, cloudProfileID).Error
	case model.TaskTypeCookieRead.String(), model.TaskTypeCookieWrite.String():
		accountID, _ := task.Result["account_id"].(string)
		if accountID == "" {
			return nil
		}
		status := "read"
		if task.TaskType == model.TaskTypeCookieWrite.String() {
			status = "active"
			return db.Exec(`UPDATE media_accounts SET cookie_status = ?, active_cookie_updated_at = ?, updated_at = ? WHERE id = ?`, status, now, now, accountID).Error
		}
		return db.Exec(`UPDATE media_accounts SET cookie_status = ?, updated_at = ? WHERE id = ?`, status, now, accountID).Error
	default:
		return nil
	}
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

func ifNotEmpty(value string) string {
	return value
}
