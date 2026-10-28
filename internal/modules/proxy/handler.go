package proxy

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	agentclient "github.com/wt-media/wt-media-cloud/internal/infra/client/agent"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	profilebindingservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	proxyservice "github.com/wt-media/wt-media-cloud/internal/modules/proxy/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type LocalProxyChange struct {
	Kind            string `json:"kind"`
	ProfileID       string `json:"profile_id"`
	BitProfileID    string `json:"bit_profile_id"`
	CurrentProxyID  string `json:"current_proxy_id,omitempty"`
	TargetProxyID   string `json:"target_proxy_id,omitempty"`
	ProxyProtocol   string `json:"proxy_protocol,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	OwnershipNotice string `json:"ownership_notice,omitempty"`
}

type LocalProxyScanPreview struct {
	ScanID  string             `json:"scan_id"`
	Changes []LocalProxyChange `json:"changes"`
}

type ProxyBindingSummary struct {
	ID           string `json:"id"`
	BitProfileID string `json:"bit_profile_id"`
	Name         string `json:"name"`
	UserID       int64  `json:"user_id"`
	LocalStatus  string `json:"local_status"`
}

type ProxyBatchAssignFailure struct {
	ProfileID string `json:"profile_id"`
	Message   string `json:"message"`
}

type ProxyBatchAssignResult struct {
	Succeeded []profilebindingservice.BrowserProfile `json:"succeeded"`
	Failed    []ProxyBatchAssignFailure              `json:"failed"`
}

// proxyView 在存储行之外补页面要展示的事实。凭据仍只随详情处理器返回。
type proxyView struct {
	proxyservice.ProxyConfig
	TeamName          string `json:"team_name,omitempty"`
	Expired           bool   `json:"expired"`
	AssignmentBlocker string `json:"assignment_blocker,omitempty"`
}

var (
	errProfileProxyUnauthorized = errors.New("profile proxy operation is forbidden")
	errProxyQuotaFull           = errors.New("proxy profile quota is full")
	errProxyReadback            = errors.New("proxy agent readback failed")
	errProxyRejected            = errors.New("bitbrowser rejected proxy assignment")
	errProxyAgentUpgrade        = errors.New("local agent does not support proxy assignment")
	errProxyForbidden           = errors.New("proxy operation is forbidden")
	errProxyTeamRequired        = errors.New("proxy team is required")
	errProxyTeamUnknown         = errors.New("proxy team does not exist")
)

func ListProxies(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	requestedTeam, err := requestedTeamID(actor, c.Query("team_id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	scope, err := proxyTeamScope(actor, requestedTeam)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	proxies, err := proxyservice.List(proxyservice.ProxyFilter{
		TeamID:         scope,
		BusinessStatus: c.Query("business_status"),
		Supplier:       c.Query("supplier"),
		Region:         c.Query("region"),
		Search:         c.Query("search"),
		Limit:          200,
	})
	if err != nil {
		writeProxyError(c, err)
		return
	}
	teamNames := newTeamNameResolver()
	views := make([]proxyView, 0, len(proxies))
	for index := range proxies {
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxies[index].ID)
		if countErr != nil {
			api.InternalError(c, "代理关联窗口查询失败")
			return
		}
		views = append(views, buildProxyView(actor, proxies[index], assigned, teamNames, false))
	}
	api.Success(c, views)
}

func ProxyRecommendations(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	targetIDs := normalizedProfileIDs(strings.Split(c.Query("profile_ids"), ","))
	if len(targetIDs) == 0 {
		api.BadRequest(c, 10001, "请至少选择一个浏览器窗口")
		return
	}
	targets := make([]profilebindingservice.BrowserProfile, 0, len(targetIDs))
	for _, profileID := range targetIDs {
		profile, err := ownedActiveProfile(actor, profileID)
		if err != nil {
			api.Forbidden(c, 11003, "没有权限查看此 Profile 的代理推荐")
			return
		}
		targets = append(targets, profile)
	}
	scope, err := proxyTeamScope(actor, nil)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	candidates, err := proxyservice.List(proxyservice.ProxyFilter{TeamID: scope, Limit: 200})
	if err != nil {
		writeProxyError(c, err)
		return
	}
	teamNames := newTeamNameResolver()
	recommended := make([]proxyView, 0, len(candidates))
	for _, candidate := range candidates {
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(candidate.ID)
		if countErr != nil {
			api.InternalError(c, "代理关联窗口查询失败")
			return
		}
		required := 0
		for _, target := range targets {
			if target.ProxyID != candidate.ID {
				required++
			}
		}
		remaining := proxyservice.NormalizedMaxProfileCount(candidate.MaxProfileCount) - assigned - required
		if proxyservice.AssignmentBlockReason(candidate, remaining) != "" {
			continue
		}
		recommended = append(recommended, buildProxyView(actor, candidate, assigned, teamNames, false))
	}
	api.Success(c, recommended)
}

func GetProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	assigned, err := profilebindingservice.CountProfilesByProxyID(proxy.ID)
	if err != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	api.Success(c, buildProxyView(actor, proxy, assigned, newTeamNameResolver(), true))
}

func GetProxyBindings(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	profiles, err := profilebindingservice.ListProfiles(actor, 0, profilebindingservice.ProfileListFilter{})
	if err != nil {
		api.InternalError(c, "浏览器窗口查询失败")
		return
	}
	result := make([]ProxyBindingSummary, 0)
	for _, profile := range profiles {
		if profile.ProxyID == proxy.ID {
			result = append(result, ProxyBindingSummary{ID: profile.ID, BitProfileID: profile.BitProfileID, Name: profile.Name, UserID: int64(profile.UserID), LocalStatus: string(profile.LocalStatus)})
		}
	}
	api.Success(c, result)
}

func ParseProxy(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ProxyAddress string `json:"proxy_address"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	parsed, err := proxyservice.ParseAddress(req.ProxyAddress)
	if err != nil {
		api.BadRequest(c, 10001, "代理地址格式无效")
		return
	}
	api.Success(c, parsed)
}

