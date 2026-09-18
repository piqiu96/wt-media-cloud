// Package profileguard exposes the module's HTTP surface.
package profileguard

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/sensitive-tasks/:task_id/preflight", Preflight)
	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/renew", Renew)
	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/finish", Finish)
}
