package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"gorm.io/gorm"
)

func TestProxyRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(model.ProxyConfig) error = Create
	var _ func(string) (model.ProxyConfig, bool, error) = FindByID
	var _ func(dto.ProxyFilter) ([]model.ProxyConfig, error) = List
	var _ func(model.ProxyConfig) error = Update
	var _ func(string) error = Delete

	var _ func(*gorm.DB, model.ProxyConfig) error = create
	var _ func(*gorm.DB, string) (model.ProxyConfig, bool, error) = findByID
	var _ func(*gorm.DB, dto.ProxyFilter) ([]model.ProxyConfig, error) = list
	var _ func(*gorm.DB, model.ProxyConfig) error = update
	var _ func(*gorm.DB, string) error = delete
}
