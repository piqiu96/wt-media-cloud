package cloudagent

import (
	"reflect"
	"sort"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// The module's HTTP surface is pinned as a whole list rather than sampled, because two
// paths here are load-bearing outside this repository: `scripts/verify_m1_integration.py`
// creates its task through POST /api/v1/tasks/noop, and wt-media-agent claims, reports
// and cancels over /api/v1/cloud-agent/tasks/*. A route deleted by accident and a route
// kept by accident look the same in a diff.
func TestRegisterRoutesExposesOnlyTheAgentTaskSurface(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	got := make([]string, 0, 10)
	for _, route := range engine.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)

	want := []string{
		"GET /api/v1/cloud-agent/agents/:agent_id",
		"GET /api/v1/cloud-agent/compatibility",
		"GET /api/v1/cloud-agent/tasks/:task_id",
		"POST /api/v1/cloud-agent/agents/:agent_id/heartbeat",
		"POST /api/v1/cloud-agent/agents/register",
		"POST /api/v1/cloud-agent/tasks/:task_id/cancel",
		"POST /api/v1/cloud-agent/tasks/:task_id/report",
		"POST /api/v1/cloud-agent/tasks/:task_id/retry",
		"POST /api/v1/cloud-agent/tasks/claim",
		"POST /api/v1/tasks/noop",
	}
	if len(got) != len(want) {
		t.Fatalf("the module binds %d route(s) %v, want the measured %d %v", len(got), got, len(want), want)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
}
