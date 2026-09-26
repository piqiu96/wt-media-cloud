// Package repository persists executor-neutral file-transfer tasks.
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("file transfer task not found")

type CreateTaskInput struct {
	ID               string
	TeamID           identity.TeamID
	AssetType        model.AssetType
	AssetID          int64
	Purpose          model.Purpose
	ExecutionScope   model.ExecutionScope
	RequestedBy      identity.UserID
	AssignedNodeID   string
	DependencyTaskID string
	DedupeKey        string
	MaxAttempts      int
}

type ProgressInput struct {
	TaskID           string
	NodeID           string
	TransferredBytes int64
	TotalBytes       int64
	SpeedBytesPerSec int64
	ETASeconds       int64
}

type CompletionInput struct {
	TaskID string
	NodeID string
	Bytes  int64
	SHA256 string
}

type FailureInput struct {
	TaskID       string
	NodeID       string
	Status       model.Status
	ErrorCode    string
	ErrorMessage string
}

func CreateTask(input CreateTaskInput, now time.Time) (model.Task, error) {
	return createTask(database.DB(), input, now)
}

func createTask(db *gorm.DB, input CreateTaskInput, now time.Time) (model.Task, error) {
	if err := validateCreateInput(input); err != nil {
		return model.Task{}, err
	}
	if err := db.Exec(`INSERT INTO file_transfer_tasks (id, team_id, asset_type, asset_id, purpose, execution_scope, status, requested_by, assigned_node_id, dependency_task_id, dedupe_key, max_attempts, created_at, updated_at) VALUES (?,?,?,?,?,?, 'pending', ?,?,?,?, ?,?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`, input.ID, input.TeamID, input.AssetType, input.AssetID, input.Purpose, input.ExecutionScope, input.RequestedBy, nullIfEmpty(input.AssignedNodeID), nullIfEmpty(input.DependencyTaskID), input.DedupeKey, input.MaxAttempts, now, now).Error; err != nil {
		return model.Task{}, err
	}
	return getTask(db, input.ID)
}

func ClaimLocalTask(taskID, nodeID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	return claimLocalTask(database.DB(), taskID, nodeID, now, lease)
}

func claimLocalTask(db *gorm.DB, taskID, nodeID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(nodeID) == "" || lease <= 0 {
		return model.Task{}, false, fmt.Errorf("invalid local transfer claim")
	}
	leaseUntil := now.Add(lease)
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'running', claimed_by_node_id = ?, lease_expires_at = ?, heartbeat_at = ?, started_at = COALESCE(started_at, ?), attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND execution_scope = 'local_agent' AND assigned_node_id = ? AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= ?))`, nodeID, leaseUntil, now, now, now, taskID, nodeID, now)
	if result.Error != nil {
		return model.Task{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return model.Task{}, false, nil
	}
	task, err := getTask(db, taskID)
	return task, err == nil, err
}

func ReportProgress(input ProgressInput, now time.Time) (bool, error) {
	return reportProgress(database.DB(), input, now)
}

func reportProgress(db *gorm.DB, input ProgressInput, now time.Time) (bool, error) {
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.NodeID) == "" || input.TransferredBytes < 0 || input.TotalBytes < 0 || input.SpeedBytesPerSec < 0 || input.ETASeconds < 0 {
		return false, fmt.Errorf("invalid transfer progress")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, speed_bytes_per_sec = ?, eta_seconds = ?, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND (lease_expires_at IS NULL OR lease_expires_at > ?) AND transferred_bytes <= ?`, input.TransferredBytes, input.TotalBytes, input.SpeedBytesPerSec, input.ETASeconds, now, now, input.TaskID, input.NodeID, now, input.TransferredBytes)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// HeartbeatTask renews only the current executor's unexpired lease. A stale or
// cancelled executor cannot revive its task through heartbeat traffic.
func HeartbeatTask(taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	return heartbeatTask(database.DB(), taskID, nodeID, now, lease)
}

