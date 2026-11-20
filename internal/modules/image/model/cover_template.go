package model

import (
	"encoding/json"
	"time"
)

type CoverTemplate struct {
	ID                       uint64          `json:"id,string"`
	TeamID                   int64           `json:"team_id,string"`
	Name                     string          `json:"name"`
	CanvasWidth              int             `json:"canvas_width"`
	CanvasHeight             int             `json:"canvas_height"`
	Scope                    string          `json:"scope"`
	PlatformCodes            []string        `json:"platform_codes" gorm:"column:platform_codes_json;serializer:json"`
	CategoryIDs              []string        `json:"category_ids" gorm:"column:category_ids_json;serializer:json"`
	DraftConfig              json.RawMessage `json:"draft_config" gorm:"column:draft_config_json"`
	PublishedConfig          json.RawMessage `json:"published_config" gorm:"column:published_config_json"`
	PublishedRevision        uint32          `json:"published_revision"`
	PublishedThumbnailFileID *int64          `json:"published_thumbnail_file_id,string"`
	ThumbnailResourcePath    string          `json:"thumbnail_resource_path" gorm:"-"`
	Enabled                  bool            `json:"enabled"`
	RequestKey               string          `json:"request_key"`
	RequestHash              string          `json:"-"`
	Revision                 uint64          `json:"revision,string"`
	CreatedBy                int64           `json:"created_by,string"`
	UpdatedBy                int64           `json:"updated_by,string"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

func (CoverTemplate) TableName() string { return "cover_template" }
