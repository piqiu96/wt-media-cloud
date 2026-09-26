package service

import (
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func GetMaterial(actor identityservice.PublicUser, materialID int64) (model.Material, error) {
	return NewService(mysqlStore{}).GetMaterial(actor, materialID)
}

func AddUsage(actor identityservice.PublicUser, materialID int64) (model.MaterialUsage, error) {
	return NewService(mysqlStore{}).AddUsage(actor, materialID)
}

func ListMaterials(actor identityservice.PublicUser, search string) ([]model.Material, error) {
	return NewService(mysqlStore{}).ListMaterials(actor, search)
}

func ListMyMaterials(actor identityservice.PublicUser) ([]model.MaterialUsage, error) {
	return NewService(mysqlStore{}).ListMyMaterials(actor)
}

func RemoveUsage(actor identityservice.PublicUser, usageID int64) error {
	return NewService(mysqlStore{}).RemoveUsage(actor, usageID)
}
