package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
)

type (
	TaskStatus          = model.TaskStatus
	TaskType            = model.TaskType
	Task                = model.Task
	CreateTaskRequest   = dto.CreateTaskRequest
	ClaimTaskRequest    = dto.ClaimTaskRequest
	ReportTaskRequest   = dto.ReportTaskRequest
	ActivityTaskRequest = dto.ActivityTaskRequest
	CancelTaskRequest   = dto.CancelTaskRequest
	RetryTaskRequest    = dto.RetryTaskRequest
)

const (
	TaskStatusPending   = model.TaskStatusPending
	TaskStatusLeased    = model.TaskStatusLeased
	TaskStatusRunning   = model.TaskStatusRunning
	TaskStatusSucceeded = model.TaskStatusSucceeded
	TaskStatusFailed    = model.TaskStatusFailed
	TaskStatusCancelled = model.TaskStatusCancelled

	TaskTypeNoop          = model.TaskTypeNoop
	TaskTypeCookieRead    = model.TaskTypeCookieRead
	TaskTypeCookieWrite   = model.TaskTypeCookieWrite
	TaskTypeAccountCheck  = model.TaskTypeAccountCheck
	TaskTypeProfileCreate = model.TaskTypeProfileCreate
	TaskTypeProfileOpen   = model.TaskTypeProfileOpen
	TaskTypeProfileClose  = model.TaskTypeProfileClose
	TaskTypeProfileUpdate = model.TaskTypeProfileUpdate
	TaskTypeProxyCheck    = model.TaskTypeProxyCheck
	TaskTypeProxyMutation = model.TaskTypeProxyMutation

	DefaultLeaseS = 60
)

var (
	ErrTaskNotFound        = model.ErrTaskNotFound
	ErrNoPendingTask       = model.ErrNoPendingTask
	ErrTaskLeaseTaken      = model.ErrTaskLeaseTaken
	ErrInvalidTask         = model.ErrInvalidTask
	ErrTaskAgentMismatch   = model.ErrTaskAgentMismatch
	ErrTaskAlreadyTerminal = model.ErrTaskAlreadyTerminal
	ErrInvalidStatus       = model.ErrInvalidStatus
	ErrInvalidTaskType     = model.ErrInvalidTaskType
	ErrUnsupportedTaskType = model.ErrUnsupportedTaskType
	ErrTaskNotRetryable    = model.ErrTaskNotRetryable
	ErrStaleAttempt        = model.ErrStaleAttempt
	ErrResultUnknownRetry  = model.ErrResultUnknownRetry
)

func ParseTaskStatus(value string) (TaskStatus, bool) {
	switch value {
	case TaskStatusPending.String():
		return TaskStatusPending, true
	case TaskStatusLeased.String():
		return TaskStatusLeased, true
	case TaskStatusRunning.String():
		return TaskStatusRunning, true
	case TaskStatusSucceeded.String():
		return TaskStatusSucceeded, true
	case TaskStatusFailed.String():
		return TaskStatusFailed, true
	case TaskStatusCancelled.String():
		return TaskStatusCancelled, true
	default:
		return TaskStatusPending, false
	}
}

func ValidReportStatus(status TaskStatus) bool {
	return status == TaskStatusRunning || status == TaskStatusSucceeded || status == TaskStatusFailed
}

func IsTerminal(status TaskStatus) bool {
	return status == TaskStatusSucceeded || status == TaskStatusFailed || status == TaskStatusCancelled
}

func ParseTaskType(value string) (TaskType, bool) {
	return model.ParseTaskTypeValue(value)
}
