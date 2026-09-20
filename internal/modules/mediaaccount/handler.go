package mediaaccount

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	cloudagentservice "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accountservice "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type createAccountRequest struct {
	UserID           identityservice.UserID  `json:"user_id"`
	GameIDs          []string                `json:"game_ids"`
	Name             string                  `json:"name"`
	Platform         accountservice.Platform `json:"platform"`
	OriginalCookie   string                  `json:"original_cookie"`
	BrowserProfileID string                  `json:"browser_profile_id"`
	Remark           string                  `json:"remark"`
	Tags             []string                `json:"tags"`
}

type updateAccountRequest struct {
	BusinessStatus accountservice.BusinessStatus `json:"business_status"`
	LoginStatus    accountservice.LoginStatus    `json:"login_status"`
	Remark         *string                       `json:"remark"`
	GameIDs        *[]string                     `json:"game_ids"`
	Name           *string                       `json:"name"`
}

type identifyAccountRequest struct {
	PlatformAccountID string                     `json:"platform_account_id"`
	Name              string                     `json:"name"`
	AvatarURL         string                     `json:"avatar_url"`
	LoginStatus       accountservice.LoginStatus `json:"login_status"`
}

type tagsRequest struct {
	AccountIDs []string `json:"account_ids"`
	Tags       []string `json:"tags"`
}

type bindProfileRequest struct {
	BrowserProfileID string `json:"browser_profile_id"`
}

type startAccountCheckRequest struct {
	NodeID string `json:"node_id"`
}

type accountCheckResultRequest struct {
	TaskID            string                            `json:"task_id"`
	PlatformAccountID string                            `json:"platform_account_id"`
	Name              string                            `json:"name"`
	AvatarURL         string                            `json:"avatar_url"`
	LoginStatus       accountservice.LoginStatus        `json:"login_status"`
	Message           string                            `json:"message"`
	CheckItems        []accountservice.AccountCheckItem `json:"check_items"`
}

type startCookieReadRequest struct {
	NodeID string `json:"node_id"`
}

type cookieReadResultRequest struct {
	TaskID  string           `json:"task_id"`
	Cookies []map[string]any `json:"cookies"`
}

type createAccountGroupRequest struct {
	Name    string                             `json:"name"`
	Filters accountservice.AccountGroupFilters `json:"filters"`
}

type updateAccountGroupRequest struct {
	Name    *string                             `json:"name"`
	Filters *accountservice.AccountGroupFilters `json:"filters"`
}

