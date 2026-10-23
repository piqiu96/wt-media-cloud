package profileguard

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// 守卫端点只接受 Agent Token：空凭证和用户 session 都不行。空请求同时证明
// 路由已绑定，以及凭证类型在 handler 之前就被强制。
func TestRegisterRoutesBindsGuardEndpointsBehindTheAgentToken(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, path := range []string{
		"/api/v1/local-agent/sensitive-tasks/task-1/preflight",
		"/api/v1/local-agent/sensitive-permits/permit-1/renew",
		"/api/v1/local-agent/sensitive-permits/permit-1/finish",
	} {
		response := ut.PerformRequest(engine.Engine, consts.MethodPost, path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, response.Result().StatusCode())
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Result().Body(), &body); err != nil {
			t.Fatalf("%s decode response: %v", path, err)
		}
		if body.Error.Code != "AUTH_AGENT_TOKEN_INVALID" {
			t.Fatalf("%s error.code = %q, want AUTH_AGENT_TOKEN_INVALID", path, body.Error.Code)
		}
	}
}
