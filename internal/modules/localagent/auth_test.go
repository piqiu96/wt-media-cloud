package localagent

import (
	"context"
	"encoding/json"
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

const testTokenSecret = "test-agent-token-secret-with-at-least-32-characters"

var testNow = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

func configureSigner(t *testing.T) *agenttoken.Signer {
	t.Helper()
	signer, err := agenttoken.NewSigner([]byte(testTokenSecret))
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}
	signer.WithClock(func() time.Time { return testNow })
	agenttoken.Configure(signer)
	t.Cleanup(func() { agenttoken.Configure(nil) })
	return signer
}

func stubDesktopSession(t *testing.T, resolve func(string) (identityservice.AuthContext, error)) *int {
	t.Helper()
	calls := 0
	original := resolveDesktopSession
	resolveDesktopSession = func(sessionID string) (identityservice.AuthContext, error) {
		calls++
		return resolve(sessionID)
	}
	t.Cleanup(func() { resolveDesktopSession = original })
	return &calls
}

func currentDesktopContext(sessionID string, expiresAt time.Time) identityservice.AuthContext {
	return identityservice.AuthContext{
		User: identityservice.PublicUser{ID: 9, Username: "operator", Status: "enabled"},
		Session: identityservice.Session{
			ID: sessionID, UserID: 9, ClientType: identityservice.ClientTypeDesktop, ExpiresAt: expiresAt,
		},
	}
}

type probeResult struct {
	handled bool
	auth    AgentAuth
	status  int
	code    string
}

func probe(t *testing.T, mode Mode, token string) probeResult {
	t.Helper()
	handled := false
	var seen AgentAuth
	engine := server.New()
	engine.POST("/probe", RequireAgent(mode), func(_ context.Context, c *hertzapp.RequestContext) {
		handled = true
		seen, _ = AgentAuthFrom(c)
		api.Success(c, map[string]any{"ok": true})
	})
	var headers []ut.Header
	if token != "" {
		headers = append(headers, ut.Header{Key: "Authorization", Value: "Bearer " + token})
	}
	result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/probe", nil, headers...)
	body := result.Result().Body()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	return probeResult{handled: handled, auth: seen, status: result.Result().StatusCode(), code: envelope.Error.Code}
}

// 被拒的凭证必须停住 handler 链，而不只是写响应体：否则 handler 会替未认证调用方
// 执行，并追加第二份 JSON。
func TestRequireAgentStopsTheChainWhenTheCredentialIsMissing(t *testing.T) {
	configureSigner(t)
	result := probe(t, StrictMode, "")
	if result.status != consts.StatusUnauthorized || result.code != "AUTH_AGENT_TOKEN_INVALID" {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
	if result.handled {
		t.Fatal("the handler ran behind a rejected Agent credential")
	}
}

func TestRequireAgentRejectsAnExpiredToken(t *testing.T) {
	signer := configureSigner(t)
	token, _, err := signer.Issue(9, "session-1", testNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	signer.WithClock(func() time.Time { return testNow.Add(2 * time.Minute) })

	result := probe(t, StrictMode, token)
	if result.status != consts.StatusUnauthorized || result.code != "AUTH_AGENT_TOKEN_EXPIRED" {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
	if result.handled {
		t.Fatal("the handler ran behind an expired Agent credential")
	}
}

// 严格档决定新工作：被替换的 Desktop 必须失去它——Token 仍签名有效、未过期，
// 但 sid 已不是该用户当前的会话。
func TestStrictModeRefusesANewOperationAfterTheDesktopIsReplaced(t *testing.T) {
	signer := configureSigner(t)
	token, _, _ := signer.Issue(9, "session-1", testNow.Add(time.Hour))
	stubDesktopSession(t, func(string) (identityservice.AuthContext, error) {
		return identityservice.AuthContext{}, identityservice.ErrDesktopReplaced
	})

	result := probe(t, StrictMode, token)
	if result.status != consts.StatusUnauthorized || result.code != "AUTH_DESKTOP_REPLACED" {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
	if result.handled {
		t.Fatal("a replaced Desktop still reached the Agent handler in strict mode")
	}
}

func TestStrictModeRequiresATokenBoundToTheCurrentSession(t *testing.T) {
	signer := configureSigner(t)
	token, _, _ := signer.Issue(9, "session-1", testNow.Add(time.Hour))
	stubDesktopSession(t, func(string) (identityservice.AuthContext, error) {
		return currentDesktopContext("session-other", testNow.Add(time.Hour)), nil
	})

	result := probe(t, StrictMode, token)
	if result.code != "AUTH_AGENT_TOKEN_INVALID" || result.handled {
		t.Fatalf("status=%d code=%q handled=%t", result.status, result.code, result.handled)
	}
}

func TestStrictModeCarriesTheSessionAndUserToTheHandler(t *testing.T) {
	signer := configureSigner(t)
	token, claims, _ := signer.Issue(9, "session-1", testNow.Add(time.Hour))
	stubDesktopSession(t, func(sessionID string) (identityservice.AuthContext, error) {
		if sessionID != "session-1" {
			t.Fatalf("resolver asked for %q, want the token's session", sessionID)
		}
		return currentDesktopContext("session-1", testNow.Add(time.Hour)), nil
	})

	result := probe(t, StrictMode, token)
	if result.status != consts.StatusOK || !result.handled {
		t.Fatalf("status=%d handled=%t code=%q", result.status, result.handled, result.code)
	}
	if result.auth.Claims.UserID != claims.UserID || result.auth.User.ID != 9 || result.auth.Session.ID != "session-1" {
		t.Fatalf("handler saw %+v", result.auth)
	}
}

// 已领取工作的上报必须跨过抢占：Token 是唯一凭证，任务归属和 attempt 决定上报是否落库。
func TestReportSafeModeAcceptsAReportAfterTheDesktopIsReplaced(t *testing.T) {
	signer := configureSigner(t)
	token, claims, _ := signer.Issue(9, "session-1", testNow.Add(time.Hour))
	stubDesktopSession(t, func(string) (identityservice.AuthContext, error) {
		return identityservice.AuthContext{}, identityservice.ErrDesktopReplaced
	})

	result := probe(t, ReportSafeMode, token)
	if result.status != consts.StatusOK || !result.handled {
		t.Fatalf("status=%d handled=%t code=%q", result.status, result.handled, result.code)
	}
	if result.auth.Claims.UserID != claims.UserID || result.auth.Mode != ReportSafeMode {
		t.Fatalf("handler saw %+v", result.auth)
	}
}

// 上报档完全不查会话——连读都不读。
func TestReportSafeModeDoesNotConsultTheDesktopSession(t *testing.T) {
	signer := configureSigner(t)
	token, _, _ := signer.Issue(9, "session-1", testNow.Add(time.Hour))
	calls := stubDesktopSession(t, func(string) (identityservice.AuthContext, error) {
		return currentDesktopContext("session-1", testNow.Add(time.Hour)), nil
	})

	result := probe(t, ReportSafeMode, token)
	if result.status != consts.StatusOK || !result.handled {
		t.Fatalf("status=%d handled=%t", result.status, result.handled)
	}
	if *calls != 0 {
		t.Fatalf("report-safe mode looked up the session %d time(s)", *calls)
	}
}