func CreateMediaAccount(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req createAccountRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	gameIDs := accountservice.NormalizeGameIDs(req.GameIDs)
	account, err := accountservice.CreateAccount(actor, accountservice.CreateAccountInput{
		UserID: req.UserID, GameIDs: gameIDs, Name: req.Name, Platform: req.Platform,
		OriginalCookie: req.OriginalCookie, BrowserProfileID: req.BrowserProfileID, Remark: req.Remark, Tags: req.Tags,
	})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Created(c, account)
}
func ListMediaAccounts(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var userID identityservice.UserID
	if raw := c.Query("user_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeMediaAccountError(c, accountservice.ErrInvalidInput)
			return
		}
		userID = identityservice.UserID(parsed)
	}
	gameIDs := accountservice.NormalizeGameIDs(commaValues(c.Query("game_ids")))
	accounts, err := accountservice.ListAccounts(actor, accountservice.AccountFilter{
		UserID:         userID,
		GameIDs:        gameIDs,
		Platform:       accountservice.Platform(c.Query("platform")),
		BusinessStatus: accountservice.BusinessStatus(c.Query("business_status")),
		LoginStatus:    accountservice.LoginStatus(c.Query("login_status")),
		Search:         c.Query("search"),
		ProfileSearch:  c.Query("profile_search"),
		AnyTags:        commaValues(c.Query("any_tags")),
		AllTags:        commaValues(c.Query("all_tags")),
		ExcludeTags:    commaValues(c.Query("exclude_tags")),
	})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, accounts)
}
func AddMediaAccountTags(ctx context.Context, c *hertzapp.RequestContext) {
	changeTags(c, true)
}
func RemoveMediaAccountTags(ctx context.Context, c *hertzapp.RequestContext) {
	changeTags(c, false)
}
func GetMediaAccount(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	account, err := accountservice.GetAccount(actor, c.Param("account_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func UpdateMediaAccount(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req updateAccountRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	gameIDs := req.GameIDs
	account, err := accountservice.UpdateAccount(actor, c.Param("account_id"), accountservice.UpdateAccountInput{
		BusinessStatus: req.BusinessStatus, LoginStatus: req.LoginStatus, Remark: req.Remark, GameIDs: gameIDs, Name: req.Name,
	})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func IdentifyMediaAccount(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req identifyAccountRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	account, err := accountservice.IdentifyAccount(actor, c.Param("account_id"), accountservice.IdentifyAccountInput(req))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func StartMediaAccountCheck(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req startAccountCheckRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	start, err := accountservice.StartLocalAccountCheck(actor, c.Param("account_id"), accountservice.AccountCheckStartInput(req))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Created(c, start)
}
func ApplyMediaAccountCheckResult(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req accountCheckResultRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	account, err := accountservice.ApplyLocalAccountCheckResult(actor, c.Param("account_id"), accountservice.AccountCheckResultInput(req))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func ReadMediaAccountCookies(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	record, err := accountservice.GetOwnedAccountRecord(actor, c.Param("account_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	if record.BrowserProfileID == "" {
		writeMediaAccountError(c, accountservice.ErrProfileUnavailable)
		return
	}
	task := cloudagentservice.CreateTask(cloudagentservice.CreateTaskRequest{TaskType: cloudagentservice.TaskTypeCookieRead.String(), IdempotencyKey: "cookie-read:" + strconv.FormatInt(int64(actor.ID), 10) + ":" + record.ID + ":" + id.NewID("attempt"), Payload: map[string]any{"account_id": record.ID, "profile_id": record.BrowserProfileID, "platform": record.Platform}})
	api.Created(c, task)
}
func ReadMediaAccountCookiesSync(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req startCookieReadRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	start, err := accountservice.StartCookieRead(actor, c.Param("account_id"), accountservice.CookieReadStartInput{NodeID: req.NodeID})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Created(c, start)
}
func ApplyMediaAccountCookieReadResult(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req cookieReadResultRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	account, err := accountservice.ApplyCookieReadResult(actor, c.Param("account_id"), accountservice.CookieReadResultInput{TaskID: req.TaskID, Cookies: req.Cookies})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func GetMediaAccountCookies(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	account, err := accountservice.GetAccountRecord(actor, c.Param("account_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, map[string]interface{}{
		"original_cookie": account.OriginalCookie,
		"active_cookie":   account.ActiveCookie,
	})
}
func BindMediaAccountProfile(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req bindProfileRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	account, err := accountservice.BindProfile(actor, c.Param("account_id"), req.BrowserProfileID)
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func UnbindMediaAccountProfile(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	account, err := accountservice.UnbindProfile(actor, c.Param("account_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, account)
}
func CreateAccountGroup(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req createAccountGroupRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	group, err := accountservice.CreateAccountGroup(actor, accountservice.CreateAccountGroupInput{Name: req.Name, Filters: req.Filters})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Created(c, group)
}
func ListAccountGroups(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	groups, err := accountservice.ListAccountGroups(actor)
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, groups)
}
func ListAccountsByGroup(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	accounts, err := accountservice.ListAccountsByGroup(actor, c.Param("group_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, accounts)
}
func UpdateAccountGroup(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req updateAccountGroupRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	group, err := accountservice.UpdateAccountGroup(actor, c.Param("group_id"), accountservice.UpdateAccountGroupInput{Name: req.Name, Filters: req.Filters})
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, group)
}
func DeleteAccountGroup(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if err := accountservice.DeleteAccountGroup(actor, c.Param("group_id")); err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, nil)
}

func changeTags(c *hertzapp.RequestContext, add bool) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req tagsRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	var err error
	if add {
		err = accountservice.AddTags(actor, req.AccountIDs, req.Tags)
	} else {
		err = accountservice.RemoveTags(actor, req.AccountIDs, req.Tags)
	}
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.NoContent(c)
}

func commaValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func writeMediaAccountError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, accountservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限访问此媒体账号")
	case errors.Is(err, accountservice.ErrNotFound):
		api.NotFound(c, 20004, "媒体账号不存在")
	case errors.Is(err, accountservice.ErrDuplicateAccount):
		api.Conflict(c, 20009, "此平台账号已存在")
	case errors.Is(err, accountservice.ErrProfilePlatformTaken):
		api.Conflict(c, 23001, "此浏览器环境已绑定其他平台账号")
	case errors.Is(err, accountservice.ErrProfileUnavailable):
		api.Conflict(c, 23001, "浏览器环境不可用")
	case errors.Is(err, accountservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "媒体账号信息格式错误")
	default:
		api.InternalError(c, "媒体账号服务内部错误")
	}
}
