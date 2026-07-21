package proxy

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

	h.POST("/api/v1/proxies/import", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		_ = actor
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
			IdempotencyKey: "proxy-check:" + actor.ID + ":" + proxy.ID + ":" + common.NewID("attempt"),
			Payload:        map[string]any{"proxy_id": proxy.ID, "proxy_protocol": proxy.ProxyProtocol, "host": proxy.Host, "port": proxy.Port, "username": proxy.Username, "password": proxy.Password},
		})
		common.Created(c, task)
	})

	h.POST("/api/v1/proxies/:id/quotas", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			Platform    string `json:"platform"`
			MaxProfiles int    `json:"max_profiles"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		quota, err := service.SetQuota(c.Param("id"), req.Platform, req.MaxProfiles)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, quota)
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
