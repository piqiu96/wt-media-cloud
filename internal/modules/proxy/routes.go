package proxy

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
)

type TaskCreator interface {
	Create(cloudagent.CreateTaskRequest) cloudagent.Task
}
type ProfileLookup interface {
	GetProfile(string) (profilebinding.BrowserProfile, bool, error)
}
type ProfileVisibilityLookup interface {
	ListProfiles(identity.PublicUser, identity.UserID) ([]profilebinding.BrowserProfile, error)
}
type ProfileProxyBinder interface {
	CountProfilesByProxyID(string) (int, error)
	BindProxy(profileID, proxyID, proxyType, proxyHost string, proxyPort int) (profilebinding.BrowserProfile, error)
	UnbindProxy(profileID, expectedProxyID string) (profilebinding.BrowserProfile, error)
}
type ProfileScanController interface {
	GetScan(identity.PublicUser, string) (profilebinding.ProfileScan, error)
	ConfirmScan(identity.PublicUser, string) (profilebinding.ProfileScan, error)
}
type LocalTrustChecker interface {
	CheckLocalTrust(identity.UserID, string) error
}

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

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service, deps ...any) {
	var tasks TaskCreator
	var profiles ProfileLookup
	var visibleProfiles ProfileVisibilityLookup
	var checker SyncProxyChecker
	var extractor SyncProxyExtractor
	var mutator SyncProxyMutator
	var bindings ProfileProxyBinder
	var scans ProfileScanController
	var trust LocalTrustChecker
	for _, dep := range deps {
		if value, ok := dep.(TaskCreator); ok {
			tasks = value
		}
		if value, ok := dep.(ProfileLookup); ok {
			profiles = value
		}
		if value, ok := dep.(ProfileVisibilityLookup); ok {
			visibleProfiles = value
		}
		if value, ok := dep.(SyncProxyChecker); ok {
			checker = value
		}
		if value, ok := dep.(SyncProxyExtractor); ok {
			extractor = value
		}
		if value, ok := dep.(SyncProxyMutator); ok {
			mutator = value
		}
		if value, ok := dep.(ProfileProxyBinder); ok {
			bindings = value
		}
		if value, ok := dep.(ProfileScanController); ok {
			scans = value
		}
		if value, ok := dep.(LocalTrustChecker); ok {
			trust = value
		}
	}
	h.GET("/api/v1/proxies", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxies, err := service.List(ProxyFilter{
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
		if bindings != nil {
			for index := range proxies {
				assigned, countErr := bindings.CountProfilesByProxyID(proxies[index].ID)
				if countErr != nil {
					common.InternalError(c, "代理关联窗口查询失败")
					return
				}
				proxies[index].AssignedProfileCount = assigned
				proxies[index].RemainingProfileCount = max(0, normalizedMaxProfileCount(proxies[index].MaxProfileCount)-assigned)
			}
		}
		for index := range proxies {
			proxies[index] = publicProxy(proxies[index])
		}
		common.Success(c, proxies)
	})

	h.GET("/api/v1/proxies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, publicProxy(proxy))
	})

	h.GET("/api/v1/proxies/:id/bindings", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if _, err := service.Get(c.Param("id")); err != nil {
			writeProxyError(c, err)
			return
		}
		if visibleProfiles == nil {
			common.Failure(c, 503, 30006, "浏览器窗口查询服务不可用", nil)
			return
		}
		profiles, err := visibleProfiles.ListProfiles(actor, 0)
		if err != nil {
			common.InternalError(c, "浏览器窗口查询失败")
			return
		}
		result := make([]ProxyBindingSummary, 0)
		for _, profile := range profiles {
			if profile.ProxyID == c.Param("id") {
				result = append(result, ProxyBindingSummary{ID: profile.ID, BitProfileID: profile.BitProfileID, Name: profile.Name, UserID: int64(profile.UserID), LocalStatus: string(profile.LocalStatus)})
			}
		}
		common.Success(c, result)
	})

	h.POST("/api/v1/proxies/parse", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			ProxyAddress string `json:"proxy_address"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		parsed, err := service.ParseAddress(req.ProxyAddress)
		if err != nil {
			common.Failure(c, 400, 10001, "代理地址格式无效", nil)
			return
		}
		common.Success(c, parsed)
	})

	h.POST("/api/v1/proxies/extract-preview", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req ProxyExtractionInput
		if !common.DecodeJSON(c, &req) {
			return
		}
		if extractor == nil {
			common.Failure(c, 503, 30006, "Agent 动态代理提取服务不可用", nil)
			return
		}
		result, err := extractor.Extract(ctx, req)
		if err != nil {
			common.Failure(c, 503, 30007, "Agent 动态代理提取失败", nil)
			return
		}
		common.Success(c, result)
	})

	h.POST("/api/v1/proxies/:id/refresh", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if proxy.SourceType != ProxySourceAPI || proxy.ExtractURL == "" {
			common.Failure(c, 400, 10001, "该代理未配置动态提取来源", nil)
			return
		}
		if extractor == nil || checker == nil {
			common.Failure(c, 503, 30006, "Agent 动态代理服务不可用", nil)
			return
		}
		extracted, err := extractor.Extract(ctx, ProxyExtractionInput{ExtractURL: proxy.ExtractURL, ProxyProtocol: proxy.ProxyProtocol})
		if err != nil {
			common.Failure(c, 503, 30007, "Agent 动态代理提取失败", nil)
			return
		}
		updated, err := service.Update(proxy.ID, CreateProxyInput{SourceType: ProxySourceAPI, ExtractURL: proxy.ExtractURL, ProxyProtocol: extracted.ProxyProtocol, Host: extracted.Host, Port: extracted.Port, Username: extracted.Username, Password: extracted.Password, Region: proxy.Region, Supplier: proxy.Supplier, ExpiresAt: proxy.ExpiresAt, Remark: proxy.Remark, MaxProfileCount: proxy.MaxProfileCount})
		if err != nil {
			writeProxyError(c, err)
			return
		}
		check, err := checker.Check(ctx, ProxyCheckInput{ProxyID: updated.ID, ProxyProtocol: updated.ProxyProtocol, Host: updated.Host, Port: updated.Port})
		if err != nil {
			common.Failure(c, 503, 30007, "Agent 同步检查失败", nil)
			return
		}
		updated, err = service.RecordCheckResult(updated.ID, check.Connectivity)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, publicProxy(updated))
	})

	h.POST("/api/v1/proxies", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req CreateProxyInput
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.Create(req)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Created(c, publicProxy(proxy))
	})

	h.PATCH("/api/v1/proxies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req CreateProxyInput
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.Update(c.Param("id"), req)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, publicProxy(proxy))
	})

	h.PATCH("/api/v1/proxies/:id/status", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			BusinessStatus BusinessStatus `json:"business_status"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.UpdateStatus(c.Param("id"), req.BusinessStatus)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, publicProxy(proxy))
	})

	h.DELETE("/api/v1/proxies/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if bindings != nil {
			count, countErr := bindings.CountProfilesByProxyID(c.Param("id"))
			if countErr != nil {
				common.InternalError(c, "代理关联窗口查询失败")
				return
			}
			if count > 0 {
				common.Conflict(c, 23006, "代理仍绑定窗口，请先在浏览器窗口页解绑或更换")
				return
			}
		}
		if err := service.Delete(c.Param("id")); err != nil {
			writeProxyError(c, err)
			return
		}
		common.NoContent(c)
	})

	h.POST("/api/v1/proxies/import/preview", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			Lines []string `json:"lines"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		parsed := service.BulkParse(req.Lines)
		common.Success(c, map[string]interface{}{"parsed": parsed})
	})

	h.POST("/api/v1/proxies/import", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			Lines []string `json:"lines"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		parsed := service.BulkParse(req.Lines)
		imported, err := service.BulkImport(parsed)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Created(c, map[string]interface{}{
			"parsed":   parsed,
			"imported": imported,
		})
	})

	h.POST("/api/v1/proxies/local-scan/preview", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if scans == nil || profiles == nil {
			common.Failure(c, 503, 30006, "本机代理扫描服务不可用", nil)
			return
		}
		var req struct {
			ScanID string `json:"scan_id"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		scan, err := scans.GetScan(actor, req.ScanID)
		if err != nil {
			common.Failure(c, 404, 23001, "扫描记录不存在", nil)
			return
		}
		preview, err := buildLocalProxyScanPreview(service, profiles, scan)
		if err != nil {
			common.InternalError(c, "本机代理差异计算失败")
			return
		}
		common.Success(c, preview)
	})

	h.POST("/api/v1/proxies/local-scan/confirm", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			ScanID string `json:"scan_id"`
			NodeID string `json:"node_id"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		if strings.TrimSpace(req.NodeID) == "" {
			common.Failure(c, 400, 10001, "本机节点不能为空", nil)
			return
		}
		if scans == nil || profiles == nil || bindings == nil || trust == nil {
			common.Failure(c, 503, 30006, "本机代理扫描服务不可用", nil)
			return
		}
		if err := trust.CheckLocalTrust(actor.ID, req.NodeID); err != nil {
			common.Forbidden(c, 11003, "当前本机环境未获可信授权")
			return
		}
		scan, err := scans.ConfirmScan(actor, req.ScanID)
		if err != nil {
			confirmed, getErr := scans.GetScan(actor, req.ScanID)
			if getErr != nil || confirmed.Status != profilebinding.ScanConfirmed {
				common.Failure(c, 409, 23004, "扫描记录无法确认", nil)
				return
			}
			scan = confirmed
		}
		preview, err := buildLocalProxyScanPreview(service, profiles, scan)
		if err != nil {
			common.InternalError(c, "本机代理差异计算失败")
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
				if _, err := bindings.BindProxy(change.ProfileID, change.TargetProxyID, change.ProxyProtocol, change.Host, change.Port); err != nil {
					common.InternalError(c, "代理正式关系更新失败")
					return
				}
			case "unbound":
				if _, err := bindings.UnbindProxy(change.ProfileID, change.CurrentProxyID); err != nil {
					common.InternalError(c, "代理正式关系清除失败")
					return
				}
			case "unknown":
				created, err := service.CreateDiscovered(CreateProxyInput{ProxyProtocol: ProxyProtocol(change.ProxyProtocol), Host: change.Host, Port: change.Port}, unknownCounts[proxyAddressKey(change.ProxyProtocol, change.Host, change.Port)])
				if err != nil {
					common.InternalError(c, "扫描发现代理记录失败")
					return
				}
				change.TargetProxyID = created.ID
				if _, err := bindings.BindProxy(change.ProfileID, created.ID, change.ProxyProtocol, change.Host, change.Port); err != nil {
					common.InternalError(c, "扫描发现代理关联失败")
					return
				}
			}
		}
		common.Success(c, preview)
	})

	h.POST("/api/v1/proxies/:id/check", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if checker == nil {
			common.Failure(c, 503, 30006, "Agent 同步检查服务不可用", nil)
			return
		}
		result, err := checker.Check(ctx, ProxyCheckInput{ProxyID: proxy.ID, ProxyProtocol: proxy.ProxyProtocol, Host: proxy.Host, Port: proxy.Port})
		if err != nil {
			common.Failure(c, 503, 30007, "Agent 同步检查失败", nil)
			return
		}
		updated, err := service.RecordCheckResult(proxy.ID, result.Connectivity)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, publicProxy(updated))
	})

	h.POST("/api/v1/proxies/:id/check/background", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if tasks == nil {
			common.Failure(c, 503, 30006, "任务服务不可用", nil)
			return
		}
		task := tasks.Create(cloudagent.CreateTaskRequest{
			TaskType:       cloudagent.TaskTypeProxyCheck.String(),
			IdempotencyKey: "proxy-check:" + strconv.FormatInt(int64(actor.ID), 10) + ":" + proxy.ID + ":" + common.NewID("attempt"),
			Payload:        map[string]any{"proxy_id": proxy.ID, "proxy_protocol": proxy.ProxyProtocol, "host": proxy.Host, "port": proxy.Port, "username": proxy.Username, "password": proxy.Password},
		})
		common.Created(c, task)
	})

	h.POST("/api/v1/proxies/:id/assign", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if profiles == nil || bindings == nil || mutator == nil {
			common.Failure(c, 503, 30006, "Agent 同步写入服务不可用", nil)
			return
		}
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		profile, found, err := profiles.GetProfile(req.ProfileID)
		if err != nil {
			common.InternalError(c, "Profile 查询失败")
			return
		}
		if !found || profile.UserID != actor.ID || profile.LocalStatus != profilebinding.ProfileActive {
			common.Forbidden(c, 11003, "没有权限分配此 Profile")
			return
		}
		proxy, err := service.Get(c.Param("id"))
		if err != nil {
			writeProxyError(c, err)
			return
		}
		if err := service.CheckAssignable(proxy); err != nil {
			writeProxyError(c, err)
			return
		}
		if profile.ProxyID != proxy.ID {
			assigned, countErr := bindings.CountProfilesByProxyID(proxy.ID)
			if countErr != nil {
				common.InternalError(c, "代理关联窗口查询失败")
				return
			}
			available, quotaErr := service.CheckQuota(proxy.ID, assigned)
			if quotaErr != nil {
				writeProxyError(c, quotaErr)
				return
			}
			if !available {
				common.Conflict(c, 23005, "代理窗口配额已满")
				return
			}
		}
		result, mutateErr := mutator.Mutate(ctx, ProxyMutationInput{Operation: "assign", ProfileID: profile.BitProfileID, ProxyProtocol: proxy.ProxyProtocol, Host: proxy.Host, Port: proxy.Port, Username: proxy.Username, Password: proxy.Password})
		if mutateErr != nil || !result.Readback || result.ProfileID != profile.BitProfileID || result.ProxyProtocol != proxy.ProxyProtocol || result.Host != proxy.Host || result.Port != proxy.Port {
			common.Failure(c, 503, 30008, "Agent 写入或读回代理失败", nil)
			return
		}
		updated, bindErr := bindings.BindProxy(profile.ID, proxy.ID, string(result.ProxyProtocol), result.Host, result.Port)
		if bindErr != nil {
			common.InternalError(c, "代理正式关系更新失败")
			return
		}
		common.Success(c, updated)
	})

	h.POST("/api/v1/proxies/:id/unbind", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if profiles == nil || bindings == nil || mutator == nil {
			common.Failure(c, 503, 30006, "Agent 同步写入服务不可用", nil)
			return
		}
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		profile, found, err := profiles.GetProfile(req.ProfileID)
		if err != nil {
			common.InternalError(c, "Profile 查询失败")
			return
		}
		if !found || profile.UserID != actor.ID || profile.LocalStatus != profilebinding.ProfileActive {
			common.Forbidden(c, 11003, "没有权限解绑此 Profile")
			return
		}
		if profile.ProxyID != c.Param("id") {
			common.Conflict(c, 23004, "Profile 未绑定此代理")
			return
		}
		result, mutateErr := mutator.Mutate(ctx, ProxyMutationInput{Operation: "unbind", ProfileID: profile.BitProfileID})
		if mutateErr != nil || !result.Readback || result.Operation != "unbind" || result.ProfileID != profile.BitProfileID {
			common.Failure(c, 503, 30008, "Agent 写入或读回解绑失败", nil)
			return
		}
		updated, unbindErr := bindings.UnbindProxy(profile.ID, profile.ProxyID)
		if unbindErr != nil {
			common.InternalError(c, "代理正式关系清除失败")
			return
		}
		common.Success(c, updated)
	})

	h.POST("/api/v1/proxies/:id/quota", func(ctx context.Context, c *hertzapp.RequestContext) {
		_, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req struct {
			MaxProfiles int `json:"max_profiles"`
		}
		if !common.DecodeJSON(c, &req) {
			return
		}
		proxy, err := service.SetMaxProfileCount(c.Param("id"), req.MaxProfiles)
		if err != nil {
			writeProxyError(c, err)
			return
		}
		common.Success(c, publicProxy(proxy))
	})
}

