// Package profilebinding exposes the module's HTTP surface.
package profilebinding

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes 安装本模块的 HTTP 路由。
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/bit-browser/profile-scans", SubmitProfileScan)
	h.GET("/api/v1/bit-browser/profile-scans/:scan_id", GetProfileScan)
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/confirm", ConfirmProfileScan)
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/reject", RejectProfileScan)
	h.POST("/api/v1/local-agent/bit-account/bind", localagent.RequireAgent(localagent.StrictMode), BindBitAccount)
	h.GET("/api/v1/browser-profiles", ListBrowserProfiles)
	h.POST("/api/v1/browser-profiles", CreateBrowserProfile)
	h.POST("/api/v1/browser-profiles/:id/open", OpenBrowserProfile)
	h.POST("/api/v1/browser-profiles/:id/close", CloseBrowserProfile)
	h.PATCH("/api/v1/browser-profiles/:id", UpdateBrowserProfile)
	h.POST("/api/v1/browser-profiles/:id/assign-owner", AssignBrowserProfileOwner)
	h.DELETE("/api/v1/browser-profiles/:id", DeleteBrowserProfile)
}
