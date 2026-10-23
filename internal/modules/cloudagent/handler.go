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
	task, err := service.ClaimTask(req, auth.Claims.UserID, agentIdentity(auth.Claims.UserID))
	writeTaskResult(c, task, err)
}

func GetTask(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	task, err := service.GetTask(c.Param("task_id"), auth.Claims.UserID)
	if err == nil {
		task = redactTask(task)
	}
	writeTaskResult(c, task, err)
}

// GetUserTask 是供 UI 读取本人任务的 session 入口，与 Agent 读取共用用户范围查询。
func GetUserTask(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	task, err := service.GetTask(c.Param("task_id"), int64(actor.ID))
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
	task, err := service.ReportTask(c.Param("task_id"), req, auth.Claims.UserID)
	writeTaskResult(c, task, err)
}

// ActivityTask 只刷新 last_report_at。
func ActivityTask(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	var req service.ActivityTaskRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	if err := service.ActivityTask(c.Param("task_id"), req, auth.Claims.UserID); err != nil {
		writeTaskResult(c, service.Task{}, err)
		return
	}
	api.NoContent(c)
}

// agentIdentity 取自 Token，不取请求体，调用方无法冒充其他 Agent。
func agentIdentity(userID int64) string {
	return fmt.Sprintf("user-%d", userID)
}

func CancelTask(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req service.CancelTaskRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.CancelTask(c.Param("task_id"), req, int64(actor.ID))
	writeTaskResult(c, task, err)
}

func RetryTask(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req service.RetryTaskRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	task, err := service.RetryTask(c.Param("task_id"), int64(actor.ID), req.ConfirmResultUnknown)
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
	case errors.Is(err, service.ErrUnsupportedTaskType):
		api.FailureCode(c, 400, 10002, "不支持的任务类型", "UNSUPPORTED_TASK_TYPE", nil, false, "none")
	case errors.Is(err, service.ErrNoPendingTask):
		api.Conflict(c, 30001, "当前没有可领取的任务")
	case errors.Is(err, service.ErrTaskNotFound):
		api.NotFound(c, 20004, "任务不存在")
	case errors.Is(err, service.ErrTaskAgentMismatch):
		api.Conflict(c, 30002, "Agent 与当前任务负责人不匹配")
	case errors.Is(err, service.ErrTaskAlreadyTerminal):
		api.Conflict(c, 30003, "任务已处于终态")
	case errors.Is(err, service.ErrStaleAttempt):
		api.FailureCode(c, 409, 30009, "任务 attempt 已失效，结果不会覆盖当前执行", "STALE_ATTEMPT", nil, false, "none")
	case errors.Is(err, service.ErrResultUnknownRetry):
		api.FailureCode(c, 409, 30010, "上次执行可能有外部副作用，请先检查平台状态再确认重试", "RESULT_UNKNOWN", nil, false, "check_platform_state")
	case errors.Is(err, service.ErrTaskNotRetryable):
		api.Conflict(c, 30007, "当前任务不可重试")
	default:
		api.InternalError(c, "任务服务内部错误")
	}
}
