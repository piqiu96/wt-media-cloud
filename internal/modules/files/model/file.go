package model

import "time"

type File struct {
	ID            int64
	OwnerID       int64
	TeamID        *int64
	Name          string
	Extension     string
	Type          string
	ContentType   string
	ObjectKey     string
	SizeBytes     int64
	SHA256        string
	ResourceToken string
	UploadedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	OwnerName     string `gorm:"->"`
	TeamName      string `gorm:"->"`
}

func (File) TableName() string { return "operations_files" }

type Scope struct {
	All             bool
	OwnerID, TeamID int64
}

type View struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Filename     string    `json:"filename"`
	Type         string    `json:"type"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	SHA256       string    `json:"sha256"`
	OwnerID      int64     `json:"owner_id"`
	OwnerName    string    `json:"owner_name"`
	TeamID       *int64    `json:"team_id"`
	TeamName     string    `json:"team_name"`
	CreatedAt    time.Time `json:"created_at"`
	ResourcePath string    `json:"resource_path"`
}
