// Package profileguard composes the sensitive Profile guard module.
package profileguard

import (
	"context"
	"errors"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	guardservice "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type nodeRequest struct {
	NodeID string `json:"node_id"`
}
type renewRequest struct {
	NodeID       string `json:"node_id"`
	LeaseSeconds int    `json:"lease_seconds"`
}
type finishRequest struct {
	NodeID  string                     `json:"node_id"`
	Outcome guardservice.FinishOutcome `json:"outcome"`
}

func Preflight(ctx context.Context, c *hertzapp.RequestContext) {
	var req nodeRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	if !ok {
		writeGuardError(c, runtimeservice.ErrNodeCredentialInvalid)
		return
	}
	outcome, err := guardservice.Preflight(req.NodeID, credential, c.Param("task_id"))
	if err != nil {
		writeGuardError(c, err)
		return
	}
	api.Success(c, outcome)
}
func Renew(ctx context.Context, c *hertzapp.RequestContext) {
	var req renewRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	nodeCredential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	permitCredential := strings.TrimSpace(string(c.Request.Header.Peek("X-Profile-Permit")))
	if !ok || permitCredential == "" {
		writeGuardError(c, guardservice.ErrPermitCredentialInvalid)
		return
	}
	expiresAt, err := guardservice.Renew(req.NodeID, nodeCredential, c.Param("permit_id"), permitCredential, time.Duration(req.LeaseSeconds)*time.Second)
	if err != nil {
		writeGuardError(c, err)
		return
	}
	api.Success(c, map[string]any{"status": "renewed", "expires_at": expiresAt})
}
func Finish(ctx context.Context, c *hertzapp.RequestContext) {
	var req finishRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	nodeCredential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	permitCredential := strings.TrimSpace(string(c.Request.Header.Peek("X-Profile-Permit")))
	if !ok || permitCredential == "" {
		writeGuardError(c, guardservice.ErrPermitCredentialInvalid)
		return
	}
	if err := guardservice.Finish(req.NodeID, nodeCredential, c.Param("permit_id"), permitCredential, req.Outcome); err != nil {
		writeGuardError(c, err)
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

func writeGuardError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, runtimeservice.ErrNodeCredentialInvalid), errors.Is(err, runtimeservice.ErrBoundSessionInvalid), errors.Is(err, guardservice.ErrPermitCredentialInvalid):
		api.Unauthorized(c, 11001, "凭证无效，请重新登录")
	case errors.Is(err, guardservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "敏感任务预检请求格式错误")
	case errors.Is(err, guardservice.ErrTaskNotFound):
		api.NotFound(c, 20004, "敏感任务不存在")
	case errors.Is(err, guardservice.ErrTaskAssignmentMismatch):
		api.Forbidden(c, 11003, "敏感任务未分配给此节点")
	case errors.Is(err, guardservice.ErrRuntimeUnavailable):
		api.Conflict(c, 23003, "Profile 运行环境不可用")
	default:
		api.InternalError(c, "Profile 守卫服务内部错误")
	}
}
