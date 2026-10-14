package production

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	model "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
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
		{consts.MethodGet, "/api/v1/materials/42/video-url"},
		{consts.MethodPost, "/api/v1/materials/42/usages"},
		{consts.MethodPost, "/api/v1/materials/42/downloads"},
		{consts.MethodGet, "/api/v1/my-materials"},
		{consts.MethodDelete, "/api/v1/material-usages/5"},
		{consts.MethodPost, "/api/v1/material-usages/5/restore"},
		{consts.MethodGet, "/api/v1/compose-templates"},
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Result().StatusCode())
		}
	}
}

func TestComposeStrategyRoutesRequireAuthentication(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, request := range []struct {
		method string
		path   string
	}{
		{consts.MethodGet, "/api/v1/compose-strategies"},
		{consts.MethodPost, "/api/v1/compose-strategies"},
		{consts.MethodGet, "/api/v1/compose-strategies/5"},
		{consts.MethodPut, "/api/v1/compose-strategies/5"},
		{consts.MethodPost, "/api/v1/compose-strategies/5/copy"},
		{consts.MethodPatch, "/api/v1/compose-strategies/5/status"},
		{consts.MethodPut, "/api/v1/compose-strategies/5/schedule"},
		{consts.MethodPost, "/api/v1/compose-strategies/5/run-now"},
		{consts.MethodGet, "/api/v1/compose-strategies/5/runs"},
		{consts.MethodGet, "/api/v1/compose-strategies/5/runs/tasks"},
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Result().StatusCode())
		}
	}
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/compose-strategies/5/versions", nil)
	if response.Result().StatusCode() != consts.StatusNotFound {
		t.Fatalf("legacy version route status = %d, want 404", response.Result().StatusCode())
	}
}

