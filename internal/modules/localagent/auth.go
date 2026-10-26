// Package localagent 负责 Cloud 侧的 Local Agent 契约：签发会话绑定的 Agent Token，
// 并用它认证 Agent API 调用。
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

// Mode 决定一次 Agent 调用要证明多少会话状态。
type Mode int

const (
	// StrictMode 管新工作：Token 的 sid 必须仍是该用户当前有效的 Desktop 会话，
	// 用户也必须仍启用。
	StrictMode Mode = iota
	// ReportSafeMode 管已领取工作的收尾：只验 Token 本身，任务归属由 handler 校验。
	// 被替换的 Desktop 仍须能收尾它已经开始的活。
	ReportSafeMode
)

const agentContextKey = "local_agent_auth"

// resolveDesktopSession 是严格档的会话查询；测试可替换，进程使用真实 identity 服务。
var resolveDesktopSession = identityservice.AuthenticateAgentSession

type AgentAuth struct {
	Claims  agenttoken.Claims
	Mode    Mode
	Session identityservice.Session
	User    identityservice.PublicUser
}

// SessionFailure 是会话不再有效的原因。
type SessionFailure struct {
	Code           string
	Message        string
	RecoveryAction string
}

// CheckDesktopSession 验证 Token 的 sid 仍是该用户当前有效的 Desktop 会话。
// 上报档路由在请求发起新工作时自行调用（严格档由中间件调用）。
func CheckDesktopSession(claims agenttoken.Claims) (AgentAuth, *SessionFailure, error) {
	sessionContext, err := resolveDesktopSession(claims.SessionID)
	if err != nil {
		if failure, described := identityservice.DescribeAuthFailure(err); described {
			return AgentAuth{}, &SessionFailure{Code: failure.Code, Message: failure.Message, RecoveryAction: failure.RecoveryAction}, nil
		}
		return AgentAuth{}, nil, err
	}
	// 签名已把 sub 与 sid 绑在一起，但 Token 也不能认证成会话以外的用户。
	if int64(sessionContext.User.ID) != claims.UserID || sessionContext.Session.ID != claims.SessionID {
		return AgentAuth{}, &SessionFailure{Code: "AUTH_AGENT_TOKEN_INVALID", Message: "Agent 凭证与登录会话不一致", RecoveryAction: "reissue_agent_token"}, nil
	}
	return AgentAuth{Claims: claims, Session: sessionContext.Session, User: sessionContext.User}, nil, nil
}

// RequireAgent 只接受有效的 Agent Token；用户 session 不能在这里使用，两种凭证不可互代。
func RequireAgent(mode Mode) hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		auth, ok := authenticate(ctx, c, mode)
		if !ok {
			// Hertz 的 handler 链在中间件返回后仍会继续，只有 Abort 能停住；
			// 否则 handler 会在被拒的凭证后运行并写第二个响应体。
			c.Abort()
			return
		}
		c.Set(agentContextKey, auth)
		c.Next(ctx)
	}
}

// AgentAuthFrom 返回 RequireAgent 安装的认证信息。
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
	if mode == ReportSafeMode {
		return AgentAuth{Claims: claims, Mode: mode}, true
	}
	auth, failure, err := CheckDesktopSession(claims)
	if err != nil {
		hlog.Errorf("module=local_agent result=error path=%s error=%v", string(c.Path()), err)
		api.InternalError(c, "Agent 凭证校验失败")
		return AgentAuth{}, false
	}
	if failure != nil {
		writeAgentError(c, failure.Code, failure.Message, false, failure.RecoveryAction)
		return AgentAuth{}, false
	}
	auth.Mode = mode
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
