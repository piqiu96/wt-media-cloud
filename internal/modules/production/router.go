// Package production exposes Cloud material-library HTTP endpoints.
package production

import "github.com/cloudwego/hertz/pkg/app/server"

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/materials", ListMaterials)
	h.GET("/api/v1/materials/:material_id", GetMaterial)
	h.GET("/api/v1/materials/:material_id/video-url", GetMaterialVideoURL)
	h.POST("/api/v1/materials/:material_id/usages", AddMaterialUsage)
	h.POST("/api/v1/materials/:material_id/downloads", CreateMaterialDownload)
	h.GET("/api/v1/my-materials", ListMyMaterials)
	h.DELETE("/api/v1/material-usages/:usage_id", RemoveMaterialUsage)
}
