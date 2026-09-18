// Package profilebinding exposes the module's HTTP surface.
package profilebinding

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/bit-browser/profile-scans", SubmitProfileScan)
	h.GET("/api/v1/bit-browser/profile-scans/:scan_id", GetProfileScan)
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/confirm", ConfirmProfileScan)
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/confirm-main-identity", ConfirmScanMainIdentity)
	h.POST("/api/v1/bit-browser/main-identity", ConfirmMainIdentity)
	h.DELETE("/api/v1/users/:user_id/bit-browser-main-identity", ClearMainIdentity)
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/reject", RejectProfileScan)
	h.GET("/api/v1/browser-profiles", ListBrowserProfiles)
	h.POST("/api/v1/browser-profiles", CreateBrowserProfile)
	h.POST("/api/v1/browser-profiles/:id/open", OpenBrowserProfile)
	h.POST("/api/v1/browser-profiles/:id/close", CloseBrowserProfile)
	h.PATCH("/api/v1/browser-profiles/:id", UpdateBrowserProfile)
	h.POST("/api/v1/browser-profiles/:id/assign-owner", AssignBrowserProfileOwner)
	h.DELETE("/api/v1/browser-profiles/:id", DeleteBrowserProfile)
}
