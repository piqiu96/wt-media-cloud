package runtimebinding

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesBindsRuntimeEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/local-agent/nodes/register", nil)
	if response.Result().StatusCode() != consts.StatusBadRequest {
		t.Fatalf("empty register status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}
