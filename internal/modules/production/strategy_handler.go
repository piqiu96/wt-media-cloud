package production

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func strategyID(c *hertzapp.RequestContext) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("strategy_id"), 10, 64)
	if err != nil || id <= 0 {
		api.FailureNamed(c, consts.StatusBadRequest, 15210, "策略 ID 无效", "compose_strategy_invalid")
		return 0, false
	}
	return id, true
}

func decodeStrategyBody(c *hertzapp.RequestContext, dst any) bool {
	decoder := json.NewDecoder(bytes.NewReader(c.Request.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15210, "策略请求参数无效："+err.Error(), "compose_strategy_invalid")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		api.FailureNamed(c, consts.StatusBadRequest, 15210, "策略请求只能包含一个 JSON 对象", "compose_strategy_invalid")
		return false
	}
	return true
}

func ListComposeStrategies(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListComposeStrategies(current)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, items)
}

func GetComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	item, err := productionservice.GetComposeStrategy(current, id)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, item)
}

func CreateComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	var input productionservice.CreateStrategyInput
	if !decodeStrategyBody(c, &input) {
		return
	}
	item, err := productionservice.CreateComposeStrategy(current, input)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Created(c, item)
}

func UpdateComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	var input productionservice.UpdateStrategyInput
	if !decodeStrategyBody(c, &input) {
		return
	}
	item, err := productionservice.UpdateComposeStrategy(current, id, input)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, item)
}

func PreviewComposeStrategyNextRun(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	var schedule model.ComposeSchedule
	if !decodeStrategyBody(c, &schedule) {
		return
	}
	next, err := productionservice.PreviewComposeStrategyNextRun(current, schedule)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, map[string]any{"next_run_at": next})
}

func CopyComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decodeStrategyBody(c, &input) {
		return
	}
	item, err := productionservice.CopyComposeStrategy(current, id, input.Name)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Created(c, item)
}

func SetComposeStrategyStatus(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !decodeStrategyBody(c, &input) {
		return
	}
	item, err := productionservice.SetComposeStrategyStatus(current, id, input.Status)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, item)
}

func RunComposeStrategyNow(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	var input struct {
		RequestKey string `json:"request_key"`
	}
	if !decodeStrategyBody(c, &input) {
		return
	}
	result, err := productionservice.RunComposeStrategyNow(current, id, input.RequestKey)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, result)
}

func ListComposeStrategyRuns(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	runs, err := productionservice.ListComposeStrategyRuns(current, id)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, runs)
}

func ListComposeStrategyRunTasks(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	slot, err := time.Parse(time.RFC3339, strings.TrimSpace(c.Query("schedule_slot")))
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15210, "批次计划时间无效", "compose_strategy_invalid")
		return
	}
	view, err := productionservice.GetComposeStrategyBatch(current, id, slot)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, view)
}

func ListComposeStrategyMaterialStates(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	ids, err := parseMaterialStateIDs(c.Query("material_ids"))
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15210, err.Error(), "compose_strategy_invalid")
		return
	}
	states, err := productionservice.ListComposeStrategyMaterialStates(current, id, ids)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, states)
}

// parseMaterialStateIDs reads the comma-separated material ids of one
// material-states read. It rejects a missing or malformed list instead of
// treating it as empty: an empty list would come back as "nothing was ever
// used", which is the one wrong answer the caller cannot detect.
func parseMaterialStateIDs(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("material_ids 不能为空")
	}
	parts := strings.Split(trimmed, ",")
	if len(parts) > productionservice.ComposeMaterialStateLimit {
		return nil, fmt.Errorf("material_ids 最多 %d 个素材", productionservice.ComposeMaterialStateLimit)
	}
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || value <= 0 {
			return nil, errors.New("material_ids 必须是正整数，用英文逗号分隔")
		}
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	slices.Sort(ids)
	return ids, nil
}

func writeStrategyError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, productionservice.ErrStrategyInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, 15210, err.Error(), "compose_strategy_invalid")
	case errors.Is(err, productionservice.ErrStrategyForbidden):
		api.ForbiddenNamed(c, 15211, "没有权限操作合成策略", "compose_strategy_forbidden")
	case errors.Is(err, productionservice.ErrStrategyNotFound), errors.Is(err, repository.ErrNotFound):
		api.FailureNamed(c, consts.StatusNotFound, 15212, "合成策略不存在", "compose_strategy_not_found")
	case errors.Is(err, productionservice.ErrStrategyTemplateUnavailable):
		api.ConflictNamed(c, 15213, "合成模板版本不可用", "compose_template_unavailable")
	default:
		api.InternalError(c, "合成策略服务内部错误")
	}
}
