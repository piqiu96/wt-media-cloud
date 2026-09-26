package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type mysqlStore struct{}

func (mysqlStore) FindMaterial(id int64) (model.Material, bool, error) {
	return repository.FindMaterialUnscoped(id)
}

func (mysqlStore) ListMaterials(filter repository.MaterialFilter) ([]model.Material, error) {
	return repository.ListMaterials(filter)
}

func (mysqlStore) CreateOrRestoreUsage(input repository.CreateUsageInput, now time.Time) (model.MaterialUsage, error) {
	return repository.CreateOrRestoreUsage(input, now)
}
