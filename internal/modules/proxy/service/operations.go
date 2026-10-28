package service

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/repository"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func ReserveProfileProxy(profileID, proxyID string, teamID sharedidentity.TeamID) (repository.ProfileProxyTarget, model.Proxy, error) {
	return repository.ReserveProfileProxy(profileID, proxyID, teamID, time.Now().UTC())
}

func ReserveProfileUnbind(profileID string, teamID sharedidentity.TeamID) (repository.ProfileProxyTarget, model.Proxy, error) {
	return repository.ReserveProfileUnbind(profileID, teamID, time.Now().UTC())
}

func ResolveProfileProxyOperation(profileID, proxyID string, apply, success bool) (repository.ProfileProxyTarget, error) {
	return repository.ResolveProfileProxyOperation(profileID, proxyID, apply, success, time.Now().UTC())
}

func ReleaseProfileProxy(profileID string, teamID sharedidentity.TeamID) (repository.ProfileProxyTarget, model.Proxy, error) {
	return repository.ReleaseProfileProxy(profileID, teamID, time.Now().UTC())
}

func CountReservedProfilesByProxyID(proxyID string) (int, error) {
	return repository.CountReservedProfilesByProxyID(proxyID)
}

func BulkParse(lines []string, existingKeys []StoredConnectionKey) []dto.BulkImportRow {
	return newProxyService(mysqlStore{}).BulkParse(lines, existingKeys)
}

func Import(rows []dto.ImportRowInput, teamID sharedidentity.TeamID) (dto.ImportResult, error) {
	svc := newProxyService(mysqlStore{})
	keys, err := svc.store.ListConnectionKeys(teamID)
	if err != nil {
		return dto.ImportResult{}, err
	}
	return svc.Import(rows, teamID, keys)
}

func LookupConnectionKeys(teamID sharedidentity.TeamID) ([]StoredConnectionKey, error) {
	return newProxyService(mysqlStore{}).store.ListConnectionKeys(teamID)
}

func List(filter dto.ProxyFilter) ([]model.Proxy, error) {
	return newProxyService(mysqlStore{}).List(filter)
}

func ParseAddress(raw string) (dto.CreateProxyInput, error) {
	return newProxyService(mysqlStore{}).ParseAddress(raw)
}

func Get(id string) (model.Proxy, error) { return newProxyService(mysqlStore{}).Get(id) }

func Create(input dto.CreateProxyInput, teamID sharedidentity.TeamID) (model.Proxy, error) {
	return newProxyService(mysqlStore{}).Create(input, teamID)
}

func CreateDiscovered(input dto.CreateProxyInput, teamID sharedidentity.TeamID, observedProfileCount int) (model.Proxy, error) {
	return newProxyService(mysqlStore{}).CreateDiscovered(input, teamID, observedProfileCount)
}

func Update(id string, input dto.CreateProxyInput, govern bool) (model.Proxy, error) {
	return newProxyService(mysqlStore{}).Update(id, input, govern)
}

func UpdateGovernance(id string, enabled *bool, maxProfiles *int) (model.Proxy, error) {
	return newProxyService(mysqlStore{}).UpdateGovernance(id, enabled, maxProfiles)
}

func UpdateEnabled(id string, enabled bool) (model.Proxy, error) {
	return newProxyService(mysqlStore{}).UpdateEnabled(id, enabled)
}

func Delete(id string) error { return newProxyService(mysqlStore{}).Delete(id) }

func CheckQuota(proxyID string, currentAssigned int) (bool, error) {
	return newProxyService(mysqlStore{}).CheckQuota(proxyID, currentAssigned)
}

func CheckAssignable(proxy model.Proxy) error {
	return newProxyService(mysqlStore{}).CheckAssignable(proxy)
}

func AssignmentBlockReason(proxy model.Proxy, remaining int) string {
	return AssignmentBlocker(proxy, time.Now(), remaining)
}

func SetMaxProfiles(proxyID string, maxProfiles int) (model.Proxy, error) {
	return newProxyService(mysqlStore{}).SetMaxProfiles(proxyID, maxProfiles)
}

func RecordCheckResult(id string, checked dto.ProxyCheckInput, result, message string) (model.Proxy, bool, error) {
	return newProxyService(mysqlStore{}).RecordCheckResult(id, checked, result, message)
}

func WithAssignmentLock(proxyID string, fn func() error) error {
	return newProxyService(mysqlStore{}).WithAssignmentLock(proxyID, fn)
}

type (
	ProxyCheckInput       = dto.ProxyCheckInput
	ProxyCheckResult      = dto.ProxyCheckResult
	ProxyExtractionResult = dto.ProxyExtractionResult
	ProxyRefreshIntent    = dto.ProxyRefreshIntent
)

// CheckProxy 探测已保存的代理。检测在 Cloud 进程内执行，不依赖本机 Agent，
// 因此与哪台工作站在线无关，也不会触碰浏览器窗口。
func CheckProxy(id string) (model.Proxy, dto.ProxyCheckResult, error) {
	svc := newProxyService(mysqlStore{})
	proxy, err := svc.Get(id)
	if err != nil {
		return model.Proxy{}, dto.ProxyCheckResult{}, err
	}
	result := svc.Probe(proxy, ProbeOptions{})
	updated, applied, err := svc.RecordCheckResult(id, dto.ProxyCheckInput{
		ProxyID:  proxy.ID,
		Protocol: proxy.Protocol,
		Host:     proxy.Host,
		Port:     proxy.Port,
	}, result.Connectivity, result.Reason)
	if err != nil {
		return model.Proxy{}, result, err
	}
	if !applied {
		return updated, dto.ProxyCheckResult{
			Connectivity: CheckInvalid,
			Reason:       "代理地址在检测期间被修改，本次结果已丢弃，请重新检测",
		}, nil
	}
	return updated, result, nil
}

// MarkExpiringProxies 保留为调度入口，但到期状态改为读取时由 expires_at 现算。
// 任务只汇报页面会显示的到期项，不再把「已到期」写进检测结果列，避免健康代理
// 被所有读取方当成检测失败。
func MarkExpiringProxies(ctx context.Context) ([]model.Proxy, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	deadline := now.Add(7 * 24 * time.Hour)
	proxies, err := repository.List(repository.ProxyFilter{Enabled: boolPtr(true), Limit: 1000})
	if err != nil {
		return nil, err
	}
	expiring := make([]model.Proxy, 0)
	for _, proxy := range proxies {
		if proxy.ExpiresAt == nil {
			continue
		}
		if proxy.ExpiresAt.Before(deadline) {
			expiring = append(expiring, proxy)
		}
	}
	return expiring, nil
}

func boolPtr(value bool) *bool { return &value }
