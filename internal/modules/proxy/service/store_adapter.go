package service

import (
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/repository"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type mysqlStore struct{}

func (mysqlStore) Create(proxy model.Proxy) error { return repository.Create(proxy) }
func (mysqlStore) FindByID(id string) (model.Proxy, bool, error) {
	return repository.FindByID(id)
}
func (mysqlStore) List(filter dto.ProxyFilter) ([]model.Proxy, error) {
	return repository.List(repository.ProxyFilter{
		TeamID:   filter.TeamID,
		Platform: filter.Platform,
		Supplier: filter.Supplier,
		Enabled:  filter.Enabled,
		Region:   filter.Region,
		Search:   filter.Search,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}
func (mysqlStore) Update(proxy model.Proxy) error { return repository.Update(proxy) }
func (mysqlStore) Delete(id string) error         { return repository.Delete(id) }

func (mysqlStore) ListConnectionKeys(teamID sharedidentity.TeamID) ([]StoredConnectionKey, error) {
	keys, err := repository.ListConnectionKeys(teamID)
	if err != nil {
		return nil, err
	}
	results := make([]StoredConnectionKey, 0, len(keys))
	for _, key := range keys {
		results = append(results, StoredConnectionKey{ID: key.ID, Protocol: key.Protocol, Host: key.Host, Port: key.Port})
	}
	return results, nil
}

func (mysqlStore) SetMaxProfiles(id string, maxProfileCount int, now time.Time) (bool, error) {
	return repository.SetMaxProfiles(id, maxProfileCount, now)
}

func (mysqlStore) RecordCheckResult(id, protocol, host string, port int, result, message string, checkedAt time.Time) (bool, error) {
	return repository.RecordCheckResult(id, protocol, host, port, result, message, checkedAt)
}

func (mysqlStore) WithAssignmentLock(proxyID string, fn func() error) error {
	err := repository.WithAssignmentLock(proxyID, fn)
	if errors.Is(err, repository.ErrLockTimeout) {
		return errStoreLockTimeout
	}
	return err
}
