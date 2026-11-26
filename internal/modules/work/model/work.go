package model

import "time"

type Work struct {
	Metrics            *Metrics  `json:"metrics" gorm:"foreignKey:WorkID;references:ID"`
	AccountName        string    `json:"account_name"`
	CoverResourcePath  string    `json:"cover_resource_path"`
	ID                 uint64    `json:"id,string"`
	Platform           string    `json:"platform"`
	PlatformWorkID     string    `json:"platform_work_id"`
	WorkURL            string    `json:"work_url"`
	PlatformAccountUID string    `json:"platform_account_uid"`
	MediaAccountID     uint64    `json:"media_account_id,string"`
	OperatorID         int64     `json:"operator_id,string"`
	TeamID             int64     `json:"team_id,string"`
	CategoryID         int64     `json:"category_id,string"`
	PublicationID      uint64    `json:"publication_id,string"`
	AttemptID          uint64    `json:"attempt_id,string"`
	CompositeOutputID  int64     `json:"composite_output_id,string"`
	Title              string    `json:"title"`
	CoverFileID        int64     `json:"cover_file_id,string"`
	VideoSHA256        string    `json:"-"`
	PublishedAt        time.Time `json:"published_at"`
	FirstObservedAt    time.Time `json:"first_observed_at"`
}

func (Work) TableName() string { return "work" }
