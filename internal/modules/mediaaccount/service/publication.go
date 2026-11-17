package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/repository"
	"gorm.io/gorm"
)

type PublicationBinding = repository.PublicationBinding

func CheckPublicationBinding(tx *gorm.DB, want PublicationBinding) (bool, error) {
	return repository.CheckPublicationBinding(tx, want)
}
