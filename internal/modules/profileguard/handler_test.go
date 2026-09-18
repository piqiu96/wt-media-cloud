package profileguard

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesBindsGuardEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/local-agent/sensitive-tasks/task-1/preflight", nil)
	if response.Result().StatusCode() != consts.StatusBadRequest {
		t.Fatalf("empty preflight status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}
