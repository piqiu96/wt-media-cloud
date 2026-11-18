package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/repository"
	"gorm.io/gorm"
)

func LockPublicationProfile(tx *gorm.DB, id string, owner int64) (model.BrowserProfile, error) {
	return repository.LockPublicationProfile(tx, id, owner)
}

func LockPublicationScene(tx *gorm.DB, id string, owner int64) error {
	return repository.LockPublicationScene(tx, id, owner)
}
