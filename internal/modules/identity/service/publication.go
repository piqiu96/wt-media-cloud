package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/repository"
	"gorm.io/gorm"
)

func CheckPublicationScope(tx *gorm.DB, owner, team, category int64) (bool, error) {
	return repository.CheckPublicationScope(tx, owner, team, category)
}
