package model

import (
	"encoding/json"
	"time"
)

type ImageAsset struct {
	ID                uint64          `json:"id,string"`
	TeamID            int64           `json:"team_id,string"`
	FileID            int64           `json:"file_id,string"`
	Name              string          `json:"name"`
	AllowAsCover      bool            `json:"allow_as_cover"`
	AllowAsBackground bool            `json:"allow_as_background"`
	CategoryIDs       []string        `json:"category_ids" gorm:"column:category_ids_json;serializer:json"`
	Enabled           bool            `json:"enabled"`
	Revision          uint64          `json:"revision,string"`
	CreatedBy         int64           `json:"created_by,string"`
	UpdatedBy         int64           `json:"updated_by,string"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	MetadataJSON      json.RawMessage `json:"-" gorm:"->"`
	ResourceToken     string          `json:"-" gorm:"->"`
	SizeBytes         int64           `json:"size_bytes" gorm:"->"`
	Width             int             `json:"width" gorm:"-"`
	Height            int             `json:"height" gorm:"-"`
	Format            string          `json:"format" gorm:"-"`
	ResourcePath      string          `json:"resource_path" gorm:"-"`
}

func (ImageAsset) TableName() string { return "image_asset" }

type Scope struct {
	All             bool
	OwnerID, TeamID int64
	CategoryIDs     []string
}
