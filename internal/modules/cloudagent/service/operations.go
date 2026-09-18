package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/repository"
)

func RegisterAgent(req dto.RegisterAgentRequest) (model.AgentNode, error) {
	if req.AgentID == "" || req.Version == "" || !validMode(req.Mode) {
		return model.AgentNode{}, ErrInvalidAgent
	}
	if !IsAgentCompatible(req.ContractMajorVersion, req.ContractRevision) {
		return model.AgentNode{}, ErrIncompatibleAgent
	}
	return repository.RegisterAgent(req)
}

func Heartbeat(agentID string, req dto.HeartbeatRequest) (model.AgentNode, error) {
	if agentID == "" {
		return model.AgentNode{}, ErrInvalidAgent
	}
	status := req.Status
	if status == "" {
		status = AgentStatusOnline
	}
	if !validStatus(status) {
		return model.AgentNode{}, ErrInvalidAgent
	}
	return repository.Heartbeat(agentID, req)
}

func GetAgent(agentID string) (model.AgentNode, error) {
	return repository.GetAgent(agentID)
}

func CreateTask(req dto.CreateTaskRequest) model.Task {
	taskType := req.TaskType
	if taskType == "" {
		taskType = model.TaskTypeNoop.String()
	}
	if _, ok := ParseTaskType(taskType); !ok {
		taskType = model.TaskTypeNoop.String()
	}
	req.TaskType = taskType
	return repository.CreateTask(req)
}

func GetTask(taskID string) (model.Task, error)              { return repository.GetTask(taskID) }
func RetryTask(taskID string) (model.Task, error)            { return repository.RetryTask(taskID) }
func ClaimTask(req dto.ClaimTaskRequest) (model.Task, error) { return repository.ClaimTask(req) }
func ReportTask(taskID string, req dto.ReportTaskRequest) (model.Task, error) {
	return repository.ReportTask(taskID, req)
}
func CountTasksByStatus() map[string]int { return repository.CountTasksByStatus() }
func CancelTask(taskID string, req dto.CancelTaskRequest) (model.Task, error) {
	return repository.CancelTask(taskID, req)
}
