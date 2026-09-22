// Package dto owns content-pool request and query contracts.
package dto

import (
	"encoding/json"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type SourceInput struct {
	TeamID            *sharedidentity.TeamID
	GameID            *string
	Platform          string
	PlatformContentID string
	Title             string
	Description       string
	CoverURL          string
	SourceURL         string
	AuthorID          string
	AuthorSecUID      string
	AuthorUID         string
	AuthorHomeURL     string
	AuthorName        string
	SourceType        string
	StrategyID        *int64
	CrawlTaskID       *int64
	LikeCount         int64
	FavoriteCount     int64
	ViewCount         int64
	CommentCount      int64
	ShareCount        int64
	PublishedAt       *time.Time
	AuditNote         string
	RawJSON           json.RawMessage
}

type Filter struct {
	TeamID      *sharedidentity.TeamID
	Platform    string
	Status      model.Status
	SourceType  string
	Search      string
	StrategyID  *int64
	CrawlTaskID *int64
	MaterialID  *int64
}

type CrawlerRequest struct {
	Platform  string
	Operation string
	Config    map[string]any
}

type CrawlerResult struct {
	Items    []map[string]any
	Failures []map[string]any
	Scanned  int
	Failed   int
}

type SearchInput struct {
	Platform string
	Keyword  string
	Query    string
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
	PlatformContentID string          `json:"platform_content_id"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	CoverURL          string          `json:"cover_url"`
	SourceURL         string          `json:"source_url"`
	AuthorID          string          `json:"author_id"`
	AuthorSecUID      string          `json:"author_sec_uid,omitempty"`
	AuthorUID         string          `json:"author_uid,omitempty"`
	AuthorHomeURL     string          `json:"author_home_url,omitempty"`
	AuthorName        string          `json:"author_name"`
	LikeCount         int64           `json:"like_count"`
	FavoriteCount     int64           `json:"favorite_count"`
	ViewCount         int64           `json:"view_count"`
	CommentCount      int64           `json:"comment_count"`
	ShareCount        int64           `json:"share_count"`
	PublishedAt       *time.Time      `json:"published_at,omitempty"`
	Raw               json.RawMessage `json:"raw,omitempty"`
}

type SearchResponse struct {
	Items      []SearchResult `json:"items"`
	NextOffset int64          `json:"next_offset,omitempty"`
	MaxCursor  int64          `json:"max_cursor,omitempty"`
	HasMore    bool           `json:"has_more,omitempty"`
}

type ImportResultsRequest struct {
	TeamID     *sharedidentity.TeamID `json:"team_id,omitempty"`
	GameID     *string                `json:"game_id,omitempty"`
	Platform   string                 `json:"platform"`
	SourceType string                 `json:"source_type,omitempty"`
	Items      []SearchResult         `json:"items"`
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

type BatchOperationItem struct {
	ID       int64                `json:"id"`
	Success  bool                 `json:"success"`
	Message  string               `json:"message,omitempty"`
	Source   *model.SourceContent `json:"source,omitempty"`
	Material *model.Material      `json:"material,omitempty"`
}

type BatchOperationResponse struct {
	Succeeded int                  `json:"succeeded"`
	Failed    int                  `json:"failed"`
	Items     []BatchOperationItem `json:"items"`
}
