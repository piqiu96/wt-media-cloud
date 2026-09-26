package production

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

func TestRegisterRoutesBindsMaterialEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, request := range []struct {
		method string
		path   string
	}{
		{consts.MethodGet, "/api/v1/materials"},
		{consts.MethodGet, "/api/v1/materials/42"},
		{consts.MethodPost, "/api/v1/materials/42/usages"},
		{consts.MethodGet, "/api/v1/my-materials"},
		{consts.MethodDelete, "/api/v1/material-usages/5"},
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Result().StatusCode())
		}
	}
}

// The two "not found" outcomes in this module are different answers to the user,
// and this mapping is the only place that decides which errcode carries them.
//
// Driven directly rather than through the route: reaching `RemoveMaterialUsage`
// past `actor(c)` needs a live session row, and the service call behind it needs
// MySQL, so the success path cannot be exercised by a unit test at all. The
// mapping needs no request — but it is asserted on a real response, because the
// errcode is what the frontend branches on and a status-only assertion would not
// notice the two being swapped.
func TestWriteProductionErrorKeepsTheTwoMissingResourcesApart(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		err     error
		errcode int
	}{
		{"missing usage", productionservice.ErrUsageNotFound, 15006},
		{"missing material", productionservice.ErrNotFound, 15003},
	} {
		engine := server.New()
		engine.GET("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
			writeProductionError(c, testCase.err)
		})
		result := ut.PerformRequest(engine.Engine, consts.MethodGet, "/probe", nil)

		if got := result.Result().StatusCode(); got != consts.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", testCase.name, got)
		}
		var body struct {
			ErrCode int `json:"errcode"`
		}
		if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
			t.Fatalf("%s: decode response: %v", testCase.name, err)
		}
		if body.ErrCode != testCase.errcode {
			t.Fatalf("%s: errcode = %d, want %d", testCase.name, body.ErrCode, testCase.errcode)
		}
	}
}

// The delete route must answer through the empty-204 helper, and this is a source
// assertion because no unit test can reach it: `RemoveMaterialUsage` authenticates
// through `actor(c)` and then calls the service, which goes to MySQL. What the
// assertion protects is exactly what a regression would hide — `api.NoContent`
// answers 200 with a `data:null` envelope, which the frontend's `parseResponse`
// unwraps into a value, so the two are observably different to its caller while
// both read as "success" in a status-only test.
//
// The two spellings are distinguishable by substring: `api.NoContent(c)` does not
// occur inside `api.NoContentEmpty(c)`. Verified by mutation — switching the
// handler back to `api.NoContent(c)` turns this red.
func TestRemoveUsageRouteAnswersThroughTheEmpty204Helper(t *testing.T) {
	source, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("read production handler: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "api.NoContentEmpty(c)") {
		t.Error("the delete route must answer 204 with no body")
	}
	if strings.Contains(text, "api.NoContent(c)") {
		t.Error("a 200 data:null envelope is not the 204 this route's contract declares")
	}
}

// The one route in this module that answers two success statuses, asserted on
// real responses. The frozen contract publishes both, and the frontend cannot
// tell them apart from the body — so if this mapping collapsed, a repeat click
// would answer 201 forever and no other test in either repository would notice.
func TestWriteUsageAddedKeepsTheTwoSuccessStatusesApart(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		created bool
		want    int
	}{
		{"already active", false, consts.StatusOK},
		{"created or restored", true, consts.StatusCreated},
	} {
		engine := server.New()
		engine.POST("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
			writeUsageAdded(c, map[string]int64{"id": 11}, testCase.created)
		})
		result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/probe", nil)
		if result.Result().StatusCode() != testCase.want {
			t.Fatalf("%s: status = %d body=%s, want %d", testCase.name, result.Result().StatusCode(), result.Result().Body(), testCase.want)
		}
	}
}
