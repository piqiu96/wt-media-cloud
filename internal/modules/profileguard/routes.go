package profileguard

import (
	"context"
	"errors"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type nodeRequest struct {
	NodeID string `json:"node_id"`
}
type renewRequest struct {
	NodeID       string `json:"node_id"`
	LeaseSeconds int    `json:"lease_seconds"`
}
type finishRequest struct {
	NodeID  string        `json:"node_id"`
	Outcome FinishOutcome `json:"outcome"`
}

func RegisterRoutes(h *server.Hertz, service *Service) {
	h.POST("/api/v1/local-agent/sensitive-tasks/:task_id/preflight", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req nodeRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		if !ok {
			writeGuardError(c, runtimebinding.ErrNodeCredentialInvalid)
			return
		}
		outcome, err := service.Preflight(req.NodeID, credential, c.Param("task_id"))
		if err != nil {
			writeGuardError(c, err)
			return
		}
		common.Success(c, outcome)
	})

	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/renew", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req renewRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		nodeCredential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		permitCredential := strings.TrimSpace(string(c.Request.Header.Peek("X-Profile-Permit")))
		if !ok || permitCredential == "" {
			writeGuardError(c, ErrPermitCredentialInvalid)
			return
		}
		expiresAt, err := service.Renew(req.NodeID, nodeCredential, c.Param("permit_id"), permitCredential, time.Duration(req.LeaseSeconds)*time.Second)
		if err != nil {
			writeGuardError(c, err)
			return
		}
		common.Success(c, map[string]any{"status": "renewed", "expires_at": expiresAt})
	})

	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/finish", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req finishRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		nodeCredential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		permitCredential := strings.TrimSpace(string(c.Request.Header.Peek("X-Profile-Permit")))
		if !ok || permitCredential == "" {
			writeGuardError(c, ErrPermitCredentialInvalid)
			return
		}
		if err := service.Finish(req.NodeID, nodeCredential, c.Param("permit_id"), permitCredential, req.Outcome); err != nil {
			writeGuardError(c, err)
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

func writeGuardError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, runtimebinding.ErrNodeCredentialInvalid), errors.Is(err, runtimebinding.ErrBoundSessionInvalid), errors.Is(err, ErrPermitCredentialInvalid):
		common.Unauthorized(c, 11001, "凭证无效，请重新登录")
	case errors.Is(err, ErrInvalidInput):
		common.BadRequest(c, 10001, "敏感任务预检请求格式错误")
	case errors.Is(err, ErrTaskNotFound):
		common.NotFound(c, 20004, "敏感任务不存在")
	case errors.Is(err, ErrTaskAssignmentMismatch):
		common.Forbidden(c, 11003, "敏感任务未分配给此节点")
	case errors.Is(err, ErrRuntimeUnavailable):
		common.Conflict(c, 23003, "Profile 运行环境不可用")
	default:
		common.InternalError(c, "Profile 守卫服务内部错误")
	}
}
