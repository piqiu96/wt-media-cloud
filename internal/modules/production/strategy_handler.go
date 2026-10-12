package production

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type updateStrategyRequest struct {
	Strategy        productionservice.StrategyInput `json:"strategy"`
	ExpectedVersion uint32                          `json:"expected_version"`
}

type copyStrategyRequest struct {
	Name string `json:"name"`
}

func decodeStrategyRequest(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func bindStrategyRequest(c *hertzapp.RequestContext, target any) bool {
	if err := decodeStrategyRequest(c.Request.Body(), target); err != nil {
		writeStrategyError(c, productionservice.ErrStrategyInvalid)
		return false
	}
	return true
}

func strategyID(c *hertzapp.RequestContext) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("strategy_id"), 10, 64)
	if err != nil || id <= 0 {
		writeStrategyError(c, productionservice.ErrStrategyInvalid)
		return 0, false
	}
	return id, true
}

func ListComposeTemplates(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListStrategyTemplates(actor)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, items)
}

func ListComposeStrategies(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListStrategies(actor)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, items)
}

func GetComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	item, err := productionservice.GetStrategy(actor, id)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, item)
}

func CreateComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var input productionservice.StrategyInput
	if !bindStrategyRequest(c, &input) {
		return
	}
	item, err := productionservice.CreateStrategy(actor, input)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Created(c, item)
}

func UpdateComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	var req updateStrategyRequest
	if !bindStrategyRequest(c, &req) {
		return
	}
	item, err := productionservice.UpdateStrategy(actor, id, req.ExpectedVersion, req.Strategy)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, item)
}

func CopyComposeStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	var req copyStrategyRequest
	if !bindStrategyRequest(c, &req) {
		return
	}
	item, err := productionservice.CopyStrategy(actor, id, req.Name)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Created(c, item)
}

func ListComposeStrategyVersions(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := strategyID(c)
	if !ok {
		return
	}
	items, err := productionservice.ListStrategyVersions(actor, id)
	if err != nil {
		writeStrategyError(c, err)
		return
	}
	api.Success(c, items)
}

func writeStrategyError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, productionservice.ErrStrategyInvalid):
		api.FailureNamed(c, 400, 15201, "合成策略参数无效", "compose_strategy_invalid")
	case errors.Is(err, productionservice.ErrStrategyForbidden):
		api.ForbiddenNamed(c, 15202, "没有权限操作合成策略", "compose_strategy_forbidden")
	case errors.Is(err, productionservice.ErrStrategyNotFound):
		api.FailureNamed(c, 404, 15203, "合成策略不存在", "compose_strategy_not_found")
	case errors.Is(err, productionservice.ErrStrategyConflict):
		api.ConflictNamed(c, 15204, "策略已被更新，请重新读取", "compose_strategy_version_conflict")
	default:
		api.InternalError(c, "合成策略服务内部错误")
	}
}
