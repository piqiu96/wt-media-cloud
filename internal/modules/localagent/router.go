package localagent

import "github.com/cloudwego/hertz/pkg/app/server"

// RegisterRoutes installs the Local Agent endpoints that authenticate with a
// user session. Agent-authenticated endpoints live in their owning modules.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/agent-token", IssueAgentToken)
}
