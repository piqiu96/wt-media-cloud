package proxy

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	profilebindingservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	repository "github.com/wt-media/wt-media-cloud/internal/modules/proxy/repository"
	proxyservice "github.com/wt-media/wt-media-cloud/internal/modules/proxy/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type ProxyBindingSummary struct {
	ID           string `json:"id"`
	BitProfileID string `json:"bit_profile_id"`
	BitName      string `json:"bit_name"`
	UserID       int64  `json:"user_id"`
	LocalState   string `json:"local_state"`
}

// proxyView 在存储行之外补页面要展示的事实。凭据仍只随详情处理器返回。
type proxyView struct {
	proxyservice.Proxy
	TeamName          string `json:"team_name,omitempty"`
	Expired           bool   `json:"expired"`
	AssignmentBlocker string `json:"assignment_blocker,omitempty"`
}

var (
	errProfileProxyUnauthorized = errors.New("profile proxy operation is forbidden")
	errProxyReferenced          = errors.New("proxy is still referenced")
	errProxyForbidden           = errors.New("proxy operation is forbidden")
	errProxyTeamRequired        = errors.New("proxy team is required")
	errProxyTeamUnknown         = errors.New("proxy team does not exist")
)

func StartProfileProxyOperation(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if actor.Role == identityservice.RoleAdmin || actor.TeamID == nil {
		writeProxyError(c, errProfileProxyUnauthorized)
		return
	}
	var input proxyservice.ProfileProxyOperationInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	profile, err := ownedActiveProfile(actor, c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	teamID := actorTeamOf(actor)
	action := strings.TrimSpace(input.Action)
	var target repository.ProfileProxyTarget
	var proxy proxyservice.Proxy
	switch action {
	case "apply":
		target, proxy, err = proxyservice.ReserveProfileProxy(profile.ID, strings.TrimSpace(input.ProxyID), teamID)
	case "unbind":
		target, proxy, err = proxyservice.ReserveProfileUnbind(profile.ID, teamID)
	case "release":
		target, proxy, err = proxyservice.ReleaseProfileProxy(profile.ID, teamID)
	default:
		api.BadRequest(c, 10001, "代理操作必须是 apply 或 unbind")
		return
	}
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Created(c, proxyservice.ProfileProxyOperationStart{
		Action: action, ProfileID: target.ProfileID, BitProfileID: target.BitProfileID,
		ProxyID: proxy.ID, Protocol: proxy.Protocol, Host: proxy.Host, Port: proxy.Port,
		Username: proxy.Username, Password: proxy.Password, AgentRequired: action != "release",
	})
}

func FinishProfileProxyOperation(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if actor.Role == identityservice.RoleAdmin || actor.TeamID == nil {
		writeProxyError(c, errProfileProxyUnauthorized)
		return
	}
	if _, err := ownedActiveProfile(actor, c.Param("id")); err != nil {
		writeProxyError(c, err)
		return
	}
	var input proxyservice.ProfileProxyOperationResultInput
	if !api.DecodeJSON(c, &input) {
		return
	}
	action := strings.TrimSpace(input.Action)
	if action != "apply" && action != "unbind" {
		api.BadRequest(c, 10001, "代理操作必须是 apply 或 unbind")
		return
	}
	if input.Outcome != "success" && input.Outcome != "failed" {
		api.BadRequest(c, 10001, "执行结果必须是 success 或 failed")
		return
	}
	if _, err := proxyservice.ResolveProfileProxyOperation(c.Param("id"), strings.TrimSpace(input.ProxyID), action == "apply", input.Outcome == "success"); err != nil {
		writeProxyError(c, err)
		return
	}
	updated, err := profilebindingservice.GetActiveProfile(actor, c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, updated)
}

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
	enabled, err := parseOptionalBool(c.Query("enabled"))
	if err != nil {
		api.BadRequest(c, 10001, "启停筛选无效")
		return
	}
	proxies, err := proxyservice.List(proxyservice.ProxyFilter{
		TeamID:   scope,
		Enabled:  enabled,
		Supplier: c.Query("supplier"),
		Region:   c.Query("region"),
		Search:   c.Query("search"),
		Limit:    200,
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
		remaining := proxyservice.NormalizedMaxProfiles(candidate.MaxProfiles) - assigned - required
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
			result = append(result, ProxyBindingSummary{ID: profile.ID, BitProfileID: profile.BitProfileID, BitName: profile.BitName, UserID: int64(profile.UserID), LocalState: string(profile.LocalState)})
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
	if proxy.Source != proxyservice.ProxySourceAPI || proxy.ExtractURL == "" {
		api.BadRequest(c, 10001, "该代理未配置动态提取来源")
		return
	}
	extracted, err := proxyservice.ExtractProxy(ctx, proxyservice.ProxyExtractionInput{ExtractURL: proxy.ExtractURL, Protocol: proxy.Protocol})
	if err != nil {
		api.Failure(c, 503, 30007, "动态代理提取失败；提取需要本人桌面环境运行", nil)
		return
	}
	updated, err := proxyservice.Update(proxy.ID, proxyservice.CreateProxyInput{
		Name: proxy.Name, Source: proxyservice.ProxySourceAPI, ExtractURL: proxy.ExtractURL,
		Protocol: extracted.Protocol, Host: extracted.Host, Port: extracted.Port,
		Username: extracted.Username, Password: extracted.Password, Region: proxy.Region, Supplier: proxy.Supplier,
		Tags: proxy.Tags, ExpiresAt: proxy.ExpiresAt, Remark: proxy.Remark, MaxProfiles: proxy.MaxProfiles,
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
	if !api.DecodeJSON(c, &req) || req.Enabled == nil {
		return
	}
	var err error
	governanceOnly := req.Name == "" && req.Source == "" && req.Protocol == "" && req.Host == "" && req.Port == 0 && req.Username == "" && req.Password == "" && req.Region == "" && req.Supplier == "" && len(req.Tags) == 0 && req.ExtractURL == "" && req.ExpiresAt == nil && req.Remark == "" && req.MaxProfiles >= 0 && (req.Enabled != nil || req.MaxProfiles > 0)
	if governanceOnly {
		proxy, getErr := proxyservice.Get(c.Param("id"))
		if getErr != nil {
			writeProxyError(c, getErr)
			return
		}
		if !canGovernProxy(actor, proxy) {
			writeProxyError(c, errProxyForbidden)
			return
		}
		var updated proxyservice.Proxy
		update := func() error {
			var maxProfiles *int
			if req.MaxProfiles > 0 {
				maxProfiles = &req.MaxProfiles
			}
			var updateErr error
			updated, updateErr = proxyservice.UpdateGovernance(proxy.ID, req.Enabled, maxProfiles)
			return updateErr
		}
		if req.MaxProfiles > 0 {
			err = proxyservice.WithAssignmentLock(proxy.ID, func() error {
				assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
				if countErr != nil {
					return countErr
				}
				reserved, reservedErr := proxyservice.CountReservedProfilesByProxyID(proxy.ID)
				if reservedErr != nil {
					return reservedErr
				}
				if req.MaxProfiles < assigned+reserved {
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
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(updated.ID)
		if countErr != nil {
			api.InternalError(c, "代理关联窗口查询失败")
			return
		}
		api.Success(c, buildProxyView(actor, updated, assigned, newTeamNameResolver(), false))
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
	var updated proxyservice.Proxy
	update := func() error {
		var updateErr error
		updated, updateErr = proxyservice.Update(proxy.ID, req, govern)
		return updateErr
	}
	if govern && req.MaxProfiles > 0 {
		// 统一编辑里改配额仍然是配额变更：不能低于当前正式占用，并发绑定下也一样。
		err = proxyservice.WithAssignmentLock(proxy.ID, func() error {
			assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
			if countErr != nil {
				return countErr
			}
			reserved, reservedErr := proxyservice.CountReservedProfilesByProxyID(proxy.ID)
			if reservedErr != nil {
				return reservedErr
			}
			if req.MaxProfiles < assigned+reserved {
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
		Enabled *bool `json:"enabled"`
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
	updated, err := proxyservice.UpdateEnabled(proxy.ID, *req.Enabled)
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
		reserved, reservedErr := proxyservice.CountReservedProfilesByProxyID(proxy.ID)
		if reservedErr != nil {
			return reservedErr
		}
		if count > 0 || reserved > 0 {
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
	updated, checkResult, err := proxyservice.CheckProxy(proxy.ID)
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
	view.AssignmentBlocker = firstMessage(view.AssignmentBlocker, checkResult.Reason)
	api.Success(c, view)
}

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

func canReadProxy(actor identityservice.PublicUser, proxy proxyservice.Proxy) bool {
	if actor.Role == identityservice.RoleAdmin {
		return true
	}
	return actor.TeamID != nil && proxy.TeamID == *actor.TeamID
}

func canGovernProxy(actor identityservice.PublicUser, proxy proxyservice.Proxy) bool {
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

func firstMessage(primary, secondary string) string {
	if primary != "" {
		return primary
	}
	return secondary
}

func buildProxyView(actor identityservice.PublicUser, proxy proxyservice.Proxy, assigned int, teams *teamNameResolver, withCredentials bool) proxyView {
	proxy.AssignedProfileCount = assigned
	reserved, _ := proxyservice.CountReservedProfilesByProxyID(proxy.ID)
	proxy.ReservedProfileCount = reserved
	proxy.RemainingProfileCount = max(0, proxyservice.NormalizedMaxProfiles(proxy.MaxProfiles)-assigned-reserved)
	view := proxyView{
		Proxy:   proxy,
		Expired: proxy.Expired(time.Now()),
	}
	view.TeamName = teams.name(actor, proxy.TeamID)
	view.AssignmentBlocker = proxyservice.AssignmentBlockReason(proxy, proxy.RemainingProfileCount)
	if !withCredentials {
		view.Proxy.Username = ""
		view.Proxy.Password = ""
	}
	view.Proxy.ExtractURLConfigured = view.Proxy.ExtractURL != ""
	view.Proxy.ExtractURL = ""
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
	if !found || profile.UserID != actor.ID || profile.LocalState != profilebindingservice.ProfileActive {
		return profilebindingservice.BrowserProfile{}, errProfileProxyUnauthorized
	}
	return profile, nil
}

func writeProxyError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, errProxyForbidden), errors.Is(err, errProfileProxyUnauthorized):
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
	case errors.Is(err, repository.ErrProxyOperationBusy):
		api.Conflict(c, 20009, "该窗口已有代理操作进行中，请等待本次结果后再重试")
	case errors.Is(err, repository.ErrQuotaExceeded):
		api.Conflict(c, 23002, "代理窗口配额已满")
	case errors.Is(err, repository.ErrProxyNotAssignable):
		api.Conflict(c, 23003, "代理未启用、已到期或尚未检测成功")
	case errors.Is(err, repository.ErrProxyOperationStale):
		api.Conflict(c, 23004, "代理操作预留已失效，请重新发起")
	default:
		api.InternalError(c, "代理服务内部错误")
	}
}
