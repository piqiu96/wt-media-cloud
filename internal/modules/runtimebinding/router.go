// Package runtimebinding exposes the module's HTTP surface.
package runtimebinding

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/binding-tickets", IssueBindingTicket)
	h.POST("/api/v1/local-agent/nodes/register", RegisterLocalNode)
	h.GET("/api/v1/local-agent/device-binding", GetDeviceBinding)
	h.DELETE("/api/v1/local-agent/device-binding", UnbindDevice)
	h.POST("/api/v1/local-agent/nodes/:node_id/runtime-report", ReportRuntime)
}
