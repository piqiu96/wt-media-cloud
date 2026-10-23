// Package profileguard 暴露敏感 Profile 守卫的 HTTP 接口。
package profileguard

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
)

// RegisterRoutes：preflight 准入新操作，用严格档；renew/finish 收尾已持有的许可，
// 用上报档，否则被替换的 Desktop 无法释放锁，Profile 会一直被占到许可过期。
func RegisterRoutes(h *server.Hertz) {
	h.POST("/api/v1/local-agent/sensitive-tasks/:task_id/preflight", localagent.RequireAgent(localagent.StrictMode), Preflight)
	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/renew", localagent.RequireAgent(localagent.ReportSafeMode), Renew)
	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/finish", localagent.RequireAgent(localagent.ReportSafeMode), Finish)
}
