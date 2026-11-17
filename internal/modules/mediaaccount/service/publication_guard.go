package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/repository"
	"gorm.io/gorm"
)

func SetPublicationGuard(check func(*gorm.DB, string) (bool, error)) {
	repository.SetPublicationGuard(check)
}
