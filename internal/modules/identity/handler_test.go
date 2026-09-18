package identity

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesBindsAuthenticationRoutes(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/auth/me", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated /auth/me status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestAuthenticateRequestRejectsMissingSession(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/auth/me", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}
