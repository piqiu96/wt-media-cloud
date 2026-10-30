package identity

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

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
	Username         string `json:"username"`
	Password         string `json:"password"`
	ReplaceExisting  bool   `json:"replace_existing"`
	ReplaceSessionID string `json:"replace_session_id"`
}

type createUserRequest struct {
	Username    string                       `json:"username"`
	Password    string                       `json:"password"`
	Role        identityservice.Role         `json:"role"`
	TeamID      *identityservice.TeamID      `json:"team_id"`
	CategoryIDs []identityservice.CategoryID `json:"category_ids"`
}

type updateUserRequest struct {
	Role        identityservice.Role         `json:"role"`
	TeamID      *identityservice.TeamID      `json:"team_id"`
	Status      identityservice.UserStatus   `json:"status"`
	CategoryIDs []identityservice.CategoryID `json:"category_ids"`
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type ownProfileRequest struct {
	Nickname string `json:"nickname"`
	AvatarID string `json:"avatar_id"`
}

type teamRequest struct {
	Name string `json:"name"`
}

type categoryRequest struct {
	Type    identityservice.CategoryType `json:"type"`
	Name    string                       `json:"name"`
	Enabled *bool                        `json:"enabled"`
	Remark  string                       `json:"remark"`
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
		hlog.CtxWarnf(ctx, "module=identity action=login result=bad_request ip=%s path=%s", c.ClientIP(), string(c.Path()))
		return
	}
	username := strings.TrimSpace(req.Username)
	// Session client_type is inferred from the Origin (Tauri WebView -> desktop,
	// anything else -> web), the same signal already used below to decide between
	// the token and cookie paths. It is not a client-declared field.
	clientType := identityservice.ClientTypeWeb
	if isLocalDesktopOrigin(string(c.GetHeader("Origin"))) {
		clientType = identityservice.ClientTypeDesktop
	}
	result, err := identityservice.LoginWithOptions(username, req.Password, identityservice.LoginOptions{
		ReplaceExisting:  req.ReplaceExisting,
		ReplaceSessionID: req.ReplaceSessionID,
		ClientType:       clientType,
	})
	if err != nil {
		hlog.CtxWarnf(
			ctx,
			"module=identity action=login result=failed username=%q client_type=%s replace_existing=%t origin=%q ip=%s reason=%s error=%v",
			username, clientType, req.ReplaceExisting, string(c.GetHeader("Origin")), c.ClientIP(), loginFailureReason(err), err,
		)
		writeIdentityError(c, err)
		return
	}
	hlog.CtxInfof(
		ctx,
		"module=identity action=login result=ok username=%q user_id=%d client_type=%s session_id=%s origin=%q ip=%s",
		username, result.User.ID, clientType, result.SessionID, string(c.GetHeader("Origin")), c.ClientIP(),
	)
	sameSite, secure := sessionCookieMode(string(c.GetHeader("Origin")), config.Get().App.Server.SessionCookieSecure)
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetCookie(middleware.SessionCookieName, result.Token, maxAge, "/", "", sameSite, secure, true)
	if isLocalDesktopOrigin(string(c.GetHeader("Origin"))) {
		// Packaged Desktop cannot round-trip the cross-site cookie, so it
		// receives the session token in the body and sends it back as the
		// X-Session-Token header. Cloud Web keeps the HttpOnly cookie path.
		api.Success(c, map[string]any{
			"user":       result.User,
			"token":      result.Token,
			"expires_at": result.ExpiresAt,
		})
		return
	}
	api.Success(c, result.User)
}
func Me(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	api.Success(c, actor)
}
func UpdateOwnProfile(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req ownProfileRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	user, err := identityservice.UpdateOwnProfile(actor.ID, req.Nickname, req.AvatarID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, user)
}
func Logout(ctx context.Context, c *hertzapp.RequestContext) {
	token := sessionToken(c)
	if err := identityservice.Logout(token); err != nil {
		writeIdentityError(c, err)
		return
	}
	sameSite, secure := sessionCookieMode(string(c.GetHeader("Origin")), config.Get().App.Server.SessionCookieSecure)
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", sameSite, secure, true)
	api.NoContent(c)
}
func ForceLogout(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	userID, valid := parseUserID(c.Param("user_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	if err := identityservice.ForceLogout(actor.ID, userID); err != nil {
		writeIdentityError(c, err)
		return
	}
	api.NoContent(c)
}

func CreateUser(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
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
	actor, ok := middleware.AuthenticateRequest(c)
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
	user, err := identityservice.UpdateUser(actor.ID, userID, req.Role, req.TeamID, req.CategoryIDs, req.Status)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, user)
}
func ResetPassword(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
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
	actor, ok := middleware.AuthenticateRequest(c)
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
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", sameSite, secure, true)
	api.NoContent(c)
}
func ListUsers(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if actor.Role != identityservice.RoleAdmin && actor.Role != identityservice.RoleSeniorOperator {
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
	if actor.Role == identityservice.RoleSeniorOperator {
		// 高级运营查看成员时要用同团队名单（默认只有自己）。响应限定在本人团队，
		// 且不含停用成员和管理员，不能拿来枚举其他团队。
		if actor.TeamID == nil {
			writeIdentityError(c, identityservice.ErrForbidden)
			return
		}
		team := filtered[:0]
		for _, user := range filtered {
			if user.TeamID != nil && *user.TeamID == *actor.TeamID &&
				user.Status == identityservice.UserStatusEnabled && user.Role != identityservice.RoleAdmin {
				team = append(team, user)
			}
		}
		api.Success(c, team)
		return
	}
	api.Success(c, filtered)
}
func DeleteUser(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
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
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	teams, err := identityservice.ListTeams(actor.ID, strings.TrimSpace(c.Query("keyword")))
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, teams)
}
func CreateTeam(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
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
	actor, ok := middleware.AuthenticateRequest(c)
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
	actor, ok := middleware.AuthenticateRequest(c)
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
func ListCategories(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var categories []identityservice.Category
	var err error
	switch strings.TrimSpace(c.Query("scope")) {
	case "all":
		categories, err = identityservice.ListAllCategories(actor.ID)
	case "management":
		categories, err = identityservice.ListManagementCategories(actor.ID)
	default:
		categories, err = identityservice.ListCategories(actor.ID)
	}
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, filterCategories(categories, c))
}
func CategoryReferences(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	categoryID, valid := parseCategoryID(c.Param("category_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	references, err := identityservice.CategoryReferences(actor.ID, categoryID)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, references)
}
func CreateCategory(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req categoryRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	category, err := identityservice.CreateCategory(actor.ID, req.Type, req.Name, req.Remark)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Created(c, category)
}
func UpdateCategory(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req categoryRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	categoryID, valid := parseCategoryID(c.Param("category_id"))
	if !valid || req.Enabled == nil {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	category, err := identityservice.UpdateCategory(actor.ID, categoryID, req.Name, *req.Enabled, req.Remark)
	if err != nil {
		writeIdentityError(c, err)
		return
	}
	api.Success(c, category)
}
func DeleteCategory(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	categoryID, valid := parseCategoryID(c.Param("category_id"))
	if !valid {
		writeIdentityError(c, identityservice.ErrInvalidInput)
		return
	}
	if err := identityservice.DeleteCategory(actor.ID, categoryID); err != nil {
		writeIdentityError(c, err)
		return
	}
	api.NoContent(c)
}
func ListAuditLogs(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
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

func filterCategories(categories []identityservice.Category, c *hertzapp.RequestContext) []identityservice.Category {
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	categoryType := identityservice.CategoryType(strings.TrimSpace(c.Query("type")))
	enabledRaw := strings.TrimSpace(c.Query("enabled"))
	var enabled *bool
	if enabledRaw != "" {
		parsed, err := strconv.ParseBool(enabledRaw)
		if err == nil {
			enabled = &parsed
		}
	}
	result := make([]identityservice.Category, 0, len(categories))
	for _, category := range categories {
		if keyword != "" && !strings.Contains(strings.ToLower(strconv.FormatInt(int64(category.ID), 10)+" "+category.Name), keyword) {
			continue
		}
		if categoryType != "" && category.Type != categoryType {
			continue
		}
		if enabled != nil && category.Enabled != *enabled {
			continue
		}
		result = append(result, category)
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

func parseCategoryID(value string) (identityservice.CategoryID, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return identityservice.CategoryID(parsed), err == nil && parsed > 0
}

func validRole(role identityservice.Role) bool {
	return role == identityservice.RoleOperator || role == identityservice.RoleSeniorOperator || role == identityservice.RoleAdmin
}

func containsCategory(categoryIDs []identityservice.CategoryID, categoryID identityservice.CategoryID) bool {
	for _, candidate := range categoryIDs {
		if candidate == categoryID {
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
	categoryID, validCategoryID := parseCategoryID(c.Query("category_id"))
	if strings.TrimSpace(c.Query("category_id")) != "" && !validCategoryID {
		return nil, identityservice.ErrInvalidInput
	}
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
		if categoryID > 0 && !containsCategory(user.CategoryIDs, categoryID) {
			continue
		}
		result = append(result, user)
	}
	return result, nil
}

// sessionToken returns the current session token from the cookie first, then
// falls back to the X-Session-Token header.
func sessionToken(c *hertzapp.RequestContext) string {
	if token := string(c.Cookie(middleware.SessionCookieName)); token != "" {
		return token
	}
	return string(c.GetHeader("X-Session-Token"))
}

// loginFailureReason maps a login error to a stable, greppable token for logs.
// Credential failures stay combined in the response ("账号或密码不正确") so the
// API never reveals whether an account exists; only pre-store input checks are
// reported as distinct client errors.
func loginFailureReason(err error) string {
	switch {
	case errors.Is(err, identityservice.ErrMissingCredentials):
		return "missing_credentials"
	case errors.Is(err, identityservice.ErrLoginInputInvalid):
		return "invalid_login_input"
	case errors.Is(err, identityservice.ErrAuthenticationFailed):
		return "invalid_credentials"
	case errors.Is(err, identityservice.ErrDesktopLoginConflict):
		return "desktop_login_conflict"
	case errors.Is(err, identityservice.ErrForbidden):
		return "forbidden"
	default:
		return "internal_error"
	}
}

func writeIdentityError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, identityservice.ErrMissingCredentials):
		api.BadRequest(c, 10005, "请输入账号和密码")
	case errors.Is(err, identityservice.ErrLoginInputInvalid):
		api.BadRequest(c, 10006, "账号最长 64 位，密码需为 6-72 位")
	case errors.Is(err, identityservice.ErrAuthenticationFailed):
		api.Unauthorized(c, 11004, "账号或密码不正确")
	case errors.Is(err, identityservice.ErrDesktopLoginConflict):
		activeSessionID := ""
		var conflict *identityservice.DesktopLoginConflictError
		if errors.As(err, &conflict) {
			activeSessionID = conflict.ActiveSessionID
		}
		api.FailureCode(c, 409, 20010, "该账号已在另一台电脑登录 Desktop，是否继续？", "DESKTOP_LOGIN_CONFLICT",
			map[string]any{"active_session_id": activeSessionID}, false, "confirm_desktop_takeover")
	case errors.Is(err, identityservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限执行此操作")
	case errors.Is(err, identityservice.ErrUsernameTaken):
		api.Conflict(c, 20001, "用户名已被使用")
	case errors.Is(err, identityservice.ErrTeamNameTaken):
		api.Conflict(c, 20002, "团队名称已被使用")
	case errors.Is(err, identityservice.ErrTeamInUse):
		api.Conflict(c, 20003, "该团队下还有用户或业务引用，不能删除，请先转移用户")
	case errors.Is(err, identityservice.ErrCategoryNameTaken):
		api.Conflict(c, 20004, "类目名称已被使用")
	case errors.Is(err, identityservice.ErrInvalidCategoryType):
		api.BadRequest(c, 10004, "类目一级类型无效")
	case errors.Is(err, identityservice.ErrCategoryInUse):
		api.Conflict(c, 20005, "该类目已分配给用户或已有业务引用，不能停用或删除，请先调整用户类目范围")
	case errors.Is(err, identityservice.ErrCategoryUnavailable):
		api.BadRequest(c, 10002, "请选择已启用的类目")
	case errors.Is(err, identityservice.ErrPasswordTooShort):
		api.BadRequest(c, 10003, "密码至少需要6位")
	case errors.Is(err, identityservice.ErrInvalidInput), errors.Is(err, identityservice.ErrBootstrapUnavailable):
		api.BadRequest(c, 10001, "输入信息格式错误，请检查用户名、角色、团队、类目ID或状态")
	default:
		if failure, ok := identityservice.DescribeAuthFailure(err); ok {
			api.FailureCode(c, 401, 11001, failure.Message, failure.Code, nil, false, failure.RecoveryAction)
			return
		}
		hlog.Errorf(
			"module=identity result=error path=%s method=%s ip=%s reason=internal error=%v",
			string(c.Path()), string(c.Method()), c.ClientIP(), err,
		)
		api.InternalError(c, "用户服务内部错误")
	}
}
