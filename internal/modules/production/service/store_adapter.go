package service

import (
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
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

func (mysqlStore) ListActiveUsages(userID identityservice.UserID) ([]model.MaterialUsage, error) {
	return repository.ListActiveUsages(userID)
}

func (mysqlStore) FindUsageForUser(usageID int64, userID identityservice.UserID) (model.MaterialUsage, bool, error) {
	return repository.FindUsageForUser(usageID, userID)
}

func (mysqlStore) RemoveUsageByID(usageID int64, userID identityservice.UserID, now time.Time) (bool, error) {
	return repository.RemoveUsageByID(usageID, userID, now)
}