func TestUncertainReviewRoutesRequireAuthentication(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, method := range []string{consts.MethodGet, consts.MethodPost} {
		response := ut.PerformRequest(engine.Engine, method, "/api/v1/compose-tasks/42/uncertain-review", nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s uncertain-review status = %d, want 401", method, response.Result().StatusCode())
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

// The two 409s on the download route are a different pair to tell apart: both
// mean "not now", and a client reading only the frozen contract — which publishes
// names and statuses and no numeric errcodes — has exactly one field to branch on.
// So this asserts that field.
//
// A status-only assertion would pass with the two names swapped, and an errcode
// assertion would pass with either name; only the name distinguishes them.
func TestTheTwoDownloadConflictsCarryTheirFrozenNames(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		err     error
		errcode int
		want    string
	}{
		{"no prepared video", productionservice.ErrMaterialUnavailable, 15005, "material_unavailable"},
		{"no local node", productionservice.ErrLocalNodeUnavailable, 15105, "local_transfer_node_unavailable"},
	} {
		engine := server.New()
		engine.POST("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
			writeProductionError(c, testCase.err)
		})
		result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/probe", nil)

		if got := result.Result().StatusCode(); got != consts.StatusConflict {
			t.Fatalf("%s: status = %d, want 409", testCase.name, got)
		}
		var body struct {
			ErrCode int `json:"errcode"`
			Error   *struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
			t.Fatalf("%s: decode response: %v", testCase.name, err)
		}
		if body.ErrCode != testCase.errcode {
			t.Fatalf("%s: errcode = %d, want %d", testCase.name, body.ErrCode, testCase.errcode)
		}
		if body.Error == nil || body.Error.Type != testCase.want {
			t.Fatalf("%s: error.type = %+v, want %q", testCase.name, body.Error, testCase.want)
		}
	}
}

// The names above are copies of what `contracts/cloud-error-codes/` publishes, and
// a copy nothing checks is a copy that drifts. This reads the frozen files and
// asserts each name is declared there as a 409, so a contract revision that
// renames one fails here rather than in a client.
func TestTheDownloadConflictNamesAreTheFrozenOnes(t *testing.T) {
	for file, names := range map[string][]string{
		"content-production.yaml": {"material_unavailable"},
		"file-transfer.yaml":      {"local_transfer_node_unavailable"},
	} {
		content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-error-codes", "v1", file))
		if err != nil {
			t.Fatalf("read frozen error codes: %v", err)
		}
		for _, name := range names {
			if !strings.Contains(string(content), name+": {http_status: 409}") {
				t.Errorf("%s does not declare %s as a 409", file, name)
			}
		}
	}
}

// The output download route shares `writeComposeOutputError` with the read and
// archive routes, so its two refusals must survive that sharing: the node
// conflict keeps the same frozen name the material download answers with, and
// an out-of-scope output keeps the compose-output not-found shape. Driven
// directly for the same reason the material mapping is — the route itself needs
// a live session and MySQL.
func TestWriteComposeOutputErrorKeepsTheDownloadRefusalsStraight(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		err     error
		status  int
		errcode int
		want    string
	}{
		{"no local node", productionservice.ErrLocalNodeUnavailable, consts.StatusConflict, 15105, "local_transfer_node_unavailable"},
		{"out-of-scope output", productionservice.ErrComposeOutputNotFound, consts.StatusNotFound, 15216, "compose_output_not_found"},
	} {
		engine := server.New()
		engine.POST("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
			writeComposeOutputError(c, testCase.err)
		})
		result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/probe", nil)

		if got := result.Result().StatusCode(); got != testCase.status {
			t.Fatalf("%s: status = %d, want %d", testCase.name, got, testCase.status)
		}
		var body struct {
			ErrCode int `json:"errcode"`
			Error   *struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
			t.Fatalf("%s: decode response: %v", testCase.name, err)
		}
		if body.ErrCode != testCase.errcode {
			t.Fatalf("%s: errcode = %d, want %d", testCase.name, body.ErrCode, testCase.errcode)
		}
		if body.Error == nil || body.Error.Type != testCase.want {
			t.Fatalf("%s: error.type = %+v, want %q", testCase.name, body.Error, testCase.want)
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

// The restoration answers the same 204 as the removal it undoes: the frontend's
// client treats 204 as "no body" and reads nothing, so a `data: null` envelope or
// a stray payload here would be a shape the caller is not written against.
//
// A source assertion rather than a request, for the same reason the route test
// above stops at 401: reaching the handler past `actor(c)` needs a live session
// row and the call behind it needs MySQL.
func TestRestoreUsageRouteAnswersThroughTheEmpty204Helper(t *testing.T) {
	source, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("read production handler: %v", err)
	}
	handler := string(source)
	start := strings.Index(handler, "func RestoreMaterialUsage(")
	if start < 0 {
		t.Fatal("handler.go has no RestoreMaterialUsage")
	}
	body := handler[start:]
	if end := strings.Index(body, "\nfunc "); end >= 0 {
		body = body[:end]
	}
	if !strings.Contains(body, "api.NoContentEmpty(c)") {
		t.Error("the restore route must answer 204 with no body")
	}
	if !strings.Contains(body, "writeProductionError(c, err)") {
		t.Error("the restore route must answer through the module's shared error mapping")
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

func TestComposeTaskRoutesRequireAuthentication(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, request := range []struct {
		method string
		path   string
	}{
		{consts.MethodGet, "/api/v1/compose-tasks"},
		{consts.MethodPost, "/api/v1/compose-tasks"},
		{consts.MethodGet, "/api/v1/compose-tasks/5"},
		{consts.MethodPost, "/api/v1/compose-tasks/5/cancel"},
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Result().StatusCode())
		}
	}
}

// Every pair below is frozen by contracts/cloud-error-codes/v1/content-production.yaml
// and the OpenAPI file's named responses; a swap in either field is what the
// frontend branches on, so both are asserted on a real response.
func TestComposeTaskErrorCarriesTheFrozenPairs(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		err     error
		status  int
		errcode int
		want    string
	}{
		{"invalid", productionservice.ErrComposeTaskInvalid, consts.StatusBadRequest, 15214, "compose_task_invalid"},
		{"forbidden", productionservice.ErrComposeTaskForbidden, consts.StatusForbidden, 15215, "compose_task_forbidden"},
		{"missing", productionservice.ErrComposeTaskNotFound, consts.StatusNotFound, 15216, "compose_task_not_found"},
		{"not cancellable", productionservice.ErrComposeTaskNotCancellable, consts.StatusConflict, 15217, "compose_task_not_cancellable"},
		{"template unavailable", productionservice.ErrStrategyTemplateUnavailable, consts.StatusConflict, 15213, "compose_template_unavailable"},
	} {
		engine := server.New()
		engine.GET("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
			writeComposeTaskError(c, testCase.err)
		})
		result := ut.PerformRequest(engine.Engine, consts.MethodGet, "/probe", nil)
		if got := result.Result().StatusCode(); got != testCase.status {
			t.Fatalf("%s: status = %d, want %d", testCase.name, got, testCase.status)
		}
		var body struct {
			ErrCode int `json:"errcode"`
			Error   *struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
			t.Fatalf("%s: decode response: %v", testCase.name, err)
		}
		if body.ErrCode != testCase.errcode {
			t.Fatalf("%s: errcode = %d, want %d", testCase.name, body.ErrCode, testCase.errcode)
		}
		if body.Error == nil || body.Error.Type != testCase.want {
			t.Fatalf("%s: error.type = %+v, want %q", testCase.name, body.Error, testCase.want)
		}
	}
}

