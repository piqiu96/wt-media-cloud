package localagent

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/agenttoken"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func stubSession(t *testing.T, scope func() (identityservice.AuthContext, bool)) {
	t.Helper()
	original := authenticateSession
	authenticateSession = func(c *hertzapp.RequestContext) (identityservice.AuthContext, bool) {
		authContext, ok := scope()
		if !ok {
			api.Unauthorized(c, 11001, "请先登录或凭证已过期")
			return identityservice.AuthContext{}, false
		}
		return authContext, true
	}
	t.Cleanup(func() { authenticateSession = original })
}

func stubIssuedAudit(t *testing.T, err error) *[]string {
	t.Helper()
	recorded := make([]string, 0, 1)
	original := recordTokenIssued
	recordTokenIssued = func(userID identityservice.UserID, sessionID string) error {
		recorded = append(recorded, sessionID)
		return err
	}
	t.Cleanup(func() { recordTokenIssued = original })
	return &recorded
}

func desktopSession(expiresAt time.Time) identityservice.AuthContext {
	return identityservice.AuthContext{
		User: identityservice.PublicUser{ID: 9, Username: "operator", Role: identityservice.RoleOperator, Status: "enabled"},
		Session: identityservice.Session{
			ID: "session-desktop", UserID: 9, ClientType: identityservice.ClientTypeDesktop, ExpiresAt: expiresAt,
		},
	}
}

type issueResult struct {
	status    int
	code      string
	token     string
	expiresAt int64
}

func issueProbe(t *testing.T) issueResult {
	t.Helper()
	engine := server.New()
	engine.POST("/api/v1/local-agent/agent-token", IssueAgentToken)
	result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/local-agent/agent-token", nil)
	var body struct {
		Data struct {
			Token     string `json:"token"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
		t.Fatalf("decode response %s: %v", result.Result().Body(), err)
	}
	return issueResult{
		status: result.Result().StatusCode(), code: body.Error.Code,
		token: body.Data.Token, expiresAt: body.Data.ExpiresAt,
	}
}

// Token 的 exp 只能是会话的 expires_at：登录不会比它签出的 Agent 凭证活得更久。
func TestIssueAgentTokenBindsTheTokenToTheDesktopSession(t *testing.T) {
	signer := configureSigner(t)
	stubIssuedAudit(t, nil)
	expiresAt := testNow.Add(30 * 24 * time.Hour)
	stubSession(t, func() (identityservice.AuthContext, bool) { return desktopSession(expiresAt), true })

	result := issueProbe(t)
	if result.status != consts.StatusOK {
		t.Fatalf("status = %d, code = %q", result.status, result.code)
	}
	if result.expiresAt != expiresAt.Unix() {
		t.Fatalf("expires_at = %d, want the session's %d", result.expiresAt, expiresAt.Unix())
	}
	claims, err := signer.Verify(result.token)
	if err != nil {
		t.Fatalf("issued token does not verify: %v", err)
	}
	if claims.UserID != 9 || claims.SessionID != "session-desktop" || claims.ExpiresAt != expiresAt.Unix() {
		t.Fatalf("claims = %+v", claims)
	}
}

// Web 登录不能换来 Agent 凭证：Token 代表一个 Desktop 会话，浏览器会话不是。
func TestIssueAgentTokenRefusesAWebSession(t *testing.T) {
	configureSigner(t)
	recorded := stubIssuedAudit(t, nil)
	web := desktopSession(testNow.Add(time.Hour))
	web.Session.ClientType = identityservice.ClientTypeWeb
	stubSession(t, func() (identityservice.AuthContext, bool) { return web, true })

	result := issueProbe(t)
	if result.status != consts.StatusUnauthorized || result.code != "DESKTOP_LOGIN_REQUIRED" {
		t.Fatalf("status = %d, code = %q", result.status, result.code)
	}
	if result.token != "" {
		t.Fatal("a web session received an Agent Token")
	}
	if len(*recorded) != 0 {
		t.Fatal("a refused issuance was audited as issued")
	}
}

// 高级运营和管理员可以登录 Desktop 使用共用页面，但不能取得本机执行凭证。
func TestIssueAgentTokenRefusesNonOperatorDesktopRoles(t *testing.T) {
	for _, role := range []identityservice.Role{identityservice.RoleSeniorOperator, identityservice.RoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			configureSigner(t)
			recorded := stubIssuedAudit(t, nil)
			desktop := desktopSession(testNow.Add(time.Hour))
			desktop.User.Role = role
			stubSession(t, func() (identityservice.AuthContext, bool) { return desktop, true })

			result := issueProbe(t)
			if result.status != consts.StatusForbidden || result.code != "DESKTOP_ROLE_FORBIDDEN" {
				t.Fatalf("status = %d, code = %q", result.status, result.code)
			}
			if result.token != "" {
				t.Fatalf("%s received an Agent Token", role)
			}
			if len(*recorded) != 0 {
				t.Fatalf("%s refused issuance was audited as issued", role)
			}
		})
	}
}

func TestIssueAgentTokenRequiresASession(t *testing.T) {
	configureSigner(t)
	stubIssuedAudit(t, nil)
	stubSession(t, func() (identityservice.AuthContext, bool) { return identityservice.AuthContext{}, false })

	result := issueProbe(t)
	if result.status != consts.StatusUnauthorized || result.token != "" {
		t.Fatalf("status = %d token set = %t", result.status, result.token != "")
	}
}

// 没有 signer 时签发等于发出谁都验不了的凭证，必须失败而不是返回空串。
func TestIssueAgentTokenFailsWhenTheSignerIsNotConfigured(t *testing.T) {
	agenttoken.Configure(nil)
	stubIssuedAudit(t, nil)
	stubSession(t, func() (identityservice.AuthContext, bool) { return desktopSession(testNow.Add(time.Hour)), true })

	result := issueProbe(t)
	if result.status != consts.StatusInternalServerError || result.token != "" {
		t.Fatalf("status = %d token set = %t", result.status, result.token != "")
	}
}

// 审计记录 Token 是为哪个会话签发的；审计写不进去时请求必须失败，
// 不能留下无记录的凭证。
func TestIssueAgentTokenFailsWhenTheAuditCannotBeWritten(t *testing.T) {
	configureSigner(t)
	recorded := stubIssuedAudit(t, errors.New("audit store unavailable"))
	stubSession(t, func() (identityservice.AuthContext, bool) { return desktopSession(testNow.Add(time.Hour)), true })

	result := issueProbe(t)
	if result.status != consts.StatusInternalServerError || result.token != "" {
		t.Fatalf("status = %d token set = %t", result.status, result.token != "")
	}
	if len(*recorded) != 1 || (*recorded)[0] != "session-desktop" {
		t.Fatalf("audit saw %v", *recorded)
	}
}
