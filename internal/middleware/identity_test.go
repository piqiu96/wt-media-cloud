package middleware

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

func TestAuthenticationFailureReason(t *testing.T) {
	cases := []struct {
		name       string
		credential bool
		err        error
		want       string
	}{
		{"missing credential", false, service.ErrSessionInvalid, "missing_credential"},
		{"authentication failed", true, service.ErrAuthenticationFailed, "authentication_failed"},
		{"session invalid", true, service.ErrSessionInvalid, "session_invalid"},
		{"forbidden", true, service.ErrForbidden, "forbidden"},
		{"internal error", true, errors.New("boom"), "internal_error"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := authenticationFailureReason(testCase.credential, testCase.err); got != testCase.want {
				t.Fatalf("authenticationFailureReason() = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A rejected /auth/me must leave a diagnostic line in the log so the online
// environment can tell "no cookie" from "expired session".
func TestLogAuthenticationFailureWritesReason(t *testing.T) {
	dir := initializeRequestLogger(t)
	previous := hlog.DefaultLogger()
	hlog.SetLogger(logger.App())
	t.Cleanup(func() { hlog.SetLogger(previous) })

	engine := server.New()
	engine.GET("/api/v1/auth/me", func(_ context.Context, c *hertzapp.RequestContext) {
		logAuthenticationFailure(c, false, service.ErrSessionInvalid)
		c.Status(consts.StatusUnauthorized)
	})
	result := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/auth/me", nil)
	if result.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status = %d", result.Result().StatusCode())
	}

	raw, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("read app.log: %v", err)
	}
	logged := string(raw)
	for _, fragment := range []string{"module=auth", "result=denied", "reason=missing_credential", "path=/api/v1/auth/me"} {
		if !strings.Contains(logged, fragment) {
			t.Errorf("app.log is missing %q; got:\n%s", fragment, logged)
		}
	}
	if strings.Contains(logged, "credential_present=true") {
		t.Errorf("missing-credential log must not claim a credential was present:\n%s", logged)
	}
}
