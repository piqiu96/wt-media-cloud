// Package cloudagent exposes the module's HTTP surface.
package cloudagent

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes installs the module's root handlers. Agent endpoints require
// an Agent Token; task retry/cancel are user operations and use the session.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/cloud-agent/compatibility", Compatibility)
	h.POST("/api/v1/cloud-agent/tasks/claim", localagent.RequireAgent(localagent.StrictMode), ClaimTask)
	h.GET("/api/v1/cloud-agent/tasks/:task_id", localagent.RequireAgent(localagent.ReportSafeMode), GetTask)
	h.POST("/api/v1/cloud-agent/tasks/:task_id/report", localagent.RequireAgent(localagent.ReportSafeMode), ReportTask)
	h.POST("/api/v1/cloud-agent/tasks/:task_id/cancel", CancelTask)
	h.POST("/api/v1/cloud-agent/tasks/:task_id/retry", RetryTask)
}
