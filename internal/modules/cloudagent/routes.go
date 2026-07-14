package cloudagent

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
)

func RegisterRoutes(h *server.Hertz, registry *Registry) {
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
