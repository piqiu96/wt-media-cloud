package filetransfer

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes installs the module's handlers.
//
// The two families are distinguishable by path and by nothing else: `/api/v1/
// file-transfer-tasks/...` is the session API and is authenticated by the
// session middleware, while `/api/v1/cloud-agent/...` is the executor API and
// carries an Agent Token. They are registered together because they are one
// lifecycle, and because a reader looking for "what can be done to a transfer
// task" should not have to know that two modules answer that.
//
// Claiming is a new operation and needs the strict token — the Desktop session
// behind it must still be the current one. The three continuation routes are
// report-safe: they are bound to the user and the attempt the task was claimed
// with, so a task already running can finish and report even after the account
// logged in somewhere else.
func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/file-transfer-tasks", ListTasks)
	h.POST("/api/v1/file-transfer-tasks/:task_id/cancel", CancelTask)
	h.POST("/api/v1/file-transfer-tasks/:task_id/hide", HideDownloadHistory)

	h.POST("/api/v1/cloud-agent/file-transfer-tasks/claim", localagent.RequireAgent(localagent.StrictMode), ClaimTask)
	h.POST("/api/v1/cloud-agent/file-transfer-tasks/:task_id/heartbeat", localagent.RequireAgent(localagent.ReportSafeMode), HeartbeatTask)
	h.POST("/api/v1/cloud-agent/file-transfer-tasks/:task_id/progress", localagent.RequireAgent(localagent.ReportSafeMode), ReportProgress)
	h.POST("/api/v1/cloud-agent/file-transfer-tasks/:task_id/complete", localagent.RequireAgent(localagent.ReportSafeMode), CompleteTask)
}
