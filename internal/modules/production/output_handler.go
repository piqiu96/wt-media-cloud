package production

import (
	"context"
	"errors"
	"strconv"
	"strings"

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
	case errors.Is(err, productionservice.ErrComposeOutputUnavailable):
		api.FailureNamed(c, consts.StatusConflict, 15240, "成片文件不可用", "compose_output_unavailable")
	case errors.Is(err, productionservice.ErrComposeOutputVerificationFailed):
		api.FailureNamed(c, consts.StatusServiceUnavailable, 15241, "暂时无法核验成片文件，请稍后重试", "compose_output_verification_failed")
	case errors.Is(err, productionservice.ErrComposeOutputInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片请求参数无效："+err.Error(), "compose_output_invalid")
	case errors.Is(err, productionservice.ErrComposeOutputForbidden):
		api.FailureNamed(c, consts.StatusForbidden, 15215, "没有权限操作该成片", "compose_output_forbidden")
	case errors.Is(err, productionservice.ErrComposeOutputNotFound), errors.Is(err, repository.ErrNotFound):
		api.FailureNamed(c, consts.StatusNotFound, 15216, "成片不存在", "compose_output_not_found")
	case errors.Is(err, productionservice.ErrComposeOutputNotArchivable):
		api.FailureNamed(c, consts.StatusConflict, 15218, "成片已发布或已归档，不能归档", "compose_output_not_archivable")
	case errors.Is(err, productionservice.ErrInvalidInput):
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片请求参数无效："+err.Error(), "compose_output_invalid")
	default:
		api.InternalError(c, "成片服务内部错误")
	}
}

func ListComposeOutputs(ctx context.Context, c *hertzapp.RequestContext) {
	listOutputPage(ctx, c, false)
}
func ListComposeOutputMaterials(ctx context.Context, c *hertzapp.RequestContext) {
	listOutputPage(ctx, c, true)
}
func listOutputPage(ctx context.Context, c *hertzapp.RequestContext, materials bool) {
	current, ok := actor(c)
	if !ok {
		return
	}
	teamID, operatorID, scope, err := parseComposeListScope(c)
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片筛选参数无效："+err.Error(), "compose_output_invalid")
		return
	}
	from, before, err := parseComposeCreatedRange(strings.TrimSpace(c.Query("created_from")), strings.TrimSpace(c.Query("created_before")))
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "成片时间范围无效", "compose_output_invalid")
		return
	}
	categoryID, err := parseOptionalCategoryID(c.Query("category_id"))
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "类目 ID 无效", "compose_output_invalid")
		return
	}
	page, size := 1, 20
	for key, target := range map[string]*int{"page": &page, "page_size": &size} {
		if raw := c.Query(key); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v <= 0 || (key == "page_size" && v > 100) {
				writeComposeOutputError(c, productionservice.ErrComposeOutputInvalid)
				return
			}
			*target = v
		}
	}
	materialID, batchID := int64(0), int64(0)
	for key, target := range map[string]*int64{"material_id": &materialID, "batch_id": &batchID} {
		if raw := c.Query(key); raw != "" {
			v, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || v <= 0 {
				writeComposeOutputError(c, productionservice.ErrComposeOutputInvalid)
				return
			}
			*target = v
		}
	}
	input := productionservice.ListComposeOutputsInput{PublishStatus: c.Query("publish_status"), Source: strings.TrimSpace(c.Query("source")), Search: strings.TrimSpace(c.Query("search")), TriggerType: strings.TrimSpace(c.Query("trigger_type")), CategoryID: categoryID, TeamID: teamID, OperatorID: operatorID, OperatorScope: scope, CreatedFrom: from, CreatedBefore: before, Page: page, PageSize: size, MaterialID: materialID, BatchID: batchID, Platform: c.Query("platform"), VideoStatus: c.Query("video_status")}
	if raw := c.Query("category_ids"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, e := parseOptionalCategoryID(strings.TrimSpace(part))
			if e != nil || id <= 0 {
				writeComposeOutputError(c, productionservice.ErrComposeOutputInvalid)
				return
			}
			input.CategoryIDs = append(input.CategoryIDs, id)
		}
	}
	var items any
	if materials {
		items, err = productionservice.ListComposeOutputMaterials(ctx, current, input)
	} else {
		items, err = productionservice.ListComposeOutputPage(ctx, current, input)
	}
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
