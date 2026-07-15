package cloudagent

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
)

func RegisterRoutes(h *server.Hertz, registry *MySQLRegistry, tasks *MySQLTaskStore) {
	h.GET("/api/v1/cloud-agent/compatibility", func(ctx context.Context, c *hertzapp.RequestContext) {
		common.JSONData(c, consts.StatusOK, CurrentCompatibility())
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
		common.JSONData(c, consts.StatusOK, tasks.Create(req))
	})
	// Deprecated: use POST /api/v1/tasks instead.
	h.POST("/api/v1/tasks/noop", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req CreateTaskRequest
		if len(c.Request.Body()) > 0 && !common.DecodeJSON(c, &req) {
			return
		}
		common.JSONData(c, consts.StatusOK, tasks.Create(req))
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
		common.JSONData(c, consts.StatusOK, node)
	case errors.Is(err, ErrInvalidAgent):
		common.JSONError(c, consts.StatusBadRequest, "invalid_agent", "agent registration or heartbeat is invalid")
	case errors.Is(err, ErrIncompatibleAgent):
		common.JSONError(c, consts.StatusConflict, "incompatible_agent_contract", "agent contract version is incompatible")
	case errors.Is(err, ErrAgentNotFound):
		common.JSONError(c, consts.StatusNotFound, "agent_not_found", "agent is not registered")
	default:
		common.JSONError(c, consts.StatusInternalServerError, "agent_registry_error", "agent registry operation failed")
	}
}

func writeTaskResult(c *hertzapp.RequestContext, task Task, err error) {
	switch {
	case err == nil:
		common.JSONData(c, consts.StatusOK, task)
	case errors.Is(err, ErrInvalidTask):
		common.JSONError(c, consts.StatusBadRequest, "invalid_task", "task request is invalid")
	case errors.Is(err, ErrNoPendingTask):
		common.JSONError(c, consts.StatusConflict, "no_pending_task", "no claimable task is available")
	case errors.Is(err, ErrTaskNotFound):
		common.JSONError(c, consts.StatusNotFound, "task_not_found", "task is not found")
	case errors.Is(err, ErrTaskAgentMismatch):
		common.JSONError(c, consts.StatusConflict, "task_agent_mismatch", "agent does not hold this task lease")
	case errors.Is(err, ErrTaskAlreadyTerminal):
		common.JSONError(c, consts.StatusConflict, "task_already_terminal", "task is already in a terminal state")
	default:
		common.JSONError(c, consts.StatusInternalServerError, "task_store_error", "task operation failed")
	}
}
