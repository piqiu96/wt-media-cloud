package cloudagent

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

// 路由按完整列表固定：被误删和误留的路由在 diff 里看起来一样，只有整表比对能区分。
func TestRegisterRoutesExposesOnlyTheAgentTaskSurface(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	got := make([]string, 0, 8)
	for _, route := range engine.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)

	want := []string{
		"GET /api/v1/cloud-agent/compatibility",
		"GET /api/v1/cloud-agent/tasks/:task_id",
		"GET /api/v1/tasks/:task_id",
		"POST /api/v1/cloud-agent/tasks/:task_id/activity",
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

// 空凭证下两组路由必须给出不同的失败码，证明两种凭证不能互相代用。
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
		{"POST", "/api/v1/cloud-agent/tasks/task-1/activity", "AUTH_AGENT_TOKEN_INVALID"},
		{"POST", "/api/v1/cloud-agent/tasks/task-1/cancel", "AUTH_SESSION_INVALID"},
		{"POST", "/api/v1/cloud-agent/tasks/task-1/retry", "AUTH_SESSION_INVALID"},
		{"GET", "/api/v1/tasks/task-1", "AUTH_SESSION_INVALID"},
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
