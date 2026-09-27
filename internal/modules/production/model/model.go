// Package model owns the persisted facts for Cloud content production.
package model

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type VideoStatus string

const (
	VideoNotDownloaded VideoStatus = "not_downloaded"
	VideoDownloading   VideoStatus = "downloading"
	VideoReady         VideoStatus = "ready"
	VideoFailed        VideoStatus = "failed"
)

type MaterialUsageStatus string

const (
	MaterialUsageActive  MaterialUsageStatus = "active"
	MaterialUsageRemoved MaterialUsageStatus = "removed"
)

// Material is the production projection of an M3 source material. It contains
// a Cloud-owned readiness projection, never a user-local file location.
type Material struct {
	ID              int64           `json:"id"`
	TeamID          identity.TeamID `json:"team_id"`
	GameID          *string         `json:"game_id,omitempty"`
	SourceContentID int64           `json:"source_content_id"`
	// PlatformContentID is the provider's own id for this video, carried so the
	// worker can ask the provider for the source by the id it knows. It is not
	// SourceContentID: that is this database's row id in `source_contents`, and a
	// provider asked for it answers that no such video exists. Hidden from JSON
	// because it is worker plumbing, not part of the material the UI reads.
	PlatformContentID string `json:"-"`
	Title             string `json:"title"`
	SourceURL       string          `json:"source_url"`
	Platform        string          `json:"platform"`
	AuthorName      string          `json:"author_name,omitempty"`
	PublishedAt     *time.Time      `json:"published_at,omitempty"`
	VideoStatus     VideoStatus     `json:"video_status"`
	SourceObjectKey string          `json:"-"`
	VideoSizeBytes  *int64          `json:"video_size_bytes,omitempty"`
	VideoSHA256     string          `json:"video_sha256,omitempty"`
	VideoMedia      map[string]any  `json:"media_summary,omitempty"`
	VideoError      string          `json:"last_error,omitempty"`
	VideoPreparedAt *time.Time      `json:"video_prepared_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// MaterialUsage is the sole durable relationship behind “My Materials”.
// Removal retains the same record for traceability and later restoration.
type MaterialUsage struct {
	ID         int64               `json:"id"`
	TeamID     identity.TeamID     `json:"team_id"`
	MaterialID int64               `json:"material_id"`
	UserID     identity.UserID     `json:"user_id"`
	Status     MaterialUsageStatus `json:"status"`
	RemovedAt  *time.Time          `json:"removed_at,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
	Material   *Material           `json:"material,omitempty"`
}
