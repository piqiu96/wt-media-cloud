// Package contentpool exposes the module's HTTP surface.
package contentpool

import "github.com/cloudwego/hertz/pkg/app/server"

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/content-pool", ListContent)
	h.GET("/api/v1/content-pool/:id", GetContent)
	h.POST("/api/v1/content-pool", CreateContent)
	h.POST("/api/v1/content-pool/batch/status", BatchUpdateContentStatus)
	h.POST("/api/v1/content-pool/:id/status", UpdateContentStatus)
	h.POST("/api/v1/content-pool/:id/materialize", MaterializeContent)
	h.POST("/api/v1/content-pool/batch/materialize", BatchMaterializeContent)
	h.POST("/api/v1/discovery-scheduler/run-due", RunDueDiscovery)
	h.POST("/api/v1/content-pool/search", SearchContent)
	h.POST("/api/v1/content-pool/author-search", AuthorSearchContent)
	h.POST("/api/v1/content-pool/import-results", ImportSearchResults)
	h.POST("/api/v1/content-pool/import-url", ImportContentURL)
	h.GET("/api/v1/discovery-strategies", ListDiscoveryStrategies)
	h.POST("/api/v1/discovery-strategies", CreateDiscoveryStrategy)
	h.POST("/api/v1/discovery-strategies/:id/status", UpdateDiscoveryStrategyStatus)
	h.PUT("/api/v1/discovery-strategies/:id", UpdateDiscoveryStrategy)
	h.POST("/api/v1/discovery-strategies/:id/run", RunDiscoveryStrategy)
	h.GET("/api/v1/crawl-tasks", ListCrawlTasks)
	h.GET("/api/v1/crawl-tasks/:id", GetCrawlTask)
	h.POST("/api/v1/crawl-tasks/:id/retry-failed", RetryFailedCrawlTask)
	h.POST("/api/v1/crawl-tasks/:id/confirm", ConfirmCrawlTask)
}
