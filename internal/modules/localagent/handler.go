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

// authenticateSession resolves the caller's user session. A variable so the
// Desktop-only rule can be driven without a database; the process uses the real
// middleware.
var authenticateSession = middleware.AuthenticateRequestContext

// recordTokenIssued writes the issuance audit event. A variable for the same
// reason: the audit store is the database, and the decision under test is what
// gets issued, not how it is logged.
var recordTokenIssued = identityservice.AuditAgentTokenIssued

// IssueAgentToken exchanges the caller's Desktop session for a signed Agent
// Token. Web sessions are refused: the token exists to let one computer's
// Local Agent act for the Desktop login that is currently valid on it.
func IssueAgentToken(_ context.Context, c *hertzapp.RequestContext) {
	authContext, ok := authenticateSession(c)
	if !ok {
		return
	}
	if authContext.Session.ClientType != identityservice.ClientTypeDesktop {
		api.FailureCode(c, 401, 11001, "请先在 Desktop 登录", "DESKTOP_LOGIN_REQUIRED", nil, false, "login_desktop")
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
