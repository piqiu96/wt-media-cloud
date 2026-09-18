package runtimebinding

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	cloudagentservice "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func IssueBindingTicket(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := identity.AuthenticateRequestContext(c)
	if !ok {
		return
	}
	grant, err := runtimeservice.IssueTicket(auth.User, auth.Session.ID)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	api.Created(c, grant)
}
func RegisterLocalNode(ctx context.Context, c *hertzapp.RequestContext) {
	var input runtimeservice.RegisterLocalInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	registration, err := runtimeservice.RegisterLocal(input)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	api.Created(c, registration)
}
func ReportRuntime(ctx context.Context, c *hertzapp.RequestContext) {
	var report runtimeservice.RuntimeReport
	if !api.DecodeJSON(c, &report) {
		return
	}
	credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	if !ok {
		writeRuntimeError(c, runtimeservice.ErrNodeCredentialInvalid)
		return
	}
	if err := runtimeservice.ReportRuntime(c.Param("node_id"), credential, report); err != nil {
		writeRuntimeError(c, err)
		return
	}
	api.NoContent(c)
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
	case errors.Is(err, runtimeservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "运行时报告格式错误")
	case errors.Is(err, runtimeservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有绑定此节点的权限")
	case errors.Is(err, runtimeservice.ErrBindingTicketInvalid):
		api.Unauthorized(c, 11001, "绑定票据无效或已过期")
	case errors.Is(err, runtimeservice.ErrBoundSessionInvalid):
		api.Unauthorized(c, 11001, "用户会话已失效")
	case errors.Is(err, runtimeservice.ErrNodeCredentialInvalid):
		api.Unauthorized(c, 11001, "节点凭证无效")
	case errors.Is(err, runtimeservice.ErrProfileOwnershipMismatch):
		api.Conflict(c, 23003, "上报的 Profile 与绑定用户不匹配")
	case errors.Is(err, cloudagentservice.ErrIncompatibleAgent):
		api.Conflict(c, 30005, "Agent 合同版本不兼容")
	default:
		api.InternalError(c, "运行时绑定服务内部错误")
	}
}