// The names above are copies of the frozen file; this reads it so a rename
// there fails here instead of in a client.
func TestTheComposeTaskNamesAreTheFrozenOnes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-error-codes", "v1", "content-production.yaml"))
	if err != nil {
		t.Fatalf("read frozen error codes: %v", err)
	}
	for _, frozen := range []struct {
		name   string
		status int
	}{
		{"compose_task_invalid", consts.StatusBadRequest},
		{"compose_task_forbidden", consts.StatusForbidden},
		{"compose_task_not_found", consts.StatusNotFound},
		{"compose_task_not_cancellable", consts.StatusConflict},
	} {
		declared := fmt.Sprintf("%s: {http_status: %d}", frozen.name, frozen.status)
		if !strings.Contains(string(content), declared) {
			t.Errorf("content-production.yaml does not declare %q", declared)
		}
	}
}

func TestComposeTaskFailurePayloadsCarryTheFrozenErrcodes(t *testing.T) {
	result := productionservice.CreateComposeTasksResult{
		Created: []model.ComposeTask{{ID: 77, Status: model.ComposeTaskPending}},
		Failed: []productionservice.CreateComposeTaskFailure{
			{MaterialID: 42, RequestKey: "rk-usage-001", Err: productionservice.ErrComposeTaskInvalid, Message: "素材不在我的素材或已移出"},
			{MaterialID: 43, RequestKey: "rk-scope-001", Err: productionservice.ErrComposeTaskForbidden, Message: "素材不在你的团队范围内"},
		},
	}
	engine := server.New()
	engine.GET("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
		writeComposeTaskCreated(c, result)
	})
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/probe", nil)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, want 200", response.Result().StatusCode())
	}
	var body struct {
		Data struct {
			Created []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"created"`
			Failed []struct {
				MaterialID int64  `json:"material_id"`
				RequestKey string `json:"request_key"`
				Errcode    int    `json:"errcode"`
				Error      string `json:"error"`
				Message    string `json:"message"`
			} `json:"failed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Result().Body(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.Created) != 1 || body.Data.Created[0].ID != 77 || body.Data.Created[0].Status != "pending" {
		t.Fatalf("created = %+v", body.Data.Created)
	}
	if len(body.Data.Failed) != 2 {
		t.Fatalf("failed = %+v", body.Data.Failed)
	}
	if body.Data.Failed[0].Errcode != 15214 || body.Data.Failed[0].Error != "compose_task_invalid" || body.Data.Failed[0].Message != "素材不在我的素材或已移出" || body.Data.Failed[0].RequestKey != "rk-usage-001" {
		t.Fatalf("failed[0] = %+v", body.Data.Failed[0])
	}
	if body.Data.Failed[1].Errcode != 15215 || body.Data.Failed[1].Error != "compose_task_forbidden" {
		t.Fatalf("failed[1] = %+v", body.Data.Failed[1])
	}
}

func TestComposeOutputRoutesRequireAuthentication(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, request := range []struct {
		method string
		path   string
	}{
		{consts.MethodGet, "/api/v1/compose-outputs"},
		{consts.MethodGet, "/api/v1/compose-outputs/5"},
		{consts.MethodPost, "/api/v1/compose-outputs/5/archive"},
		{consts.MethodGet, "/api/v1/compose-outputs/5/video-url"},
		{consts.MethodPost, "/api/v1/compose-outputs/5/downloads"},
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Result().StatusCode())
		}
	}
}
