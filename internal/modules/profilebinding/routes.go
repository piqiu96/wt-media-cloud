package profilebinding

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type TaskCreator interface {
	Create(cloudagent.CreateTaskRequest) cloudagent.Task
}

type LocalTrustChecker interface {
	CheckLocalTrust(userID identity.UserID, nodeID string) error
}

type updateProfileRequest struct {
	Remark         *string                `json:"remark"`
	BusinessStatus *ProfileBusinessStatus `json:"business_status"`
}

type localSensitiveRequest struct {
	NodeID string `json:"node_id"`
}

type assignProfileOwnerRequest struct {
	UserID identity.UserID `json:"user_id"`
}

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service, tasks TaskCreator, trust LocalTrustChecker) {
	_ = tasks
	h.POST("/api/v1/bit-browser/profile-scans", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var input SnapshotInput
		if !common.DecodeJSON(c, &input) {
			return
		}
		nodeID := strings.TrimSpace(input.NodeID)
		if nodeID == "" {
			writeProfileError(c, runtimebinding.ErrInvalidInput)
			return
		}
		if !checkLocalTrust(c, trust, actor.ID, nodeID) {
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
		nodeID, ok := decodeLocalSensitiveNode(c)
		if !ok {
			writeProfileError(c, runtimebinding.ErrInvalidInput)
			return
		}
		if !checkLocalTrust(c, trust, actor.ID, nodeID) {
			return
		}
		scan, err := service.ConfirmScan(actor, c.Param("scan_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, scan)
	})
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/confirm-main-identity", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		scan, err := service.ConfirmMainIdentity(actor, c.Param("scan_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, scan)
	})
	h.POST("/api/v1/bit-browser/main-identity", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var input MainIdentityInput
		if !common.DecodeJSON(c, &input) {
			return
		}
		binding, err := service.ConfirmMainIdentityDirect(actor, input)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, binding)
	})
	h.DELETE("/api/v1/users/:user_id/bit-browser-main-identity", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		userID, valid := parseProfileUserID(c.Param("user_id"))
		if !valid {
			writeProfileError(c, ErrInvalidInput)
			return
		}
		if err := service.ClearMainIdentity(actor, userID); err != nil {
			writeProfileError(c, err)
			return
		}
		common.NoContent(c)
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
		var userID identity.UserID
		if raw := c.Query("user_id"); raw != "" {
			parsed, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || parsed <= 0 {
				writeProfileError(c, ErrForbidden)
				return
			}
			userID = identity.UserID(parsed)
		}
		profiles, err := service.ListProfiles(actor, userID)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, profiles)
	})
	h.POST("/api/v1/browser-profiles", func(ctx context.Context, c *hertzapp.RequestContext) {
		if _, ok := identity.AuthenticateRequest(c, identityService); !ok {
			return
		}
		writeDesktopOnlyProfileOperation(c)
	})
	h.POST("/api/v1/browser-profiles/:id/open", func(ctx context.Context, c *hertzapp.RequestContext) {
		if _, ok := identity.AuthenticateRequest(c, identityService); !ok {
			return
		}
		writeDesktopOnlyProfileOperation(c)
	})
	h.POST("/api/v1/browser-profiles/:id/close", func(ctx context.Context, c *hertzapp.RequestContext) {
		if _, ok := identity.AuthenticateRequest(c, identityService); !ok {
			return
		}
		writeDesktopOnlyProfileOperation(c)
	})
	h.PATCH("/api/v1/browser-profiles/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req updateProfileRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		profile, err := service.UpdateProfile(actor, c.Param("id"), req.Remark, req.BusinessStatus)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, profile)
	})
	h.POST("/api/v1/browser-profiles/:id/assign-owner", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req assignProfileOwnerRequest
		if !common.DecodeJSON(c, &req) || req.UserID <= 0 {
			writeProfileError(c, ErrInvalidInput)
			return
		}
		target, found, err := identityService.ResolveUser(req.UserID)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		if !found {
			writeProfileError(c, ErrInvalidInput)
			return
		}
		profile, err := service.AssignProfileOwner(actor, c.Param("id"), target)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.Success(c, profile)
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

func parseProfileUserID(raw string) (identity.UserID, bool) {
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return identity.UserID(parsed), true
}

func extractNodeID(input map[string]any) (string, bool) {
	raw, ok := input["node_id"]
	delete(input, "node_id")
	if !ok {
		return "", false
	}
	nodeID := strings.TrimSpace(strconvAny(raw))
	return nodeID, nodeID != ""
}

func decodeLocalSensitiveNode(c *hertzapp.RequestContext) (string, bool) {
	var req localSensitiveRequest
	if !common.DecodeJSON(c, &req) {
		return "", false
	}
	nodeID := strings.TrimSpace(req.NodeID)
	return nodeID, nodeID != ""
}

func writeDesktopOnlyProfileOperation(c *hertzapp.RequestContext) {
	common.Conflict(c, 23004, "浏览器窗口本机操作只能在Desktop执行；Cloud Web只展示已保存的窗口信息")
}

func checkLocalTrust(c *hertzapp.RequestContext, trust LocalTrustChecker, userID identity.UserID, nodeID string) bool {
	if trust == nil {
		writeProfileError(c, runtimebinding.ErrLocalTrustUnavailable)
		return false
	}
	if err := trust.CheckLocalTrust(userID, nodeID); err != nil {
		writeProfileError(c, err)
		return false
	}
	return true
}

func strconvAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func createProfileTask(c *hertzapp.RequestContext, tasks TaskCreator, taskType string, actorID identity.UserID, payload map[string]any) {
	if tasks == nil {
		common.Failure(c, 503, 30006, "任务服务不可用", nil)
		return
	}
	idempotency := taskType + ":" + strconv.FormatInt(int64(actorID), 10) + ":" + common.NewID("attempt")
	task := tasks.Create(cloudagent.CreateTaskRequest{TaskType: taskType, IdempotencyKey: idempotency, Payload: payload})
	common.Created(c, task)
}

func writeProfileError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		common.BadRequest(c, 10001, "请求参数无效")
	case errors.Is(err, runtimebinding.ErrInvalidInput):
		common.BadRequest(c, 10001, "当前电脑缺少本地环境确认信息")
	case errors.Is(err, runtimebinding.ErrBoundSessionInvalid), errors.Is(err, runtimebinding.ErrNodeCredentialInvalid):
		common.Unauthorized(c, 11001, "当前电脑登录状态已失效，请重新登录 Desktop 并刷新本机状态")
	case errors.Is(err, runtimebinding.ErrLocalTrustUnavailable), errors.Is(err, runtimebinding.ErrProfileOwnershipMismatch):
		common.Conflict(c, 23003, "当前电脑尚未完成本地环境确认，暂时不能扫描或操作浏览器窗口")
	case errors.Is(err, ErrForbidden):
		common.Forbidden(c, 11003, "没有权限执行此 Profile 操作")
	case errors.Is(err, ErrIdentityUnverifiable):
		common.Conflict(c, 23002, "BitBrowser Profile 身份无法验证")
	case errors.Is(err, ErrIdentityMismatch):
		common.Conflict(c, 23002, "当前比特浏览器登录账号与系统绑定账号不一致")
	case errors.Is(err, ErrScanNotFound), errors.Is(err, ErrProfileNotFound):
		common.NotFound(c, 20004, "Profile 扫描或 Profile 不存在")
	case errors.Is(err, ErrScanExpired):
		common.Failure(c, 410, 20004, "Profile 扫描已过期", nil)
	case errors.Is(err, ErrScanNotReady):
		common.Conflict(c, 20009, "Profile 扫描未就绪")
	case errors.Is(err, ErrProfileReferenced):
		common.Conflict(c, 20003, "浏览器窗口已被媒体账号引用，不能直接分配给其他用户")
	case errors.Is(err, ErrProfileNotDisabled):
		common.Conflict(c, 20011, "仅已停用的浏览器窗口可同步删除；请先停用该窗口")
	default:
		common.InternalError(c, "Profile 服务内部错误")
	}
}
