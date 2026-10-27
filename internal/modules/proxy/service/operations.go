package service

import (
	"context"
	"time"

	agentclient "github.com/wt-media/wt-media-cloud/internal/infra/client/agent"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/repository"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

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

func List(filter dto.ProxyFilter) ([]model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).List(filter)
}

func ParseAddress(raw string) (dto.CreateProxyInput, error) {
	return newProxyService(mysqlStore{}).ParseAddress(raw)
}

func Get(id string) (model.ProxyConfig, error) { return newProxyService(mysqlStore{}).Get(id) }

func Create(input dto.CreateProxyInput, teamID sharedidentity.TeamID) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).Create(input, teamID)
}

func CreateDiscovered(input dto.CreateProxyInput, teamID sharedidentity.TeamID, observedProfileCount int) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).CreateDiscovered(input, teamID, observedProfileCount)
}

func Update(id string, input dto.CreateProxyInput, govern bool) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).Update(id, input, govern)
}

func UpdateStatus(id string, status model.BusinessStatus) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).UpdateStatus(id, status)
}

func Delete(id string) error { return newProxyService(mysqlStore{}).Delete(id) }

func CheckQuota(proxyID string, currentAssigned int) (bool, error) {
	return newProxyService(mysqlStore{}).CheckQuota(proxyID, currentAssigned)
}

func CheckAssignable(proxy model.ProxyConfig) error {
	return newProxyService(mysqlStore{}).CheckAssignable(proxy)
}

func AssignmentBlockReason(proxy model.ProxyConfig, remaining int) string {
	return AssignmentBlocker(proxy, time.Now(), remaining)
}

func SetMaxProfileCount(proxyID string, maxProfiles int) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).SetMaxProfileCount(proxyID, maxProfiles)
}

func RecordCheckResult(id string, checked dto.ProxyCheckInput, result string) (model.ProxyConfig, bool, error) {
	return newProxyService(mysqlStore{}).RecordCheckResult(id, checked, result)
}

func WithAssignmentLock(proxyID string, fn func() error) error {
	return newProxyService(mysqlStore{}).WithAssignmentLock(proxyID, fn)
}

type (
	ProxyCheckInput       = dto.ProxyCheckInput
	ProxyCheckResult      = dto.ProxyCheckResult
	ProxyExtractionInput  = dto.ProxyExtractionInput
	ProxyExtractionResult = dto.ProxyExtractionResult
	ProxyMutationInput    = dto.ProxyMutationInput
	ProxyMutationResult   = dto.ProxyMutationResult
)

// CheckProxy 探测已保存的代理。检测在 Cloud 进程内执行，不依赖本机 Agent，
// 因此与哪台工作站在线无关，也不会触碰浏览器窗口。
func CheckProxy(id string) (model.ProxyConfig, dto.ProxyCheckResult, error) {
	svc := newProxyService(mysqlStore{})
	proxy, err := svc.Get(id)
	if err != nil {
		return model.ProxyConfig{}, dto.ProxyCheckResult{}, err
	}
	result := svc.Probe(proxy, ProbeOptions{})
	updated, applied, err := svc.RecordCheckResult(id, dto.ProxyCheckInput{
		ProxyID:       proxy.ID,
		ProxyProtocol: proxy.ProxyProtocol,
		Host:          proxy.Host,
		Port:          proxy.Port,
	}, result.Connectivity)
	if err != nil {
		return model.ProxyConfig{}, result, err
	}
	if !applied {
		return updated, dto.ProxyCheckResult{
			Connectivity: CheckInvalid,
			Reason:       "代理地址在检测期间被修改，本次结果已丢弃，请重新检测",
		}, nil
	}
	return updated, result, nil
}

// ExtractProxy 仍在运营工作站执行：动态代理商 URL 要用运营可用的网络抓取，
// 取到的地址再由 Cloud 保存并检测。
func ExtractProxy(ctx context.Context, input dto.ProxyExtractionInput) (dto.ProxyExtractionResult, error) {
	result, err := agentclient.Get().ExtractProxy(ctx, agentclient.ProxyExtractionRequest{
		ExtractURL: input.ExtractURL, ProxyProtocol: string(input.ProxyProtocol),
	})
	if err != nil {
		return dto.ProxyExtractionResult{}, err
	}
	return dto.ProxyExtractionResult{
		ProxyProtocol: model.ProxyProtocol(result.ProxyProtocol), Host: result.Host, Port: result.Port,
		Username: result.Username, Password: result.Password,
	}, nil
}

func MutateProxy(ctx context.Context, input dto.ProxyMutationInput) (dto.ProxyMutationResult, error) {
	result, err := agentclient.Get().MutateProxy(ctx, agentclient.ProxyMutationRequest{
		Operation: input.Operation, ProfileID: input.ProfileID, ProxyProtocol: string(input.ProxyProtocol),
		Host: input.Host, Port: input.Port, Username: input.Username, Password: input.Password,
	})
	if err != nil {
		return dto.ProxyMutationResult{}, err
	}
	return dto.ProxyMutationResult{
		Operation: result.Operation, ProfileID: result.ProfileID, ProxyProtocol: model.ProxyProtocol(result.ProxyProtocol),
		Host: result.Host, Port: result.Port, Readback: result.Readback,
	}, nil
}

// MarkExpiringProxies 保留为调度入口，但到期状态改为读取时由 expires_at 现算。
// 任务只汇报页面会显示的到期项，不再把「已到期」写进检测结果列，避免健康代理
// 被所有读取方当成检测失败。
func MarkExpiringProxies(ctx context.Context) ([]model.ProxyConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	deadline := now.Add(7 * 24 * time.Hour)
	proxies, err := repository.List(repository.ProxyFilter{BusinessStatus: string(model.BizActive), Limit: 1000})
	if err != nil {
		return nil, err
	}
	expiring := make([]model.ProxyConfig, 0)
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
