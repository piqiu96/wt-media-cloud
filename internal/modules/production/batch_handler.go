package production

import (
	"context"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"strconv"
	"strings"
)

func parseTaskListInput(c *hertzapp.RequestContext) (productionservice.ListComposeTasksInput, error) {
	in := productionservice.ListComposeTasksInput{Status: strings.TrimSpace(c.Query("status")), TriggerType: strings.TrimSpace(c.Query("trigger_type")), Search: strings.TrimSpace(c.Query("search"))}
	team, operator, scope, err := parseComposeListScope(c)
	if err != nil {
		return in, err
	}
	in.TeamID, in.OperatorID, in.OperatorScope = team, operator, scope
	in.CreatedFrom, in.CreatedBefore, err = parseComposeCreatedRange(strings.TrimSpace(c.Query("created_from")), strings.TrimSpace(c.Query("created_before")))
	if err != nil {
		return in, err
	}
	for key, dest := range map[string]*int64{"batch_id": &in.BatchID, "compose_strategy_id": &in.StrategyID} {
		if raw := strings.TrimSpace(c.Query(key)); raw != "" {
			*dest, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || *dest <= 0 {
				return in, productionservice.ErrComposeTaskInvalid
			}
		}
	}
	in.Page, err = strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		return in, err
	}
	in.PageSize, err = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return in, err
}
func ListComposeTaskBatches(_ context.Context, c *hertzapp.RequestContext) {
	a, ok := actor(c)
	if !ok {
		return
	}
	in, err := parseTaskListInput(c)
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "筛选参数无效", "compose_task_invalid")
		return
	}
	result, err := productionservice.ListComposeTaskBatches(a, in)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, result)
}
func GetComposeTaskBatch(_ context.Context, c *hertzapp.RequestContext) {
	a, ok := actor(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("batch_id"), 10, 64)
	if err != nil || id <= 0 {
		writeComposeTaskError(c, productionservice.ErrComposeTaskInvalid)
		return
	}
	result, err := productionservice.GetComposeTaskBatch(a, id)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, result)
}
func bulkComposeTasks(c *hertzapp.RequestContext, retry bool) {
	a, ok := actor(c)
	if !ok {
		return
	}
	var in productionservice.ComposeBulkInput
	if !decodeTaskBody(c, &in) {
		return
	}
	result, err := productionservice.BulkComposeTasks(a, in, retry)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, result)
}
func BatchRetryComposeTasks(_ context.Context, c *hertzapp.RequestContext) { bulkComposeTasks(c, true) }
func BatchCancelComposeTasks(_ context.Context, c *hertzapp.RequestContext) {
	bulkComposeTasks(c, false)
}
