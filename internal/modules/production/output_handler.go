package production

import (
	"context"
	"errors"
	"strconv"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func outputID(c *hertzapp.RequestContext) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("output_id"), 10, 64)
	if err != nil || id <= 0 {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片 ID 无效", "compose_output_invalid")
		return 0, false
	}
	return id, true
}

// writeComposeOutputError is the single mapping from an output-service error
// to the frozen contract pair, the same way the compose-task handler maps.
func writeComposeOutputError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, productionservice.ErrComposeOutputInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片请求参数无效："+err.Error(), "compose_output_invalid")
	case errors.Is(err, productionservice.ErrComposeOutputForbidden):
		api.FailureNamed(c, consts.StatusForbidden, 15215, "没有权限操作该成片", "compose_output_forbidden")
	case errors.Is(err, productionservice.ErrComposeOutputNotFound), errors.Is(err, repository.ErrNotFound):
		api.FailureNamed(c, consts.StatusNotFound, 15216, "成片不存在", "compose_output_not_found")
	case errors.Is(err, productionservice.ErrComposeOutputNotArchivable):
		api.FailureNamed(c, consts.StatusConflict, 15218, "成片已发布或已归档，不能归档", "compose_output_not_archivable")
	// The download route's own conflict, the same frozen name the material
	// download answers with: the client branches on the name, not the errcode.
	case errors.Is(err, productionservice.ErrLocalNodeUnavailable):
		api.ConflictNamed(c, 15105, "本机没有可用的下载节点", "local_transfer_node_unavailable")
	case errors.Is(err, productionservice.ErrInvalidInput):
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片请求参数无效："+err.Error(), "compose_output_invalid")
	default:
		api.InternalError(c, "成片服务内部错误")
	}
}

func ListComposeOutputs(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListComposeOutputs(current, productionservice.ListComposeOutputsInput{PublishStatus: c.Query("publish_status")})
	if err != nil {
		writeComposeOutputError(c, err)
		return
	}
	api.Success(c, items)
}

func GetComposeOutput(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := outputID(c)
	if !ok {
		return
	}
	item, err := productionservice.GetComposeOutput(current, id)
	if err != nil {
		writeComposeOutputError(c, err)
		return
	}
	api.Success(c, item)
}

func ArchiveComposeOutput(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := outputID(c)
	if !ok {
		return
	}
	item, err := productionservice.ArchiveComposeOutput(current, id)
	if err != nil {
		writeComposeOutputError(c, err)
		return
	}
	api.Success(c, item)
}

func GetComposeOutputVideoURL(ctx context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := outputID(c)
	if !ok {
		return
	}
	url, err := productionservice.ComposeOutputVideoURL(ctx, current, id)
	if err != nil {
		writeComposeOutputError(c, err)
		return
	}
	api.Success(c, VideoLink{URL: url})
}

// CreateOutputDownload queues a download of a finished compose output to the
// actor's own Local Agent, and answers 202 for the same reason the material
// download does: nothing has been downloaded yet, and the client's next honest
// source of progress is `GET /api/v1/file-transfer-tasks`.
func CreateOutputDownload(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := outputID(c)
	if !ok {
		return
	}
	task, err := productionservice.CreateOutputDownload(current, id)
	if err != nil {
		writeComposeOutputError(c, err)
		return
	}
	api.Accepted(c, task)
}