func PreviewProxyExtraction(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req proxyservice.ProxyExtractionInput
	if !api.DecodeJSON(c, &req) {
		return
	}
	result, err := proxyservice.ExtractProxy(ctx, req)
	if err != nil {
		api.Failure(c, 503, 30007, "动态代理提取失败；提取需要本人桌面环境运行", nil)
		return
	}
	api.Success(c, result)
}

func RefreshProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canGovernProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	if proxy.SourceType != proxyservice.ProxySourceAPI || proxy.ExtractURL == "" {
		api.BadRequest(c, 10001, "该代理未配置动态提取来源")
		return
	}
	extracted, err := proxyservice.ExtractProxy(ctx, proxyservice.ProxyExtractionInput{ExtractURL: proxy.ExtractURL, ProxyProtocol: proxy.ProxyProtocol})
	if err != nil {
		api.Failure(c, 503, 30007, "动态代理提取失败；提取需要本人桌面环境运行", nil)
		return
	}
	updated, err := proxyservice.Update(proxy.ID, proxyservice.CreateProxyInput{
		Name: proxy.Name, SourceType: proxyservice.ProxySourceAPI, ExtractURL: proxy.ExtractURL,
		ProxyProtocol: extracted.ProxyProtocol, Host: extracted.Host, Port: extracted.Port,
		Username: extracted.Username, Password: extracted.Password, Region: proxy.Region, Supplier: proxy.Supplier,
		Tags: proxy.Tags, ExpiresAt: proxy.ExpiresAt, Remark: proxy.Remark, MaxProfileCount: proxy.MaxProfileCount,
	}, true)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	updated, result, err := proxyservice.CheckProxy(updated.ID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	assigned, err := profilebindingservice.CountProfilesByProxyID(updated.ID)
	if err != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	view := buildProxyView(actor, updated, assigned, newTeamNameResolver(), true)
	view.AssignmentBlocker = firstMessage(view.AssignmentBlocker, result.Reason)
	api.Success(c, view)
}

func CreateProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req proxyservice.CreateProxyInput
	if !api.DecodeJSON(c, &req) {
		return
	}
	teamID, err := resolveProxyWriteTeam(actor, req.TeamID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	proxy, err := proxyservice.Create(req, teamID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Created(c, buildProxyView(actor, proxy, 0, newTeamNameResolver(), false))
}

func UpdateProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req proxyservice.CreateProxyInput
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	govern := canGovernProxy(actor, proxy)
	var updated proxyservice.ProxyConfig
	update := func() error {
		var updateErr error
		updated, updateErr = proxyservice.Update(proxy.ID, req, govern)
		return updateErr
	}
	if govern && req.MaxProfileCount > 0 {
		// 统一编辑里改配额仍然是配额变更：不能低于当前正式占用，并发绑定下也一样。
		err = proxyservice.WithAssignmentLock(proxy.ID, func() error {
			assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
			if countErr != nil {
				return countErr
			}
			if req.MaxProfileCount < assigned {
				return proxyservice.ErrQuotaBelowUsage
			}
			return update()
		})
	} else {
		err = update()
	}
	if err != nil {
		writeProxyError(c, err)
		return
	}
	assigned, err := profilebindingservice.CountProfilesByProxyID(updated.ID)
	if err != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	api.Success(c, buildProxyView(actor, updated, assigned, newTeamNameResolver(), false))
}

