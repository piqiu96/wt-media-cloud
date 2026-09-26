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

// Every module's surface has to be reachable once `registerModuleRoutes` has run.
//
// The assertion is deliberately "not 404" and not a status: this test exists to
// catch a module dropped from the list, and what each module answers to a
// credential-less caller is that module's own business — three different answers
// are already visible above (401 for a session route, 400 where the handler
// decodes before it authenticates, 200 for a route that is public on purpose).
// Pinning those here would make this file a second, silently-drifting copy of
// each module's routing contract.
//
// The control keeps the assertion from being vacuous: an engine that answered
// anything but 404 would pass this test with the list emptied.
func TestRegisterModuleRoutesReachesEveryModule(t *testing.T) {
	engine := server.New()
	registerModuleRoutes(engine)

	for _, route := range []struct {
		module string
		method string
		path   string
	}{
		{"cloudagent", consts.MethodGet, "/api/v1/cloud-agent/compatibility"},
		{"identity", consts.MethodGet, "/api/v1/auth/me"},
		{"runtimebinding", consts.MethodPost, "/api/v1/local-agent/binding-tickets"},
		{"profileguard", consts.MethodPost, "/api/v1/local-agent/sensitive-tasks/task-1/preflight"},
		{"profilebinding", consts.MethodGet, "/api/v1/bit-browser/profile-scans/7"},
		{"mediaaccount", consts.MethodGet, "/api/v1/media-accounts"},
		{"proxy", consts.MethodGet, "/api/v1/proxies"},
		{"contentpool", consts.MethodGet, "/api/v1/content-pool"},
		{"production", consts.MethodGet, "/api/v1/materials"},
		{"filetransfer", consts.MethodGet, "/api/v1/file-transfer-tasks"},
	} {
		response := ut.PerformRequest(engine.Engine, route.method, route.path, nil)
		if response.Result().StatusCode() == consts.StatusNotFound {
			t.Fatalf("%s is not registered: %s %s status = 404", route.module, route.method, route.path)
		}
	}

	control := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/no-such-module", nil)
	if control.Result().StatusCode() != consts.StatusNotFound {
		t.Fatalf("unregistered path status = %d, want 404: this engine cannot tell a registered module from an absent one", control.Result().StatusCode())
	}
}
