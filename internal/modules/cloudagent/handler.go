package cloudagent

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func Compatibility(ctx context.Context, c *hertzapp.RequestContext) {
	api.Success(c, service.CurrentCompatibility())
}
func RegisterAgent(ctx context.Context, c *hertzapp.RequestContext) {
	var req service.RegisterAgentRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	node, err := service.RegisterAgent(req)
	writeAgentResult(c, node, err)
}
func Heartbeat(ctx context.Context, c *hertzapp.RequestContext) {
	var req service.HeartbeatRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	node, err := service.Heartbeat(c.Param("agent_id"), req)
	writeAgentResult(c, node, err)
}
func GetAgent(ctx context.Context, c *hertzapp.RequestContext) {
	node, err := service.GetAgent(c.Param("agent_id"))
	writeAgentResult(c, node, err)
}
func CreateNoopTask(ctx context.Context, c *hertzapp.RequestContext) {
	var req service.CreateTaskRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	api.Created(c, service.CreateTask(req))
}
func ClaimTask(ctx context.Context, c *hertzapp.RequestContext) {
	var req service.ClaimTaskRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.ClaimTask(req)
	writeTaskResult(c, task, err)
}
func GetTask(ctx context.Context, c *hertzapp.RequestContext) {
	task, err := service.GetTask(c.Param("task_id"))
	if err == nil {
		task = redactTask(task)
	}
	writeTaskResult(c, task, err)
}
func ReportTask(ctx context.Context, c *hertzapp.RequestContext) {
	var req service.ReportTaskRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.ReportTask(c.Param("task_id"), req)
	writeTaskResult(c, task, err)
}
func CancelTask(ctx context.Context, c *hertzapp.RequestContext) {
	var req service.CancelTaskRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.CancelTask(c.Param("task_id"), req)
	writeTaskResult(c, task, err)
}
func RetryTask(ctx context.Context, c *hertzapp.RequestContext) {
	task, err := service.RetryTask(c.Param("task_id"))
	writeTaskResult(c, task, err)
}

func redactTask(task service.Task) service.Task {
	if task.Payload == nil {
		return task
	}
	redacted := map[string]any{}
	for key, value := range task.Payload {
		switch key {
		case "password", "proxy_password", "cookie", "cookies", "original_cookie", "active_cookie", "token", "sms_token":
			redacted[key] = "[REDACTED]"
		default:
			redacted[key] = value
		}
	}
	task.Payload = redacted
	return task
}

func writeAgentResult(c *hertzapp.RequestContext, node service.AgentNode, err error) {
	switch {
	case err == nil:
		api.Success(c, node)
	case errors.Is(err, service.ErrInvalidAgent):
		api.BadRequest(c, 30004, "Agent 注册信息无效")
	case errors.Is(err, service.ErrIncompatibleAgent):
		api.Conflict(c, 30005, "Agent 合同版本不兼容")
	case errors.Is(err, service.ErrAgentNotFound):
		api.NotFound(c, 30004, "Agent 未注册")
	default:
		api.InternalError(c, "Agent 注册服务内部错误")
	}
}

func writeTaskResult(c *hertzapp.RequestContext, task service.Task, err error) {
	switch {
	case err == nil:
		api.Success(c, task)
	case errors.Is(err, service.ErrInvalidTask):
		api.BadRequest(c, 10001, "任务请求格式错误")
	case errors.Is(err, service.ErrNoPendingTask):
		api.Conflict(c, 30001, "当前没有可领取的任务")
	case errors.Is(err, service.ErrTaskNotFound):
		api.NotFound(c, 20004, "任务不存在")
	case errors.Is(err, service.ErrTaskAgentMismatch):
		api.Conflict(c, 30002, "Agent 与当前任务负责人不匹配")
	case errors.Is(err, service.ErrTaskAlreadyTerminal):
		api.Conflict(c, 30003, "任务已处于终态")
	case errors.Is(err, service.ErrTaskNotRetryable):
		api.Conflict(c, 30007, "当前任务不可重试")
	default:
		api.InternalError(c, "任务服务内部错误")
	}
}
