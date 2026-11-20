package image

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"strconv"
	"strings"
)

func registerTemplateRoutes(h *server.Hertz) {
	h.GET("/api/v1/cover-template", ListCoverTemplate)
	h.POST("/api/v1/cover-template", CreateCoverTemplate)
	h.GET("/api/v1/cover-template/:id", GetCoverTemplate)
	h.DELETE("/api/v1/cover-template/:id", DeleteCoverTemplate)
	h.PUT("/api/v1/cover-template/:id/draft", SaveCoverTemplateDraft)
	h.POST("/api/v1/cover-template/:id/validate", ValidateCoverTemplate)
	h.POST("/api/v1/cover-template/:id/publish", PublishCoverTemplate)
	h.PUT("/api/v1/cover-template/:id/usage", SetCoverTemplateUsage)
	h.POST("/api/v1/cover-template/:id/copy", CopyCoverTemplate)
	h.POST("/api/v1/cover/render", RenderCover)
	h.GET("/api/v1/image-font/:family/:weight", GetImageFont)
}
func ListCoverTemplate(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	f := repository.TemplateFilter{Search: c.Query("search"), ContentStatus: c.Query("content_status"), CategoryID: c.Query("category_id"), Platform: c.Query("platform"), Scope: c.Query("scope"), Page: 1, PageSize: 10}
	for name, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize, "width": &f.Width, "height": &f.Height} {
		if raw := c.Query(name); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil {
				failTemplate(c, service.ErrInvalid)
				return
			}
			*target = n
		}
	}
	if raw := c.Query("enabled"); raw != "" {
		if raw != "true" && raw != "false" {
			failTemplate(c, service.ErrInvalid)
			return
		}
		v := raw == "true"
		f.Enabled = &v
	}
	rows, total, e := service.WiredTemplate().ListCoverTemplate(a, f)
	if e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, map[string]any{"items": rows, "total": total, "page": f.Page, "page_size": f.PageSize})
}
func CreateCoverTemplate(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.CreateTemplateInput
	if e := decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	v, e := service.WiredTemplate().CreateCoverTemplate(ctx, a, in)
	if e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, v)
}
func GetCoverTemplate(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	v, e := service.WiredTemplate().GetCoverTemplate(a, id)
	if e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, v)
}
func SaveCoverTemplateDraft(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	var in service.DraftInput
	if e = decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	v, e := service.WiredTemplate().SaveCoverTemplateDraft(a, id, in)
	if e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, v)
}
func SetCoverTemplateUsage(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	var in service.UsageInput
	if e = decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	v, e := service.WiredTemplate().SetCoverTemplateUsage(a, id, in)
	if e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, v)
}
func DeleteCoverTemplate(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	revision, e := strconv.ParseUint(c.Query("expected_revision"), 10, 64)
	if e != nil {
		failTemplate(c, service.ErrInvalid)
		return
	}
	if e = service.WiredTemplate().DeleteCoverTemplate(a, id, revision); e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, map[string]any{"deleted": true})
}
func CopyCoverTemplate(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	var in service.CreateTemplateInput
	if e = decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	v, e := service.WiredTemplate().CopyCoverTemplate(ctx, a, id, in)
	if e != nil {
		failTemplate(c, e)
		return
	}
	api.Success(c, v)
}
func PublishCoverTemplate(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	var in service.TemplateCommand
	if e = decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	v, issues, e := service.WiredTemplate().PublishCoverTemplate(ctx, a, id, in)
	if e != nil {
		if len(issues) > 0 {
			api.FailureCode(c, 400, 19001, "模板校验未通过", "template_invalid", issues, false, "none")
		} else {
			failTemplate(c, e)
		}
		return
	}
	api.Success(c, map[string]any{"item": v, "issues": issues})
}
func ValidateCoverTemplate(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		failTemplate(c, e)
		return
	}
	var in service.ValidateTemplateInput
	if e = decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	s := service.WiredTemplate()
	v, e := s.GetCoverTemplate(a, id)
	if e != nil {
		failTemplate(c, e)
		return
	}
	if in.ExpectedRevision == 0 || v.Revision != in.ExpectedRevision {
		failTemplate(c, service.ErrConflict)
		return
	}
	if in.Document != nil {
		if e = render.ValidateDraft(*in.Document); e != nil {
			failTemplate(c, service.ErrInvalid)
			return
		}
	}
	if in.Title == "" {
		failTemplate(c, service.ErrInvalid)
		return
	}
	api.StreamOne(c, api.StreamItem{UserID: int64(a.ID), RowKey: strconv.FormatUint(id, 10), InputRevision: strconv.FormatUint(in.ExpectedRevision, 10)}, func(ctx context.Context, progress func(string) error) (any, error) {
		if e := progress("rendering"); e != nil {
			return nil, e
		}
		result, issues, e := s.ValidateCoverTemplate(ctx, a, id, in)
		if e != nil {
			return map[string]any{"issues": issues}, generationError(e)
		}
		return map[string]any{"artwork": result, "issues": issues}, nil
	})
}
func RenderCover(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.ArtworkInput
	if e := decodeInput(c, &in); e != nil {
		failTemplate(c, e)
		return
	}
	s := service.WiredTemplate()
	v, e := s.GetCoverTemplate(a, in.TemplateID)
	if e != nil {
		failTemplate(c, e)
		return
	}
	if !v.Enabled || v.PublishedRevision == 0 || v.PublishedRevision != in.PublishedRevision {
		failTemplate(c, service.ErrConflict)
		return
	}
	if in.Title == "" || in.RowKey == "" || in.InputRevision == "" {
		failTemplate(c, service.ErrInvalid)
		return
	}
	var doc render.Document
	if json.Unmarshal(v.PublishedConfig, &doc) != nil {
		failTemplate(c, service.ErrInvalid)
		return
	}
	api.StreamOne(c, api.StreamItem{UserID: int64(a.ID), RowKey: in.RowKey, InputRevision: in.InputRevision}, func(ctx context.Context, progress func(string) error) (any, error) {
		if e := progress("rendering"); e != nil {
			return nil, e
		}
		result, issues, e := s.RenderCover(ctx, a, in)
		if e != nil {
			return map[string]any{"issues": issues}, generationError(e)
		}
		return map[string]any{"artwork": result, "issues": issues}, nil
	})
}
func GetImageFont(_ context.Context, c *app.RequestContext) {
	if _, ok := middleware.AuthenticateRequest(c); !ok {
		return
	}
	data, e := render.FontData(c.Param("family"), c.Param("weight"))
	if e != nil {
		failTemplate(c, service.ErrInvalid)
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("ETag", `"`+render.Version+"-"+c.Param("family")+"-"+c.Param("weight")+`"`)
	c.Data(200, "font/otf", data)
}

func generationError(e error) error {
	if errors.Is(e, service.ErrInvalid) {
		return &api.StreamError{Code: "image_invalid", Message: strings.TrimPrefix(e.Error(), service.ErrInvalid.Error()+": ")}
	}
	if errors.Is(e, service.ErrConflict) {
		return &api.StreamError{Code: "image_conflict", Message: "模板已变化，请重新选择后生成"}
	}
	if errors.Is(e, service.ErrNotFound) {
		return &api.StreamError{Code: "image_not_found", Message: "图片或模板不存在或没有访问权限"}
	}
	return e
}

func failTemplate(c *app.RequestContext, e error) {
	if errors.Is(e, service.ErrInvalid) {
		message := "请检查模板名称、规格、适用范围和草稿内容"
		if e.Error() != service.ErrInvalid.Error() {
			message = strings.TrimPrefix(e.Error(), service.ErrInvalid.Error()+": ")
		}
		api.FailureCode(c, 400, 19001, message, "template_invalid", nil, false, "none")
		return
	}
	fail(c, e)
}
