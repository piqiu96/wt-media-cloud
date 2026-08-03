package app

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func TestBootstrapIdentityRequiresCredentialPair(t *testing.T) {
	service := identity.NewService(identity.NewMemoryStore())
	if err := bootstrapIdentity(service, "", ""); err != nil {
		t.Fatalf("empty bootstrap configuration error = %v", err)
	}
	if err := bootstrapIdentity(service, "tech", ""); err == nil {
		t.Fatalf("partial bootstrap configuration error = nil")
	}
}

func TestBootstrapIdentityDoesNotResetExistingAdmin(t *testing.T) {
	service := identity.NewService(identity.NewMemoryStore())
	if err := bootstrapIdentity(service, "tech", "a-long-initial-password"); err != nil {
		t.Fatalf("first bootstrap error = %v", err)
	}
	if err := bootstrapIdentity(service, "replacement", "a-long-replacement-password"); err != nil {
		t.Fatalf("second bootstrap error = %v", err)
	}
	if _, err := service.Login("tech", "a-long-initial-password"); err != nil {
		t.Fatalf("original admin login error = %v", err)
	}
}

func TestLocalDesktopCORSMiddlewareAllowsPackagedDesktopPreflight(t *testing.T) {
	engine := server.New()
	engine.Use(localDesktopCORSMiddleware())
	registerHealthRoutes(engine)

	response := ut.PerformRequest(
		engine.Engine,
		"OPTIONS",
		"/api/v1/auth/login",
		nil,
		ut.Header{Key: "Origin", Value: "http://tauri.localhost"},
		ut.Header{Key: "Access-Control-Request-Method", Value: "POST"},
	)

	if response.Result().StatusCode() != consts.StatusNoContent {
		t.Fatalf("preflight status = %d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
	if got := string(response.Result().Header.Peek("Access-Control-Allow-Origin")); got != "http://tauri.localhost" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := string(response.Result().Header.Peek("Access-Control-Allow-Credentials")); got != "true" {
		t.Fatalf("allow credentials = %q", got)
	}
}
