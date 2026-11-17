package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"gorm.io/gorm"
)

func OperationsFileReferenced(tx *gorm.DB, token string) (bool, error) {
	return repository.OperationsFileReferenced(tx, token)
}
