package cloudagent

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

// The module's HTTP surface is pinned as a whole list rather than sampled. The
// agent-facing routes require an Agent Token; retry and cancel are user
// operations and stay on the session. A route deleted by accident and a route
// kept by accident look the same in a diff.
func TestRegisterRoutesExposesOnlyTheAgentTaskSurface(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	got := make([]string, 0, 6)
	for _, route := range engine.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)

	want := []string{
		"GET /api/v1/cloud-agent/compatibility",
		"GET /api/v1/cloud-agent/tasks/:task_id",
		"POST /api/v1/cloud-agent/tasks/:task_id/cancel",
		"POST /api/v1/cloud-agent/tasks/:task_id/report",
		"POST /api/v1/cloud-agent/tasks/:task_id/retry",
		"POST /api/v1/cloud-agent/tasks/claim",
	}
	if len(got) != len(want) {
		t.Fatalf("the module binds %d route(s) %v, want the measured %d %v", len(got), got, len(want), want)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
}

// The same route list proves nothing about which credential each route accepts.
// Driven without credentials, the two groups must fail differently: an Agent
// route demands an Agent Token, a user route demands a login session, and
// neither accepts the other's credential kind.
func TestAgentRoutesRefuseAnEmptyCredentialAsAnAgentToken(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	for _, testCase := range []struct {
		method string
		path   string
		code   string
	}{
		{"POST", "/api/v1/cloud-agent/tasks/claim", "AUTH_AGENT_TOKEN_INVALID"},
		{"GET", "/api/v1/cloud-agent/tasks/task-1", "AUTH_AGENT_TOKEN_INVALID"},
		{"POST", "/api/v1/cloud-agent/tasks/task-1/report", "AUTH_AGENT_TOKEN_INVALID"},
		{"POST", "/api/v1/cloud-agent/tasks/task-1/cancel", "AUTH_SESSION_INVALID"},
		{"POST", "/api/v1/cloud-agent/tasks/task-1/retry", "AUTH_SESSION_INVALID"},
	} {
		result := ut.PerformRequest(engine.Engine, testCase.method, testCase.path, nil)
		if result.Result().StatusCode() != 401 {
			t.Fatalf("%s %s status = %d, want 401", testCase.method, testCase.path, result.Result().StatusCode())
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
			t.Fatalf("%s %s decode response: %v", testCase.method, testCase.path, err)
		}
		if body.Error.Code != testCase.code {
			t.Fatalf("%s %s error.code = %q, want %q", testCase.method, testCase.path, body.Error.Code, testCase.code)
		}
	}
}
