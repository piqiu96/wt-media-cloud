package identity

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/common"
)

const SessionCookieName = "wt_media_session"

type RouteConfig struct {
	CookieSecure bool
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type createUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Role     Role     `json:"role"`
	GameIDs  []string `json:"game_ids"`
}

type updateUserRequest struct {
	Role    Role       `json:"role"`
	Status  UserStatus `json:"status"`
	GameIDs []string   `json:"game_ids"`
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func RegisterRoutes(h *server.Hertz, service *Service, cfg RouteConfig) {
	h.POST("/api/v1/auth/login", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req loginRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		if context, err := service.AuthenticateContext(string(c.Cookie(SessionCookieName))); err == nil && context.User.Username == strings.TrimSpace(req.Username) {
			common.Success(c, context.User)
			return
		}
		result, err := service.Login(req.Username, req.Password)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		c.SetCookie(SessionCookieName, result.Token, 0, "/", "", protocol.CookieSameSiteLaxMode, cfg.CookieSecure, true)
		common.Success(c, result.User)
	})

	h.GET("/api/v1/auth/me", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		common.Success(c, actor)
	})

	h.POST("/api/v1/auth/logout", func(ctx context.Context, c *hertzapp.RequestContext) {
		token := string(c.Cookie(SessionCookieName))
		if err := service.Logout(token); err != nil {
			writeIdentityError(c, err)
			return
		}
		c.SetCookie(SessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, cfg.CookieSecure, true)
		common.NoContent(c)
	})

	h.POST("/api/v1/users", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		var req createUserRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		user, err := service.CreateUser(actor.ID, CreateUserInput(req))
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Created(c, user)
	})

	h.PATCH("/api/v1/users/:user_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		var req updateUserRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		user, err := service.UpdateUserAccess(actor.ID, c.Param("user_id"), req.Role, req.GameIDs)
		if err == nil && req.Status != "" {
			err = service.SetUserStatus(actor.ID, c.Param("user_id"), req.Status)
			user.Status = req.Status
		}
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, user)
	})

	h.POST("/api/v1/users/:user_id/reset-password", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		var req passwordRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		if err := service.ResetPassword(actor.ID, c.Param("user_id"), req.NewPassword); err != nil {
			writeIdentityError(c, err)
			return
		}
		common.NoContent(c)
	})

	h.POST("/api/v1/auth/change-password", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		var req passwordRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		if err := service.ChangeOwnPassword(actor.ID, req.CurrentPassword, req.NewPassword); err != nil {
			writeIdentityError(c, err)
			return
		}
		c.SetCookie(SessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, cfg.CookieSecure, true)
		common.NoContent(c)
	})

	h.GET("/api/v1/users", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		if actor.Role != RoleTechnician {
			writeIdentityError(c, ErrForbidden)
			return
		}
		users, err := service.ListUsers()
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, users)
	})
	h.GET("/api/v1/audit-logs", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		logs, err := service.ListAuditLogs(50)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, logs)
	})
}

// AuthenticateRequest resolves the server-side session Cookie for other Cloud
// business modules. Callers never receive the raw session token.
func AuthenticateRequest(c *hertzapp.RequestContext, service *Service) (PublicUser, bool) {
	context, ok := AuthenticateRequestContext(c, service)
	return context.User, ok
}

// AuthenticateRequestContext is for trusted Cloud modules that must bind a
// resource to the current server-side session ID. It never returns the raw
// Cookie token to an API response.
func AuthenticateRequestContext(c *hertzapp.RequestContext, service *Service) (AuthContext, bool) {
	context, err := service.AuthenticateContext(string(c.Cookie(SessionCookieName)))
	if err != nil {
		writeIdentityError(c, err)
		return AuthContext{}, false
	}
	return context, true
}

func writeIdentityError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrAuthenticationFailed), errors.Is(err, ErrSessionInvalid):
		common.Unauthorized(c, 11001, "请先登录或凭证已过期")
	case errors.Is(err, ErrForbidden):
		common.Forbidden(c, 11003, "没有权限执行此操作")
	case errors.Is(err, ErrUsernameTaken):
		common.Conflict(c, 20001, "用户名已被使用")
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrBootstrapUnavailable):
		common.BadRequest(c, 10001, "用户信息格式错误")
	default:
		common.InternalError(c, "用户服务内部错误")
	}
}
