package localagent

import (
	"context"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/agenttoken"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

// authenticateSession 是测试接缝：进程用真实中间件，用例不必连数据库。
var authenticateSession = middleware.AuthenticateRequestContext

// recordTokenIssued 同样可替换：审计写库失败必须让签发失败，而用例不关心审计如何落库。
var recordTokenIssued = identityservice.AuditAgentTokenIssued

// IssueAgentToken 用 Desktop 会话换取签名 Agent Token。Web 会话被拒绝：
// Token 让本机 Agent 代表这台电脑当前有效的 Desktop 登录行事。
func IssueAgentToken(_ context.Context, c *hertzapp.RequestContext) {
	authContext, ok := authenticateSession(c)
	if !ok {
		return
	}
	if authContext.Session.ClientType != identityservice.ClientTypeDesktop {
		api.FailureCode(c, 401, 11001, "请先在 Desktop 登录", "DESKTOP_LOGIN_REQUIRED", nil, false, "login_desktop")
		return
	}
	// Agent Token 代表这台电脑执行本机浏览器任务，只属于普通运营；
	// 高级运营和管理员可以登录 Desktop 使用共用页面，但不能取得本机执行凭证。
	if authContext.User.Role != identityservice.RoleOperator {
		api.FailureCode(c, 403, 11003, "本机执行仅支持普通运营账号", "DESKTOP_ROLE_FORBIDDEN", nil, false, "use_cloud_web")
		return
	}
	signer, err := agenttoken.Current()
	if err != nil {
		hlog.Errorf("module=local_agent result=error reason=signer_unavailable")
		api.InternalError(c, "Agent 凭证服务不可用")
		return
	}
	token, claims, err := signer.Issue(int64(authContext.User.ID), authContext.Session.ID, authContext.Session.ExpiresAt)
	if err != nil {
		hlog.Errorf("module=local_agent result=error reason=issue_failed error=%v", err)
		api.InternalError(c, "Agent 凭证签发失败")
		return
	}
	if err := recordTokenIssued(authContext.User.ID, authContext.Session.ID); err != nil {
		hlog.Errorf("module=local_agent result=error reason=audit_failed error=%v", err)
		api.InternalError(c, "Agent 凭证签发失败")
		return
	}
	api.Success(c, map[string]any{
		"token":      token,
		"expires_at": claims.ExpiresAt,
	})
}
