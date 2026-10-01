package filetransfer

import (
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRoutes installs the module's handlers.
//
// The two families are distinguishable by path and by nothing else: `/api/v1/
// file-transfer-tasks/...` is the session API and is authenticated by the
// session middleware, while `/api/v1/cloud-agent/...` is the executor API and
// carries a node bearer credential. They are registered together because they
// are one lifecycle, and because a reader looking for "what can be done to a
// transfer task" should not have to know that two modules answer that.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/file-transfer-tasks", ListTasks)
	h.POST("/api/v1/file-transfer-tasks/:task_id/cancel", CancelTask)

	h.POST("/api/v1/cloud-agent/file-transfer-tasks/claim", ClaimTask)
	h.POST("/api/v1/cloud-agent/file-transfer-tasks/:task_id/heartbeat", HeartbeatTask)
	h.POST("/api/v1/cloud-agent/file-transfer-tasks/:task_id/progress", ReportProgress)
	h.POST("/api/v1/cloud-agent/file-transfer-tasks/:task_id/complete", CompleteTask)
}
