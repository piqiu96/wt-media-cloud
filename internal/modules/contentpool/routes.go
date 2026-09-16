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
