package contentpool

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type sourceRequest struct {
	TeamID            *identity.TeamID `json:"team_id"`
	Platform          string           `json:"platform"`
	PlatformContentID string           `json:"platform_content_id"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	CoverURL          string           `json:"cover_url"`
	SourceURL         string           `json:"source_url"`
	AuthorID          string           `json:"author_id"`
	AuthorName        string           `json:"author_name"`
	SourceType        string           `json:"source_type"`
	PublishedAt       string           `json:"published_at"`
	RawJSON           json.RawMessage  `json:"raw_json"`
}
type statusRequest struct {
	Status Status `json:"status"`
	Reason string `json:"reason"`
}

type batchStatusRequest struct {
	IDs    []int64 `json:"ids"`
	Status Status  `json:"status"`
	Reason string  `json:"reason"`
}

type strategyRequest struct {
	TeamID       *identity.TeamID `json:"team_id"`
	Name         string           `json:"name"`
	StrategyType string           `json:"strategy_type"`
	Platform     string           `json:"platform"`
	Config       map[string]any   `json:"config"`
	Schedule     string           `json:"schedule"`
	Timezone     string           `json:"timezone"`
	Status       StrategyStatus   `json:"status"`
}

type manualSearchRequest struct {
	Platform string   `json:"platform"`
	Keyword  string   `json:"keyword"`
	Author   string   `json:"author"`
	URL      string   `json:"url"`
	URLs     []string `json:"urls"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
}

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service) {
	h.GET("/api/v1/content-pool", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var team *identity.TeamID
		if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
			n, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || n <= 0 {
				common.BadRequest(c, 10001, "团队参数无效")
				return
			}
			v := identity.TeamID(n)
			team = &v
		}
		items, e := service.List(actor, Filter{TeamID: team, Platform: c.Query("platform"), Status: Status(c.Query("status")), SourceType: c.Query("source_type"), Search: c.Query("search")})
		if e != nil {
			writeError(c, e)
			return
		}
		common.Success(c, items)
	})
	h.GET("/api/v1/content-pool/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, e := strconv.ParseInt(c.Param("id"), 10, 64)
		if e != nil || id <= 0 {
			common.BadRequest(c, 10001, "内容 ID 无效")
			return
		}
		v, found, e := service.Get(actor, id)
		if e != nil {
			writeError(c, e)
			return
		}
		if !found {
			common.NotFound(c, 14001, "内容不存在")
			return
		}
		common.Success(c, v)
	})
	// This provider-owned ingest endpoint is consumed by future M3-B adapters; it does not fetch external data.
	h.POST("/api/v1/content-pool", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req sourceRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		var published *time.Time
		if req.PublishedAt != "" {
			v, e := time.Parse(time.RFC3339, req.PublishedAt)
			if e != nil {
				common.BadRequest(c, 10001, "发布时间格式无效")
				return
			}
			published = &v
		}
		v, e := service.CreateSource(actor, SourceInput{TeamID: req.TeamID, Platform: req.Platform, PlatformContentID: req.PlatformContentID, Title: req.Title, Description: req.Description, CoverURL: req.CoverURL, SourceURL: req.SourceURL, AuthorID: req.AuthorID, AuthorName: req.AuthorName, SourceType: req.SourceType, PublishedAt: published, RawJSON: req.RawJSON})
		if e != nil {
			writeError(c, e)
			return
		}
		common.Created(c, v)
	})
	h.POST("/api/v1/content-pool/batch/status", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req batchStatusRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		items, err := service.BatchSetStatus(actor, req.IDs, req.Status, req.Reason)
		if err != nil {
			writeError(c, err)
			return
		}
		common.Success(c, items)
	})
	h.POST("/api/v1/content-pool/:id/status", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, e := strconv.ParseInt(c.Param("id"), 10, 64)
		if e != nil || id <= 0 {
			common.BadRequest(c, 10001, "内容 ID 无效")
			return
		}
		var req statusRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		v, e := service.SetStatus(actor, id, req.Status, req.Reason)
		if e != nil {
			writeError(c, e)
			return
		}
		common.Success(c, v)
	})
	h.POST("/api/v1/content-pool/:id/materialize", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, e := strconv.ParseInt(c.Param("id"), 10, 64)
		if e != nil || id <= 0 {
			common.BadRequest(c, 10001, "内容 ID 无效")
			return
		}
		v, e := service.Materialize(actor, id)
		if e != nil {
			writeError(c, e)
			return
		}
		common.Created(c, v)
	})
}

