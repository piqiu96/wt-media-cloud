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
		User: identityservice.PublicUser{ID: 9, Username: "operator", Status: "enabled"},
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

// The issued token's exp is the session's expires_at and nothing else: that is
// what keeps a login from outliving the Agent credential it produced.
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

// A web login must not yield an Agent credential: the token speaks for one
// Desktop session, and a browser session is not one.
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

func TestIssueAgentTokenRequiresASession(t *testing.T) {
	configureSigner(t)
	stubIssuedAudit(t, nil)
	stubSession(t, func() (identityservice.AuthContext, bool) { return identityservice.AuthContext{}, false })

	result := issueProbe(t)
	if result.status != consts.StatusUnauthorized || result.token != "" {
		t.Fatalf("status = %d token set = %t", result.status, result.token != "")
	}
}

// Issuing without a signer would hand out tokens nobody can verify, so it must
// fail instead of returning an empty credential.
func TestIssueAgentTokenFailsWhenTheSignerIsNotConfigured(t *testing.T) {
	agenttoken.Configure(nil)
	stubIssuedAudit(t, nil)
	stubSession(t, func() (identityservice.AuthContext, bool) { return desktopSession(testNow.Add(time.Hour)), true })

	result := issueProbe(t)
	if result.status != consts.StatusInternalServerError || result.token != "" {
		t.Fatalf("status = %d token set = %t", result.status, result.token != "")
	}
}

// The audit records which session the token was issued for; a failed audit must
// fail the request rather than leave an unlogged credential in the wild.
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
