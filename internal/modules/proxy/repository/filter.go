package repository

import sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"

// ProxyFilter is the persistence projection of proxy query criteria.
type ProxyFilter struct {
	TeamID     *sharedidentity.TeamID
	TeamSearch string
	Platform   string
	Supplier   string
	Enabled    *bool
	Region     string
	Search     string
	Limit      int
	Offset     int
}
