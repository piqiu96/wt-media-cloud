package proxy

import (
	"context"
	"errors"
	"strconv"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
)

type TaskCreator interface {
	Create(cloudagent.CreateTaskRequest) cloudagent.Task
}
type ProfileLookup interface {
	GetProfile(string) (profilebinding.BrowserProfile, bool, error)
}

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service, deps ...any) {
	var tasks TaskCreator
	var profiles ProfileLookup
	var checker SyncProxyChecker
	for _, dep := range deps {
		if value, ok := dep.(TaskCreator); ok {
			tasks = value
		}
		if value, ok := dep.(ProfileLookup); ok {
			profiles = value
		}
		if value, ok := dep.(SyncProxyChecker); ok {
			checker = value
		}
	}
	h.GET("/api/v1/proxies", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxies, err := service.List(ProxyFilter{
			BusinessStatus: c.Query("business_status"),
			Supplier:       c.Query("supplier"),
			Region:         c.Query("region"),
			Search:         c.Query("search"),
			Limit:          200,
		})
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, proxies)
	})

	h.GET("/api/v1/proxies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, proxy)
	})

	h.POST("/api/v1/proxies", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req CreateProxyInput
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.Create(req)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Created(c, proxy)
	})

	h.PATCH("/api/v1/proxies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req CreateProxyInput
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.Update(c.Param("id"), req)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, proxy)
	})

	h.PATCH("/api/v1/proxies/:id/status", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			BusinessStatus BusinessStatus `json:"business_status"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.UpdateStatus(c.Param("id"), req.BusinessStatus)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, proxy)
	})

	h.DELETE("/api/v1/proxies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if err := service.Delete(c.Param("id")); err != nil {
			writeProxyError(c, err)
			return
		}
		common.NoContent(c)
	})

	h.POST("/api/v1/proxies/import/preview", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			Lines []string `json:"lines"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		parsed := service.BulkParse(req.Lines)
		common.Success(c, map[string]interface{}{"parsed": parsed})
	})

	h.POST("/api/v1/proxies/import", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			Lines []string `json:"lines"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		parsed := service.BulkParse(req.Lines)
		imported, err := service.BulkImport(parsed)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Created(c, map[string]interface{}{
			"parsed":   parsed,
			"imported": imported,
		})
	})

	h.POST("/api/v1/proxies/:id/check", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if checker == nil {
			common.Failure(c, 503, 30006, "Agent 同步检查服务不可用", nil)
			return
		}
		result, err := checker.Check(ctx, ProxyCheckInput{ProxyID: proxy.ID, ProxyProtocol: proxy.ProxyProtocol, Host: proxy.Host, Port: proxy.Port})
		if err != nil {
			common.Failure(c, 503, 30007, "Agent 同步检查失败", nil)
			return
		}
		updated, err := service.RecordCheckResult(proxy.ID, result.Connectivity)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, updated)
	})

	h.POST("/api/v1/proxies/:id/check/background", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if tasks == nil {
			common.Failure(c, 503, 30006, "任务服务不可用", nil)
			return
		}
		task := tasks.Create(cloudagent.CreateTaskRequest{
			TaskType:       cloudagent.TaskTypeProxyCheck.String(),
			IdempotencyKey: "proxy-check:" + strconv.FormatInt(int64(actor.ID), 10) + ":" + proxy.ID + ":" + common.NewID("attempt"),
			Payload:        map[string]any{"proxy_id": proxy.ID, "proxy_protocol": proxy.ProxyProtocol, "host": proxy.Host, "port": proxy.Port, "username": proxy.Username, "password": proxy.Password},
		})
		common.Created(c, task)
	})

	h.POST("/api/v1/proxies/:id/assign", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if tasks == nil || profiles == nil {
			common.Failure(c, 503, 30006, "任务服务不可用", nil)
			return
		}
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		profile, found, err := profiles.GetProfile(req.ProfileID)
		if err != nil {
			common.InternalError(c, "Profile 查询失败")
			return
		}
		if !found || profile.UserID != actor.ID || profile.LocalStatus != profilebinding.ProfileActive {
			common.Forbidden(c, 11003, "没有权限分配此 Profile")
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if proxy.BusinessStatus != BizActive {
			common.Conflict(c, 23004, "代理不可用")
			return
		}
		task := tasks.Create(cloudagent.CreateTaskRequest{TaskType: cloudagent.TaskTypeProxyMutation.String(), IdempotencyKey: "proxy-assign:" + strconv.FormatInt(int64(actor.ID), 10) + ":" + profile.ID + ":" + proxy.ID + ":" + common.NewID("attempt"), Payload: map[string]any{"cloud_profile_id": profile.ID, "profile_id": profile.BitProfileID, "proxy_id": proxy.ID, "proxy_protocol": proxy.ProxyProtocol, "host": proxy.Host, "port": proxy.Port, "username": proxy.Username, "password": proxy.Password}})
		common.Created(c, task)
	})

	h.POST("/api/v1/proxies/:id/quota", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			MaxProfiles int `json:"max_profiles"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.SetMaxProfileCount(c.Param("id"), req.MaxProfiles)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, proxy)
	})
}

func writeProxyError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		common.NotFound(c, 20004, "代理不存在")
	case errors.Is(err, ErrInvalidInput):
		common.BadRequest(c, 10001, "代理信息格式错误")
	default:
		common.InternalError(c, "代理服务内部错误")
	}
}
