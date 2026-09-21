package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/repository"
)

type mysqlStore struct{}

func (mysqlStore) Create(proxy model.ProxyConfig) error { return repository.Create(proxy) }
func (mysqlStore) FindByID(id string) (model.ProxyConfig, bool, error) {
	return repository.FindByID(id)
}
func (mysqlStore) List(filter dto.ProxyFilter) ([]model.ProxyConfig, error) {
	return repository.List(repository.ProxyFilter{
		Platform:       filter.Platform,
		Supplier:       filter.Supplier,
		BusinessStatus: filter.BusinessStatus,
		Region:         filter.Region,
		Search:         filter.Search,
		Limit:          filter.Limit,
		Offset:         filter.Offset,
	})
}
func (mysqlStore) Update(proxy model.ProxyConfig) error { return repository.Update(proxy) }
func (mysqlStore) Delete(id string) error               { return repository.Delete(id) }
