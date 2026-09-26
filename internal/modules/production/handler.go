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
	usage, created, err := productionservice.AddUsage(actor, id)
	if err != nil {
		writeProductionError(c, err)
		return
	}
	writeUsageAdded(c, usage, created)
}

// CreateMaterialDownload queues a download of a prepared material to the actor's
// own Local Agent, and answers 202 because nothing has been downloaded yet: the
// task is pending until a node claims it, and the client's next honest source of
// progress is `GET /api/v1/file-transfer-tasks`.
func CreateMaterialDownload(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := materialID(c)
	if !ok {
		return
	}
	task, err := productionservice.CreateDownload(actor, id)
	if err != nil {
		writeProductionError(c, err)
		return
	}
	api.Accepted(c, task)
}

// writeUsageAdded answers 201 for a new or restored relation and 200 for a click
// on one that was already active. The bodies are identical, which is exactly why
// the service has to report the difference: the frontend's `parseResponse`
// returns `data` either way, so nothing downstream can recover the distinction
// from the response.
//
// Split out from the handler so the mapping is reachable from a test: getting
// past `actor(c)` needs a live session row, but the decision itself needs only
// the flag.
func writeUsageAdded(c *hertzapp.RequestContext, usage any, created bool) {
	if !created {
		api.Success(c, usage)
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
	case errors.Is(err, productionservice.ErrUsageForbidden):
		api.ForbiddenNamed(c, 15007, "没有权限操作该素材关系", "material_usage_forbidden")
	case errors.Is(err, productionservice.ErrMaterialUnavailable):
		api.ConflictNamed(c, 15005, "素材视频尚未准备好", "material_unavailable")
	case errors.Is(err, productionservice.ErrLocalNodeUnavailable):
		api.ConflictNamed(c, 15105, "本机没有可用的下载节点", "local_transfer_node_unavailable")
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
