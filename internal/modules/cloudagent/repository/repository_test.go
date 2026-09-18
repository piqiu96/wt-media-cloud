package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"gorm.io/gorm"
)

func TestCloudAgentRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(dto.RegisterAgentRequest) (model.AgentNode, error) = RegisterAgent
	var _ func(string, dto.HeartbeatRequest) (model.AgentNode, error) = Heartbeat
	var _ func(string) (model.AgentNode, error) = GetAgent
	var _ func(dto.CreateTaskRequest) model.Task = CreateTask
	var _ func(string) (model.Task, error) = GetTask
	var _ func(dto.ClaimTaskRequest) (model.Task, error) = ClaimTask
	var _ func(string, dto.ReportTaskRequest) (model.Task, error) = ReportTask
	var _ func(string, dto.CancelTaskRequest) (model.Task, error) = CancelTask

	var _ func(*gorm.DB, dto.RegisterAgentRequest) (model.AgentNode, error) = registerAgent
	var _ func(*gorm.DB, string, dto.HeartbeatRequest) (model.AgentNode, error) = heartbeat
	var _ func(*gorm.DB, string) (model.AgentNode, error) = getAgent
	var _ func(*gorm.DB, dto.CreateTaskRequest) model.Task = createTask
	var _ func(*gorm.DB, string) (model.Task, error) = getTask
	var _ func(*gorm.DB, dto.ClaimTaskRequest) (model.Task, error) = claimTask
	var _ func(*gorm.DB, string, dto.ReportTaskRequest) (model.Task, error) = reportTask
	var _ func(*gorm.DB, string, dto.CancelTaskRequest) (model.Task, error) = cancelTask
}
