package model

import "time"

type Profile struct {
	ClipSeconds float64 `json:"clip_seconds"`
	MaxClips    int     `json:"max_clips"`
	CRF         int     `json:"crf"`
	Preset      string  `json:"preset"`
}
type Clip struct {
	SourceID string  `json:"source_id"`
	Title    string  `json:"title"`
	Location string  `json:"location"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
	SHA256   string  `json:"sha256"`
}
type Index struct {
	Version int    `json:"version"`
	Clips   []Clip `json:"clips"`
}
type Source struct {
	SourceID      string `gorm:"primaryKey"`
	SHA256        string
	OriginalTitle string
	Basename      string
	ProfileJSON   string
	ClipsJSON     *string
	ReviewReason  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Source) TableName() string { return "slice_library_sources" }
