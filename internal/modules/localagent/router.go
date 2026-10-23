package localagent

import "github.com/cloudwego/hertz/pkg/app/server"

// RegisterRoutes 注册用用户会话认证的 Local Agent 端点；Agent Token 认证的端点在各模块内。
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/agent-token", IssueAgentToken)
}
