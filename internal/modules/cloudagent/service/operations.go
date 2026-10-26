package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/repository"
)

// CreateTask 拒绝空类型或未知类型，不降级成 noop。
func CreateTask(req dto.CreateTaskRequest, userID int64) (model.Task, error) {
	if _, ok := ParseTaskType(req.TaskType); !ok {
		return model.Task{}, model.ErrUnsupportedTaskType
	}
	return repository.CreateTask(repository.CreateTaskInput{
		UserID:         userID,
		TaskType:       req.TaskType,
		IdempotencyKey: req.IdempotencyKey,
		Payload:        req.Payload,
	})
}

func GetTask(taskID string, userID int64) (model.Task, error) {
	return repository.GetTask(taskID, userID)
}

func RetryTask(taskID string, userID int64, confirmResultUnknown bool) (model.Task, error) {
	return repository.RetryTask(taskID, repository.RetryTaskInput{
		UserID:               userID,
		ConfirmResultUnknown: confirmResultUnknown,
	})
}

func ClaimTask(req dto.ClaimTaskRequest, userID int64, agentID string) (model.Task, error) {
	return repository.ClaimTask(repository.ClaimTaskInput{
		UserID:       userID,
		AgentID:      agentID,
		LeaseSeconds: req.LeaseSeconds,
	})
}

func ReportTask(taskID string, req dto.ReportTaskRequest, userID int64) (model.Task, error) {
	return repository.ReportTask(taskID, repository.ReportTaskInput{
		UserID:         userID,
		AttemptID:      req.AttemptID,
		Status:         req.Status,
		Progress:       req.Progress,
		Message:        req.Message,
		ErrorCode:      req.ErrorCode,
		ErrorMessage:   req.ErrorMessage,
		Retryable:      req.Retryable,
		RecoveryAction: req.RecoveryAction,
		Result:         req.Result,
	})
}

func ActivityTask(taskID string, req dto.ActivityTaskRequest, userID int64) error {
	return repository.ActivityTask(taskID, repository.ActivityTaskInput{
		UserID:    userID,
		AttemptID: req.AttemptID,
		Message:   req.Message,
		Stage:     req.Stage,
	})
}

func CancelTask(taskID string, req dto.CancelTaskRequest, userID int64) (model.Task, error) {
	return repository.CancelTask(taskID, repository.CancelTaskInput{UserID: userID, Message: req.Message})
}

func RecoverTimedOutTasks(now time.Time, limit int) ([]repository.RecoveredTask, error) {
	return repository.RecoverTimedOutTasks(now, limit)
}
