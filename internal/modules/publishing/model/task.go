package model

import (
	"encoding/json"
	"time"
)

type Content struct {
	Description string `json:"description"`
	CoverFileID int64  `json:"cover_file_id,string"`
	Topics      string `json:"topics"`
	Declaration string `json:"declaration"`
	Confirmed   bool   `json:"confirmed"`
}
type Task struct {
	AuditActor         int64           `json:"-" gorm:"-"`
	ID                 uint64          `json:"id,string"`
	TeamID             int64           `json:"team_id,string"`
	OperatorID         int64           `json:"operator_id,string"`
	Platform           string          `json:"platform"`
	CategoryID         int64           `json:"category_id,string"`
	MediaAccountID     uint64          `json:"media_account_id,string"`
	CompositeOutputID  int64           `json:"composite_output_id,string"`
	PlatformAccountUID string          `json:"platform_account_uid"`
	CoverFileID        int64           `json:"-"`
	ResourceSnapshot   json.RawMessage `json:"resources" gorm:"column:resource_snapshot_json"`
	Content            Content         `json:"content" gorm:"column:content_json;serializer:json"`
	Status             string          `json:"status"`
	CurrentAttemptID   *uint64         `json:"current_attempt_id,string"`
	AttemptCount       uint32          `json:"attempt_count"`
	NotBefore          *time.Time      `json:"not_before"`
	AuthorizedAt       *time.Time      `json:"authorized_at"`
	Revision           uint64          `json:"revision,string"`
	CreatedBy          int64           `json:"created_by,string"`
	RequestKey         string          `json:"request_key"`
	RequestHash        string          `json:"-"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func (Task) TableName() string { return "publication_task" }
