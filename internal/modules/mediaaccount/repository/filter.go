package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// AccountFilter is the persistence projection of media-account query criteria.
type AccountFilter struct {
	View                string
	TeamID              identity.TeamID
	CategoryType        string
	Offset              int
	Limit               int
	UserID              identity.UserID
	CategoryIDs         []identity.CategoryID
	Platform            model.Platform
	Enabled             *bool
	PlatformLoginStatus model.PlatformLoginStatus
	Search              string
	ProfileSearch       string
	AnyTags             []string
	AllTags             []string
	ExcludeTags         []string
}
