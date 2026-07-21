package profilebinding

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type TaskCreator interface {
	Create(cloudagent.CreateTaskRequest) cloudagent.Task
}

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service, taskStores ...TaskCreator) {
	var tasks TaskCreator
	if len(taskStores) > 0 {
		tasks = taskStores[0]
	}
	h.POST("/api/v1/bit-browser/profile-scans", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var input SnapshotInput
		if !common.DecodeJSON(c, &input) {
			return
		}
		scan, err := service.SubmitScan(actor, input)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Created(c, scan)
	})
	h.GET("/api/v1/bit-browser/profile-scans/:scan_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		scan, err := service.GetScan(actor, c.Param("scan_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, scan)
	})
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/confirm", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		scan, err := service.ConfirmScan(actor, c.Param("scan_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, scan)
	})
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/reject", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if err := service.RejectScan(actor, c.Param("scan_id")); err != nil {
			writeProfileError(c, err)
			return
		}
		common.NoContent(c)
	})
	h.GET("/api/v1/browser-profiles", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		profiles, err := service.ListProfiles(actor, c.Query("user_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, profiles)
	})
	h.POST("/api/v1/browser-profiles", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var input map[string]any
		if !common.DecodeJSON(c, &input) {
			return
		}
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileCreate.String(), actor.ID, input)
	})
	h.POST("/api/v1/browser-profiles/:id/open", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if _, err := service.GetActiveProfile(actor, c.Param("id")); err != nil {
			writeProfileError(c, err)
			return
		}
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileOpen.String(), actor.ID, map[string]any{"profile_id": c.Param("id")})
	})
	h.POST("/api/v1/browser-profiles/:id/close", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if _, err := service.GetActiveProfile(actor, c.Param("id")); err != nil {
			writeProfileError(c, err)
			return
		}
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileClose.String(), actor.ID, map[string]any{"profile_id": c.Param("id")})
	})
	h.PATCH("/api/v1/browser-profiles/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if _, err := service.GetActiveProfile(actor, c.Param("id")); err != nil {
			writeProfileError(c, err)
			return
		}
		var input map[string]any
		if !common.DecodeJSON(c, &input) {
			return
		}
		input["profile_id"] = c.Param("id")
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileUpdate.String(), actor.ID, input)
	})
	h.DELETE("/api/v1/browser-profiles/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		_ = actor
		if err := service.DeleteProfile(actor, c.Param("id")); err != nil {
			writeProfileError(c, err)
			return
		}
		common.NoContent(c)
	})
}

func createProfileTask(c *hertzapp.RequestContext, tasks TaskCreator, taskType, actorID string, payload map[string]any) {
	if tasks == nil {
		common.Failure(c, 503, 30006, "任务服务不可用", nil)
		return
	}
	idempotency := taskType + ":" + actorID + ":" + common.NewID("attempt")
	task := tasks.Create(cloudagent.CreateTaskRequest{TaskType: taskType, IdempotencyKey: idempotency, Payload: payload})
	common.Created(c, task)
}

func writeProfileError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		common.Forbidden(c, 11003, "没有权限执行此 Profile 操作")
	case errors.Is(err, ErrIdentityUnverifiable):
		common.Conflict(c, 23002, "BitBrowser Profile 身份无法验证")
	case errors.Is(err, ErrIdentityMismatch):
		common.Conflict(c, 23002, "BitBrowser 身份与绑定用户不匹配")
	case errors.Is(err, ErrScanNotFound), errors.Is(err, ErrProfileNotFound):
		common.NotFound(c, 20004, "Profile 扫描或 Profile 不存在")
	case errors.Is(err, ErrScanExpired):
		common.Failure(c, 410, 20004, "Profile 扫描已过期", nil)
	case errors.Is(err, ErrScanNotReady):
		common.Conflict(c, 20009, "Profile 扫描未就绪")
	default:
		common.InternalError(c, "Profile 服务内部错误")
	}
}
