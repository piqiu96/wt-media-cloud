package mediaaccount

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accountservice "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type createAccountRequest struct {
	UserID           identityservice.UserID       `json:"user_id"`
	CategoryIDs      []identityservice.CategoryID `json:"category_ids"`
	PlatformName     string                       `json:"platform_name"`
	Platform         accountservice.Platform      `json:"platform"`
	OriginalCookie   string                       `json:"original_cookie"`
	BrowserProfileID string                       `json:"browser_profile_id"`
	Remark           string                       `json:"remark"`
	Tags             []string                     `json:"tags"`
}

type updateAccountRequest struct {
	Enabled             *bool                              `json:"enabled"`
	PlatformLoginStatus accountservice.PlatformLoginStatus `json:"platform_login_status"`
	Remark              *string                            `json:"remark"`
	CategoryIDs         *[]identityservice.CategoryID      `json:"category_ids"`
	PlatformName        *string                            `json:"platform_name"`
}

type identifyAccountRequest struct {
	PlatformAccountID   string                             `json:"platform_account_id"`
	PlatformName        string                             `json:"platform_name"`
	PlatformAvatarURL   string                             `json:"platform_avatar_url"`
	PlatformLoginStatus accountservice.PlatformLoginStatus `json:"platform_login_status"`
}

type tagsRequest struct {
	AccountIDs []string `json:"account_ids"`
	Tags       []string `json:"tags"`
}

type bindProfileRequest struct {
	BrowserProfileID string `json:"browser_profile_id"`
}

type accountCheckResultRequest struct {
	OperationID         string                             `json:"operation_id"`
	PlatformAccountID   string                             `json:"platform_account_id"`
	PlatformName        string                             `json:"platform_name"`
	PlatformAvatarURL   string                             `json:"platform_avatar_url"`
	PlatformLoginStatus accountservice.PlatformLoginStatus `json:"platform_login_status"`
	Message             string                             `json:"message"`
	LastCheckItems      []accountservice.AccountCheckItem  `json:"check_items"`
}

type cookieReadResultRequest struct {
	OperationID string           `json:"operation_id"`
	Cookies     []map[string]any `json:"cookies"`
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
	categoryIDs := accountservice.NormalizeCategoryIDs(req.CategoryIDs)
	account, err := accountservice.CreateAccount(actor, accountservice.CreateAccountInput{
		UserID: req.UserID, CategoryIDs: categoryIDs, PlatformName: req.PlatformName, Platform: req.Platform,
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
	categoryIDs := accountservice.NormalizeCategoryIDs(parseCategoryIDs(commaValues(c.Query("category_ids"))))
	enabled, err := parseOptionalBool(c.Query("enabled"))
	if err != nil {
		writeMediaAccountError(c, accountservice.ErrInvalidInput)
		return
	}
	accounts, err := accountservice.ListAccounts(actor, accountservice.AccountFilter{
		UserID:              userID,
		CategoryIDs:         categoryIDs,
		Platform:            accountservice.Platform(c.Query("platform")),
		Enabled:             enabled,
		PlatformLoginStatus: accountservice.PlatformLoginStatus(c.Query("platform_login_status")),
		Search:              c.Query("search"),
		ProfileSearch:       c.Query("profile_search"),
		AnyTags:             commaValues(c.Query("any_tags")),
		AllTags:             commaValues(c.Query("all_tags")),
		ExcludeTags:         commaValues(c.Query("exclude_tags")),
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
	categoryIDs := req.CategoryIDs
	account, err := accountservice.UpdateAccount(actor, c.Param("account_id"), accountservice.UpdateAccountInput{
		Enabled: req.Enabled, PlatformLoginStatus: req.PlatformLoginStatus, Remark: req.Remark, CategoryIDs: categoryIDs, PlatformName: req.PlatformName,
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
	start, err := accountservice.StartLocalAccountCheck(actor, c.Param("account_id"), accountservice.AccountCheckStartInput{})
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
func ReadMediaAccountCookiesSync(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	start, err := accountservice.StartCookieRead(actor, c.Param("account_id"), accountservice.CookieReadStartInput{})
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
	account, err := accountservice.ApplyCookieReadResult(actor, c.Param("account_id"), accountservice.CookieReadResultInput{OperationID: req.OperationID, Cookies: req.Cookies})
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

func parseOptionalBool(value string) (*bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func commaValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func parseCategoryIDs(values []string) []identityservice.CategoryID {
	result := make([]identityservice.CategoryID, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil && parsed > 0 {
			result = append(result, identityservice.CategoryID(parsed))
		}
	}
	return result
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
	case errors.Is(err, accountservice.ErrProfileNotFound):
		api.FailureCode(c, 404, 20004, "未找到对应的浏览器环境", "PROFILE_NOT_FOUND", nil, false, "choose_profile")
	case errors.Is(err, accountservice.ErrProfileAssignedToOtherUser):
		api.FailureCode(c, 403, 11003, "浏览器环境属于其他用户", "PROFILE_ASSIGNED_TO_OTHER_USER", nil, false, "choose_own_profile")
	case errors.Is(err, accountservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "媒体账号信息格式错误")
	default:
		api.InternalError(c, "媒体账号服务内部错误")
	}
}
