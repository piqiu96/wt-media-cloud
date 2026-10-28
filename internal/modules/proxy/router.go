// Package proxy exposes the module's HTTP surface.
package proxy

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/browser-profiles/:id/proxy-operation", StartProfileProxyOperation)
	h.POST("/api/v1/browser-profiles/:id/proxy-operation/result", FinishProfileProxyOperation)
	h.GET("/api/v1/proxies", ListProxies)
	h.GET("/api/v1/proxies/recommendations", ProxyRecommendations)
	h.GET("/api/v1/proxies/:id", GetProxy)
	h.GET("/api/v1/proxies/:id/bindings", GetProxyBindings)
	h.POST("/api/v1/proxies/parse", ParseProxy)
	h.POST("/api/v1/proxies/:id/refresh", RefreshProxy)
	h.POST("/api/v1/proxies/:id/refresh/result", FinishRefreshProxy)
	h.POST("/api/v1/proxies", CreateProxy)
	h.PATCH("/api/v1/proxies/:id", UpdateProxy)
	h.DELETE("/api/v1/proxies/:id", DeleteProxy)
	h.POST("/api/v1/proxies/import/preview", PreviewProxyImport)
	h.POST("/api/v1/proxies/import", ImportProxies)
	h.POST("/api/v1/proxies/:id/check", CheckProxy)
}
