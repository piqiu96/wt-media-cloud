package service

import (
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"gorm.io/gorm"
)

func CheckArtworkFileReferences(tx *gorm.DB, id int64) error {
	used, err := repository.ArtworkFileReferenced(tx, id)
	if err != nil {
		return err
	}
	if used {
		return files.ErrInUse
	}
	return nil
}
func ListArtworkFileReference(a identity.PublicUser, id int64) ([]model.Reference, error) {
	sc, err := imageAssetScope(a)
	if err != nil {
		return nil, err
	}
	return repository.ListArtworkFileReference(database.DB(), sc, id)
}
