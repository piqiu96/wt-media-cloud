package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
)

func RegisterRoutes(h *server.Hertz) {
	registerTemplateRoutes(h)
	h.GET("/api/v1/image-asset", ListImageAsset)
	h.POST("/api/v1/image-asset", RegisterImageAsset)
	h.POST("/api/v1/image-asset/select", SelectImageAsset)
	h.POST("/api/v1/image-asset/batch", BatchImageAsset)
	h.GET("/api/v1/image-asset/:id/reference", ListImageAssetReference)
	h.DELETE("/api/v1/image-asset/:id", DeleteImageAsset)
	h.GET("/api/v1/image-asset/:id", GetImageAsset)
	h.PUT("/api/v1/image-asset/:id", UpdateImageAsset)
	h.PUT("/api/v1/image-asset/:id/usage", SetImageAssetUsage)
}
func fail(c *app.RequestContext, e error) {
	switch {
	case errors.Is(e, service.ErrInvalid), errors.Is(e, files.ErrInvalid):
		api.FailureCode(c, consts.StatusBadRequest, errcode.ImageInvalid, "请检查图片名称、用途、类目和尺寸要求", "image_invalid", nil, false, "none")
	case errors.Is(e, service.ErrNotFound), errors.Is(e, files.ErrNotFound), errors.Is(e, files.ErrForbidden):
		api.FailureCode(c, consts.StatusNotFound, errcode.ImageNotFound, "图片不存在或没有访问权限", "image_not_found", nil, false, "none")
	case errors.Is(e, files.ErrInUse):
		api.FailureCode(c, consts.StatusConflict, errcode.ImageConflict, "图片被模板或正式任务引用，请先停用", "image_in_use", nil, false, "none")
	case errors.Is(e, service.ErrConflict):
		api.FailureCode(c, consts.StatusConflict, errcode.ImageConflict, "图片已变化或该文件已登记，请重新加载", "image_conflict", nil, false, "reload")
	default:
		api.FailureCode(c, consts.StatusServiceUnavailable, errcode.ImageUnavailable, "图片操作未完成，请保留当前内容后重试", "image_unavailable", nil, true, "retry")
	}
}
func decodeInput(c *app.RequestContext, in any) error {
	d := json.NewDecoder(bytes.NewReader(c.Request.Body()))
	d.DisallowUnknownFields()
	if e := d.Decode(in); e != nil {
		return service.ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return service.ErrInvalid
	}
	return nil
}
func assetID(c *app.RequestContext) (uint64, error) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil || id == 0 {
		return 0, service.ErrInvalid
	}
	return id, nil
}
func ListImageAsset(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	f := repository.Filter{Search: c.Query("search"), Usage: c.Query("usage"), CategoryID: c.Query("category_id"), Aspect: c.Query("aspect"), Source: c.Query("source"), Page: 1, PageSize: 20}
	for name, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize} {
		if raw := c.Query(name); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil {
				fail(c, service.ErrInvalid)
				return
			}
			*target = v
		}
	}
	if raw := c.Query("enabled"); raw != "" {
		if raw != "true" && raw != "false" {
			fail(c, service.ErrInvalid)
			return
		}
		v := raw == "true"
		f.Enabled = &v
	}
	rows, total, e := service.Wired().ListImageAsset(a, f)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, map[string]any{"items": rows, "total": total, "page": f.Page, "page_size": f.PageSize})
}
func RegisterImageAsset(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.RegisterInput
	if e := decodeInput(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().RegisterImageAsset(ctx, a, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func GetImageAsset(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().GetImageAsset(a, id)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func UpdateImageAsset(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		fail(c, e)
		return
	}
	var in service.UpdateInput
	if e = decodeInput(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().UpdateImageAsset(a, id, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func SetImageAssetUsage(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		fail(c, e)
		return
	}
	var in service.UsageInput
	if e = decodeInput(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().SetImageAssetUsage(a, id, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func SelectImageAsset(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.SelectInput
	if e := decodeInput(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().SelectImageAsset(a, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}

func BatchImageAsset(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.BatchInput
	if e := decodeInput(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().BatchImageAsset(a, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, map[string]any{"items": v})
}
func ListImageAssetReference(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := service.Wired().ListImageAssetReference(a, id)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, map[string]any{"items": v})
}
func DeleteImageAsset(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := assetID(c)
	if e != nil {
		fail(c, e)
		return
	}
	revision, e := strconv.ParseUint(c.Query("expected_revision"), 10, 64)
	if e != nil {
		fail(c, service.ErrInvalid)
		return
	}
	if e = service.Wired().DeleteImageAsset(a, id, revision); e != nil {
		fail(c, e)
		return
	}
	api.Success(c, map[string]any{"deleted": true})
}
