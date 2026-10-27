package repository

import sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"

// ProxyFilter is the persistence projection of proxy query criteria.
type ProxyFilter struct {
	TeamID         *sharedidentity.TeamID
	Platform       string
	Supplier       string
	BusinessStatus string
	Region         string
	Search         string
	Limit          int
	Offset         int
}
