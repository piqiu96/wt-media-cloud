package service

import (
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"gorm.io/gorm"
	"strconv"
)

func CheckOutputFileReferences(tx *gorm.DB, id int64) error {
	used, err := repository.OutputFileReferenced(tx, id)
	if err != nil {
		return err
	}
	if used {
		return files.ErrInUse
	}
	return nil
}

func ListOutputFileReference(actor identity.PublicUser, id int64) ([]string, error) {
	s := wiredComposeOutputService()
	filter, empty, err := s.outputFilter(actor, ListComposeOutputsInput{})
	if err != nil {
		return nil, err
	}
	if empty {
		return []string{}, nil
	}
	ids, err := repository.OutputFileReferenceIDs(database.DB(), filter, id)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out, nil
}
