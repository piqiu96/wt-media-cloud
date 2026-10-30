package cloudagent

import (
	"reflect"
	"sort"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesExposesOnlyCompatibility(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	got := make([]string, 0, 1)
	for _, route := range engine.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)
	want := []string{"GET /api/v1/cloud-agent/compatibility"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
}

func TestRetiredGenericTaskRoutesAreNotBound(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, path := range []string{
		"/api/v1/cloud-agent/tasks/claim",
		"/api/v1/cloud-agent/tasks/task-1",
		"/api/v1/tasks/task-1",
	} {
		response := ut.PerformRequest(engine.Engine, consts.MethodGet, path, nil)
		if response.Result().StatusCode() != consts.StatusNotFound {
			t.Fatalf("retired route %s status = %d, want 404", path, response.Result().StatusCode())
		}
	}
}
