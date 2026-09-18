// Package cloudagent exposes the module's HTTP surface.
package cloudagent

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/cloud-agent/compatibility", Compatibility)
	h.POST("/api/v1/cloud-agent/agents/register", RegisterAgent)
	h.POST("/api/v1/cloud-agent/agents/:agent_id/heartbeat", Heartbeat)
	h.GET("/api/v1/cloud-agent/agents/:agent_id", GetAgent)
	h.GET("/api/v1/tasks/stats", TaskStats)
	h.POST("/api/v1/tasks", CreateTask)
	h.POST("/api/v1/tasks/noop", CreateNoopTask)
	h.POST("/api/v1/cloud-agent/tasks/claim", ClaimTask)
	h.GET("/api/v1/cloud-agent/tasks/:task_id", GetTask)
	h.POST("/api/v1/cloud-agent/tasks/:task_id/report", ReportTask)
	h.POST("/api/v1/cloud-agent/tasks/:task_id/cancel", CancelTask)
	h.POST("/api/v1/cloud-agent/tasks/:task_id/retry", RetryTask)
}
