package identity

import (
	"context"
	"encoding/json"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type errorEnvelope struct {
	ErrCode int    `json:"errcode"`
	Message string `json:"message"`
	Error   struct {
		Type           string         `json:"type"`
		Code           string         `json:"code"`
		Details        map[string]any `json:"details"`
		Retryable      bool           `json:"retryable"`
		RecoveryAction string         `json:"recovery_action"`
	} `json:"error"`
}

func performErrorRoute(t *testing.T, err error) (int, errorEnvelope) {
	t.Helper()
	engine := server.New()
	engine.GET("/test/error", func(_ context.Context, c *hertzapp.RequestContext) {
		writeIdentityError(c, err)
	})
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/test/error", nil)
	var envelope errorEnvelope
	if decodeErr := json.Unmarshal(response.Result().Body(), &envelope); decodeErr != nil {
		t.Fatalf("decode error envelope: %v, body = %s", decodeErr, response.Result().Body())
	}
	return response.Result().StatusCode(), envelope
}

func TestRegisterRoutesBindsAuthenticationRoutes(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/auth/me", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated /auth/me status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestAuthenticateRequestRejectsMissingSession(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/auth/me", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestForceLogoutRouteRequiresAuthentication(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/users/7/force-logout", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated force-logout status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestDesktopLoginConflictUsesTypedEnvelope(t *testing.T) {
	status, envelope := performErrorRoute(t, &identityservice.DesktopLoginConflictError{ActiveSessionID: "session-active"})
	if status != consts.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if envelope.ErrCode != 20010 || envelope.Error.Code != "DESKTOP_LOGIN_CONFLICT" || envelope.Error.Type != "DESKTOP_LOGIN_CONFLICT" {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.Error.Retryable || envelope.Error.RecoveryAction != "confirm_desktop_takeover" {
		t.Fatalf("retryable/recovery = %v/%q", envelope.Error.Retryable, envelope.Error.RecoveryAction)
	}
	if envelope.Error.Details["active_session_id"] != "session-active" {
		t.Fatalf("details = %+v", envelope.Error.Details)
	}
}

func TestReplacedDesktopSessionUsesTypedEnvelope(t *testing.T) {
	status, envelope := performErrorRoute(t, identityservice.ErrDesktopReplaced)
	if status != consts.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	if envelope.ErrCode != 11001 || envelope.Error.Code != "AUTH_DESKTOP_REPLACED" {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.Message != "你的账号已在另一台电脑登录" || envelope.Error.RecoveryAction != "login_again" {
		t.Fatalf("message/recovery = %q/%q", envelope.Message, envelope.Error.RecoveryAction)
	}
}
