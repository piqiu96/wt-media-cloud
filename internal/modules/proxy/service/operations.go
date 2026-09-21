package service

import (
	"context"
	"strings"
	"time"

	agentclient "github.com/wt-media/wt-media-cloud/internal/infra/client/agent"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/repository"
)

func BulkParse(lines []string) []dto.BulkImportRow {
	return newProxyService(mysqlStore{}).BulkParse(lines)
}

func BulkImport(rows []dto.BulkImportRow) ([]model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).BulkImport(rows)
}

func List(filter dto.ProxyFilter) ([]model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).List(filter)
}

func ParseAddress(raw string) (dto.CreateProxyInput, error) {
	return newProxyService(mysqlStore{}).ParseAddress(raw)
}

func Get(id string) (model.ProxyConfig, error) { return newProxyService(mysqlStore{}).Get(id) }
func Create(input dto.CreateProxyInput) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).Create(input)
}
func CreateDiscovered(input dto.CreateProxyInput, observedProfileCount int) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).CreateDiscovered(input, observedProfileCount)
}
func Update(id string, input dto.CreateProxyInput) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).Update(id, input)
}
func UpdateStatus(id string, status model.BusinessStatus) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).UpdateStatus(id, status)
}
func Delete(id string) error { return newProxyService(mysqlStore{}).Delete(id) }
func TriggerCheck(id string) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).TriggerCheck(id)
}
func CheckQuota(proxyID string, currentAssigned int) (bool, error) {
	return newProxyService(mysqlStore{}).CheckQuota(proxyID, currentAssigned)
}
func CheckAssignable(proxy model.ProxyConfig) error {
	return newProxyService(mysqlStore{}).CheckAssignable(proxy)
}
func SetMaxProfileCount(proxyID string, maxProfiles int) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).SetMaxProfileCount(proxyID, maxProfiles)
}
func RecordCheckResult(id, result string) (model.ProxyConfig, error) {
	return newProxyService(mysqlStore{}).RecordCheckResult(id, result)
}

type (
	ProxyCheckInput       = dto.ProxyCheckInput
	ProxyCheckResult      = dto.ProxyCheckResult
	ProxyExtractionInput  = dto.ProxyExtractionInput
	ProxyExtractionResult = dto.ProxyExtractionResult
	ProxyMutationInput    = dto.ProxyMutationInput
	ProxyMutationResult   = dto.ProxyMutationResult
)

func CheckProxy(ctx context.Context, input dto.ProxyCheckInput) (dto.ProxyCheckResult, error) {
	result, err := agentclient.Get().CheckProxy(ctx, agentclient.ProxyCheckRequest{
		ProxyID: input.ProxyID, ProxyProtocol: string(input.ProxyProtocol), Host: input.Host, Port: input.Port,
	})
	if err != nil {
		return dto.ProxyCheckResult{}, err
	}
	return dto.ProxyCheckResult{Connectivity: result.Connectivity}, nil
}

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

func MarkExpiringProxies(ctx context.Context) ([]model.ProxyConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	proxies, err := repository.List(repository.ProxyFilter{BusinessStatus: string(model.BizActive), Limit: 1000})
	if err != nil {
		return nil, err
	}
	updated := make([]model.ProxyConfig, 0)
	for _, proxy := range proxies {
		if proxy.ExpiresAt == nil || proxy.ExpiresAt.After(now.Add(7*24*time.Hour)) || proxy.ExpiresAt.Before(now) {
			continue
		}
		if strings.HasPrefix(proxy.LastCheckResult, "expiring_") {
			continue
		}
		label := "expiring_7days"
		switch {
		case proxy.ExpiresAt.Before(now.Add(24 * time.Hour)):
			label = "expiring_today"
		case proxy.ExpiresAt.Before(now.Add(3 * 24 * time.Hour)):
			label = "expiring_3days"
		}
		proxy.LastCheckResult = label
		proxy.UpdatedAt = now
		if err := repository.Update(proxy); err != nil {
			return updated, err
		}
		updated = append(updated, proxy)
	}
	return updated, nil
}
