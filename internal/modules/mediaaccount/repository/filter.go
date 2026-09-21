package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// AccountFilter is the persistence projection of media-account query criteria.
type AccountFilter struct {
	UserID         identity.UserID
	GameIDs        []string
	Platform       model.Platform
	BusinessStatus model.BusinessStatus
	LoginStatus    model.LoginStatus
	Search         string
	ProfileSearch  string
	AnyTags        []string
	AllTags        []string
	ExcludeTags    []string
}
