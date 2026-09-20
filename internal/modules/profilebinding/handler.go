package profilebinding

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	profileservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type updateProfileRequest struct {
	CloudRemark    *string                               `json:"cloud_remark"`
	BusinessStatus *profileservice.ProfileBusinessStatus `json:"business_status"`
}

type localSensitiveRequest struct {
	NodeID string `json:"node_id"`
}

type assignProfileOwnerRequest struct {
	UserID identityservice.UserID `json:"user_id"`
}

func SubmitProfileScan(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var input profileservice.SnapshotInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	nodeID := strings.TrimSpace(input.NodeID)
	if nodeID == "" {
		writeProfileError(c, runtimeservice.ErrInvalidInput)
		return
	}
	if !checkLocalTrust(c, actor.ID, nodeID) {
		return
	}
	scan, err := profileservice.SubmitScan(actor, input)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Created(c, scan)
}
func GetProfileScan(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	scan, err := profileservice.GetScan(actor, c.Param("scan_id"))
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, scan)
}
func ConfirmProfileScan(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	nodeID, ok := decodeLocalSensitiveNode(c)
	if !ok {
		writeProfileError(c, runtimeservice.ErrInvalidInput)
		return
	}
	if !checkLocalTrust(c, actor.ID, nodeID) {
		return
	}
	scan, err := profileservice.ConfirmScan(actor, c.Param("scan_id"))
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, scan)
}
func ConfirmScanMainIdentity(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	scan, err := profileservice.ConfirmMainIdentity(actor, c.Param("scan_id"))
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, scan)
}
func ConfirmMainIdentity(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var input profileservice.MainIdentityInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	binding, err := profileservice.ConfirmMainIdentityDirect(actor, input)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, binding)
}
func ClearMainIdentity(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	userID, valid := parseProfileUserID(c.Param("user_id"))
	if !valid {
		writeProfileError(c, profileservice.ErrInvalidInput)
		return
	}
	if err := profileservice.ClearMainIdentity(actor, userID); err != nil {
		writeProfileError(c, err)
		return
	}
	api.NoContent(c)
}
func RejectProfileScan(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if err := profileservice.RejectScan(actor, c.Param("scan_id")); err != nil {
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
	profiles, err := profileservice.ListProfiles(actor, userID)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, profiles)
}
func CreateBrowserProfile(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := middleware.AuthenticateRequest(c); !ok {
		return
	}
	writeDesktopOnlyProfileOperation(c)
}
func OpenBrowserProfile(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := middleware.AuthenticateRequest(c); !ok {
		return
	}
	writeDesktopOnlyProfileOperation(c)
}
func CloseBrowserProfile(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := middleware.AuthenticateRequest(c); !ok {
		return
	}
	writeDesktopOnlyProfileOperation(c)
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
	profile, err := profileservice.UpdateProfile(actor, c.Param("id"), req.CloudRemark, req.BusinessStatus)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	api.Success(c, profile)
}
func AssignBrowserProfileOwner(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req assignProfileOwnerRequest
	if !api.DecodeJSON(c, &req) || req.UserID <= 0 {
		writeProfileError(c, profileservice.ErrInvalidInput)
		return
	}
	target, found, err := identityservice.ResolveUser(req.UserID)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	if !found {
		writeProfileError(c, profileservice.ErrInvalidInput)
		return
	}
	profile, err := profileservice.AssignProfileOwner(actor, c.Param("id"), target)
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

func parseProfileUserID(raw string) (identityservice.UserID, bool) {
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return identityservice.UserID(parsed), true
}

func extractNodeID(input map[string]any) (string, bool) {
	raw, ok := input["node_id"]
	delete(input, "node_id")
	if !ok {
		return "", false
	}
	nodeID := strings.TrimSpace(strconvAny(raw))
	return nodeID, nodeID != ""
}

func decodeLocalSensitiveNode(c *hertzapp.RequestContext) (string, bool) {
	var req localSensitiveRequest
	if !api.DecodeJSON(c, &req) {
		return "", false
	}
	nodeID := strings.TrimSpace(req.NodeID)
	return nodeID, nodeID != ""
}

func writeDesktopOnlyProfileOperation(c *hertzapp.RequestContext) {
	api.Conflict(c, 23004, "浏览器窗口本机操作只能在Desktop执行；Cloud Web只展示已保存的窗口信息")
}

func checkLocalTrust(c *hertzapp.RequestContext, userID identityservice.UserID, nodeID string) bool {
	if err := runtimeservice.CheckLocalTrust(userID, nodeID); err != nil {
		writeProfileError(c, err)
		return false
	}
	return true
}

func strconvAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func writeProfileError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, profileservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "请求参数无效")
	case errors.Is(err, runtimeservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "当前电脑缺少本地环境确认信息")
	case errors.Is(err, runtimeservice.ErrBoundSessionInvalid), errors.Is(err, runtimeservice.ErrNodeCredentialInvalid):
		api.Unauthorized(c, 11001, "当前电脑登录状态已失效，请重新登录 Desktop 并刷新本机状态")
	case errors.Is(err, runtimeservice.ErrLocalTrustUnavailable), errors.Is(err, runtimeservice.ErrProfileOwnershipMismatch):
		api.Conflict(c, 23003, "当前电脑尚未完成本地环境确认，暂时不能扫描或操作浏览器窗口")
	case errors.Is(err, profileservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限执行此 Profile 操作")
	case errors.Is(err, profileservice.ErrIdentityUnverifiable):
		api.Conflict(c, 23002, "BitBrowser Profile 身份无法验证")
	case errors.Is(err, profileservice.ErrIdentityMismatch):
		api.Conflict(c, 23002, "当前比特浏览器登录账号与系统绑定账号不一致")
	case errors.Is(err, profileservice.ErrScanNotFound), errors.Is(err, profileservice.ErrProfileNotFound):
		api.NotFound(c, 20004, "Profile 扫描或 Profile 不存在")
	case errors.Is(err, profileservice.ErrScanExpired):
		api.Failure(c, 410, 20004, "Profile 扫描已过期", nil)
	case errors.Is(err, profileservice.ErrScanNotReady):
		api.Conflict(c, 20009, "Profile 扫描未就绪")
	case errors.Is(err, profileservice.ErrProfileReferenced):
		api.Conflict(c, 20003, "浏览器窗口已被媒体账号引用，不能直接分配给其他用户")
	case errors.Is(err, profileservice.ErrProfileNotDisabled):
		api.Conflict(c, 20011, "仅已停用的浏览器窗口可同步删除；请先停用该窗口")
	default:
		api.InternalError(c, "Profile 服务内部错误")
	}
}
