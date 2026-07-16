package cloudagent

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
)

func RegisterRoutes(h *server.Hertz, registry *MySQLRegistry, tasks *MySQLTaskStore) {
	h.GET("/api/v1/cloud-agent/compatibility", func(ctx context.Context, c *hertzapp.RequestContext) {
		common.Success(c, CurrentCompatibility())
	})
	h.POST("/api/v1/cloud-agent/agents/register", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req RegisterAgentRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		node, err := registry.Register(req)
		writeAgentResult(c, node, err)
	})
	h.POST("/api/v1/cloud-agent/agents/:agent_id/heartbeat", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req HeartbeatRequest
		if len(c.Request.Body()) > 0 && !common.DecodeJSON(c, &req) {
			return
		}
		node, err := registry.Heartbeat(c.Param("agent_id"), req)
		writeAgentResult(c, node, err)
	})
	h.GET("/api/v1/cloud-agent/agents/:agent_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		node, err := registry.Get(c.Param("agent_id"))
		writeAgentResult(c, node, err)
	})
	// Generic task creation (supports any task_type).
	h.POST("/api/v1/tasks", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req CreateTaskRequest
		if len(c.Request.Body()) > 0 && !common.DecodeJSON(c, &req) {
			return
		}
		common.Created(c, tasks.Create(req))
	})
	h.POST("/api/v1/tasks/noop", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req CreateTaskRequest
		if len(c.Request.Body()) > 0 && !common.DecodeJSON(c, &req) {
			return
		}
		common.Created(c, tasks.Create(req))
	})
	h.POST("/api/v1/cloud-agent/tasks/claim", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req ClaimTaskRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		task, err := tasks.Claim(req)
		writeTaskResult(c, task, err)
	})
	h.GET("/api/v1/cloud-agent/tasks/:task_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		task, err := tasks.Get(c.Param("task_id"))
		writeTaskResult(c, task, err)
	})
	h.POST("/api/v1/cloud-agent/tasks/:task_id/report", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req ReportTaskRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		task, err := tasks.Report(c.Param("task_id"), req)
		writeTaskResult(c, task, err)
	})
	h.POST("/api/v1/cloud-agent/tasks/:task_id/cancel", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req CancelTaskRequest
		if len(c.Request.Body()) > 0 && !common.DecodeJSON(c, &req) {
			return
		}
		task, err := tasks.Cancel(c.Param("task_id"), req)
		writeTaskResult(c, task, err)
	})
}

func writeAgentResult(c *hertzapp.RequestContext, node AgentNode, err error) {
	switch {
	case err == nil:
		common.Success(c, node)
	case errors.Is(err, ErrInvalidAgent):
		common.BadRequest(c, 30004, "Agent 注册信息无效")
	case errors.Is(err, ErrIncompatibleAgent):
		common.Conflict(c, 30005, "Agent 合同版本不兼容")
	case errors.Is(err, ErrAgentNotFound):
		common.NotFound(c, 30004, "Agent 未注册")
	default:
		common.InternalError(c, "Agent 注册服务内部错误")
	}
}

func writeTaskResult(c *hertzapp.RequestContext, task Task, err error) {
	switch {
	case err == nil:
		common.Success(c, task)
	case errors.Is(err, ErrInvalidTask):
		common.BadRequest(c, 10001, "任务请求格式错误")
	case errors.Is(err, ErrNoPendingTask):
		common.Conflict(c, 30001, "当前没有可领取的任务")
	case errors.Is(err, ErrTaskNotFound):
		common.NotFound(c, 20004, "任务不存在")
	case errors.Is(err, ErrTaskAgentMismatch):
		common.Conflict(c, 30002, "Agent 与当前任务负责人不匹配")
	case errors.Is(err, ErrTaskAlreadyTerminal):
		common.Conflict(c, 30003, "任务已处于终态")
	default:
		common.InternalError(c, "任务服务内部错误")
	}
}