func RegisterDiscoveryRoutes(h *server.Hertz, service *DiscoveryService, identityService *identity.Service) {
	h.POST("/api/v1/content-pool/search", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req manualSearchRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}
		item, err := service.CreateManualRun(actor, req.Platform, "keyword", map[string]any{"keyword": req.Keyword, "limit": req.Limit, "offset": req.Offset})
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Created(c, item)
	})
	h.POST("/api/v1/content-pool/author-search", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req manualSearchRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}
		item, err := service.CreateManualRun(actor, req.Platform, "author", map[string]any{"author": req.Author, "limit": req.Limit, "offset": req.Offset})
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Created(c, item)
	})
	h.POST("/api/v1/content-pool/import-url", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req manualSearchRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		urls := make([]string, 0, len(req.URLs)+1)
		if strings.TrimSpace(req.URL) != "" {
			urls = append(urls, strings.TrimSpace(req.URL))
		}
		for _, value := range req.URLs {
			if value = strings.TrimSpace(value); value != "" {
				urls = append(urls, value)
			}
		}
		if len(urls) == 0 {
			writeDiscoveryError(c, ErrDiscoveryInvalid)
			return
		}
		config := map[string]any{"url": urls[0]}
		if len(urls) > 1 {
			config = map[string]any{"urls": urls}
		}
		item, err := service.CreateManualRun(actor, req.Platform, "url", config)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Created(c, item)
	})
	h.GET("/api/v1/discovery-strategies", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		items, err := service.ListStrategies(actor)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Success(c, items)
	})
	h.POST("/api/v1/discovery-strategies", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req strategyRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		team := identity.TeamID(0)
		if req.TeamID != nil {
			team = *req.TeamID
		} else if actor.TeamID != nil {
			team = *actor.TeamID
		}
		item, err := service.CreateStrategy(actor, DiscoveryStrategy{TeamID: team, Name: req.Name, StrategyType: req.StrategyType, Platform: req.Platform, Config: req.Config, Schedule: req.Schedule, Timezone: req.Timezone, Status: req.Status})
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Created(c, item)
	})
	h.POST("/api/v1/discovery-strategies/:id/status", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, err := parseID(c.Param("id"))
		if err != nil {
			common.BadRequest(c, 10001, "策略 ID 无效")
			return
		}
		var req struct {
			Status StrategyStatus `json:"status"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		item, err := service.SetStrategyStatus(actor, id, req.Status)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Success(c, item)
	})
	h.PUT("/api/v1/discovery-strategies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, err := parseID(c.Param("id"))
		if err != nil {
			common.BadRequest(c, 10001, "策略 ID 无效")
			return
		}
		var req strategyRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		item, err := service.UpdateStrategy(actor, id, DiscoveryStrategy{Name: req.Name, StrategyType: req.StrategyType, Platform: req.Platform, Config: req.Config, Schedule: req.Schedule, Timezone: req.Timezone, Status: req.Status})
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Success(c, item)
	})
	h.POST("/api/v1/discovery-strategies/:id/run", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, err := parseID(c.Param("id"))
		if err != nil {
			common.BadRequest(c, 10001, "策略 ID 无效")
			return
		}
		item, err := service.CreateRun(actor, id)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Created(c, item)
	})
	h.GET("/api/v1/crawl-tasks", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var strategyID *int64
		if raw := strings.TrimSpace(c.Query("strategy_id")); raw != "" {
			id, err := parseID(raw)
			if err != nil {
				common.BadRequest(c, 10001, "策略 ID 无效")
				return
			}
			strategyID = &id
		}
		items, err := service.ListCrawlTasks(actor, strategyID)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Success(c, items)
	})
	h.GET("/api/v1/crawl-tasks/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, err := parseID(c.Param("id"))
		if err != nil {
			common.BadRequest(c, 10001, "任务 ID 无效")
			return
		}
		item, found, err := service.GetCrawlTask(actor, id)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		if !found {
			common.NotFound(c, 14004, "挖掘任务不存在")
			return
		}
		common.Success(c, item)
	})
	h.POST("/api/v1/crawl-tasks/:id/confirm", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		id, err := parseID(c.Param("id"))
		if err != nil {
			common.BadRequest(c, 10001, "任务 ID 无效")
			return
		}
		var req struct {
			IDs []string `json:"ids"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		item, err := service.ConfirmResults(actor, id, req.IDs)
		if err != nil {
			writeDiscoveryError(c, err)
			return
		}
		common.Success(c, item)
	})
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func writeDiscoveryError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrDiscoveryForbidden):
		common.Forbidden(c, 11003, "没有权限访问该团队资源")
	case errors.Is(err, ErrStrategyNotFound):
		common.NotFound(c, 14005, "挖掘策略不存在")
	case errors.Is(err, ErrCrawlTaskNotFound):
		common.NotFound(c, 14004, "挖掘任务不存在")
	case errors.Is(err, ErrDiscoveryInvalid):
		common.BadRequest(c, 14006, "挖掘参数无效")
	case errors.Is(err, ErrStrategyDuplicate):
		common.Conflict(c, 14007, "策略名称已存在")
	case errors.Is(err, ErrDiscoverySelection):
		common.BadRequest(c, 14008, "请选择可入池的搜索结果")
	default:
		common.InternalError(c, "挖掘服务内部错误")
	}
}
func writeError(c *hertzapp.RequestContext, e error) {
	switch {
	case errors.Is(e, ErrForbidden):
		common.Forbidden(c, 11003, "没有权限访问该团队内容")
	case errors.Is(e, ErrNotFound):
		common.NotFound(c, 14001, "内容不存在")
	case errors.Is(e, ErrInvalidInput):
		common.BadRequest(c, 14002, "内容参数无效")
	case errors.Is(e, ErrInvalidTransition):
		common.Conflict(c, 14003, "已转素材内容不能恢复为待处理")
	case errors.Is(e, ErrDuplicate):
		common.Conflict(c, 14004, "内容已存在")
	default:
		common.InternalError(c, "内容池服务内部错误")
	}
}
