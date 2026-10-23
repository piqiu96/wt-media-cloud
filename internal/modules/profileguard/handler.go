// Package profileguard 暴露敏感 Profile 守卫的 HTTP 接口。
package profileguard

import (
	"context"
	"errors"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/executionpreflight"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	guardservice "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type preflightRequest struct {
	Facts executionpreflight.FactsInput `json:"facts"`
}

type renewRequest struct {
	LeaseSeconds int `json:"lease_seconds"`
}

type finishRequest struct {
	Outcome guardservice.FinishOutcome `json:"outcome"`
}

// Preflight 是新操作：严格档证明 Desktop 会话仍是当前会话，许可归 Token 用户。
func Preflight(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.InternalError(c, "Profile 守卫服务内部错误")
		return
	}
	var req preflightRequest
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	outcome, err := guardservice.Preflight(authUserID(auth), c.Param("task_id"), guardservice.PreflightInput{
		AgentFacts:          req.Facts.AgentFacts(),
		DesktopSessionValid: true,
	})
	if err != nil {
		writeGuardError(c, err)
		return
	}
	api.Success(c, outcome)
}

// Renew 与 Finish 走上报档：许可已经持有，即使账号在别处登录，也要能收尾。
// permit id 不是凭证，Token 用户必须与许可行一致。
func Renew(_ context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.InternalError(c, "Profile 守卫服务内部错误")
		return
	}
	var req renewRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	expiresAt, err := guardservice.Renew(authUserID(auth), c.Param("permit_id"), time.Duration(req.LeaseSeconds)*time.Second)
	if err != nil {
		writeGuardError(c, err)
		return
	}
	api.Success(c, map[string]any{"status": "renewed", "expires_at": expiresAt})
}

func Finish(_ context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.InternalError(c, "Profile 守卫服务内部错误")
		return
	}
	var req finishRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	if err := guardservice.Finish(authUserID(auth), c.Param("permit_id"), req.Outcome); err != nil {
		writeGuardError(c, err)
		return
	}
	api.NoContent(c)
}

func authUserID(auth localagent.AgentAuth) identityservice.UserID {
	return identityservice.UserID(auth.Claims.UserID)
}

func writeGuardError(c *hertzapp.RequestContext, err error) {
	var failure *executionpreflight.Failure
	if errors.As(err, &failure) {
		executionpreflight.WriteFailure(c, failure)
		return
	}
	switch {
	case errors.Is(err, guardservice.ErrPermitInvalid):
		api.ConflictNamed(c, 23003, "Profile 操作许可已失效", "profile_permit_invalid")
	case errors.Is(err, guardservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "敏感任务预检请求格式错误")
	case errors.Is(err, guardservice.ErrTaskNotFound):
		api.NotFound(c, 20004, "敏感任务不存在")
	case errors.Is(err, guardservice.ErrTaskAssignmentMismatch):
		api.Forbidden(c, 11003, "敏感任务与当前用户或 Profile 不匹配")
	case errors.Is(err, guardservice.ErrRuntimeUnavailable):
		api.Conflict(c, 23003, "Profile 运行环境不可用")
	default:
		api.InternalError(c, "Profile 守卫服务内部错误")
	}
}
