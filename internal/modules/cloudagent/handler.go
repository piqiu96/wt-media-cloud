package cloudagent

import (
	"context"
	"errors"
	"fmt"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func Compatibility(ctx context.Context, c *hertzapp.RequestContext) {
	api.Success(c, service.CurrentCompatibility())
}
func ClaimTask(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	var req service.ClaimTaskRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.ClaimTask(req, agentIdentity(auth.Claims.UserID))
	writeTaskResult(c, task, err)
}
func GetTask(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := localagent.AgentAuthFrom(c); !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	task, err := service.GetTask(c.Param("task_id"))
	if err == nil {
		task = redactTask(task)
	}
	writeTaskResult(c, task, err)
}
func ReportTask(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	var req service.ReportTaskRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.ReportTask(c.Param("task_id"), req, agentIdentity(auth.Claims.UserID))
	writeTaskResult(c, task, err)
}

// agentIdentity is the audit label written on the task row. It is derived from
// the token, never from the request body, so a caller cannot claim or report as
// another agent.
func agentIdentity(userID int64) string {
	return fmt.Sprintf("user-%d", userID)
}
func CancelTask(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := middleware.AuthenticateRequest(c); !ok {
		return
	}
	var req service.CancelTaskRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.CancelTask(c.Param("task_id"), req)
	writeTaskResult(c, task, err)
}
func RetryTask(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := middleware.AuthenticateRequest(c); !ok {
		return
	}
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
