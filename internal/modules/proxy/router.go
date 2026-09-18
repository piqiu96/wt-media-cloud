// Package proxy exposes the module's HTTP surface.
package proxy

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/proxies", ListProxies)
	h.GET("/api/v1/proxies/recommendations", ProxyRecommendations)
	h.GET("/api/v1/proxies/:id", GetProxy)
	h.GET("/api/v1/proxies/:id/bindings", GetProxyBindings)
	h.POST("/api/v1/proxies/parse", ParseProxy)
	h.POST("/api/v1/proxies/extract-preview", PreviewProxyExtraction)
	h.POST("/api/v1/proxies/:id/refresh", RefreshProxy)
	h.POST("/api/v1/proxies", CreateProxy)
	h.PATCH("/api/v1/proxies/:id", UpdateProxy)
	h.PATCH("/api/v1/proxies/:id/status", UpdateProxyStatus)
	h.DELETE("/api/v1/proxies/:id", DeleteProxy)
	h.POST("/api/v1/proxies/import/preview", PreviewProxyImport)
	h.POST("/api/v1/proxies/import", ImportProxies)
	h.POST("/api/v1/proxies/local-scan/preview", PreviewLocalProxyScan)
	h.POST("/api/v1/proxies/local-scan/confirm", ConfirmLocalProxyScan)
	h.POST("/api/v1/proxies/:id/check", CheckProxy)
	h.POST("/api/v1/proxies/:id/check/background", CheckProxyInBackground)
	h.POST("/api/v1/proxies/:id/assign", AssignProxy)
	h.POST("/api/v1/proxies/:id/assign-batch", AssignProxyBatch)
	h.POST("/api/v1/proxies/:id/unbind", UnbindProxy)
	h.POST("/api/v1/proxies/:id/quota", SetProxyQuota)
}
