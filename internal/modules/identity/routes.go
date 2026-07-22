package identity

import (
	"context"
	"errors"
	"strconv"
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
	TeamID   *TeamID  `json:"team_id"`
	GameIDs  []string `json:"game_ids"`
}

type updateUserRequest struct {
	Role    Role       `json:"role"`
	TeamID  *TeamID    `json:"team_id"`
	Status  UserStatus `json:"status"`
	GameIDs []string   `json:"game_ids"`
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type teamRequest struct {
	Name string `json:"name"`
}

type oneTimePasswordResult struct {
	User            PublicUser `json:"user"`
	OneTimePassword string     `json:"one_time_password"`
}

type passwordResetResult struct {
	OneTimePassword string `json:"one_time_password"`
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
		common.Created(c, oneTimePasswordResult{User: user, OneTimePassword: req.Password})
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
		userID, valid := parseUserID(c.Param("user_id"))
		if !valid {
			writeIdentityError(c, ErrInvalidInput)
			return
		}
		user, err := service.UpdateUser(actor.ID, userID, req.Role, req.TeamID, req.GameIDs, req.Status)
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
		userID, valid := parseUserID(c.Param("user_id"))
		if !valid {
			writeIdentityError(c, ErrInvalidInput)
			return
		}
		if err := service.ResetPassword(actor.ID, userID, req.NewPassword); err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, passwordResetResult{OneTimePassword: req.NewPassword})
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
		if actor.Role != RoleAdmin {
			writeIdentityError(c, ErrForbidden)
			return
		}
		users, err := service.ListUsers()
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		filtered, err := filterUsers(users, c)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, filtered)
	})

	h.DELETE("/api/v1/users/:user_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		userID, valid := parseUserID(c.Param("user_id"))
		if !valid {
			writeIdentityError(c, ErrInvalidInput)
			return
		}
		if err := service.DeleteUser(actor.ID, userID); err != nil {
			writeIdentityError(c, err)
			return
		}
		common.NoContent(c)
	})

	h.GET("/api/v1/operation-teams", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		teams, err := service.ListTeams(actor.ID)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, teams)
	})

	h.POST("/api/v1/operation-teams", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		var req teamRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		team, err := service.CreateTeam(actor.ID, req.Name)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Created(c, team)
	})

	h.PATCH("/api/v1/operation-teams/:team_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		teamID, valid := parseTeamID(c.Param("team_id"))
		if !valid {
			writeIdentityError(c, ErrInvalidInput)
			return
		}
		var req teamRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		team, err := service.RenameTeam(actor.ID, teamID, req.Name)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		common.Success(c, team)
	})

	h.DELETE("/api/v1/operation-teams/:team_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		teamID, valid := parseTeamID(c.Param("team_id"))
		if !valid {
			writeIdentityError(c, ErrInvalidInput)
			return
		}
		if err := service.DeleteTeam(actor.ID, teamID); err != nil {
			writeIdentityError(c, err)
			return
		}
		common.NoContent(c)
	})
	h.GET("/api/v1/audit-logs", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := AuthenticateRequest(c, service)
		if !ok {
			return
		}
		if actor.Role != RoleAdmin {
			writeIdentityError(c, ErrForbidden)
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

func parseUserID(value string) (UserID, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return UserID(parsed), err == nil && parsed > 0
}

func parseTeamID(value string) (TeamID, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return TeamID(parsed), err == nil && parsed > 0
}

func filterUsers(users []PublicUser, c *hertzapp.RequestContext) ([]PublicUser, error) {
	var uid UserID
	if raw := strings.TrimSpace(c.Query("uid")); raw != "" {
		parsed, valid := parseUserID(raw)
		if !valid {
			return nil, ErrInvalidInput
		}
		uid = parsed
	}
	var teamID TeamID
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		parsed, valid := parseTeamID(raw)
		if !valid {
			return nil, ErrInvalidInput
		}
		teamID = parsed
	}
	username := strings.ToLower(strings.TrimSpace(c.Query("username")))
	role := Role(strings.TrimSpace(c.Query("role")))
	status := UserStatus(strings.TrimSpace(c.Query("status")))
	gameID := strings.TrimSpace(c.Query("game_id"))
	if role != "" && !validRole(role) {
		return nil, ErrInvalidInput
	}
	if status != "" && status != UserStatusEnabled && status != UserStatusDisabled {
		return nil, ErrInvalidInput
	}
	result := make([]PublicUser, 0, len(users))
	for _, user := range users {
		if uid > 0 && user.ID != uid {
			continue
		}
		if username != "" && !strings.Contains(strings.ToLower(user.Username), username) {
			continue
		}
		if role != "" && user.Role != role {
			continue
		}
		if teamID > 0 && (user.TeamID == nil || *user.TeamID != teamID) {
			continue
		}
		if status != "" && user.Status != status {
			continue
		}
		if gameID != "" && !containsGame(user.GameIDs, gameID) {
			continue
		}
		result = append(result, user)
	}
	return result, nil
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
	case errors.Is(err, ErrTeamNameTaken):
		common.Conflict(c, 20002, "分组名称已被使用")
	case errors.Is(err, ErrTeamInUse):
		common.Conflict(c, 20003, "对象仍有业务引用，不能删除")
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrBootstrapUnavailable):
		common.BadRequest(c, 10001, "用户信息格式错误")
	default:
		common.InternalError(c, "用户服务内部错误")
	}
}
