// Package mediaaccount exposes the module's HTTP surface.
package mediaaccount

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/media-accounts", CreateMediaAccount)
	h.GET("/api/v1/media-accounts", ListMediaAccounts)
	h.POST("/api/v1/media-accounts/tags/add", AddMediaAccountTags)
	h.POST("/api/v1/media-accounts/tags/remove", RemoveMediaAccountTags)
	h.GET("/api/v1/media-accounts/:account_id", GetMediaAccount)
	h.PATCH("/api/v1/media-accounts/:account_id", UpdateMediaAccount)
	h.POST("/api/v1/media-accounts/:account_id/identify", IdentifyMediaAccount)
	h.POST("/api/v1/media-accounts/:account_id/check", StartMediaAccountCheck)
	h.POST("/api/v1/media-accounts/:account_id/check/result", ApplyMediaAccountCheckResult)
	h.POST("/api/v1/media-accounts/:account_id/cookies/read", ReadMediaAccountCookies)
	h.POST("/api/v1/media-accounts/:account_id/cookies/read-sync", ReadMediaAccountCookiesSync)
	h.POST("/api/v1/media-accounts/:account_id/cookies/read-sync/result", ApplyMediaAccountCookieReadResult)
	h.GET("/api/v1/media-accounts/:account_id/cookies", GetMediaAccountCookies)
	h.PATCH("/api/v1/media-accounts/:account_id/profile", BindMediaAccountProfile)
	h.DELETE("/api/v1/media-accounts/:account_id/profile", UnbindMediaAccountProfile)
	h.POST("/api/v1/account-groups", CreateAccountGroup)
	h.GET("/api/v1/account-groups", ListAccountGroups)
	h.GET("/api/v1/account-groups/:group_id/accounts", ListAccountsByGroup)
	h.PATCH("/api/v1/account-groups/:group_id", UpdateAccountGroup)
	h.DELETE("/api/v1/account-groups/:group_id", DeleteAccountGroup)
}
