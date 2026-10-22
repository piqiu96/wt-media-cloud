// Package profileguard exposes the module's HTTP surface.
package profileguard

import (
	"context"
	"errors"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	guardservice "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type renewRequest struct {
	LeaseSeconds int `json:"lease_seconds"`
}

type finishRequest struct {
	Outcome guardservice.FinishOutcome `json:"outcome"`
}

// Preflight is a new operation: the strict token proves the Desktop session is
// still current, and the permit it takes belongs to the token's user.
func Preflight(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.InternalError(c, "Profile 守卫服务内部错误")
		return
	}
	outcome, err := guardservice.Preflight(authUserID(auth), c.Param("task_id"))
	if err != nil {
		writeGuardError(c, err)
		return
	}
	api.Success(c, outcome)
}

// Renew and Finish are report-safe: the permit is already held, and the work it
// guards must be able to finish even after the account has logged in elsewhere.
// The permit id alone is not a credential — the token's user has to match the
// row.
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
