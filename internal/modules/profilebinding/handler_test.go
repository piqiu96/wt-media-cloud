package profilebinding

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesBindsProfileEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/browser-profiles", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated profile list status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}
