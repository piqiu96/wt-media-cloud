package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// Filter is the persistence projection of content-source query criteria.
type Filter struct {
	ID            *int64
	TeamID        *identity.TeamID
	CategoryIDs   []identity.CategoryID
	Platform      string
	Status        model.Status
	SourceType    string
	Search        string
	PublishedFrom string
	PublishedTo   string
	SortBy        string
	SortOrder     string
	StrategyID    *int64
	CrawlTaskID   *int64
	MaterialID    *int64
}
