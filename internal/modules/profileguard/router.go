// Package profileguard exposes the module's HTTP surface.
package profileguard

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes installs the module's root handlers.
//
// Preflight admits a new sensitive operation and requires the strict token.
// Renew and finish complete an operation already permitted, so they use the
// report-safe token: a replaced Desktop must still be able to release the lock
// it holds rather than leaving the Profile blocked until the permit expires.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/sensitive-tasks/:task_id/preflight", localagent.RequireAgent(localagent.StrictMode), Preflight)
	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/renew", localagent.RequireAgent(localagent.ReportSafeMode), Renew)
	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/finish", localagent.RequireAgent(localagent.ReportSafeMode), Finish)
}
