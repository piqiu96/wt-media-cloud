// Package dto owns content-pool request and query contracts.
package dto

import (
	"encoding/json"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

type SourceInput struct {
	TeamID            *identitymodel.TeamID
	Platform          string
	PlatformContentID string
	Title             string
	Description       string
	CoverURL          string
	SourceURL         string
	AuthorID          string
	AuthorName        string
	SourceType        string
	PublishedAt       *time.Time
	RawJSON           json.RawMessage
}

type Filter struct {
	TeamID     *identitymodel.TeamID
	Platform   string
	Status     model.Status
	SourceType string
	Search     string
}

type CrawlerRequest struct {
	Platform  string
	Operation string
	Config    map[string]any
}

type CrawlerResult struct {
	Items   []map[string]any
	Scanned int
	Failed  int
}
