package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	"gorm.io/gorm"
)

func LockSavedPublicationFile(tx *gorm.DB, id int64) (model.File, error) {
	if e := LockAvailable(tx, []int64{id}); e != nil {
		return model.File{}, e
	}
	return (repository.Store{DB: tx}).Find(id)
}