func heartbeatTask(db *gorm.DB, taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(nodeID) == "" || lease <= 0 {
		return false, fmt.Errorf("invalid transfer heartbeat")
	}
	leaseUntil := now.Add(lease)
	result := db.Exec(`UPDATE file_transfer_tasks SET heartbeat_at = ?, lease_expires_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND (lease_expires_at IS NULL OR lease_expires_at > ?)`, now, leaseUntil, now, taskID, nodeID, now)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func CompleteTask(input CompletionInput, now time.Time) (bool, error) {
	return completeTask(database.DB(), input, now)
}

func completeTask(db *gorm.DB, input CompletionInput, now time.Time) (bool, error) {
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.NodeID) == "" || input.Bytes < 0 || len(strings.TrimSpace(input.SHA256)) != 64 {
		return false, fmt.Errorf("invalid transfer completion")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = 'success', transferred_bytes = ?, total_bytes = CASE WHEN total_bytes = 0 THEN ? ELSE total_bytes END, integrity_sha256 = ?, integrity_bytes = ?, finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, error_code = NULL, error_message = NULL, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ? AND (total_bytes = 0 OR total_bytes = ?) AND ? = ?`, input.Bytes, input.Bytes, input.SHA256, input.Bytes, now, now, now, input.TaskID, input.NodeID, input.Bytes, input.Bytes, input.Bytes)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// FailTask records a failed or cancelled terminal outcome only from the node
// that currently owns the running task lease.
func FailTask(input FailureInput, now time.Time) (bool, error) {
	return failTask(database.DB(), input, now)
}

func failTask(db *gorm.DB, input FailureInput, now time.Time) (bool, error) {
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.NodeID) == "" || (input.Status != model.StatusFailed && input.Status != model.StatusCancelled) || strings.TrimSpace(input.ErrorCode) == "" || len(input.ErrorCode) > 128 || len(input.ErrorMessage) > 500 {
		return false, fmt.Errorf("invalid transfer terminal failure")
	}
	result := db.Exec(`UPDATE file_transfer_tasks SET status = ?, error_code = ?, error_message = ?, finished_at = ?, lease_expires_at = NULL, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = 'running' AND claimed_by_node_id = ?`, input.Status, input.ErrorCode, input.ErrorMessage, now, now, now, input.TaskID, input.NodeID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func getTask(db *gorm.DB, taskID string) (model.Task, error) {
	var task model.Task
	err := scanTask(db.Raw(`SELECT id, team_id, asset_type, asset_id, purpose, execution_scope, status, requested_by, assigned_node_id, claimed_by_node_id, dependency_task_id, total_bytes, transferred_bytes, speed_bytes_per_sec, eta_seconds, attempt_count, max_attempts, lease_expires_at, heartbeat_at, started_at, finished_at, error_code, error_message, integrity_sha256, integrity_bytes, created_at, updated_at FROM file_transfer_tasks WHERE id = ?`, taskID).Row(), &task)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, ErrNotFound
	}
	return task, err
}

func validateCreateInput(input CreateTaskInput) error {
	if strings.TrimSpace(input.ID) == "" || input.TeamID <= 0 || input.AssetID <= 0 || input.RequestedBy <= 0 || strings.TrimSpace(input.DedupeKey) == "" || input.MaxAttempts <= 0 {
		return fmt.Errorf("invalid file transfer task")
	}
	if input.AssetType != model.AssetMaterial {
		return fmt.Errorf("unsupported transfer asset type")
	}
	if (input.Purpose == model.PurposeComposeInputPrepare && input.ExecutionScope != model.ExecutionCloud) || (input.Purpose == model.PurposeUserDownload && input.ExecutionScope != model.ExecutionLocalAgent) {
		return fmt.Errorf("invalid transfer purpose and execution scope")
	}
	if input.Purpose != model.PurposeComposeInputPrepare && input.Purpose != model.PurposeUserDownload {
		return fmt.Errorf("unsupported transfer purpose")
	}
	if input.ExecutionScope == model.ExecutionLocalAgent && strings.TrimSpace(input.AssignedNodeID) == "" {
		return fmt.Errorf("local transfer requires assigned node")
	}
	return nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

type rowScanner interface{ Scan(...any) error }

func scanTask(row rowScanner, task *model.Task) error {
	var teamID, requestedBy int64
	var assetType, purpose, scope, status string
	var assignedNode, claimedNode, dependency, errorCode, errorMessage, sha256 sql.NullString
	var eta, integrityBytes sql.NullInt64
	var lease, heartbeat, started, finished sql.NullTime
	if err := row.Scan(&task.ID, &teamID, &assetType, &task.AssetID, &purpose, &scope, &status, &requestedBy, &assignedNode, &claimedNode, &dependency, &task.TotalBytes, &task.TransferredBytes, &task.SpeedBytesPerSec, &eta, &task.AttemptCount, &task.MaxAttempts, &lease, &heartbeat, &started, &finished, &errorCode, &errorMessage, &sha256, &integrityBytes, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return err
	}
	task.TeamID = identity.TeamID(teamID)
	task.AssetType = model.AssetType(assetType)
	task.Purpose = model.Purpose(purpose)
	task.ExecutionScope = model.ExecutionScope(scope)
	task.Status = model.Status(status)
	task.RequestedBy = identity.UserID(requestedBy)
	task.AssignedNodeID = assignedNode.String
	task.ClaimedByNodeID = claimedNode.String
	task.DependencyTaskID = dependency.String
	task.ErrorCode = errorCode.String
	task.ErrorMessage = errorMessage.String
	task.IntegritySHA256 = sha256.String
	if eta.Valid {
		value := eta.Int64
		task.ETASeconds = &value
	}
	if integrityBytes.Valid {
		value := integrityBytes.Int64
		task.IntegrityBytes = &value
	}
	if lease.Valid {
		task.LeaseExpiresAt = &lease.Time
	}
	if heartbeat.Valid {
		task.HeartbeatAt = &heartbeat.Time
	}
	if started.Valid {
		task.StartedAt = &started.Time
	}
	if finished.Valid {
		task.FinishedAt = &finished.Time
	}
	return nil
}
