// Package identity exposes the module's HTTP surface.
package identity

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's root handlers.
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/auth/login", Login)
	h.GET("/api/v1/auth/me", Me)
	h.PATCH("/api/v1/auth/me", UpdateOwnProfile)
	h.POST("/api/v1/auth/logout", Logout)
	h.POST("/api/v1/users", CreateUser)
	h.PATCH("/api/v1/users/:user_id", UpdateUser)
	h.POST("/api/v1/users/:user_id/reset-password", ResetPassword)
	h.POST("/api/v1/users/:user_id/force-logout", ForceLogout)
	h.POST("/api/v1/auth/change-password", ChangePassword)
	h.GET("/api/v1/users", ListUsers)
	h.DELETE("/api/v1/users/:user_id", DeleteUser)
	h.GET("/api/v1/operation-teams", ListTeams)
	h.POST("/api/v1/operation-teams", CreateTeam)
	h.PATCH("/api/v1/operation-teams/:team_id", UpdateTeam)
	h.DELETE("/api/v1/operation-teams/:team_id", DeleteTeam)
	h.GET("/api/v1/categories", ListCategories)
	h.GET("/api/v1/categories/:category_id/references", CategoryReferences)
	h.POST("/api/v1/categories", CreateCategory)
	h.PATCH("/api/v1/categories/:category_id", UpdateCategory)
	h.DELETE("/api/v1/categories/:category_id", DeleteCategory)
	h.GET("/api/v1/audit-logs", ListAuditLogs)
}
