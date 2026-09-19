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

type SearchInput struct {
	Platform string
	Keyword  string
	Limit    int
	Offset   int
}

type AuthorSearchInput struct {
	Platform  string
	Author    string
	Limit     int
	MaxCursor int64
}

type SearchResult struct {
	PlatformContentID string     `json:"platform_content_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	CoverURL          string     `json:"cover_url"`
	SourceURL         string     `json:"source_url"`
	AuthorID          string     `json:"author_id"`
	AuthorName        string     `json:"author_name"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
}

type SearchResponse struct {
	Items      []SearchResult `json:"items"`
	NextOffset int64          `json:"next_offset,omitempty"`
	MaxCursor  int64          `json:"max_cursor,omitempty"`
	HasMore    bool           `json:"has_more,omitempty"`
}

type ImportResultsRequest struct {
	TeamID     *identitymodel.TeamID `json:"team_id,omitempty"`
	Platform   string                `json:"platform"`
	SourceType string                `json:"source_type,omitempty"`
	Items      []SearchResult        `json:"items"`
}

type ImportItemResult struct {
	PlatformContentID string `json:"platform_content_id"`
	Status            string `json:"status"`
	SourceID          int64  `json:"source_id,omitempty"`
	Message           string `json:"message,omitempty"`
}

type ImportResultsResponse struct {
	Imported  int                `json:"imported"`
	Duplicate int                `json:"duplicate"`
	Failed    int                `json:"failed"`
	Items     []ImportItemResult `json:"items"`
}
