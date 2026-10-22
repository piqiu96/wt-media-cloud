// Package localagent owns the Cloud side of the Local Agent contract: issuing
// the session-bound Agent Token and authenticating Agent API calls with it.
package localagent

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/agenttoken"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

// Mode selects how much session state an Agent call must prove.
type Mode int

const (
	// StrictMode gates new work: the token's sid must still be the user's one
	// active Desktop session and the user must still be enabled.
	StrictMode Mode = iota
	// ReportSafeMode gates completion of work already claimed. It verifies only
	// the token itself; task ownership is checked by the handler. A replaced
	// Desktop must still be able to finish what it already started.
	ReportSafeMode
)

const agentContextKey = "local_agent_auth"

// resolveDesktopSession is the identity lookup strict mode performs. A variable
// so the takeover branches can be driven without a database; the process uses
// the real identity service.
var resolveDesktopSession = identityservice.AuthenticateAgentSession

// AgentAuth is the resolved caller of an Agent API.
type AgentAuth struct {
	Claims  agenttoken.Claims
	Mode    Mode
	Session identityservice.Session
	User    identityservice.PublicUser
}

// RequireAgent rejects requests that carry anything other than a valid Agent
// Token. A user session is deliberately not accepted here: the two credential
// kinds must not be interchangeable.
func RequireAgent(mode Mode) hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		auth, ok := authenticate(ctx, c, mode)
		if !ok {
			// Hertz's chain keeps calling handlers after a middleware returns;
			// only Abort stops it. Without this the handler would run behind a
			// rejected token and write a second body.
			c.Abort()
			return
		}
		c.Set(agentContextKey, auth)
		c.Next(ctx)
	}
}

// AgentAuthFrom returns the authentication installed by RequireAgent.
func AgentAuthFrom(c *hertzapp.RequestContext) (AgentAuth, bool) {
	cached, exists := c.Get(agentContextKey)
	if !exists {
		return AgentAuth{}, false
	}
	auth, valid := cached.(AgentAuth)
	return auth, valid
}

func authenticate(_ context.Context, c *hertzapp.RequestContext, mode Mode) (AgentAuth, bool) {
	token, ok := bearerToken(c)
	if !ok {
		writeAgentError(c, "AUTH_AGENT_TOKEN_INVALID", "Agent 凭证缺失或格式错误", false, "reissue_agent_token")
		return AgentAuth{}, false
	}
	signer, err := agenttoken.Current()
	if err != nil {
		hlog.Errorf("module=local_agent result=error reason=signer_unavailable path=%s", string(c.Path()))
		api.InternalError(c, "Agent 凭证服务不可用")
		return AgentAuth{}, false
	}
	claims, err := signer.Verify(token)
	if err != nil {
		if errors.Is(err, agenttoken.ErrExpired) {
			writeAgentError(c, "AUTH_AGENT_TOKEN_EXPIRED", "Agent 凭证已过期，请重新获取", false, "reissue_agent_token")
			return AgentAuth{}, false
		}
		hlog.Warnf("module=local_agent result=denied reason=token_invalid path=%s", string(c.Path()))
		writeAgentError(c, "AUTH_AGENT_TOKEN_INVALID", "Agent 凭证无效", false, "reissue_agent_token")
		return AgentAuth{}, false
	}
	auth := AgentAuth{Claims: claims, Mode: mode}
	if mode == ReportSafeMode {
		return auth, true
	}

	sessionContext, err := resolveDesktopSession(claims.SessionID)
	if err != nil {
		if failure, described := identityservice.DescribeAuthFailure(err); described {
			writeAgentError(c, failure.Code, failure.Message, false, failure.RecoveryAction)
			return AgentAuth{}, false
		}
		hlog.Errorf("module=local_agent result=error path=%s error=%v", string(c.Path()), err)
		api.InternalError(c, "Agent 凭证校验失败")
		return AgentAuth{}, false
	}
	// The signature already binds sub and sid together, but a token must never
	// authenticate a different user than the session it names.
	if int64(sessionContext.User.ID) != claims.UserID || sessionContext.Session.ID != claims.SessionID {
		writeAgentError(c, "AUTH_AGENT_TOKEN_INVALID", "Agent 凭证与登录会话不一致", false, "reissue_agent_token")
		return AgentAuth{}, false
	}
	auth.Session = sessionContext.Session
	auth.User = sessionContext.User
	return auth, true
}

func writeAgentError(c *hertzapp.RequestContext, code, message string, retryable bool, recoveryAction string) {
	api.FailureCode(c, 401, 11001, message, code, nil, retryable, recoveryAction)
}

func bearerToken(c *hertzapp.RequestContext) (string, bool) {
	const prefix = "Bearer "
	header := string(c.GetHeader("Authorization"))
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}
