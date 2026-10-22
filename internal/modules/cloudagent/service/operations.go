package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/repository"
)

func CreateTask(req dto.CreateTaskRequest) model.Task {
	taskType := req.TaskType
	if taskType == "" {
		taskType = model.TaskTypeNoop.String()
	}
	if _, ok := ParseTaskType(taskType); !ok {
		taskType = model.TaskTypeNoop.String()
	}
	req.TaskType = taskType
	return repository.CreateTask(repository.CreateTaskInput{
		TaskType:       req.TaskType,
		IdempotencyKey: req.IdempotencyKey,
		Payload:        req.Payload,
	})
}

func GetTask(taskID string) (model.Task, error)   { return repository.GetTask(taskID) }
func RetryTask(taskID string) (model.Task, error) { return repository.RetryTask(taskID) }
func ClaimTask(req dto.ClaimTaskRequest, agentID string) (model.Task, error) {
	return repository.ClaimTask(repository.ClaimTaskInput{
		AgentID:      agentID,
		LeaseSeconds: req.LeaseSeconds,
	})
}
func ReportTask(taskID string, req dto.ReportTaskRequest, agentID string) (model.Task, error) {
	return repository.ReportTask(taskID, repository.ReportTaskInput{
		AgentID:   agentID,
		Status:    req.Status,
		Progress:  req.Progress,
		Message:   req.Message,
		ErrorCode: req.ErrorCode,
		Result:    req.Result,
	})
}
func CancelTask(taskID string, req dto.CancelTaskRequest) (model.Task, error) {
	return repository.CancelTask(taskID, repository.CancelTaskInput{Message: req.Message})
}
