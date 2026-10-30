package mediaaccount

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accountservice "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type createAccountRequest struct {
	CategoryIDs []identityservice.CategoryID `json:"category_ids"`
	Platform    accountservice.Platform      `json:"platform"`
	Remark      string                       `json:"remark"`
	Tags        []string                     `json:"tags"`
}

type updateAccountRequest struct {
	Enabled     *bool                         `json:"enabled"`
	Tags        *[]string                     `json:"tags"`
	Remark      *string                       `json:"remark"`
	CategoryIDs *[]identityservice.CategoryID `json:"category_ids"`
}

type bindProfileRequest struct {
	BrowserProfileID string `json:"browser_profile_id"`
}

type accountCheckResultRequest struct {
	ExecutionStatus     string                             `json:"execution_status"`
	OperationID         string                             `json:"operation_id"`
	PlatformAccountID   string                             `json:"platform_account_id"`
	PlatformName        string                             `json:"platform_name"`
	PlatformAvatarURL   string                             `json:"platform_avatar_url"`
	PlatformLoginStatus accountservice.PlatformLoginStatus `json:"platform_login_status"`
	Message             string                             `json:"message"`
	LastCheckItems      []accountservice.AccountCheckItem  `json:"check_items"`
}

func CreateMediaAccount(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if !accountservice.CanStartLocalOperation(actor.Role) {
		api.Forbidden(c, 11003, "管理员只管理已有账号")
		return
	}
	var req createAccountRequest
	if !decodeAccountJSON(c, &req) {
		return
	}
	categoryIDs := accountservice.NormalizeCategoryIDs(req.CategoryIDs)
	account, err := accountservice.CreateAccount(actor, accountservice.CreateAccountInput{
		CategoryIDs: categoryIDs, Platform: req.Platform, Remark: req.Remark, Tags: req.Tags,
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
	page, size := 1, 20
	if raw := c.Query("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			api.BadRequest(c, 10001, "分页参数无效")
			return
		}
		page = v
	}
	if raw := c.Query("page_size"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			api.BadRequest(c, 10001, "分页参数无效")
			return
		}
		size = v
	}
	var teamID identityservice.TeamID
	if raw := c.Query("team_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			api.BadRequest(c, 10001, "团队参数无效")
			return
		}
		teamID = identityservice.TeamID(v)
	}
	filter := accountservice.AccountFilter{TeamID: teamID, CategoryType: c.Query("category_type"), View: c.Query("view"),
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
	}
	if tags := commaValues(c.Query("tags")); len(tags) > 0 {
		switch c.Query("tag_match") {
		case "", "any":
			filter.AnyTags = tags
		case "all":
			filter.AllTags = tags
		case "exclude":
			filter.ExcludeTags = tags
		default:
			api.BadRequest(c, 10001, "标签匹配方式无效")
			return
		}
	}
	accounts, err := accountservice.ListAccountPage(actor, filter, page, size)
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, accounts)
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
	if !decodeAccountJSON(c, &req) {
		return
	}
	categoryIDs := req.CategoryIDs
	account, err := accountservice.UpdateAccount(actor, c.Param("account_id"), accountservice.UpdateAccountInput{
		Enabled: req.Enabled, Tags: req.Tags, Remark: req.Remark, CategoryIDs: categoryIDs,
	})
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
func GetMediaAccountCredentials(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if !accountservice.CanStartLocalOperation(actor.Role) {
		api.Forbidden(c, 11003, "仅本人运营账号可查看凭证")
		return
	}
	record, err := accountservice.GetOwnedAccountRecord(actor, c.Param("account_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, map[string]any{"original_cookie": record.OriginalCookie, "active_cookie": record.ActiveCookie, "cookie_updated_at": record.CookieUpdatedAt, "current_update": accountservice.CurrentCookieUpdate(record)})
}
func StartMediaAccountCookieUpdate(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var input accountservice.CookieStartInput
	if !decodeAccountJSON(c, &input) {
		return
	}
	out, err := accountservice.StartCookieUpdate(actor, c.Param("account_id"), input)
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, out)
}
func SnapshotMediaAccountCookieUpdate(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var input accountservice.CookieSnapshotInput
	if !decodeAccountJSON(c, &input) {
		return
	}
	out, err := accountservice.SnapshotCookieUpdate(actor, c.Param("account_id"), c.Param("operation_id"), input)
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, out)
}
func SettleMediaAccountCookieUpdate(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var input accountservice.CookieResultInput
	if !decodeAccountJSON(c, &input) {
		return
	}
	out, err := accountservice.SettleCookieUpdate(actor, c.Param("account_id"), c.Param("operation_id"), input)
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, out)
}
func GetMediaAccountCookieUpdate(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	out, err := accountservice.GetCookieUpdate(actor, c.Param("account_id"), c.Param("operation_id"))
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	api.Success(c, out)
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
	case errors.Is(err, accountservice.ErrImportConflict):
		api.FailureCode(c, 409, 23004, "接入进度已变化，请刷新后继续", "import_conflict", nil, false, "refresh")
	case errors.Is(err, accountservice.ErrCookieConflict):
		api.FailureCode(c, 409, 23003, "窗口正在处理 Cookie 或操作已变化，请核实后继续", "cookie_operation_conflict", nil, false, "reconcile")
	case errors.Is(err, accountservice.ErrStaleCheck):
		api.FailureCode(c, 409, 23002, "检查目标已变化或结果已过期，请刷新后重新检查", "stale_account_check", nil, false, "refresh")
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

func decodeAccountJSON(c *hertzapp.RequestContext, dst any) bool {
	decoder := json.NewDecoder(bytes.NewReader(c.Request.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		api.BadRequest(c, 10001, "账号资料包含不支持的字段或格式错误")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		api.BadRequest(c, 10001, "账号资料格式错误")
		return false
	}
	return true
}
