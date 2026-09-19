package identity

import (
	"context"
	"errors"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/requestctx"
)

const SessionCookieName = "wt_media_session"
const identityContextKey = "identity_auth_context"

type RouteConfig struct {
	CookieSecure bool
}

// isLocalDesktopOrigin reports whether the request originates from the packaged
// Desktop WebView. It mirrors the process CORS allowlist so token/cookie
// handling agrees with which origins are treated as the Desktop.
func isLocalDesktopOrigin(origin string) bool {
	switch strings.TrimSpace(origin) {
	case "http://tauri.localhost", "https://tauri.localhost", "tauri://localhost":
		return true
	}
	return false
}

// sessionCookieMode returns the SameSite/Secure attributes for the session
// cookie. The packaged Desktop page runs on http://tauri.localhost, which is
// cross-site to the local Cloud API (127.0.0.1); SameSite=Lax cookies are not
// sent on cross-site fetches, so the session would never round-trip. Use
// SameSite=None + Secure for the Desktop origin and keep Lax for same-site
// Cloud Web clients.
func sessionCookieMode(origin string, defaultSecure bool) (protocol.CookieSameSite, bool) {
	if isLocalDesktopOrigin(origin) {
		return protocol.CookieSameSiteNoneMode, true
	}
	return protocol.CookieSameSiteLaxMode, defaultSecure
}

type loginRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ReplaceExisting bool   `json:"replace_existing"`
}

type createUserRequest struct {
	Username string                  `json:"username"`
	Password string                  `json:"password"`
	Role     identityservice.Role    `json:"role"`
	TeamID   *identityservice.TeamID `json:"team_id"`
	GameIDs  []string                `json:"game_ids"`
}

