// Package production exposes Cloud material-library HTTP endpoints.
package production

import "github.com/cloudwego/hertz/pkg/app/server"

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/compose-templates", ListComposeTemplates)
	h.GET("/api/v1/compose-tasks", ListComposeTasks)
	h.POST("/api/v1/compose-tasks", CreateComposeTasks)
	h.GET("/api/v1/compose-outputs", ListComposeOutputs)
	h.GET("/api/v1/compose-outputs/:output_id", GetComposeOutput)
	h.POST("/api/v1/compose-outputs/:output_id/archive", ArchiveComposeOutput)
	h.GET("/api/v1/compose-outputs/:output_id/video-url", GetComposeOutputVideoURL)
	h.POST("/api/v1/compose-outputs/:output_id/downloads", CreateOutputDownload)
	h.GET("/api/v1/compose-tasks/:task_id", GetComposeTask)
	h.GET("/api/v1/compose-tasks/:task_id/attempts", ListComposeTaskAttempts)
	h.GET("/api/v1/compose-tasks/:task_id/uncertain-review", ListComposeUncertainReviews)
	h.POST("/api/v1/compose-tasks/:task_id/uncertain-review", ResolveComposeUncertain)
	h.POST("/api/v1/compose-tasks/:task_id/retry", RetryComposeTask)
	h.POST("/api/v1/compose-tasks/:task_id/cancel", CancelComposeTask)
	h.GET("/api/v1/compose-strategies", ListComposeStrategies)
	h.POST("/api/v1/compose-strategies", CreateComposeStrategy)
	h.GET("/api/v1/compose-strategies/:strategy_id", GetComposeStrategy)
	h.PUT("/api/v1/compose-strategies/:strategy_id", UpdateComposeStrategy)
	h.POST("/api/v1/compose-strategies/:strategy_id/copy", CopyComposeStrategy)
	h.PATCH("/api/v1/compose-strategies/:strategy_id/status", SetComposeStrategyStatus)
	h.PUT("/api/v1/compose-strategies/:strategy_id/schedule", SetComposeStrategySchedule)
	h.POST("/api/v1/compose-strategies/:strategy_id/run-now", RunComposeStrategyNow)
	h.GET("/api/v1/compose-strategies/:strategy_id/runs", ListComposeStrategyRuns)
	h.GET("/api/v1/compose-strategies/:strategy_id/runs/tasks", ListComposeStrategyRunTasks)
	h.GET("/api/v1/materials", ListMaterials)
	h.GET("/api/v1/materials/:material_id", GetMaterial)
	h.GET("/api/v1/materials/:material_id/video-url", GetMaterialVideoURL)
	h.POST("/api/v1/materials/:material_id/usages", AddMaterialUsage)
	h.POST("/api/v1/materials/:material_id/downloads", CreateMaterialDownload)
	h.GET("/api/v1/my-materials", ListMyMaterials)
	h.DELETE("/api/v1/material-usages/:usage_id", RemoveMaterialUsage)
	h.POST("/api/v1/material-usages/:usage_id/restore", RestoreMaterialUsage)
}
