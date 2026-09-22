// Package model owns content-pool and discovery domain values.
package model

import (
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type Status string

const (
	StatusPending         Status = "pending"
	StatusMaterialCreated Status = "material_created"
	StatusIgnored         Status = "ignored"
)

type SourceContent struct {
	ID                int64                 `json:"id"`
	TeamID            sharedidentity.TeamID `json:"team_id"`
	Platform          string                `json:"platform"`
	PlatformContentID string                `json:"platform_content_id"`
	Title             string                `json:"title"`
	Description       string                `json:"description,omitempty"`
	CoverURL          string                `json:"cover_url,omitempty"`
	SourceURL         string                `json:"source_url,omitempty"`
	AuthorID          string                `json:"author_id,omitempty"`
	AuthorName        string                `json:"author_name,omitempty"`
	SourceType        string                `json:"source_type"`
	StrategyID        *int64                `json:"strategy_id,omitempty"`
	CrawlTaskID       *int64                `json:"crawl_task_id,omitempty"`
	LikeCount         int64                 `json:"like_count"`
	FavoriteCount     int64                 `json:"favorite_count"`
	PublishedAt       *time.Time            `json:"published_at,omitempty"`
	Status            Status                `json:"status"`
	IgnoredReason     string                `json:"ignored_reason,omitempty"`
	AuditNote         string                `json:"audit_note,omitempty"`
	FailureReason     string                `json:"failure_reason,omitempty"`
	MaterialID        *int64                `json:"material_id,omitempty"`
	CreatedBy         sharedidentity.UserID `json:"created_by"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type SourceContentView struct {
	SourceContent
	StrategyName  string `json:"strategy_name,omitempty"`
	CrawlTaskName string `json:"crawl_task_name,omitempty"`
}

type Material struct {
	ID              int64                 `json:"id"`
	TeamID          sharedidentity.TeamID `json:"team_id"`
	SourceContentID int64                 `json:"source_content_id"`
	Title           string                `json:"title"`
	SourceSnapshot  map[string]any        `json:"source_snapshot"`
	CreatedBy       sharedidentity.UserID `json:"created_by"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type StrategyStatus string

const (
	StrategyEnabled  StrategyStatus = "enabled"
	StrategyDisabled StrategyStatus = "disabled"
)

type DiscoveryStrategy struct {
	ID           int64                 `json:"id"`
	TeamID       sharedidentity.TeamID `json:"team_id"`
	Name         string                `json:"name"`
	StrategyType string                `json:"strategy_type"`
	Platform     string                `json:"platform"`
	Config       map[string]any        `json:"config"`
	Schedule     string                `json:"schedule"`
	Timezone     string                `json:"timezone"`
	Status       StrategyStatus        `json:"status"`
	CreatedBy    sharedidentity.UserID `json:"created_by"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type CrawlStatus string

const (
	CrawlPending        CrawlStatus = "pending"
	CrawlRunning        CrawlStatus = "running"
	CrawlSuccess        CrawlStatus = "success"
	CrawlPartialSuccess CrawlStatus = "partial_success"
	CrawlFailed         CrawlStatus = "failed"
)

type CrawlTask struct {
	ID           int64                 `json:"id"`
	TeamID       sharedidentity.TeamID `json:"team_id"`
	StrategyID   *int64                `json:"strategy_id,omitempty"`
	ParentTaskID *int64                `json:"parent_task_id,omitempty"`
	ScheduleKey  string                `json:"schedule_key,omitempty"`
	TaskID       string                `json:"task_id,omitempty"`
	TaskType     string                `json:"task_type"`
	Platform     string                `json:"platform"`
	Status       CrawlStatus           `json:"status"`
	Snapshot     map[string]any        `json:"snapshot"`
	Stats        CrawlStats            `json:"stats"`
	Results      []map[string]any      `json:"results,omitempty"`
	Error        string                `json:"error,omitempty"`
	StartedAt    *time.Time            `json:"started_at,omitempty"`
	FinishedAt   *time.Time            `json:"finished_at,omitempty"`
	CreatedBy    sharedidentity.UserID `json:"created_by"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type CrawlStats struct {
	Scanned          int `json:"scanned"`
	Found            int `json:"found"`
	Added            int `json:"added"`
	Duplicate        int `json:"duplicate"`
	Failed           int `json:"failed"`
	AutoMaterialized int `json:"auto_materialized"`
	Pending          int `json:"pending"`
}
