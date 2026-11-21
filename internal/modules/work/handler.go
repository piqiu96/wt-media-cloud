package work

import (
	"context"
	"errors"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
	"strconv"
	"time"
)

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/work", List)
	h.GET("/api/v1/work/summary", Summary)
	h.GET("/api/v1/work/:id", Get)
}
func fail(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, service.ErrInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, errcode.WorkInvalid, "请检查作品筛选条件", "work_invalid")
	case errors.Is(err, service.ErrNotFound):
		api.FailureNamed(c, consts.StatusNotFound, errcode.WorkNotFound, "作品不存在或没有访问权限", "work_not_found")
	default:
		api.FailureCode(c, consts.StatusServiceUnavailable, errcode.WorkUnavailable, "作品暂时无法读取，请重试", "work_unavailable", nil, true, "retry")
	}
}
func filter(c *app.RequestContext) (service.Filter, error) {
	f := service.Filter{Page: 1, PageSize: 20, Search: c.Query("search"), Platform: c.Query("platform")}
	for _, p := range []struct {
		name  string
		value *int
	}{{"page", &f.Page}, {"page_size", &f.PageSize}} {
		if raw := c.Query(p.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				return f, service.ErrInvalid
			}
			*p.value = n
		}
	}
	if raw := c.Query("category_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			return f, service.ErrInvalid
		}
		f.CategoryID = n
	}
	if raw := c.Query("media_account_id"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || n == 0 {
			return f, service.ErrInvalid
		}
		f.MediaAccountID = n
	}
	for _, p := range []struct {
		name  string
		value **time.Time
	}{{"published_from", &f.PublishedFrom}, {"published_before", &f.PublishedBefore}} {
		if raw := c.Query(p.name); raw != "" {
			v, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				return f, service.ErrInvalid
			}
			*p.value = &v
		}
	}
	return f, nil
}
func List(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	f, err := filter(c)
	if err != nil {
		fail(c, err)
		return
	}
	rows, total, err := service.Wired().List(a, f)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, map[string]any{"items": rows, "total": total, "page": f.Page, "page_size": f.PageSize})
}
func Summary(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	f, err := filter(c)
	if err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Summary(a, f, time.Now().UTC())
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
func Get(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		fail(c, service.ErrInvalid)
		return
	}
	v, err := service.Wired().Get(a, id)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