func buildLocalProxyScanPreview(service *Service, profiles ProfileLookup, scan profilebinding.ProfileScan) (LocalProxyScanPreview, error) {
	known, err := service.List(ProxyFilter{Limit: 200})
	if err != nil {
		return LocalProxyScanPreview{}, err
	}
	byAddress := make(map[string][]ProxyConfig, len(known))
	for _, proxy := range known {
		byAddress[proxyAddressKey(string(proxy.ProxyProtocol), proxy.Host, proxy.Port)] = append(byAddress[proxyAddressKey(string(proxy.ProxyProtocol), proxy.Host, proxy.Port)], proxy)
	}
	changes := make([]LocalProxyChange, 0)
	for _, observed := range scan.Profiles {
		current, found, err := profiles.GetProfile(observed.ID)
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

func publicProxy(proxy ProxyConfig) ProxyConfig {
	if proxy.SourceType == "" {
		proxy.SourceType = ProxySourceStatic
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
	case errors.Is(err, ErrNotFound):
		common.NotFound(c, 20004, "代理不存在")
	case errors.Is(err, ErrInvalidInput):
		common.BadRequest(c, 10001, "代理信息格式错误")
	case errors.Is(err, ErrNotAssignable):
		common.Conflict(c, 23007, "代理未通过检测、已停用或已过期，不能绑定窗口")
	default:
		common.InternalError(c, "代理服务内部错误")
	}
}
