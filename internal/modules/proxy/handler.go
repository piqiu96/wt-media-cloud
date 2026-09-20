package proxy

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	cloudagentservice "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	profilebindingservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	proxyservice "github.com/wt-media/wt-media-cloud/internal/modules/proxy/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type LocalProxyChange struct {
	Kind           string `json:"kind"`
	ProfileID      string `json:"profile_id"`
	BitProfileID   string `json:"bit_profile_id"`
	CurrentProxyID string `json:"current_proxy_id,omitempty"`
	TargetProxyID  string `json:"target_proxy_id,omitempty"`
	ProxyProtocol  string `json:"proxy_protocol,omitempty"`
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
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

var (
	errProfileProxyUnauthorized = errors.New("profile proxy operation is forbidden")
	errProxyQuotaFull           = errors.New("proxy profile quota is full")
	errProxyReadback            = errors.New("proxy agent readback failed")
)

func ListProxies(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxies, err := proxyservice.List(proxyservice.ProxyFilter{
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
	for index := range proxies {
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxies[index].ID)
		if countErr != nil {
			api.InternalError(c, "代理关联窗口查询失败")
			return
		}
		proxies[index].AssignedProfileCount = assigned
		proxies[index].RemainingProfileCount = max(0, proxyservice.NormalizedMaxProfileCount(proxies[index].MaxProfileCount)-assigned)
	}
	for index := range proxies {
		proxies[index] = publicProxy(proxies[index])
	}
	api.Success(c, proxies)
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
	candidates, err := proxyservice.List(proxyservice.ProxyFilter{Limit: 200})
	if err != nil {
		writeProxyError(c, err)
		return
	}
	recommended := make([]proxyservice.ProxyConfig, 0, len(candidates))
	for _, candidate := range candidates {
		if proxyservice.CheckAssignable(candidate) != nil {
			continue
		}
		required := 0
		for _, target := range targets {
			if target.ProxyID != candidate.ID {
				required++
			}
		}
		if required > 0 {
			assigned, countErr := profilebindingservice.CountProfilesByProxyID(candidate.ID)
			if countErr != nil {
				api.InternalError(c, "代理关联窗口查询失败")
				return
			}
			available, quotaErr := proxyservice.CheckQuota(candidate.ID, assigned+required-1)
			if quotaErr != nil {
				writeProxyError(c, quotaErr)
				return
			}
			if !available {
				continue
			}
		}
		recommended = append(recommended, publicProxy(candidate))
	}
	api.Success(c, recommended)
}
func GetProxy(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, publicProxy(proxy))
}
func GetProxyBindings(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if _, err := proxyservice.Get(c.Param("id")); err != nil {
		writeProxyError(c, err)
		return
	}
	profiles, err := profilebindingservice.ListProfiles(actor, 0)
	if err != nil {
		api.InternalError(c, "浏览器窗口查询失败")
		return
	}
	result := make([]ProxyBindingSummary, 0)
	for _, profile := range profiles {
		if profile.ProxyID == c.Param("id") {
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
		api.Failure(c, 400, 10001, "代理地址格式无效", nil)
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
		api.Failure(c, 503, 30007, "Agent 动态代理提取失败", nil)
		return
	}
	api.Success(c, result)
}
func RefreshProxy(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	if proxy.SourceType != proxyservice.ProxySourceAPI || proxy.ExtractURL == "" {
		api.Failure(c, 400, 10001, "该代理未配置动态提取来源", nil)
		return
	}
	extracted, err := proxyservice.ExtractProxy(ctx, proxyservice.ProxyExtractionInput{ExtractURL: proxy.ExtractURL, ProxyProtocol: proxy.ProxyProtocol})
	if err != nil {
		api.Failure(c, 503, 30007, "Agent 动态代理提取失败", nil)
		return
	}
	updated, err := proxyservice.Update(proxy.ID, proxyservice.CreateProxyInput{SourceType: proxyservice.ProxySourceAPI, ExtractURL: proxy.ExtractURL, ProxyProtocol: extracted.ProxyProtocol, Host: extracted.Host, Port: extracted.Port, Username: extracted.Username, Password: extracted.Password, Region: proxy.Region, Supplier: proxy.Supplier, ExpiresAt: proxy.ExpiresAt, Remark: proxy.Remark, MaxProfileCount: proxy.MaxProfileCount})
	if err != nil {
		writeProxyError(c, err)
		return
	}
	check, err := proxyservice.CheckProxy(ctx, proxyservice.ProxyCheckInput{ProxyID: updated.ID, ProxyProtocol: updated.ProxyProtocol, Host: updated.Host, Port: updated.Port})
	if err != nil {
		api.Failure(c, 503, 30007, "Agent 同步检查失败", nil)
		return
	}
	updated, err = proxyservice.RecordCheckResult(updated.ID, check.Connectivity)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, publicProxy(updated))
}
func CreateProxy(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req proxyservice.CreateProxyInput
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Create(req)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Created(c, publicProxy(proxy))
}
func UpdateProxy(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req proxyservice.CreateProxyInput
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.Update(c.Param("id"), req)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, publicProxy(proxy))
}
func UpdateProxyStatus(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		BusinessStatus proxyservice.BusinessStatus `json:"business_status"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.UpdateStatus(c.Param("id"), req.BusinessStatus)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, publicProxy(proxy))
}
func DeleteProxy(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	count, countErr := profilebindingservice.CountProfilesByProxyID(c.Param("id"))
	if countErr != nil {
		api.InternalError(c, "代理关联窗口查询失败")
		return
	}
	if count > 0 {
		api.Conflict(c, 23006, "代理仍绑定窗口，请先在浏览器窗口页解绑或更换")
		return
	}
	if err := proxyservice.Delete(c.Param("id")); err != nil {
		writeProxyError(c, err)
		return
	}
	api.NoContent(c)
}
func PreviewProxyImport(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		Lines []string `json:"lines"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	parsed := proxyservice.BulkParse(req.Lines)
	api.Success(c, map[string]interface{}{"parsed": parsed})
}
func ImportProxies(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		Lines []string `json:"lines"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	parsed := proxyservice.BulkParse(req.Lines)
	imported, err := proxyservice.BulkImport(parsed)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Created(c, map[string]interface{}{
		"parsed":   parsed,
		"imported": imported,
	})
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
		api.Failure(c, 404, 23001, "扫描记录不存在", nil)
		return
	}
	preview, err := buildLocalProxyScanPreview(scan)
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
		ScanID string `json:"scan_id"`
		NodeID string `json:"node_id"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.NodeID) == "" {
		api.Failure(c, 400, 10001, "本机节点不能为空", nil)
		return
	}
	if err := runtimeservice.CheckLocalTrust(actor.ID, req.NodeID); err != nil {
		api.Forbidden(c, 11003, "当前本机环境未获可信授权")
		return
	}
	scan, err := profilebindingservice.ConfirmScan(actor, req.ScanID)
	if err != nil {
		confirmed, getErr := profilebindingservice.GetScan(actor, req.ScanID)
		if getErr != nil || confirmed.Status != profilebindingservice.ScanConfirmed {
			api.Failure(c, 409, 23004, "扫描记录无法确认", nil)
			return
		}
		scan = confirmed
	}
	preview, err := buildLocalProxyScanPreview(scan)
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
			if _, err := profilebindingservice.BindProxy(change.ProfileID, change.TargetProxyID, change.ProxyProtocol, change.Host, change.Port); err != nil {
				api.InternalError(c, "代理正式关系更新失败")
				return
			}
		case "unbound":
			if _, err := profilebindingservice.UnbindProxy(change.ProfileID, change.CurrentProxyID); err != nil {
				api.InternalError(c, "代理正式关系清除失败")
				return
			}
		case "unknown":
			created, err := proxyservice.CreateDiscovered(proxyservice.CreateProxyInput{ProxyProtocol: proxyservice.ProxyProtocol(change.ProxyProtocol), Host: change.Host, Port: change.Port}, unknownCounts[proxyAddressKey(change.ProxyProtocol, change.Host, change.Port)])
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
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	result, err := proxyservice.CheckProxy(ctx, proxyservice.ProxyCheckInput{ProxyID: proxy.ID, ProxyProtocol: proxy.ProxyProtocol, Host: proxy.Host, Port: proxy.Port})
	if err != nil {
		api.Failure(c, 503, 30007, "Agent 同步检查失败", nil)
		return
	}
	updated, err := proxyservice.RecordCheckResult(proxy.ID, result.Connectivity)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, publicProxy(updated))
}
func CheckProxyInBackground(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	proxy, err := proxyservice.Get(c.Param("id"))
	if err != nil {
		writeProxyError(c, err)
		return
	}
	task := cloudagentservice.CreateTask(cloudagentservice.CreateTaskRequest{
		TaskType:       cloudagentservice.TaskTypeProxyCheck.String(),
		IdempotencyKey: "proxy-check:" + strconv.FormatInt(int64(actor.ID), 10) + ":" + proxy.ID + ":" + id.NewID("attempt"),
		Payload:        map[string]any{"proxy_id": proxy.ID, "proxy_protocol": proxy.ProxyProtocol, "host": proxy.Host, "port": proxy.Port, "username": proxy.Username, "password": proxy.Password},
	})
	api.Created(c, task)
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
	updated, err := assignProxyToProfile(ctx, actor, c.Param("id"), req.ProfileID)
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
		updated, err := assignProxyToProfile(ctx, actor, c.Param("id"), profileID)
		if err != nil {
			result.Failed = append(result.Failed, ProxyBatchAssignFailure{ProfileID: profileID, Message: proxyAssignmentFailureMessage(err)})
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
	profile, found, err := profilebindingservice.GetProfile(req.ProfileID)
	if err != nil {
		api.InternalError(c, "Profile 查询失败")
		return
	}
	if !found || profile.UserID != actor.ID || profile.LocalStatus != profilebindingservice.ProfileActive {
		api.Forbidden(c, 11003, "没有权限解绑此 Profile")
		return
	}
	if profile.ProxyID != c.Param("id") {
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
func SetProxyQuota(ctx context.Context, c *hertzapp.RequestContext) {
	_, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var req struct {
		MaxProfiles int `json:"max_profiles"`
	}
	if !api.DecodeJSON(c, &req) {
		return
	}
	proxy, err := proxyservice.SetMaxProfileCount(c.Param("id"), req.MaxProfiles)
	if err != nil {
		writeProxyError(c, err)
		return
	}
	api.Success(c, publicProxy(proxy))
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

func assignProxyToProfile(ctx context.Context, actor identityservice.PublicUser, proxyID, profileID string) (profilebindingservice.BrowserProfile, error) {
	profile, err := ownedActiveProfile(actor, profileID)
	if err != nil {
		return profilebindingservice.BrowserProfile{}, err
	}
	proxy, err := proxyservice.Get(proxyID)
	if err != nil {
		return profilebindingservice.BrowserProfile{}, err
	}
	if err := proxyservice.CheckAssignable(proxy); err != nil {
		return profilebindingservice.BrowserProfile{}, err
	}
	if profile.ProxyID != proxy.ID {
		assigned, countErr := profilebindingservice.CountProfilesByProxyID(proxy.ID)
		if countErr != nil {
			return profilebindingservice.BrowserProfile{}, countErr
		}
		available, quotaErr := proxyservice.CheckQuota(proxy.ID, assigned)
		if quotaErr != nil {
			return profilebindingservice.BrowserProfile{}, quotaErr
		}
		if !available {
			return profilebindingservice.BrowserProfile{}, errProxyQuotaFull
		}
	}
	result, mutateErr := proxyservice.MutateProxy(ctx, proxyservice.ProxyMutationInput{Operation: "assign", ProfileID: profile.BitProfileID, ProxyProtocol: proxy.ProxyProtocol, Host: proxy.Host, Port: proxy.Port, Username: proxy.Username, Password: proxy.Password})
	if mutateErr != nil || !result.Readback || result.ProfileID != profile.BitProfileID || result.ProxyProtocol != proxy.ProxyProtocol || result.Host != proxy.Host || result.Port != proxy.Port {
		return profilebindingservice.BrowserProfile{}, errProxyReadback
	}
	updated, bindErr := profilebindingservice.BindProxy(profile.ID, proxy.ID, string(result.ProxyProtocol), result.Host, result.Port)
	if bindErr != nil {
		return profilebindingservice.BrowserProfile{}, bindErr
	}
	return updated, nil
}

func writeProxyAssignmentError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, errProfileProxyUnauthorized):
		api.Forbidden(c, 11003, "没有权限分配此 Profile")
	case errors.Is(err, errProxyQuotaFull):
		api.Conflict(c, 23005, "代理窗口配额已满")
	case errors.Is(err, errProxyReadback):
		api.Failure(c, 503, 30008, "Agent 写入或读回代理失败", nil)
	case errors.Is(err, proxyservice.ErrNotFound), errors.Is(err, proxyservice.ErrInvalidInput), errors.Is(err, proxyservice.ErrNotAssignable):
		writeProxyError(c, err)
	default:
		api.InternalError(c, "代理正式关系更新失败")
	}
}

func proxyAssignmentFailureMessage(err error) string {
	switch {
	case errors.Is(err, errProfileProxyUnauthorized):
		return "没有权限分配此 Profile"
	case errors.Is(err, errProxyQuotaFull):
		return "代理窗口配额已满"
	case errors.Is(err, proxyservice.ErrNotAssignable):
		return "代理未通过检测、已停用或已过期"
	case errors.Is(err, proxyservice.ErrNotFound):
		return "代理不存在"
	case errors.Is(err, errProxyReadback):
		return "Agent 写入或读回代理失败"
	default:
		return "代理正式关系更新失败"
	}
}

func buildLocalProxyScanPreview(scan profilebindingservice.ProfileScan) (LocalProxyScanPreview, error) {
	known, err := proxyservice.List(proxyservice.ProxyFilter{Limit: 200})
	if err != nil {
		return LocalProxyScanPreview{}, err
	}
	byAddress := make(map[string][]proxyservice.ProxyConfig, len(known))
	for _, proxy := range known {
		byAddress[proxyAddressKey(string(proxy.ProxyProtocol), proxy.Host, proxy.Port)] = append(byAddress[proxyAddressKey(string(proxy.ProxyProtocol), proxy.Host, proxy.Port)], proxy)
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

func publicProxy(proxy proxyservice.ProxyConfig) proxyservice.ProxyConfig {
	if proxy.SourceType == "" {
		proxy.SourceType = proxyservice.ProxySourceStatic
	}
	proxy.ExtractURLConfigured = proxy.ExtractURL != ""
	proxy.ExtractURL = ""
	proxy.Username = ""
	proxy.Password = ""
	return proxy
}

func proxyAddressKey(protocol, host string, port int) string {
	return strings.ToLower(strings.TrimSpace(protocol)) + "://" + strings.ToLower(strings.TrimSpace(host)) + ":" + strconv.Itoa(port)
}

func writeProxyError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, proxyservice.ErrNotFound):
		api.NotFound(c, 20004, "代理不存在")
	case errors.Is(err, proxyservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "代理信息格式错误")
	case errors.Is(err, proxyservice.ErrNotAssignable):
		api.Conflict(c, 23007, "代理未通过检测、已停用或已过期，不能绑定窗口")
	default:
		api.InternalError(c, "代理服务内部错误")
	}
}
