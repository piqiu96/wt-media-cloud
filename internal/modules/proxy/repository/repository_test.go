package repository

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"gorm.io/gorm"
)

func TestProxyRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(model.Proxy) error = Create
	var _ func(string) (model.Proxy, bool, error) = FindByID
	var _ func(ProxyFilter) ([]model.Proxy, error) = List
	var _ func(model.Proxy) error = Update
	var _ func(string) error = Delete

	var _ func(*gorm.DB, model.Proxy) error = create
	var _ func(*gorm.DB, string) (model.Proxy, bool, error) = findByID
	var _ func(*gorm.DB, ProxyFilter) ([]model.Proxy, error) = list
	var _ func(*gorm.DB, model.Proxy) error = update
	var _ func(*gorm.DB, string) error = delete
}
