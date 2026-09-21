package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// Filter is the persistence projection of content-source query criteria.
type Filter struct {
	TeamID      *identity.TeamID
	Platform    string
	Status      model.Status
	SourceType  string
	Search      string
	StrategyID  *int64
	CrawlTaskID *int64
}
