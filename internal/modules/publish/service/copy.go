package service

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

func (s *RecoveryService) Copy(actor identity.PublicUser, id, revision uint64) (model.PublicationCopy, error) {
	v, err := s.readable(actor, id)
	if err != nil {
		return model.PublicationCopy{}, err
	}
	if revision == 0 {
		return model.PublicationCopy{}, ErrInvalid
	}
	if v.Revision != revision {
		return model.PublicationCopy{}, &ConflictError{TaskID: v.ID, Revision: v.Revision}
	}
	switch v.Status {
	case model.StatusPublished, model.StatusCancelled, model.StatusAbandoned:
	case model.StatusFailed:
		if v.CurrentAttemptID == nil {
			return model.PublicationCopy{}, ErrConflict
		}
		original, err := s.Repo.OriginalAttempt(v.ID, *v.CurrentAttemptID, v.OperatorID)
		if err != nil {
			return model.PublicationCopy{}, err
		}
		if original.AttemptID != *v.CurrentAttemptID || !original.SceneResolved || original.ResolvedStatus != model.StatusFailed {
			return model.PublicationCopy{}, ErrConflict
		}
	default:
		return model.PublicationCopy{}, ErrConflict
	}
	return model.PublicationCopy{Platform: v.Platform, CategoryID: v.CategoryID, Content: model.CopyContent{Description: v.Content.Description, Topics: v.Content.Topics, Declaration: v.Content.Declaration}}, nil
}