type updateUserRequest struct {
	Role    identityservice.Role       `json:"role"`
	TeamID  *identityservice.TeamID    `json:"team_id"`
	Status  identityservice.UserStatus `json:"status"`
	GameIDs []string                   `json:"game_ids"`
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type teamRequest struct {
	Name string `json:"name"`
}

type gameRequest struct {
	ID     string                     `json:"id"`
	Name   string                     `json:"name"`
	Status identityservice.GameStatus `json:"status"`
	Remark string                     `json:"remark"`
}

type oneTimePasswordResult struct {
	User            identityservice.PublicUser `json:"user"`
	OneTimePassword string                     `json:"one_time_password"`
}

type passwordResetResult struct {
	OneTimePassword string `json:"one_time_password"`
}

func Login(ctx context.Context, c *hertzapp.RequestContext) {
	var req loginRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	if context, err := identityservice.AuthenticateContext(sessionToken(c)); err == nil && context.User.Username == strings.TrimSpace(req.Username) {
		api.Success(c, context.User)
		return
	}
	result, err := identityservice.LoginWithOptions(req.Username, req.Password, identityservice.LoginOptions{ReplaceExisting: req.ReplaceExisting})
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	sameSite, secure := sessionCookieMode(string(c.GetHeader("Origin")), config.Get().App.Server.SessionCookieSecure)
	c.SetCookie(SessionCookieName, result.Token, 0, "/", "", sameSite, secure, true)
	if isLocalDesktopOrigin(string(c.GetHeader("Origin"))) {
		// Packaged Desktop cannot round-trip the cross-site cookie, so it
		// receives the session token in the body and sends it back as the
		// X-Session-Token header. Cloud Web keeps the HttpOnly cookie path.
		api.Success(c, map[string]any{"user": result.User, "token": result.Token})
		return
	}
	api.Success(c, result.User)
}
func Me(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	api.Success(c, actor)
}
func Logout(ctx context.Context, c *hertzapp.RequestContext) {
	token := string(c.Cookie(SessionCookieName))
	if err := identityservice.Logout(token); err != nil {
		writeIdentityError(c, err)
		return
	}
	sameSite, secure := sessionCookieMode(string(c.GetHeader("Origin")), config.Get().App.Server.SessionCookieSecure)
	c.SetCookie(SessionCookieName, "", -1, "/", "", sameSite, secure, true)
	api.NoContent(c)
}
func CreateUser(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req createUserRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	user, err := identityservice.CreateUser(actor.ID, identityservice.CreateUserInput(req))
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Created(c, oneTimePasswordResult{User: user, OneTimePassword: req.Password})
}
func UpdateUser(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req updateUserRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	userID, valid := parseUserID(c.Param("user_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	user, err := identityservice.UpdateUser(actor.ID, userID, req.Role, req.TeamID, req.GameIDs, req.Status)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, user)
}
func ResetPassword(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req passwordRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	userID, valid := parseUserID(c.Param("user_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	if err := identityservice.ResetPassword(actor.ID, userID, req.NewPassword); err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, passwordResetResult{OneTimePassword: req.NewPassword})
}
func ChangePassword(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req passwordRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	if err := identityservice.ChangeOwnPassword(actor.ID, req.CurrentPassword, req.NewPassword); err != nil {
		writeIdentityError(c, err)
		return
	}
	sameSite, secure := sessionCookieMode(string(c.GetHeader("Origin")), config.Get().App.Server.SessionCookieSecure)
	c.SetCookie(SessionCookieName, "", -1, "/", "", sameSite, secure, true)
	api.NoContent(c)
}
func ListUsers(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	if actor.Role != identityservice.RoleAdmin {
		writeIdentityError(c, identityservice.ErrForbidden)
		return
	}
	users, err := identityservice.ListUsers()
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	filtered, err := filterUsers(users, c)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, filtered)
}
func DeleteUser(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	userID, valid := parseUserID(c.Param("user_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	if err := identityservice.DeleteUser(actor.ID, userID); err != nil {
		writeIdentityError(c, err)
		return
	}
	api.NoContent(c)
}
func ListTeams(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	teams, err := identityservice.ListTeams(actor.ID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, teams)
}
func CreateTeam(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req teamRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	team, err := identityservice.CreateTeam(actor.ID, req.Name)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Created(c, team)
}
func UpdateTeam(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	teamID, valid := parseTeamID(c.Param("team_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	var req teamRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	team, err := identityservice.RenameTeam(actor.ID, teamID, req.Name)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, team)
}
func DeleteTeam(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	teamID, valid := parseTeamID(c.Param("team_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	if err := identityservice.DeleteTeam(actor.ID, teamID); err != nil {
		writeIdentityError(c, err)
		return
	}
	api.NoContent(c)
}
func ListGames(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	games, err := identityservice.ListGames(actor.ID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, filterGames(games, c))
}
func GameReferences(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	references, err := identityservice.GameReferences(actor.ID, c.Param("game_id"))
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, references)
}
func CreateGame(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req gameRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	game, err := identityservice.CreateGame(actor.ID, req.ID, req.Name, req.Remark)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Created(c, game)
}
func UpdateGame(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	var req gameRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	game, err := identityservice.UpdateGame(actor.ID, c.Param("game_id"), req.Name, req.Status, req.Remark)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, game)
}
func DeleteGame(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	if err := identityservice.DeleteGame(actor.ID, c.Param("game_id")); err != nil {
		writeIdentityError(c, err)
		return
	}
	api.NoContent(c)
}
func ListAuditLogs(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := AuthenticateRequest(c)
	if !ok {
		return
	}
	if actor.Role != identityservice.RoleAdmin {
		writeIdentityError(c, identityservice.ErrForbidden)
		return
	}
	logs, err := identityservice.ListAuditLogs(50)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, logs)
}

func filterGames(games []identityservice.OperationGame, c *hertzapp.RequestContext) []identityservice.OperationGame {
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	status := identityservice.GameStatus(strings.TrimSpace(c.Query("status")))
	result := make([]identityservice.OperationGame, 0, len(games))
	for _, game := range games {
		if keyword != "" && !strings.Contains(strings.ToLower(game.ID+" "+game.Name), keyword) {
			continue
		}
		if status != "" && game.Status != status {
			continue
		}
		result = append(result, game)
	}
	return result
}

func parseUserID(value string) (identityservice.UserID, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return identityservice.UserID(parsed), err == nil && parsed > 0
}

func parseTeamID(value string) (identityservice.TeamID, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return identityservice.TeamID(parsed), err == nil && parsed > 0
}

func validRole(role identityservice.Role) bool {
	return role == identityservice.RoleOperator || role == identityservice.RoleSeniorOperator || role == identityservice.RoleAdmin
}

func containsGame(gameIDs []string, gameID string) bool {
	for _, candidate := range gameIDs {
		if candidate == gameID {
			return true
		}
	}
	return false
}

func filterUsers(users []identityservice.PublicUser, c *hertzapp.RequestContext) ([]identityservice.PublicUser, error) {
	var uid identityservice.UserID
	if raw := strings.TrimSpace(c.Query("uid")); raw != "" {
		parsed, valid := parseUserID(raw)
		if !valid {
			return nil, identityservice.ErrInvalidInput
		}
		uid = parsed
	}
	var teamID identityservice.TeamID
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		parsed, valid := parseTeamID(raw)
		if !valid {
			return nil, identityservice.ErrInvalidInput
		}
		teamID = parsed
	}
	username := strings.ToLower(strings.TrimSpace(c.Query("username")))
	role := identityservice.Role(strings.TrimSpace(c.Query("role")))
	status := identityservice.UserStatus(strings.TrimSpace(c.Query("status")))
	gameID := strings.TrimSpace(c.Query("game_id"))
	if role != "" && !validRole(role) {
		return nil, identityservice.ErrInvalidInput
	}
	if status != "" && status != identityservice.UserStatusEnabled && status != identityservice.UserStatusDisabled {
		return nil, identityservice.ErrInvalidInput
	}
	result := make([]identityservice.PublicUser, 0, len(users))
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

// IdentityContext opportunistically resolves a valid session before business
// routes. Missing or invalid credentials do not reject public endpoints.
func IdentityContext() hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		token := sessionToken(c)
		if token == "" {
			c.Next(ctx)
			return
		}
		authContext, err := identityservice.AuthenticateContext(token)
		if err != nil {
			c.Next(ctx)
			return
		}
		c.Set(identityContextKey, authContext)
		c.Set("user_id", int64(authContext.User.ID))
		c.Next(requestctx.WithUserID(ctx, int64(authContext.User.ID)))
	}
}

// AuthenticateRequest resolves the server-side session Cookie for other Cloud
// business modules. Callers never receive the raw session token.
func AuthenticateRequest(c *hertzapp.RequestContext) (identityservice.PublicUser, bool) {
	context, ok := AuthenticateRequestContext(c)
	return context.User, ok
}

// sessionToken returns the current session token from the cookie first, then
// falls back to the X-Session-Token header. The packaged Desktop WebView is
// cross-site to the local Cloud API and does not round-trip cookies, so it
// carries the token as a header instead.
func sessionToken(c *hertzapp.RequestContext) string {
	if token := string(c.Cookie(SessionCookieName)); token != "" {
		return token
	}
	return string(c.GetHeader("X-Session-Token"))
}

// AuthenticateRequestContext is for trusted Cloud modules that must bind a
// resource to the current server-side session ID. It never returns the raw
// Cookie token to an API api.
func AuthenticateRequestContext(c *hertzapp.RequestContext) (identityservice.AuthContext, bool) {
	if cached, exists := c.Get(identityContextKey); exists {
		if authContext, valid := cached.(identityservice.AuthContext); valid {
			return authContext, true
		}
	}
	authContext, err := identityservice.AuthenticateContext(sessionToken(c))
	if err != nil {
		writeIdentityError(c, err)
		return identityservice.AuthContext{}, false
	}
	c.Set(identityContextKey, authContext)
	c.Set("user_id", int64(authContext.User.ID))
	return authContext, true
}

func writeIdentityError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, identityservice.ErrAuthenticationFailed), errors.Is(err, identityservice.ErrSessionInvalid):
		api.Unauthorized(c, 11001, "请先登录或凭证已过期")
	case errors.Is(err, identityservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限执行此操作")
	case errors.Is(err, identityservice.ErrUsernameTaken):
		api.Conflict(c, 20001, "用户名已被使用")
	case errors.Is(err, identityservice.ErrTeamNameTaken):
		api.Conflict(c, 20002, "分组名称已被使用")
	case errors.Is(err, identityservice.ErrTeamInUse):
		api.Conflict(c, 20003, "该分组下还有用户或业务引用，不能删除，请先转移用户")
	case errors.Is(err, identityservice.ErrGameIDTaken):
		api.Conflict(c, 20006, "游戏ID已被使用")
	case errors.Is(err, identityservice.ErrInvalidGameID):
		api.BadRequest(c, 10004, "游戏ID仅支持32位以内字母或数字")
	case errors.Is(err, identityservice.ErrGameNameTaken):
		api.Conflict(c, 20004, "游戏名称已被使用")
	case errors.Is(err, identityservice.ErrGameInUse):
		api.Conflict(c, 20005, "该游戏已分配给用户或已有业务引用，不能停用或删除，请先调整用户游戏范围")
	case errors.Is(err, identityservice.ErrGameUnavailable):
		api.BadRequest(c, 10002, "请选择已启用的游戏")
	case errors.Is(err, identityservice.ErrPasswordTooShort):
		api.BadRequest(c, 10003, "密码至少需要6位")
	case errors.Is(err, identityservice.ErrSessionReplaceNeeded):
		api.Conflict(c, 20010, "当前账号已在其他位置登录，请确认是否替换旧会话")
	case errors.Is(err, identityservice.ErrInvalidInput), errors.Is(err, identityservice.ErrBootstrapUnavailable):
		api.BadRequest(c, 10001, "输入信息格式错误，请检查用户名、角色、分组、游戏ID或状态")
	default:
		api.InternalError(c, "用户服务内部错误")
	}
}
