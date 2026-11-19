package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
	"io"
	"strconv"
	"time"
)

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/publication", List)
	h.POST("/api/v1/publication", Create)
	h.GET("/api/v1/publication/:id", Get)
	h.PUT("/api/v1/publication/:id/content", Edit)
	h.POST("/api/v1/publication/:id/start", Start)
	h.GET("/api/v1/publication/:id/attempts", Attempts)
	h.POST("/api/v1/cloud-agent/publication-tasks/claim", localagent.RequireAgent(localagent.StrictMode), Claim)
	h.POST("/api/v1/cloud-agent/publication-tasks/:id/heartbeat", localagent.RequireAgent(localagent.ReportSafeMode), Heartbeat)
	h.POST("/api/v1/cloud-agent/publication-tasks/:id/progress", localagent.RequireAgent(localagent.ReportSafeMode), Progress)
	h.POST("/api/v1/cloud-agent/publication-tasks/:id/complete", localagent.RequireAgent(localagent.ReportSafeMode), Complete)
}
func fail(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, service.ErrInvalid), errors.Is(err, files.ErrInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, errcode.PublishInvalid, "请检查发布资源并确认文案、封面和声明", "publication_invalid")
	case errors.Is(err, service.ErrNotFound), errors.Is(err, files.ErrNotFound), errors.Is(err, accounts.ErrNotFound), errors.Is(err, accounts.ErrForbidden), errors.Is(err, production.ErrComposeOutputNotFound), errors.Is(err, production.ErrComposeOutputForbidden):
		api.FailureNamed(c, consts.StatusNotFound, errcode.PublishNotFound, "任务或资源不存在，或没有访问权限", "publication_not_found")
	case errors.Is(err, service.ErrConflict):
		var conflict *service.ConflictError
		var details any
		if errors.As(err, &conflict) {
			details = conflict
		}
		api.FailureCode(c, consts.StatusConflict, errcode.PublishConflict, "任务或输入已变化，请重新加载后编辑", "publication_conflict", details, false, "reload")
	case errors.Is(err, service.ErrOccupied):
		api.FailureNamed(c, consts.StatusConflict, errcode.PublishOutputOccupied, "该成片已被当前平台的任务占用", "publication_output_occupied")
	case errors.Is(err, service.ErrClosed):
		api.FailureNamed(c, consts.StatusConflict, errcode.PublishPlatformClosed, "当前平台尚未开放该操作", "publication_platform_closed")
	default:
		api.FailureNamed(c, consts.StatusServiceUnavailable, errcode.PublishUnavailable, "操作尚未完成，请保留当前内容并重试", "publication_unavailable")
	}
}
func decode(c *app.RequestContext, in any) error {
	d := json.NewDecoder(bytes.NewReader(c.Request.Body()))
	d.DisallowUnknownFields()
	if err := d.Decode(in); err != nil {
		return service.ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return service.ErrInvalid
	}
	return nil
}
func taskID(c *app.RequestContext) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, service.ErrInvalid
	}
	return id, nil
}
func List(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	f := service.Filter{Page: 1, PageSize: 20, Search: c.Query("search"), Status: c.Query("status"), Platform: c.Query("platform")}
	if raw := c.Query("category_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			fail(c, service.ErrInvalid)
			return
		}
		f.CategoryID = id
	}
	for key, target := range map[string]**time.Time{"created_from": &f.CreatedFrom, "created_before": &f.CreatedBefore} {
		if raw := c.Query(key); raw != "" {
			v, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				fail(c, service.ErrInvalid)
				return
			}
			*target = &v
		}
	}
	for key, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize} {
		if raw := c.Query(key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				fail(c, service.ErrInvalid)
				return
			}
			*target = n
		}
	}
	s := service.Wired()
	items, total, err := s.List(a, f)
	if err != nil {
		fail(c, err)
		return
	}
	summary, err := s.Summary(a, f)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, map[string]any{"summary": summary, "items": items, "total": total, "page": f.Page, "page_size": f.PageSize, "execution_enabled": service.WiredExecution().Enabled})
}
func Create(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.CreateInput
	if err := decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Create(ctx, a, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Created(c, v)
}
func Get(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, err := taskID(c)
	if err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Get(a, id)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
func Edit(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, err := taskID(c)
	if err != nil {
		fail(c, err)
		return
	}
	var in service.EditInput
	if err := decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Edit(ctx, a, id, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
