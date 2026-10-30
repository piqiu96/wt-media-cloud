package profilebinding

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	profileservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type updateProfileRequest struct {
	Remark  *string `json:"remark"`
	Enabled *bool   `json:"enabled"`
}

func SubmitProfileSync(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var input profileservice.SnapshotInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	scan, err := profileservice.SubmitScan(actor, input)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Created(c, scan)
}

func GetProfileSync(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	scan, err := profileservice.GetCurrentSync(actor)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, scan)
}

func ConfirmProfileSync(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		SyncID             string   `json:"sync_id"`
		AdoptBitProfileIDs []string `json:"adopt_bit_profile_ids"`
	}
	if len(c.Request.Body()) > 0 && !api.DecodeJSON(c, &req) {
		return
	}
	var scan profileservice.ProfileScan
	var err error
	if req.SyncID != "" {
		scan, err = profileservice.ConfirmScan(actor, req.SyncID, req.AdoptBitProfileIDs)
	} else {
		scan, err = profileservice.ConfirmCurrentSync(actor, req.AdoptBitProfileIDs)
	}
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, scan)
}

// BindBitAccount 只接受 Agent Token 严格档。主账号是 Local Agent 从 BitBrowser
// 实读的事实，用户 session 与请求体都不能建立绑定。
func BindBitAccount(ctx context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	var input profileservice.BitAccountBindInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	binding, err := profileservice.BindBitAccount(identityservice.UserID(auth.Claims.UserID), input)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, binding)
}

// GetBitAccount 让页面看到当前账号绑定的是哪个 BitBrowser 主账号。
// 只回掩码：完整主账号留在 Agent 与 Cloud 之间使用。
type bitAccountView struct {
	Bound               bool       `json:"bound"`
	BitMainUserIDMasked string     `json:"main_user_id_masked,omitempty"`
	BoundAt             *time.Time `json:"bound_at,omitempty"`
	LastVerifiedAt      *time.Time `json:"last_verified_at,omitempty"`
}

func GetBitAccount(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	binding, found, err := profileservice.GetBitAccount(actor.ID)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	view := bitAccountView{Bound: found}
	if found {
		view.BitMainUserIDMasked = maskBitMainUserID(binding.BitMainUserID)
		view.BoundAt = binding.BoundAt
		view.LastVerifiedAt = binding.LastVerifiedAt
	}
	api.Success(c, view)
}

func maskBitMainUserID(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

func RejectProfileSync(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if err := profileservice.RejectCurrentSync(actor); err != nil {
		writeProfileError(c, err)
		return
	}
	api.NoContent(c)
}

func ListBrowserProfiles(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var userID identityservice.UserID
	if raw := c.Query("user_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeProfileError(c, profileservice.ErrForbidden)
			return
		}
		userID = identityservice.UserID(parsed)
	}
	enabled, err := parseOptionalBool(c.Query("enabled"))
	if err != nil {
		writeProfileError(c, profileservice.ErrInvalidInput)
		return
	}
	filter := profileservice.ProfileListFilter{
		ID:           strings.TrimSpace(c.Query("id")),
		BitName:      strings.TrimSpace(c.Query("bit_name")),
		BitGroup:     strings.TrimSpace(c.Query("bit_group")),
		BitProfileID: strings.TrimSpace(c.Query("bit_profile_id")),
		Remark:       strings.TrimSpace(c.Query("remark")),
		LocalState:   strings.TrimSpace(c.Query("local_state")),
		Enabled:      enabled,
	}
	switch filter.LocalState {
	case "", string(profileservice.ProfileActive), string(profileservice.ProfileLocalMissing), string(profileservice.ProfileArchived):
	default:
		api.BadRequest(c, 10001, "窗口状态筛选无效")
		return
	}
	profiles, err := profileservice.ListProfiles(actor, userID, filter)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, profiles)
}
func UpdateBrowserProfile(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req updateProfileRequest
	if !api.DecodeJSON(c, &req) {
		return
	}
	profile, err := profileservice.UpdateProfile(actor, c.Param("id"), req.Remark, req.Enabled)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, profile)
}
func DeleteBrowserProfile(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	_ = actor
	if err := profileservice.DeleteProfile(actor, c.Param("id")); err != nil {
		writeProfileError(c, err)
		return
	}
	api.NoContent(c)
}

func writeDesktopOnlyProfileOperation(c *hertzapp.RequestContext) {
	api.Conflict(c, 23004, "浏览器窗口本机操作只能在Desktop执行；Cloud Web只展示已保存的窗口信息")
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

func writeProfileError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, profileservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "请求参数无效")
	case errors.Is(err, profileservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限执行此 Profile 操作")
	case errors.Is(err, profileservice.ErrIdentityUnverifiable):
		api.FailureCode(c, 409, 23002, "BitBrowser Profile 身份无法验证", "BIT_MAIN_USER_MISMATCH", nil, false, "rebind_bitbrowser")
	case errors.Is(err, profileservice.ErrBitNotBound):
		api.FailureCode(c, 409, 23008, "尚未绑定 BitBrowser 主账号", "BIT_NOT_BOUND", nil, false, "bind_bitbrowser")
	case errors.Is(err, profileservice.ErrBitBrowserOffline):
		api.FailureCode(c, 409, 23009, "BitBrowser 未运行或观察已过期", "BIT_BROWSER_OFFLINE", nil, true, "start_bitbrowser")
	case errors.Is(err, profileservice.ErrBitMainUserMismatch):
		api.FailureCode(c, 409, 23002, "当前比特浏览器登录账号与系统绑定账号不一致", "BIT_MAIN_USER_MISMATCH", nil, false, "rebind_bitbrowser")
	case errors.Is(err, profileservice.ErrScanNotFound), errors.Is(err, profileservice.ErrProfileNotFound):
		api.NotFound(c, 20004, "Profile 扫描或 Profile 不存在")
	case errors.Is(err, profileservice.ErrScanExpired):
		api.Failure(c, 410, 20004, "Profile 扫描已过期", nil)
	case errors.Is(err, profileservice.ErrProfileProxyPending):
		api.FailureCode(c, 409, 20009, "窗口有代理结果待核实，请先在管理代理中核实后再接受变化", "PROFILE_PROXY_PENDING", nil, false, "verify_proxy")
	case errors.Is(err, profileservice.ErrScanNotReady):
		api.Conflict(c, 20009, "Profile 扫描未就绪")
	case errors.Is(err, profileservice.ErrProfileReferenced):
		api.Conflict(c, 20003, "浏览器窗口已被媒体账号引用，不能直接分配给其他用户")
	case errors.Is(err, profileservice.ErrProfileNotMissing):
		api.Conflict(c, 20011, "仅最近一次同步中已缺失的窗口可删除；请先核实比特中的窗口")
	default:
		api.InternalError(c, "Profile 服务内部错误")
	}
}
