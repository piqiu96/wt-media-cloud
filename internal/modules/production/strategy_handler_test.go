package production

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

func TestStrategyFailuresUseContractNames(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		name   string
	}{
		{productionservice.ErrStrategyInvalid, 400, "compose_strategy_invalid"},
		{productionservice.ErrStrategyForbidden, 403, "compose_strategy_forbidden"},
		{productionservice.ErrStrategyNotFound, 404, "compose_strategy_not_found"},
		{productionservice.ErrStrategyTemplateUnavailable, 409, "compose_template_unavailable"},
	} {
		engine := server.New()
		engine.GET("/probe", func(_ context.Context, c *hertzapp.RequestContext) { writeStrategyError(c, tc.err) })
		response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/probe", nil)
		var body struct {
			Error *struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Result().Body(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Result().StatusCode() != tc.status || body.Error == nil || body.Error.Type != tc.name {
			t.Fatalf("error=%v status=%d body=%s", tc.err, response.Result().StatusCode(), response.Result().Body())
		}
	}
}

func TestStrategyBodyRejectsFieldsOutsideContract(t *testing.T) {
	engine := server.New()
	engine.POST("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
		var input productionservice.CreateStrategyInput
		if decodeStrategyBody(c, &input) {
			t.Fatal("unknown field accepted")
		}
	})
	body := `{"name":"x","future_nodes":[]}`
	response := ut.PerformRequest(engine.Engine, consts.MethodPost, "/probe", &ut.Body{Body: strings.NewReader(body), Len: len(body)})
	if response.Result().StatusCode() != consts.StatusBadRequest || !json.Valid(response.Result().Body()) {
		t.Fatalf("status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}
