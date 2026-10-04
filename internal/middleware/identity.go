package middleware

import (
	"context"
	"errors"
	"fmt"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/requestctx"
)

const (
	// SessionCookieName is the Cloud session cookie shared by identity login
	// and request authentication.
	SessionCookieName  = "wt_media_session"
	identityContextKey = "identity_auth_context"
)

// IdentityContext opportunistically resolves a valid session before business
// routes. Missing or invalid credentials do not reject public endpoints.
func IdentityContext() hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		token := sessionToken(c)
		if token == "" {
			c.Next(ctx)
			return
		}
		authContext, err := service.AuthenticateContext(token)
		if err != nil {
			c.Next(ctx)
			return
		}
		c.Set(identityContextKey, authContext)
		c.Set("user_id", int64(authContext.User.ID))
		c.Next(requestctx.WithUserID(ctx, int64(authContext.User.ID)))
	}
}

// AuthenticateRequest resolves the server-side session Cookie for Cloud
// business modules. Callers never receive the raw session token.
func AuthenticateRequest(c *hertzapp.RequestContext) (service.PublicUser, bool) {
	context, ok := AuthenticateRequestContext(c)
	return context.User, ok
}

// AuthenticateRequestContext is for trusted Cloud modules that must bind a
// resource to the current server-side session ID.
func AuthenticateRequestContext(c *hertzapp.RequestContext) (service.AuthContext, bool) {
	if cached, exists := c.Get(identityContextKey); exists {
		if authContext, valid := cached.(service.AuthContext); valid {
			return authContext, true
		}
	}
	token := sessionToken(c)
	authContext, err := service.AuthenticateContext(token)
	if err != nil {
		logAuthenticationFailure(c, token != "", err)
		writeAuthenticationError(c, err)
		return service.AuthContext{}, false
	}
	c.Set(identityContextKey, authContext)
	c.Set("user_id", int64(authContext.User.ID))
	return authContext, true
}

// logAuthenticationFailure records why a protected request was rejected so a
// 401 can be diagnosed from the online logs. It never records the session token
// itself, only whether one was presented.
func logAuthenticationFailure(c *hertzapp.RequestContext, credentialPresent bool, err error) {
	reason := authenticationFailureReason(credentialPresent, err)
	fields := fmt.Sprintf(
		"module=auth result=denied method=%s path=%s credential_present=%t ip=%s origin=%q reason=%s",
		string(c.Method()), string(c.Path()), credentialPresent, c.ClientIP(), string(c.GetHeader("Origin")), reason,
	)
	if reason == "internal_error" {
		hlog.Errorf("%s error=%v", fields, err)
		return
	}
	hlog.Warnf("%s", fields)
}

func authenticationFailureReason(credentialPresent bool, err error) string {
	switch {
	case !credentialPresent:
		return "missing_credential"
	case errors.Is(err, service.ErrAuthenticationFailed):
		return "authentication_failed"
	case errors.Is(err, service.ErrSessionInvalid):
		return "session_invalid"
	case errors.Is(err, service.ErrForbidden):
		return "forbidden"
	default:
		return "internal_error"
	}
}

func sessionToken(c *hertzapp.RequestContext) string {
	if token := string(c.Cookie(SessionCookieName)); token != "" {
		return token
	}
	return string(c.GetHeader("X-Session-Token"))
}

func writeAuthenticationError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, service.ErrAuthenticationFailed), errors.Is(err, service.ErrSessionInvalid):
		api.Unauthorized(c, 11001, "请先登录或凭证已过期")
	case errors.Is(err, service.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限执行此操作")
	default:
		api.InternalError(c, "用户服务内部错误")
	}
}
