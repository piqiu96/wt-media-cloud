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

// taskColumns 由所有读取路径共用；scanTask 的扫描顺序必须与它一致。
const taskColumns = `task_id, task_type, status, COALESCE(user_id, 0), attempt, COALESCE(attempt_id, ''),
	COALESCE(idempotency_key, ''), COALESCE(agent_id, ''), created_at, COALESCE(started_at, ''),
	COALESCE(last_report_at, ''), COALESCE(finished_at, ''), COALESCE(lease_expires_at, ''),
	progress, COALESCE(message, ''), COALESCE(updated_at, ''), COALESCE(error_code, ''),
	COALESCE(error_message, ''), execution_timeout_seconds, external_side_effect, retryable,
	COALESCE(recovery_action, ''), COALESCE(settled_by, ''), payload_json, result_json`

type RecoveredTask struct {
	TaskID    string
	ErrorCode string
}

func CreateTask(input CreateTaskInput) (model.Task, error) {
	return createTask(database.DB(), input)
}

func createTask(db *gorm.DB, input CreateTaskInput) (model.Task, error) {
	if input.UserID <= 0 {
		return model.Task{}, model.ErrInvalidTask
	}
	taskType, ok := model.ParseTaskTypeValue(input.TaskType)
	if !ok {
		return model.Task{}, model.ErrUnsupportedTaskType
	}
	spec := taskType.Spec()
	if input.IdempotencyKey != "" {
		var existingID string
		err := db.Raw(`SELECT task_id FROM tasks WHERE user_id = ? AND idempotency_key = ?`,
			input.UserID, input.IdempotencyKey).Row().Scan(&existingID)
		if err == nil {
			return getTaskForUser(db, existingID, input.UserID)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return model.Task{}, err
		}
	}

	now := time.Now().UTC()
	taskID := id.NewID("task")
	// 空键必须写 NULL；唯一键 (user_id, idempotency_key) 会把空串当成真实值。
	var idempotencyKey any
	if input.IdempotencyKey != "" {
		idempotencyKey = input.IdempotencyKey
	}
	if err := db.Exec(`INSERT INTO tasks
		(task_id, task_type, status, user_id, attempt, idempotency_key, payload_json, created_at,
		 execution_timeout_seconds, external_side_effect)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		taskID, input.TaskType, model.TaskStatusPending.String(), input.UserID,
		idempotencyKey, jsonValue(input.Payload), now,
		spec.TimeoutSeconds, spec.ExternalSideEffect,
	).Error; err != nil {
		return model.Task{}, err
	}
	return getTaskForUser(db, taskID, input.UserID)
}

func GetTask(taskID string, userID int64) (model.Task, error) {
	return getTaskForUser(database.DB(), taskID, userID)
}

func getTask(db *gorm.DB, taskID string) (model.Task, error) {
	return scanTask(db.Raw(`SELECT `+taskColumns+` FROM tasks WHERE task_id = ?`, taskID).Row())
}

func getTaskForUser(db *gorm.DB, taskID string, userID int64) (model.Task, error) {
	return scanTask(db.Raw(`SELECT `+taskColumns+` FROM tasks WHERE task_id = ? AND user_id = ?`, taskID, userID).Row())
}

func scanTask(row *sql.Row) (model.Task, error) {
	var task model.Task
	var payload, result []byte
	err := row.Scan(&task.TaskID, &task.TaskType, &task.Status, &task.UserID, &task.Attempt, &task.AttemptID,
		&task.IdempotencyKey, &task.AgentID, &task.CreatedAt, &task.StartedAt,
		&task.LastReportAt, &task.FinishedAt, &task.LeaseExpiresAt,
		&task.Progress, &task.Message, &task.UpdatedAt, &task.ErrorCode,
		&task.ErrorMessage, &task.ExecutionTimeoutSeconds, &task.ExternalSideEffect, &task.Retryable,
		&task.RecoveryAction, &task.SettledBy, &payload, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrTaskNotFound
	}
	if err != nil {
		return model.Task{}, fmt.Errorf("scan task: %w", err)
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &task.Payload)
	}
	if len(result) > 0 {
		_ = json.Unmarshal(result, &task.Result)
	}
	return task, nil
}

func RetryTask(taskID string, input RetryTaskInput) (model.Task, error) {
	return retryTask(database.DB(), taskID, input)
}

func retryTask(db *gorm.DB, taskID string, input RetryTaskInput) (model.Task, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		var status, errorCode string
		err := tx.Raw(`SELECT status, COALESCE(error_code, '') FROM tasks
			WHERE task_id = ? AND user_id = ? FOR UPDATE`, taskID, input.UserID).Row().Scan(&status, &errorCode)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrTaskNotFound
		}
		if err != nil {
			return err
		}
		if status != model.TaskStatusFailed.String() && status != model.TaskStatusCancelled.String() {
			return model.ErrTaskNotRetryable
		}
		if errorCode == "RESULT_UNKNOWN" && !input.ConfirmResultUnknown {
			return model.ErrResultUnknownRetry
		}
		now := time.Now().UTC()
		return tx.Exec(`UPDATE tasks SET status = 'pending', progress = 0, message = '', agent_id = NULL,
			attempt_id = NULL, started_at = NULL, last_report_at = NULL, finished_at = NULL,
			error_code = NULL, error_message = NULL, retryable = 0, recovery_action = NULL,
			settled_by = NULL, result_json = NULL, updated_at = ?
			WHERE task_id = ? AND user_id = ?`, now, taskID, input.UserID).Error
	})
	if err != nil {
		return model.Task{}, err
	}
	return getTaskForUser(db, taskID, input.UserID)
}

func ClaimTask(input ClaimTaskInput) (model.Task, error) {
	return claimTask(database.DB(), input)
}

func claimTask(db *gorm.DB, input ClaimTaskInput) (model.Task, error) {
	if input.UserID <= 0 {
		return model.Task{}, model.ErrInvalidTask
	}
	leaseSeconds := input.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}
	now := time.Now().UTC()
	leaseExpiresAt := now.Add(time.Duration(leaseSeconds) * time.Second)
	attemptID := id.NewID("attempt")

	var taskID string
	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Raw(`SELECT task_id FROM tasks
			WHERE user_id = ?
			AND (status = 'pending' OR (status = 'leased' AND lease_expires_at <= ?))
			ORDER BY created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`, input.UserID, now).Row().Scan(&taskID)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNoPendingTask
		}
		if err != nil {
			return err
		}
		// 清空上一 attempt 的终态与结果，否则旧结果会被误读成这一次的。
		return tx.Exec(`UPDATE tasks SET status = 'leased', agent_id = ?, lease_expires_at = ?,
			attempt = attempt + 1, attempt_id = ?, started_at = ?, last_report_at = ?,
			finished_at = NULL, error_code = NULL, error_message = NULL, retryable = 0,
			recovery_action = NULL, settled_by = NULL, result_json = NULL, progress = 0,
			message = '', updated_at = ?
			WHERE task_id = ?`, input.AgentID, leaseExpiresAt, attemptID, now, now, now, taskID).Error
	})
	if err != nil {
		return model.Task{}, err
	}
	return getTaskForUser(db, taskID, input.UserID)
}

func ReportTask(taskID string, input ReportTaskInput) (model.Task, error) {
	return reportTask(database.DB(), taskID, input)
}

func reportTask(db *gorm.DB, taskID string, input ReportTaskInput) (model.Task, error) {
	reportStatus, ok := parseTaskStatus(input.Status)
	if !ok || !validReportStatus(reportStatus) {
		return model.Task{}, model.ErrInvalidStatus
	}
	if input.AttemptID == "" || input.Progress < 0 || input.Progress > 100 {
		return model.Task{}, model.ErrInvalidTask
	}

	var settledNow bool
	err := db.Transaction(func(tx *gorm.DB) error {
		var userID int64
		var attemptID, status, settledBy string
		err := tx.Raw(`SELECT COALESCE(user_id, 0), COALESCE(attempt_id, ''), status, COALESCE(settled_by, '')
			FROM tasks WHERE task_id = ? FOR UPDATE`, taskID).
			Row().Scan(&userID, &attemptID, &status, &settledBy)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrTaskNotFound
		}
		if err != nil {
			return err
		}
		if userID != input.UserID {
			return model.ErrTaskNotFound
		}
		if attemptID != input.AttemptID {
			return model.ErrStaleAttempt
		}
		now := time.Now().UTC()

		if reportStatus == model.TaskStatusRunning {
			if status != model.TaskStatusLeased.String() && status != model.TaskStatusRunning.String() {
				return nil
			}
			return tx.Exec(`UPDATE tasks SET status = 'running', progress = ?, message = ?,
				started_at = COALESCE(started_at, ?), last_report_at = ?, updated_at = ?
				WHERE task_id = ?`, input.Progress, input.Message, now, now, now, taskID).Error
		}

		terminal := status == model.TaskStatusLeased.String() || status == model.TaskStatusRunning.String()
		// 同一 attempt 的迟到结果仍可收敛；新 attempt 已在上面按 attempt_id 拒绝。
		lateConvergence := status == model.TaskStatusFailed.String() && settledBy == "timeout"
		if !terminal && !lateConvergence {
			return nil
		}
		if err := tx.Exec(`UPDATE tasks SET status = ?, progress = ?, message = ?,
			error_code = NULLIF(?, ''), error_message = NULLIF(?, ''), retryable = ?,
			recovery_action = NULLIF(?, ''), result_json = ?, finished_at = ?,
			started_at = COALESCE(started_at, ?), last_report_at = ?, settled_by = 'agent', updated_at = ?
			WHERE task_id = ?`,
			input.Status, input.Progress, input.Message, input.ErrorCode, input.ErrorMessage,
			input.Retryable, input.RecoveryAction, jsonValue(input.Result), now, now, now, now, taskID,
		).Error; err != nil {
			return err
		}
		settledNow = true
		return nil
	})
	if err != nil {
		return model.Task{}, err
	}
	updated, err := getTaskForUser(db, taskID, input.UserID)
	if err != nil {
		return model.Task{}, err
	}
	if settledNow && updated.Status == model.TaskStatusSucceeded.String() && updated.Result != nil {
		_ = projectResult(db, updated)
	}
	return updated, nil
}

func ActivityTask(taskID string, input ActivityTaskInput) error {
	return activityTask(database.DB(), taskID, input)
}

func activityTask(db *gorm.DB, taskID string, input ActivityTaskInput) error {
	if input.AttemptID == "" {
		return model.ErrStaleAttempt
	}
	now := time.Now().UTC()
	result := db.Exec(`UPDATE tasks SET last_report_at = ?, updated_at = ?
		WHERE task_id = ? AND user_id = ? AND attempt_id = ? AND status IN ('leased', 'running')`,
		now, now, taskID, input.UserID, input.AttemptID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.ErrStaleAttempt
	}
	return nil
}

func CancelTask(taskID string, input CancelTaskInput) (model.Task, error) {
	return cancelTask(database.DB(), taskID, input)
}

func cancelTask(db *gorm.DB, taskID string, input CancelTaskInput) (model.Task, error) {
	now := time.Now().UTC()
	result := db.Exec(`UPDATE tasks SET status = 'cancelled', message = ?, finished_at = ?,
		settled_by = 'user', updated_at = ?
		WHERE task_id = ? AND user_id = ? AND status NOT IN ('succeeded', 'failed', 'cancelled')`,
		input.Message, now, now, taskID, input.UserID)
	if result.Error != nil {
		return model.Task{}, result.Error
	}
	if result.RowsAffected == 0 {
		task, err := getTaskForUser(db, taskID, input.UserID)
		if err != nil {
			return model.Task{}, err
		}
		return task, model.ErrTaskAlreadyTerminal
	}
	return getTaskForUser(db, taskID, input.UserID)
}

// RecoverTimedOutTasks 收口一批静默的 running 任务；每条更新重判超时，
// select 之后补报过的任务不会被误收口。
func RecoverTimedOutTasks(now time.Time, limit int) ([]RecoveredTask, error) {
	return recoverTimedOutTasks(database.DB(), now, limit)
}

func recoverTimedOutTasks(db *gorm.DB, now time.Time, limit int) ([]RecoveredTask, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Raw(`SELECT task_id, COALESCE(attempt_id, ''), external_side_effect FROM tasks
		WHERE status = 'running'
		AND (last_report_at IS NULL OR last_report_at < DATE_SUB(?, INTERVAL execution_timeout_seconds SECOND))
		ORDER BY last_report_at ASC LIMIT ?`, now, limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type timedOutTask struct {
		taskID             string
		attemptID          string
		externalSideEffect bool
	}
	var candidates []timedOutTask
	for rows.Next() {
		var candidate timedOutTask
		if err := rows.Scan(&candidate.taskID, &candidate.attemptID, &candidate.externalSideEffect); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	recovered := make([]RecoveredTask, 0, len(candidates))
	for _, candidate := range candidates {
		code, message, retryable, action := "EXECUTION_INTERRUPTED", "任务执行中断且没有外部副作用，可以重新执行", true, "retry"
		if candidate.externalSideEffect {
			code, message, retryable, action = "RESULT_UNKNOWN", "任务执行中断且可能有外部副作用，请先检查平台实际状态", false, "check_platform_state"
		}
		result := db.Exec(`UPDATE tasks SET status = 'failed', error_code = ?, error_message = ?,
			retryable = ?, recovery_action = ?, settled_by = 'timeout', finished_at = ?, updated_at = ?
			WHERE task_id = ? AND attempt_id = ? AND status = 'running'
			AND (last_report_at IS NULL OR last_report_at < DATE_SUB(?, INTERVAL execution_timeout_seconds SECOND))`,
			code, message, retryable, action, now, now, candidate.taskID, candidate.attemptID, now)
		if result.Error != nil {
			return recovered, result.Error
		}
		if result.RowsAffected == 1 {
			recovered = append(recovered, RecoveredTask{TaskID: candidate.taskID, ErrorCode: code})
		}
	}
	return recovered, nil
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
		// Cloud 的 login_status 受 CHECK 约束，只认 normal/expired/environment_error。
		switch loginStatus {
		case "logged_in":
			loginStatus = "normal"
		case "not_logged_in":
			loginStatus = "expired"
		default:
			loginStatus = "environment_error"
		}
		return db.Exec(`UPDATE media_accounts SET login_status = ?, last_checked_at = ?, updated_at = ?
			WHERE id = ? AND user_id = ?`, loginStatus, now, now, accountID, task.UserID).Error
	case model.TaskTypeProxyCheck.String():
		proxyID, _ := task.Result["proxy_id"].(string)
		connectivity, _ := task.Result["connectivity"].(string)
		if proxyID == "" || connectivity == "" {
			return nil
		}
		if connectivity == "reachable" {
			connectivity = "ok"
		}
		return db.Exec(`UPDATE proxy_configs SET last_check_result = ?, last_check_at = ?, updated_at = ?
			WHERE id = ?`, connectivity, now, now, proxyID).Error
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
		return db.Exec(`UPDATE browser_profiles SET bit_status = ?, updated_at = ?
			WHERE id = ? AND user_id = ?
			AND main_user_id = (SELECT bit_main_user_id FROM users WHERE id = ?)`,
			status, now, cloudProfileID, task.UserID, task.UserID).Error
	case model.TaskTypeProxyMutation.String():
		cloudProfileID, _ := task.Result["cloud_profile_id"].(string)
		host, _ := task.Result["proxy_host"].(string)
		port, _ := task.Result["proxy_port"].(float64)
		if cloudProfileID == "" || host == "" {
			return nil
		}
		return db.Exec(`UPDATE browser_profiles SET proxy_host = ?, proxy_port = ?, updated_at = ?
			WHERE id = ? AND user_id = ?
			AND main_user_id = (SELECT bit_main_user_id FROM users WHERE id = ?)`,
			host, int(port), now, cloudProfileID, task.UserID, task.UserID).Error
	case model.TaskTypeCookieRead.String(), model.TaskTypeCookieWrite.String():
		accountID, _ := task.Result["account_id"].(string)
		if accountID == "" {
			return nil
		}
		status := "read"
		if task.TaskType == model.TaskTypeCookieWrite.String() {
			status = "active"
			return db.Exec(`UPDATE media_accounts SET cookie_status = ?, active_cookie_updated_at = ?, updated_at = ?
				WHERE id = ? AND user_id = ?`, status, now, now, accountID, task.UserID).Error
		}
		return db.Exec(`UPDATE media_accounts SET cookie_status = ?, updated_at = ?
			WHERE id = ? AND user_id = ?`, status, now, accountID, task.UserID).Error
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
