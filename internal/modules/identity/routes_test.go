package identity

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestLoginUsesHttpOnlyCookieAndReplacementRejectsOldSession(t *testing.T) {
	service := NewService(NewMemoryStore())
	if _, err := service.BootstrapTechnician("tech", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	engine := server.New()
	RegisterRoutes(engine, service, RouteConfig{CookieSecure: false})

	first := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"tech","password":"a-long-initial-password"}`)
	if first.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("first login status = %d, body = %s", first.Result().StatusCode(), first.Result().Body())
	}
	firstCookie := string(first.Result().Header.Peek("Set-Cookie"))
	if !strings.Contains(firstCookie, SessionCookieName+"=") || !strings.Contains(firstCookie, "HttpOnly") || !strings.Contains(firstCookie, "SameSite=Lax") {
		t.Fatalf("first Set-Cookie = %q", firstCookie)
	}
	if strings.Contains(string(first.Result().Body()), "session-first") || strings.Contains(string(first.Result().Body()), "password") {
		t.Fatalf("login response leaked secret material: %s", first.Result().Body())
	}

	second := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"tech","password":"a-long-initial-password"}`)
	secondCookie := string(second.Result().Header.Peek("Set-Cookie"))
	if secondCookie == "" || secondCookie == firstCookie {
		t.Fatalf("replacement Set-Cookie = %q", secondCookie)
	}

	oldMe := ut.PerformRequest(engine.Engine, "GET", "/api/v1/auth/me", nil, ut.Header{Key: "Cookie", Value: firstCookie})
	if oldMe.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("old session status = %d, body = %s", oldMe.Result().StatusCode(), oldMe.Result().Body())
	}
	newMe := ut.PerformRequest(engine.Engine, "GET", "/api/v1/auth/me", nil, ut.Header{Key: "Cookie", Value: secondCookie})
	if newMe.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("new session status = %d, body = %s", newMe.Result().StatusCode(), newMe.Result().Body())
	}
}

func performJSON(engine *server.Hertz, method, path, payload string) *ut.ResponseRecorder {
	return ut.PerformRequest(
		engine.Engine,
		method,
		path,
		&ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
}
