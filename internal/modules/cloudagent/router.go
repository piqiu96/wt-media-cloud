// Package cloudagent exposes the module's HTTP surface.
package cloudagent

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes keeps the Cloud-Agent version handshake. The former generic
// task lifecycle endpoints are retired; file transfer has its own contract.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/cloud-agent/compatibility", Compatibility)
}
