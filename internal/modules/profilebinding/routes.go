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

type localSensitiveRequest struct {
	NodeID string `json:"node_id"`
}

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service, tasks TaskCreator, trust LocalTrustChecker) {
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
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var input map[string]any
		if !common.DecodeJSON(c, &input) {
			return
		}
		nodeID, ok := extractNodeID(input)
		if !ok {
			writeProfileError(c, runtimebinding.ErrInvalidInput)
			return
		}
		if !checkLocalTrust(c, trust, actor.ID, nodeID) {
			return
		}
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileCreate.String(), actor.ID, input)
	})
	h.POST("/api/v1/browser-profiles/:id/open", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		profile, err := service.GetActiveProfile(actor, c.Param("id"))
		if err != nil {
			writeProfileError(c, err)
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
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileOpen.String(), actor.ID, map[string]any{"cloud_profile_id": profile.ID, "profile_id": profile.BitProfileID})
	})
	h.POST("/api/v1/browser-profiles/:id/close", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		profile, err := service.GetActiveProfile(actor, c.Param("id"))
		if err != nil {
			writeProfileError(c, err)
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
		createProfileTask(c, tasks, cloudagent.TaskTypeProfileClose.String(), actor.ID, map[string]any{"cloud_profile_id": profile.ID, "profile_id": profile.BitProfileID})
	})
	h.PATCH("/api/v1/browser-profiles/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		profile, err := service.GetActiveProfile(actor, c.Param("id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		var input map[string]any
		if !common.DecodeJSON(c, &input) {
			return
		}
		nodeID, ok := extractNodeID(input)
		if !ok {
			writeProfileError(c, runtimebinding.ErrInvalidInput)
			return
		}
		if !checkLocalTrust(c, trust, actor.ID, nodeID) {
			return
		}
		input["cloud_profile_id"] = profile.ID
		input["profile_id"] = profile.BitProfileID
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
	case errors.Is(err, runtimebinding.ErrInvalidInput):
		common.BadRequest(c, 10001, "本地敏感操作请求缺少可信节点")
	case errors.Is(err, runtimebinding.ErrBoundSessionInvalid), errors.Is(err, runtimebinding.ErrNodeCredentialInvalid):
		common.Unauthorized(c, 11001, "本机会话已失效，请重新登录并绑定Desktop")
	case errors.Is(err, runtimebinding.ErrLocalTrustUnavailable), errors.Is(err, runtimebinding.ErrProfileOwnershipMismatch):
		common.Conflict(c, 23003, "当前Desktop、Local Agent或BitBrowser身份不可信，已阻止本地敏感操作")
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
