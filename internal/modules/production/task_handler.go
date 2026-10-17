package production

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func taskID(c *hertzapp.RequestContext) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil || id <= 0 {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "合成任务 ID 无效", "compose_task_invalid")
		return 0, false
	}
	return id, true
}

func decodeTaskBody(c *hertzapp.RequestContext, dst any) bool {
	decoder := json.NewDecoder(bytes.NewReader(c.Request.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "合成任务请求参数无效："+err.Error(), "compose_task_invalid")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "合成任务请求只能包含一个 JSON 对象", "compose_task_invalid")
		return false
	}
	return true
}

type composeTaskErrorShape struct {
	status  int
	errcode int
	name    string
	message string
}

// composeTaskErrorShape is the single mapping from a service error to the
// frozen contract pair — the errcode the frontend branches on and the error
// name the contract publishes. Both the whole-command error writer and the
// per-item failure payloads go through it, so the two can never disagree.
func composeTaskErrorFor(err error) (composeTaskErrorShape, bool) {
	switch {
	case errors.Is(err, productionservice.ErrComposeTaskInvalid):
		return composeTaskErrorShape{consts.StatusBadRequest, 15214, "compose_task_invalid", err.Error()}, true
	case errors.Is(err, productionservice.ErrComposeTaskForbidden):
		return composeTaskErrorShape{consts.StatusForbidden, 15215, "compose_task_forbidden", "没有权限操作该合成任务"}, true
	case errors.Is(err, productionservice.ErrComposeTaskNotFound), errors.Is(err, repository.ErrNotFound):
		return composeTaskErrorShape{consts.StatusNotFound, 15216, "compose_task_not_found", "合成任务不存在"}, true
	case errors.Is(err, productionservice.ErrComposeTaskNotCancellable):
		return composeTaskErrorShape{consts.StatusConflict, 15217, "compose_task_not_cancellable", "合成任务已结束，不能取消"}, true
	case errors.Is(err, productionservice.ErrComposeTaskNotRetryable):
		return composeTaskErrorShape{consts.StatusConflict, 15217, "compose_task_not_retryable", "该任务当前状态不支持重试"}, true
	case errors.Is(err, productionservice.ErrComposeTaskConflict):
		return composeTaskErrorShape{consts.StatusConflict, 15219, "compose_task_conflict", "该素材在这个策略下已有任务记录，不能再次创建；如需重跑请在任务列表点「重试」"}, true
	case errors.Is(err, productionservice.ErrStrategyTemplateUnavailable):
		return composeTaskErrorShape{consts.StatusConflict, 15213, "compose_template_unavailable", "合成模板版本不可用"}, true
	}
	return composeTaskErrorShape{}, false
}

func writeComposeTaskError(c *hertzapp.RequestContext, err error) {
	shape, ok := composeTaskErrorFor(err)
	if !ok {
		api.InternalError(c, "合成任务服务内部错误")
		return
	}
	api.FailureNamed(c, shape.status, shape.errcode, shape.message, shape.name)
}

type composeTaskFailurePayload struct {
	MaterialID int64  `json:"material_id"`
	RequestKey string `json:"request_key"`
	Errcode    int    `json:"errcode"`
	Error      string `json:"error"`
	Message    string `json:"message"`
}

func composeTaskFailurePayloads(failed []productionservice.CreateComposeTaskFailure) []composeTaskFailurePayload {
	out := make([]composeTaskFailurePayload, 0, len(failed))
	for _, item := range failed {
		shape, ok := composeTaskErrorFor(item.Err)
		if !ok {
			shape = composeTaskErrorShape{errcode: 0, name: "internal_error", message: item.Err.Error()}
		}
		message := shape.message
		if item.Message != "" {
			message = item.Message
		}
		out = append(out, composeTaskFailurePayload{MaterialID: item.MaterialID, RequestKey: item.RequestKey, Errcode: shape.errcode, Error: shape.name, Message: message})
	}
	return out
}

func writeComposeTaskCreated(c *hertzapp.RequestContext, result productionservice.CreateComposeTasksResult) {
	api.Success(c, struct {
		Created []model.ComposeTask         `json:"created"`
		Failed  []composeTaskFailurePayload `json:"failed"`
	}{Created: result.Created, Failed: composeTaskFailurePayloads(result.Failed)})
}

func ListComposeTasks(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	input := productionservice.ListComposeTasksInput{Status: strings.TrimSpace(c.Query("status")), TriggerType: strings.TrimSpace(c.Query("trigger_type"))}
	from, before, err := parseComposeCreatedRange(strings.TrimSpace(c.Query("created_from")), strings.TrimSpace(c.Query("created_before")))
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15214, "合成任务时间范围无效", "compose_task_invalid")
		return
	}
	input.CreatedFrom, input.CreatedBefore = from, before
	if raw := strings.TrimSpace(c.Query("compose_strategy_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			api.FailureNamed(c, consts.StatusBadRequest, 15214, "合成任务筛选参数无效", "compose_task_invalid")
			return
		}
		input.StrategyID = value
	}
	items, err := productionservice.ListComposeTasks(current, input)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, items)
}

func CreateComposeTasks(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	var input productionservice.CreateComposeTasksInput
	if !decodeTaskBody(c, &input) {
		return
	}
	result, err := productionservice.CreateComposeTasks(current, input)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	writeComposeTaskCreated(c, result)
}

func GetComposeTask(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := taskID(c)
	if !ok {
		return
	}
	item, err := productionservice.GetComposeTask(current, id)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, item)
}

func ListComposeTaskAttempts(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := taskID(c)
	if !ok {
		return
	}
	items, err := productionservice.ListComposeTaskAttempts(current, id)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, items)
}

func ListComposeUncertainReviews(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := taskID(c)
	if !ok {
		return
	}
	items, err := productionservice.ListComposeUncertainReviews(current, id)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, items)
}

func ResolveComposeUncertain(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := taskID(c)
	if !ok {
		return
	}
	var input productionservice.ResolveComposeUncertainInput
	if !decodeTaskBody(c, &input) {
		return
	}
	result, err := productionservice.ResolveComposeUncertain(current, id, input)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, result)
}

func RetryComposeTask(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := taskID(c)
	if !ok {
		return
	}
	item, err := productionservice.RetryComposeTask(current, id)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, item)
}

func CancelComposeTask(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := taskID(c)
	if !ok {
		return
	}
	item, err := productionservice.CancelComposeTask(current, id)
	if err != nil {
		writeComposeTaskError(c, err)
		return
	}
	api.Success(c, item)
}
