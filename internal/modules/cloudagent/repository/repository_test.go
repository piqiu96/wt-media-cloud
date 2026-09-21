package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"gorm.io/gorm"
)

func TestCloudAgentRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(RegisterAgentInput) (model.AgentNode, error) = RegisterAgent
	var _ func(string, HeartbeatInput) (model.AgentNode, error) = Heartbeat
	var _ func(string) (model.AgentNode, error) = GetAgent
	var _ func(CreateTaskInput) model.Task = CreateTask
	var _ func(string) (model.Task, error) = GetTask
	var _ func(ClaimTaskInput) (model.Task, error) = ClaimTask
	var _ func(string, ReportTaskInput) (model.Task, error) = ReportTask
	var _ func(string, CancelTaskInput) (model.Task, error) = CancelTask

	var _ func(*gorm.DB, RegisterAgentInput) (model.AgentNode, error) = registerAgent
	var _ func(*gorm.DB, string, HeartbeatInput) (model.AgentNode, error) = heartbeat
	var _ func(*gorm.DB, string) (model.AgentNode, error) = getAgent
	var _ func(*gorm.DB, CreateTaskInput) model.Task = createTask
	var _ func(*gorm.DB, string) (model.Task, error) = getTask
	var _ func(*gorm.DB, ClaimTaskInput) (model.Task, error) = claimTask
	var _ func(*gorm.DB, string, ReportTaskInput) (model.Task, error) = reportTask
	var _ func(*gorm.DB, string, CancelTaskInput) (model.Task, error) = cancelTask
}
