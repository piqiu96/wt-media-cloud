package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"gorm.io/gorm"
)

func TestCloudAgentRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(CreateTaskInput) (model.Task, error) = CreateTask
	var _ func(string, int64) (model.Task, error) = GetTask
	var _ func(string, RetryTaskInput) (model.Task, error) = RetryTask
	var _ func(ClaimTaskInput) (model.Task, error) = ClaimTask
	var _ func(string, ReportTaskInput) (model.Task, error) = ReportTask
	var _ func(string, ActivityTaskInput) error = ActivityTask
	var _ func(string, CancelTaskInput) (model.Task, error) = CancelTask

	var _ func(*gorm.DB, CreateTaskInput) (model.Task, error) = createTask
	var _ func(*gorm.DB, string, int64) (model.Task, error) = getTaskForUser
	var _ func(*gorm.DB, string, ReportTaskInput) (model.Task, error) = reportTask
	var _ func(*gorm.DB, string, ActivityTaskInput) error = activityTask
	var _ func(*gorm.DB, string, CancelTaskInput) (model.Task, error) = cancelTask
}
