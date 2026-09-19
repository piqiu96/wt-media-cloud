package bootstrap

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
)

func TestBootstrapIdentityRequiresCredentialPair(t *testing.T) {
	if err := bootstrapIdentity("", ""); err != nil {
		t.Fatalf("empty bootstrap configuration error = %v", err)
	}
	if err := bootstrapIdentity("tech", ""); err == nil {
		t.Fatalf("partial bootstrap configuration error = nil")
	}
}

func TestBootstrapCORSAllowsPackagedDesktopPreflight(t *testing.T) {
	engine := server.New()
	engine.Use(middleware.LocalDesktopCORS())
	registerHealthRoutes(engine)
	api := ut.PerformRequest(engine.Engine, "OPTIONS", "/api/v1/auth/login", nil, ut.Header{Key: "Origin", Value: "http://tauri.localhost"}, ut.Header{Key: "Access-Control-Request-Method", Value: "POST"})
	if api.Result().StatusCode() != consts.StatusNoContent {
		t.Fatalf("preflight status = %d body=%s", api.Result().StatusCode(), api.Result().Body())
	}
	if got := string(api.Result().Header.Peek("Access-Control-Allow-Origin")); got != "http://tauri.localhost" {
		t.Fatalf("allow origin = %q", got)
	}
}
