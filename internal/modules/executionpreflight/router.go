package executionpreflight

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes 注册统一准入入口。
//
// 路由用上报档：同一端点也服务已领取任务，严格档会在 handler 读到 attempt_id
// 之前就拒绝被替换的 Desktop。不带 attempt_id 的新操作由 handler 自行做严格会话校验。
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/execution-preflight", localagent.RequireAgent(localagent.ReportSafeMode), ExecutionPreflight)
}
