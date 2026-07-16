package runtimebinding

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service) {
	h.POST("/api/v1/local-agent/binding-tickets", func(ctx context.Context, c *hertzapp.RequestContext) {
		auth, ok := identity.AuthenticateRequestContext(c, identityService)
		if !ok {
			return
		}
		grant, err := service.IssueTicket(auth.User, auth.Session.ID)
		if err != nil {
			writeRuntimeError(c, err)
			return
		}
		common.Created(c, grant)
	})

	h.POST("/api/v1/local-agent/nodes/register", func(ctx context.Context, c *hertzapp.RequestContext) {
		var input RegisterLocalInput
		if !common.DecodeJSON(c, &input) {
			return
		}
		registration, err := service.RegisterLocal(input)
		if err != nil {
			writeRuntimeError(c, err)
			return
		}
		common.Created(c, registration)
	})

	h.POST("/api/v1/local-agent/nodes/:node_id/runtime-report", func(ctx context.Context, c *hertzapp.RequestContext) {
		var report RuntimeReport
		if !common.DecodeJSON(c, &report) {
			return
		}
		credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		if !ok {
			writeRuntimeError(c, ErrNodeCredentialInvalid)
			return
		}
		if err := service.ReportRuntime(c.Param("node_id"), credential, report); err != nil {
			writeRuntimeError(c, err)
			return
		}
		common.NoContent(c)
	})
}

func bearerCredential(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return value, value != ""
}

func writeRuntimeError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		common.BadRequest(c, 10001, "运行时报告格式错误")
	case errors.Is(err, ErrForbidden):
		common.Forbidden(c, 11003, "没有绑定此节点的权限")
	case errors.Is(err, ErrBindingTicketInvalid):
		common.Unauthorized(c, 11001, "绑定票据无效或已过期")
	case errors.Is(err, ErrBoundSessionInvalid):
		common.Unauthorized(c, 11001, "用户会话已失效")
	case errors.Is(err, ErrNodeCredentialInvalid):
		common.Unauthorized(c, 11001, "节点凭证无效")
	case errors.Is(err, ErrProfileOwnershipMismatch):
		common.Conflict(c, 23003, "上报的 Profile 与绑定用户不匹配")
	case errors.Is(err, cloudagent.ErrIncompatibleAgent):
		common.Conflict(c, 30005, "Agent 合同版本不兼容")
	default:
		common.InternalError(c, "运行时绑定服务内部错误")
	}
}
