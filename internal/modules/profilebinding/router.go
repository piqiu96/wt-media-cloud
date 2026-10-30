// Package profilebinding exposes the module's HTTP surface.
package profilebinding

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes 安装本模块的 HTTP 路由。
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/browser-profiles/sync", SubmitProfileSync)
	h.GET("/api/v1/browser-profiles/sync", GetProfileSync)
	h.POST("/api/v1/browser-profiles/sync/confirm", ConfirmProfileSync)
	h.DELETE("/api/v1/browser-profiles/sync", RejectProfileSync)
	h.GET("/api/v1/local-agent/bit-account", GetBitAccount)
	h.POST("/api/v1/local-agent/bit-account/bind", localagent.RequireAgent(localagent.StrictMode), BindBitAccount)
	h.GET("/api/v1/browser-profiles", ListBrowserProfiles)
	h.PATCH("/api/v1/browser-profiles/:id", UpdateBrowserProfile)
	h.DELETE("/api/v1/browser-profiles/:id", DeleteBrowserProfile)
}
