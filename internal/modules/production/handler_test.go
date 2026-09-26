package production

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
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
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Result().StatusCode())
		}
	}
}
