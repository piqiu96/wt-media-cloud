package production

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func actor(c *hertzapp.RequestContext) (identityservice.PublicUser, bool) {
	return middleware.AuthenticateRequest(c)
}

func materialID(c *hertzapp.RequestContext) (int64, bool) {
	value, err := strconv.ParseInt(c.Param("material_id"), 10, 64)
	if err != nil || value <= 0 {
		api.BadRequest(c, 15001, "素材 ID 无效")
		return 0, false
	}
	return value, true
}

func usageID(c *hertzapp.RequestContext) (int64, bool) {
	value, err := strconv.ParseInt(c.Param("usage_id"), 10, 64)
	if err != nil || value <= 0 {
		api.BadRequest(c, 15004, "我的素材关系 ID 无效")
		return 0, false
	}
	return value, true
}

func ListMaterials(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListMaterials(actor, strings.TrimSpace(c.Query("search")))
	if err != nil {
		writeProductionError(c, err)
		return
	}
	api.Success(c, items)
}

func GetMaterial(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := materialID(c)
	if !ok {
		return
	}
	item, err := productionservice.GetMaterial(actor, id)
	if err != nil {
		writeProductionError(c, err)
		return
	}
	api.Success(c, item)
}

func AddMaterialUsage(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := materialID(c)
	if !ok {
		return
	}
	usage, err := productionservice.AddUsage(actor, id)
	if err != nil {
		writeProductionError(c, err)
		return
	}
	api.Created(c, usage)
}

func ListMyMaterials(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListMyMaterials(actor)
	if err != nil {
		writeProductionError(c, err)
		return
	}
	api.Success(c, items)
}

func RemoveMaterialUsage(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := usageID(c)
	if !ok {
		return
	}
	if err := productionservice.RemoveUsage(actor, id); err != nil {
		writeProductionError(c, err)
		return
	}
	// A real 204 with no body, not the 200 `api.NoContent` envelope: this is the
	// one route whose frozen contract says 204, and the frontend's
	// `parseResponse` unwraps a 200 `data:null` into a returned value, so the two
	// are observably different to its caller.
	api.NoContentEmpty(c)
}

func writeProductionError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, productionservice.ErrForbidden):
		api.Forbidden(c, 15002, "没有权限访问该素材")
	case errors.Is(err, productionservice.ErrUsageNotFound):
		api.NotFound(c, 15006, "我的素材中不存在该记录")
	case errors.Is(err, productionservice.ErrNotFound):
		api.NotFound(c, 15003, "素材不存在")
	case errors.Is(err, productionservice.ErrInvalidInput):
		api.BadRequest(c, 15001, "素材参数无效")
	default:
		api.InternalError(c, "素材生产服务内部错误")
	}
}
