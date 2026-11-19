package service

import (
	"context"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"gorm.io/gorm"
)

func LockSavedPublicationOutput(tx *gorm.DB, id int64) (model.CompositeOutput, bool, error) {
	return repository.LockSavedPublicationOutput(tx, id)
}

// Only a business service that has authorized its saved reference may use this
// grant. It deliberately does not grant another user's general output access.
func SavedPublicationOutputURL(ctx context.Context, id int64, sha string, size int64) (string, error) {
	s := wiredComposeOutputService()
	o, found, e := s.outputs.Find(id)
	if e != nil {
		return "", e
	}
	if !found || o.PublishStatus == model.OutputPublishArchived || o.SHA256 != sha || o.FileSizeBytes != size {
		return "", ErrComposeOutputUnavailable
	}
	if e = s.requireObject(ctx, o); e != nil {
		return "", e
	}
	return s.links.PresignObjectURL(ctx, o.ObjectKey)
}
