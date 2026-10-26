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

// 运行过 registerModuleRoutes 后，每个模块的入口都必须可达。
//
// 断言故意只要求“不是 404”：这个用例负责发现被漏掉的模块，而各模块对无凭证调用方
// 的回答各不相同（401、400、200 都有），那是各模块自己的路由契约。把它们钉在这里，
// 这个文件就会变成契约的第二份、会悄悄漂移的副本。
//
// 末尾的对照防止断言空转：如果引擎对任意路径都不返回 404，清空列表也能通过。
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
		{"localagent", consts.MethodPost, "/api/v1/local-agent/agent-token"},
		{"executionpreflight", consts.MethodPost, "/api/v1/local-agent/execution-preflight"},
		{"profileguard", consts.MethodPost, "/api/v1/local-agent/sensitive-tasks/task-1/preflight"},
		{"profilebinding", consts.MethodGet, "/api/v1/bit-browser/profile-scans/7"},
		{"mediaaccount", consts.MethodGet, "/api/v1/media-accounts"},
		{"proxy", consts.MethodGet, "/api/v1/proxies"},
		{"contentpool", consts.MethodGet, "/api/v1/content-pool"},
		{"production", consts.MethodGet, "/api/v1/materials"},
		{"production", consts.MethodGet, "/api/v1/materials/42/video-url"},
		{"production", consts.MethodPost, "/api/v1/compose-outputs/5/downloads"},
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