func UpdateProxyStatus(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		BusinessStatus proxyservice.BusinessStatus `json:"business_status"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canGovernProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	updated, err := proxyservice.UpdateStatus(proxy.ID, req.BusinessStatus)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	assigned, err := profilebindingservice.CountProfilesByProxyID(updated.ID)
	if err != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	api.Success(c, buildProxyView(actor, updated, assigned, newTeamNameResolver(), false))
}

func DeleteProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canGovernProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	var deleteBlocked error
	err = proxyservice.WithAssignmentLock(proxy.ID, func() error {
		count, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
		if countErr != nil {
			return countErr
		}
		if count > 0 {
			deleteBlocked = errProxyReferenced
			return nil
		}
		return proxyservice.Delete(proxy.ID)
	})
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if deleteBlocked != nil {
		api.Conflict(c, 23006, "代理仍绑定窗口，请在浏览器窗口页解绑或更换后再删除")
		return
	}
	api.NoContent(c)
}

func PreviewProxyImport(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		TeamID *identityservice.TeamID `json:"team_id"`
		Lines  []string                `json:"lines"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	teamID, err := resolveProxyWriteTeam(actor, req.TeamID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	keys, err := proxyservice.LookupConnectionKeys(teamID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	rows := proxyservice.BulkParse(req.Lines, keys)
	api.Success(c, map[string]interface{}{"rows": rows, "total": len(rows)})
}

func ImportProxies(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		TeamID *identityservice.TeamID       `json:"team_id"`
		Rows   []proxyservice.ImportRowInput `json:"rows"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	teamID, err := resolveProxyWriteTeam(actor, req.TeamID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	selected := 0
	for _, row := range req.Rows {
		if row.Selected {
			selected++
		}
	}
	if len(req.Rows) == 0 || selected == 0 {
		api.BadRequest(c, 10001, "请至少选择一条有效代理记录")
		return
	}
	result, err := proxyservice.Import(req.Rows, teamID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Created(c, result)
}

func PreviewLocalProxyScan(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ScanID string `json:"scan_id"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	scan, err := profilebindingservice.GetScan(actor, req.ScanID)
	if err != nil {
		api.NotFound(c, 23001, "扫描记录不存在")
		return
	}
	preview, err := buildLocalProxyScanPreview(actor, scan)
	if err != nil {
		api.InternalError(c, "本机代理差异计算失败")
		return
	}
	api.Success(c, preview)
}

func ConfirmLocalProxyScan(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ScanID             string   `json:"scan_id"`
		SelectedProfileIDs []string `json:"selected_profile_ids"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	// 只有运营接受了本机窗口快照后才核对代理关系。接受扫描就是这一决定，
	// 本接口不会自行认领窗口。
	scan, err := profilebindingservice.GetScan(actor, req.ScanID)
	if err != nil || scan.UserID != actor.ID {
		api.Failure(c, 409, 23004, "扫描记录无法确认", nil)
		return
	}
	if scan.Status != profilebindingservice.ScanConfirmed {
		api.Failure(c, 409, 23004, "请先接受本机窗口变化，再处理代理关系", nil)
		return
	}
	preview, err := buildLocalProxyScanPreview(actor, scan)
	if err != nil {
		api.InternalError(c, "本机代理差异计算失败")
		return
	}
	unknownCounts := make(map[string]int)
	for _, change := range preview.Changes {
		if change.Kind == "unknown" {
			unknownCounts[proxyAddressKey(change.ProxyProtocol, change.Host, change.Port)]++
		}
	}
	for index := range preview.Changes {
		change := &preview.Changes[index]
		switch change.Kind {
		case "changed":
			// 只核对当前地址仍与扫描观测一致的窗口；扫描是本机事实，不是写权限。
			err := proxyservice.WithAssignmentLock(change.TargetProxyID, func() error {
				_, bindErr := profilebindingservice.BindProxy(change.ProfileID, change.TargetProxyID, change.ProxyProtocol, change.Host, change.Port)
				return bindErr
			})
			if err != nil && !errors.Is(err, proxyservice.ErrAssignmentLocked) {
				api.InternalError(c, "代理正式关系更新失败")
				return
			}
		case "unbound":
			if _, err := profilebindingservice.UnbindProxy(change.ProfileID, change.CurrentProxyID); err != nil {
				api.InternalError(c, "代理正式关系清除失败")
				return
			}
		case "unknown":
			if !containsProfileID(req.SelectedProfileIDs, change.ProfileID) {
				change.Kind = "unregistered"
				continue
			}
			created, err := proxyservice.CreateDiscovered(proxyservice.CreateProxyInput{
				ProxyProtocol: proxyservice.ProxyProtocol(change.ProxyProtocol), Host: change.Host, Port: change.Port,
			}, actorTeamOf(actor), unknownCounts[proxyAddressKey(change.ProxyProtocol, change.Host, change.Port)])
			if err != nil {
				api.InternalError(c, "扫描发现代理记录失败")
				return
			}
			change.TargetProxyID = created.ID
			if _, err := profilebindingservice.BindProxy(change.ProfileID, created.ID, change.ProxyProtocol, change.Host, change.Port); err != nil {
				api.InternalError(c, "扫描发现代理关联失败")
				return
			}
		}
	}
	api.Success(c, preview)
}

func CheckProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	updated, result, err := proxyservice.CheckProxy(proxy.ID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	assigned, err := profilebindingservice.CountProfilesByProxyID(updated.ID)
	if err != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	view := buildProxyView(actor, updated, assigned, newTeamNameResolver(), false)
	view.AssignmentBlocker = firstMessage(view.AssignmentBlocker, result.Reason)
	api.Success(c, view)
}

func AssignProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ProfileID string `json:"profile_id"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	updated, err := assignProxyToProfile(ctx, actor, proxy.ID, req.ProfileID)
	if err != nil {
		writeProxyAssignmentError(c, err)
		return
	}
	api.Success(c, updated)
}

func AssignProxyBatch(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ProfileIDs []string `json:"profile_ids"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	profileIDs := normalizedProfileIDs(req.ProfileIDs)
	if len(profileIDs) == 0 {
		api.BadRequest(c, 10001, "请至少选择一个浏览器窗口")
		return
	}
	result := ProxyBatchAssignResult{
		Succeeded: make([]profilebindingservice.BrowserProfile, 0, len(profileIDs)),
		Failed:    make([]ProxyBatchAssignFailure, 0),
	}
	for _, profileID := range profileIDs {
		updated, assignErr := assignProxyToProfile(ctx, actor, proxy.ID, profileID)
		if assignErr != nil {
			result.Failed = append(result.Failed, ProxyBatchAssignFailure{ProfileID: profileID, Message: proxyAssignmentFailureMessage(assignErr)})
			continue
		}
		result.Succeeded = append(result.Succeeded, updated)
	}
	api.Success(c, result)
}

func UnbindProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ProfileID string `json:"profile_id"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	profile, found, err := profilebindingservice.GetProfile(req.ProfileID)
	if err != nil {
		api.InternalError(c, "Profile 查询失败")
		return
	}
	if !found || !actor.CanAccessOwnedResource(profile.UserID, profile.TeamID) || profile.UserID != actor.ID || profile.LocalStatus != profilebindingservice.ProfileActive {
		api.Forbidden(c, 11003, "没有权限解绑此 Profile")
		return
	}
	if profile.ProxyID != proxy.ID {
		api.Conflict(c, 23004, "Profile 未绑定此代理")
		return
	}
	result, mutateErr := proxyservice.MutateProxy(ctx, proxyservice.ProxyMutationInput{Operation: "unbind", ProfileID: profile.BitProfileID})
	if mutateErr != nil || !result.Readback || result.Operation != "unbind" || result.ProfileID != profile.BitProfileID {
		api.Failure(c, 503, 30008, "Agent 写入或读回解绑失败", nil)
		return
	}
	updated, unbindErr := profilebindingservice.UnbindProxy(profile.ID, profile.ProxyID)
	if unbindErr != nil {
		api.InternalError(c, "代理正式关系清除失败")
		return
	}
	api.Success(c, updated)
}

// UnbindProfileProxy 清除比特自管窗口的实际代理，即「解绑并改为直连」。
// 托管窗口必须先解除正式关系，避免两步结果互相矛盾。
func UnbindProfileProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ProfileID string `json:"profile_id"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	profile, err := ownedActiveProfile(actor, req.ProfileID)
	if err != nil {
		if errors.Is(err, errProfileProxyUnauthorized) {
			api.Forbidden(c, 11003, "没有权限操作此 Profile")
			return
		}
		writeProxyAssignmentError(c, err)
		return
	}
	if profile.ProxyID != "" {
		api.Conflict(c, 23004, "该窗口仍绑定系统代理，请先改为比特自管")
		return
	}
	result, mutateErr := proxyservice.MutateProxy(ctx, proxyservice.ProxyMutationInput{Operation: "unbind", ProfileID: profile.BitProfileID})
	if mutateErr != nil || !result.Readback || result.Operation != "unbind" || result.ProfileID != profile.BitProfileID {
		api.Failure(c, 503, 30008, "Agent 写入或读回解绑失败", nil)
		return
	}
	updated, clearErr := profilebindingservice.ClearObservedProxy(profile.ID)
	if clearErr != nil {
		api.InternalError(c, "窗口代理状态清除失败")
		return
	}
	api.Success(c, updated)
}

// ReleaseProxy 只解除系统托管关系，不改比特窗口里的实际地址：窗口继续用原代理，
// 只是不再占用配额。
func ReleaseProxy(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		ProfileID string `json:"profile_id"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canReadProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	profile, err := ownedActiveProfile(actor, req.ProfileID)
	if err != nil {
		if errors.Is(err, errProfileProxyUnauthorized) {
			api.Forbidden(c, 11003, "没有权限操作此 Profile")
			return
		}
		writeProxyAssignmentError(c, err)
		return
	}
	if profile.ProxyID != proxy.ID {
		api.Conflict(c, 23004, "Profile 未绑定此代理")
		return
	}
	var updated profilebindingservice.BrowserProfile
	lockErr := proxyservice.WithAssignmentLock(proxy.ID, func() error {
		released, releaseErr := profilebindingservice.ReleaseProxy(profile.ID, proxy.ID)
		if releaseErr != nil {
			return releaseErr
		}
		updated = released
		return nil
	})
	if lockErr != nil {
		writeProxyAssignmentError(c, lockErr)
		return
	}
	api.Success(c, updated)
}

func SetProxyQuota(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		MaxProfiles int `json:"max_profiles"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if !canGovernProxy(actor, proxy) {
		writeProxyError(c, errProxyForbidden)
		return
	}
	if req.MaxProfiles < 1 {
		api.BadRequest(c, 10001, "最大窗口数必须是正整数")
		return
	}
	var belowUsage bool
	err = proxyservice.WithAssignmentLock(proxy.ID, func() error {
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
		if countErr != nil {
			return countErr
		}
		if req.MaxProfiles < assigned {
			belowUsage = true
			return nil
		}
		_, updateErr := proxyservice.SetMaxProfileCount(proxy.ID, req.MaxProfiles)
		return updateErr
	})
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if belowUsage {
		api.BadRequest(c, 10001, "最大窗口数不能低于当前已占用数")
		return
	}
	updated, err := proxyservice.Get(proxy.ID)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	assigned, err := profilebindingservice.CountProfilesByProxyID(updated.ID)
	if err != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	api.Success(c, buildProxyView(actor, updated, assigned, newTeamNameResolver(), false))
}

// --- 范围辅助 ---

func requestedTeamID(actor identityservice.PublicUser, raw string) (*identityservice.TeamID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, errProxyTeamUnknown
	}
	if actor.Role != identityservice.RoleAdmin {
		return nil, errProxyForbidden
	}
	teamID := identityservice.TeamID(parsed)
	return &teamID, nil
}

// proxyTeamScope 把操作者映射到可读团队。运营和高级运营固定为本人团队；
// 「全部团队」只是管理员视图，永远不是合法写目标。
func proxyTeamScope(actor identityservice.PublicUser, requested *identityservice.TeamID) (*identityservice.TeamID, error) {
	if actor.Role == identityservice.RoleAdmin {
		return requested, nil
	}
	if actor.TeamID == nil {
		return nil, errProxyForbidden
	}
	if requested != nil && *requested != *actor.TeamID {
		return nil, errProxyForbidden
	}
	teamID := *actor.TeamID
	return &teamID, nil
}

func resolveProxyWriteTeam(actor identityservice.PublicUser, requested *identityservice.TeamID) (identityservice.TeamID, error) {
	if actor.Role != identityservice.RoleAdmin {
		if actor.TeamID == nil {
			return 0, errProxyTeamRequired
		}
		return *actor.TeamID, nil
	}
	if requested == nil || *requested <= 0 {
		return 0, errProxyTeamRequired
	}
	teams, err := identityservice.ListTeams(actor.ID, "")
	if err != nil {
		return 0, err
	}
	for _, team := range teams {
		if team.ID == *requested {
			return *requested, nil
		}
	}
	return 0, errProxyTeamUnknown
}

func canReadProxy(actor identityservice.PublicUser, proxy proxyservice.ProxyConfig) bool {
	if actor.Role == identityservice.RoleAdmin {
		return true
	}
	return actor.TeamID != nil && proxy.TeamID == *actor.TeamID
}

func canGovernProxy(actor identityservice.PublicUser, proxy proxyservice.ProxyConfig) bool {
	return canReadProxy(actor, proxy) &&
		(actor.Role == identityservice.RoleAdmin || actor.Role == identityservice.RoleSeniorOperator)
}

func actorTeamOf(actor identityservice.PublicUser) identityservice.TeamID {
	if actor.TeamID == nil {
		return 0
	}
	return *actor.TeamID
}

func containsProfileID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func firstMessage(primary, secondary string) string {
	if primary != "" {
		return primary
	}
	return secondary
}

func buildProxyView(actor identityservice.PublicUser, proxy proxyservice.ProxyConfig, assigned int, teams *teamNameResolver, withCredentials bool) proxyView {
	proxy.AssignedProfileCount = assigned
	proxy.RemainingProfileCount = max(0, proxyservice.NormalizedMaxProfileCount(proxy.MaxProfileCount)-assigned)
	view := proxyView{
		ProxyConfig: proxy,
		Expired:     proxy.Expired(time.Now()),
	}
	view.TeamName = teams.name(actor, proxy.TeamID)
	view.AssignmentBlocker = proxyservice.AssignmentBlockReason(proxy, proxy.RemainingProfileCount)
	if !withCredentials {
		view.ProxyConfig.Username = ""
		view.ProxyConfig.Password = ""
	}
	view.ProxyConfig.ExtractURLConfigured = view.ProxyConfig.ExtractURL != ""
	view.ProxyConfig.ExtractURL = ""
	return view
}

// teamNameResolver 按需批量取团队名，避免列表逐行查询，同时保证详情仍能显示团队。
type teamNameResolver struct {
	loaded bool
	names  map[identityservice.TeamID]string
}

func newTeamNameResolver() *teamNameResolver {
	return &teamNameResolver{names: make(map[identityservice.TeamID]string)}
}

func (r *teamNameResolver) name(actor identityservice.PublicUser, teamID identityservice.TeamID) string {
	if teamID <= 0 {
		return ""
	}
	if !r.loaded {
		r.loaded = true
		if teams, err := identityservice.ListTeams(actor.ID, ""); err == nil {
			for _, team := range teams {
				r.names[team.ID] = team.Name
			}
		}
	}
	return r.names[teamID]
}

func normalizedProfileIDs(raw []string) []string {
	seen := make(map[string]bool, len(raw))
	profileIDs := make([]string, 0, len(raw))
	for _, value := range raw {
		profileID := strings.TrimSpace(value)
		if profileID == "" || seen[profileID] {
			continue
		}
		seen[profileID] = true
		profileIDs = append(profileIDs, profileID)
	}
	return profileIDs
}

func ownedActiveProfile(actor identityservice.PublicUser, profileID string) (profilebindingservice.BrowserProfile, error) {
	profile, found, err := profilebindingservice.GetProfile(strings.TrimSpace(profileID))
	if err != nil {
		return profilebindingservice.BrowserProfile{}, err
	}
	if !found || profile.UserID != actor.ID || profile.LocalStatus != profilebindingservice.ProfileActive {
		return profilebindingservice.BrowserProfile{}, errProfileProxyUnauthorized
	}
	return profile, nil
}

func classifyAssignmentAck(result agentclient.ProxyAssignmentResult, err error, profileID string) error {
	if err != nil {
		var response *agentclient.ResponseError
		if errors.As(err, &response) {
			switch response.Status {
			case 400, 409:
				return errProxyRejected
			case 404:
				return errProxyAgentUpgrade
			}
		}
		return errProxyReadback
	}
	if !result.Accepted || result.ProfileID != profileID {
		return errProxyReadback
	}
	return nil
}

func assignProxyToProfile(ctx context.Context, actor identityservice.PublicUser, proxyID, profileID string) (profilebindingservice.BrowserProfile, error) {
	profile, err := ownedActiveProfile(actor, profileID)
	if err != nil {
		return profilebindingservice.BrowserProfile{}, err
	}
	var updated profilebindingservice.BrowserProfile
	var assignmentFailure error
	err = proxyservice.WithAssignmentLock(proxyID, func() error {
		proxy, getErr := proxyservice.Get(proxyID)
		if getErr != nil {
			return getErr
		}
		if !canReadProxy(actor, proxy) {
			return errProxyForbidden
		}
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
		if countErr != nil {
			return countErr
		}
		remaining := proxyservice.NormalizedMaxProfileCount(proxy.MaxProfileCount) - assigned
		if reason := proxyservice.AssignmentBlockerForProfile(proxy, time.Now(), remaining, profile.ProxyID == proxy.ID); reason != "" {
			assignmentFailure = errors.Join(proxyservice.ErrNotAssignable, errors.New(reason))
			return nil
		}
		result, mutateErr := proxyservice.ApplyProxy(ctx, proxyservice.ProxyMutationInput{
			Operation: "assign", ProfileID: profile.BitProfileID, ProxyProtocol: proxy.ProxyProtocol,
			Host: proxy.Host, Port: proxy.Port, Username: proxy.Username, Password: proxy.Password,
		})
		if ackErr := classifyAssignmentAck(result, mutateErr, profile.BitProfileID); ackErr != nil {
			assignmentFailure = ackErr
			return nil
		}
		bound, bindErr := profilebindingservice.BindProxy(profile.ID, proxy.ID, string(proxy.ProxyProtocol), proxy.Host, proxy.Port)
		if bindErr != nil {
			assignmentFailure = errProxyReadback
			return nil
		}
		updated = bound
		return nil
	})
	if err != nil {
		return profilebindingservice.BrowserProfile{}, err
	}
	if assignmentFailure != nil {
		return profilebindingservice.BrowserProfile{}, assignmentFailure
	}
	return updated, nil
}

var errProxyReferenced = errors.New("proxy is referenced by browser profiles")

func writeProxyAssignmentError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, errProfileProxyUnauthorized):
		api.Forbidden(c, 11003, "没有权限分配此 Profile")
	case errors.Is(err, proxyservice.ErrAssignmentLocked):
		api.Conflict(c, 23009, "代理正在被其他操作占用，请稍后重试")
	case errors.Is(err, errProxyReadback):
		api.Failure(c, 503, 30008, "代理应用结果待确认，请读取比特配置核对", nil)
	case errors.Is(err, errProxyRejected):
		api.Failure(c, 409, 23101, "比特拒绝应用代理，原托管关系未更改", nil)
	case errors.Is(err, errProxyAgentUpgrade):
		api.Failure(c, 409, 23102, "本机 Agent 版本过旧，请升级后重试", nil)
	case errors.Is(err, proxyservice.ErrNotFound), errors.Is(err, proxyservice.ErrInvalidInput), errors.Is(err, proxyservice.ErrNotAssignable), errors.Is(err, errProxyForbidden):
		writeProxyError(c, err)
	default:
		api.InternalError(c, "代理正式关系更新失败")
	}
}

func proxyAssignmentFailureMessage(err error) string {
	reason := err.Error()
	if strings.Contains(reason, ":") {
		parts := strings.SplitN(reason, ":", 2)
		if len(parts) == 2 {
			reason = strings.TrimSpace(parts[1])
		}
	}
	switch {
	case errors.Is(err, errProfileProxyUnauthorized):
		return "没有权限分配此 Profile"
	case errors.Is(err, errProxyForbidden):
		return "没有权限使用此代理"
	case errors.Is(err, proxyservice.ErrNotAssignable):
		return reason
	case errors.Is(err, proxyservice.ErrNotFound):
		return "代理不存在"
	case errors.Is(err, proxyservice.ErrAssignmentLocked):
		return "代理正在被其他操作占用，请稍后重试"
	case errors.Is(err, errProxyReadback):
		return "代理应用结果待确认，请读取比特配置核对"
	case errors.Is(err, errProxyRejected):
		return "比特拒绝应用代理，原托管关系未更改"
	case errors.Is(err, errProxyAgentUpgrade):
		return "本机 Agent 版本过旧，请升级后重试"
	default:
		return "代理正式关系更新失败"
	}
}

func buildLocalProxyScanPreview(actor identityservice.PublicUser, scan profilebindingservice.ProfileScan) (LocalProxyScanPreview, error) {
	scope, err := proxyTeamScope(actor, nil)
	if err != nil {
		return LocalProxyScanPreview{}, err
	}
	known, err := proxyservice.List(proxyservice.ProxyFilter{TeamID: scope, Limit: 200})
	if err != nil {
		return LocalProxyScanPreview{}, err
	}
	byAddress := make(map[string][]proxyservice.ProxyConfig, len(known))
	for _, proxy := range known {
		key := proxyAddressKey(string(proxy.ProxyProtocol), proxy.Host, proxy.Port)
		byAddress[key] = append(byAddress[key], proxy)
	}
	changes := make([]LocalProxyChange, 0)
	for _, observed := range scan.Profiles {
		current, found, err := profilebindingservice.GetProfile(observed.ID)
		if err != nil {
			return LocalProxyScanPreview{}, err
		}
		if !found {
			continue
		}
		if current.UserID != actor.ID {
			// 其他运营的记录可见，但不会被本次本机扫描核对。
			if current.TeamID != nil && actor.TeamID != nil && *current.TeamID == *actor.TeamID {
				changes = append(changes, LocalProxyChange{
					Kind: "ownership_conflict", ProfileID: current.ID, BitProfileID: current.BitProfileID,
					OwnershipNotice: "窗口已登记在其他运营名下，本机扫描不会改动其归属",
				})
			}
			continue
		}
		protocol, host, port := strings.ToLower(strings.TrimSpace(observed.ProxyType)), strings.TrimSpace(observed.ProxyHost), observed.ProxyPort
		if protocol == "" || protocol == "noproxy" || host == "" || port <= 0 {
			if current.ProxyID != "" {
				changes = append(changes, LocalProxyChange{Kind: "unbound", ProfileID: current.ID, BitProfileID: current.BitProfileID, CurrentProxyID: current.ProxyID})
			}
			continue
		}
		matches := byAddress[proxyAddressKey(protocol, host, port)]
		base := LocalProxyChange{ProfileID: current.ID, BitProfileID: current.BitProfileID, CurrentProxyID: current.ProxyID, ProxyProtocol: protocol, Host: host, Port: port}
		switch len(matches) {
		case 0:
			base.Kind = "unknown"
		case 1:
			if current.ProxyID == matches[0].ID {
				continue
			}
			base.Kind, base.TargetProxyID = "changed", matches[0].ID
		default:
			base.Kind = "conflict"
		}
		changes = append(changes, base)
	}
	return LocalProxyScanPreview{ScanID: scan.ID, Changes: changes}, nil
}

func proxyAddressKey(protocol, host string, port int) string {
	return strings.ToLower(strings.TrimSpace(protocol)) + "://" + strings.ToLower(strings.TrimSpace(host)) + ":" + strconv.Itoa(port)
}

func writeProxyError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, errProxyForbidden):
		api.Forbidden(c, 11003, "没有权限执行此代理操作")
	case errors.Is(err, errProxyTeamRequired):
		api.BadRequest(c, 10001, "请选择目标团队")
	case errors.Is(err, errProxyTeamUnknown):
		api.BadRequest(c, 10001, "目标团队不存在")
	case errors.Is(err, proxyservice.ErrAssignmentLocked):
		api.Conflict(c, 23009, "代理正在被其他操作占用，请稍后重试")
	case errors.Is(err, proxyservice.ErrNotFound):
		api.NotFound(c, 20004, "代理不存在")
	case errors.Is(err, proxyservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "代理信息格式错误")
	case errors.Is(err, proxyservice.ErrQuotaBelowUsage):
		api.BadRequest(c, 10001, "最大窗口数不能低于当前已占用数")
	case errors.Is(err, proxyservice.ErrNotAssignable):
		api.Conflict(c, 23007, "代理未通过检测、已停用或已过期，不能绑定窗口")
	default:
		api.InternalError(c, "代理服务内部错误")
	}
}
