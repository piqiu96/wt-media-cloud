// Package production exposes Cloud material-library HTTP endpoints.
package production

import "github.com/cloudwego/hertz/pkg/app/server"

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/compose-templates", ListComposeTemplates)
	h.GET("/api/v1/compose-strategies", ListComposeStrategies)
	h.POST("/api/v1/compose-strategies", CreateComposeStrategy)
	h.GET("/api/v1/compose-strategies/:strategy_id", GetComposeStrategy)
	h.PUT("/api/v1/compose-strategies/:strategy_id", UpdateComposeStrategy)
	h.POST("/api/v1/compose-strategies/:strategy_id/copy", CopyComposeStrategy)
	h.PATCH("/api/v1/compose-strategies/:strategy_id/status", SetComposeStrategyStatus)
	h.GET("/api/v1/materials", ListMaterials)
	h.GET("/api/v1/materials/:material_id", GetMaterial)
	h.GET("/api/v1/materials/:material_id/video-url", GetMaterialVideoURL)
	h.POST("/api/v1/materials/:material_id/usages", AddMaterialUsage)
	h.POST("/api/v1/materials/:material_id/downloads", CreateMaterialDownload)
	h.GET("/api/v1/my-materials", ListMyMaterials)
	h.DELETE("/api/v1/material-usages/:usage_id", RemoveMaterialUsage)
	h.POST("/api/v1/material-usages/:usage_id/restore", RestoreMaterialUsage)
}
