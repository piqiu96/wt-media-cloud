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

func TestStrategyPageQueryParsesAllDimensionsAndRejectsMalformedValues(t *testing.T) {
	parse := func(raw string) (productionservice.ComposeStrategyListQuery, error) {
		var query productionservice.ComposeStrategyListQuery
		var err error
		engine := server.New()
		engine.GET("/probe", func(_ context.Context, c *hertzapp.RequestContext) { query, err = parseStrategyListQuery(c) })
		ut.PerformRequest(engine.Engine, "GET", "/probe?"+raw, nil)
		return query, err
	}
	query, err := parse("page=3&page_size=100&team_id=7&operator_id=9&category_id=1&template_id=3&status=enabled&search=older")
	if err != nil || query.Page != 3 || query.PageSize != 100 || query.TeamID != 7 || query.OperatorID != 9 || query.CategoryID != 1 || query.TemplateID != 3 || query.Status != "enabled" || query.Search != "older" {
		t.Fatalf("query=%+v err=%v", query, err)
	}
	empty, err := parse("")
	if err != nil || empty.Page != 1 || empty.PageSize != 20 {
		t.Fatalf("default=%+v err=%v", empty, err)
	}
	for _, bad := range []string{"page=0", "page=-1", "page=oops", "page_size=0", "page_size=101", "team_id=0", "team_id=-1", "operator_id=bad", "operator_id=-1", "category_id=bad", "template_id=0"} {
		if _, err := parse(bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
