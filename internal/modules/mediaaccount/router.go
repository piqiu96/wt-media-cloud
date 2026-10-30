// Package mediaaccount exposes the module's HTTP surface.
package mediaaccount

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/media-accounts", CreateMediaAccount)
	h.GET("/api/v1/media-accounts", ListMediaAccounts)
	h.GET("/api/v1/media-accounts/:account_id", GetMediaAccount)
	h.PATCH("/api/v1/media-accounts/:account_id", UpdateMediaAccount)
	h.POST("/api/v1/media-accounts/:account_id/check", StartMediaAccountCheck)
	h.POST("/api/v1/media-accounts/:account_id/check/result", ApplyMediaAccountCheckResult)
	h.GET("/api/v1/media-accounts/:account_id/credentials", GetMediaAccountCredentials)
	h.POST("/api/v1/media-accounts/:account_id/cookie-updates", StartMediaAccountCookieUpdate)
	h.GET("/api/v1/media-accounts/:account_id/cookie-updates/:operation_id", GetMediaAccountCookieUpdate)
	h.PUT("/api/v1/media-accounts/:account_id/cookie-updates/:operation_id/snapshot", SnapshotMediaAccountCookieUpdate)
	h.PUT("/api/v1/media-accounts/:account_id/cookie-updates/:operation_id/result", SettleMediaAccountCookieUpdate)
	h.PATCH("/api/v1/media-accounts/:account_id/profile", BindMediaAccountProfile)
	h.DELETE("/api/v1/media-accounts/:account_id/profile", UnbindMediaAccountProfile)
}
