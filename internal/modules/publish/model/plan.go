package model

import (
	"encoding/json"
	"time"
)

type PlanGoal struct {
	Platform    string `json:"platform"`
	CategoryID  int64  `json:"category_id,string"`
	TargetCount int64  `json:"target_count"`
}
type PlanStructureBatch struct {
	BatchID    *uint64       `json:"batch_id,string,omitempty"`
	RowKey     string        `json:"row_key"`
	Name       string        `json:"name"`
	Platform   string        `json:"platform"`
	GroupOrder uint32        `json:"group_order"`
	BatchOrder uint32        `json:"batch_order"`
	Targets    []BatchTarget `json:"targets"`
}
type Plan struct {
	ID               uint64          `json:"id,string"`
	TeamID           int64           `json:"team_id,string"`
	OwnerID          int64           `json:"owner_id,string"`
	CreatedBy        int64           `json:"created_by,string"`
	Name             string          `json:"name"`
	DateStart        string          `json:"date_start"`
	DateEnd          string          `json:"date_end"`
	Note             string          `json:"note"`
	Goals            []PlanGoal      `json:"goals" gorm:"column:goals_json;serializer:json"`
	Status           string          `json:"status"`
	PauseReason      string          `json:"pause_reason"`
	CurrentBatchID   *uint64         `json:"current_batch_id,string"`
	StartedAt        *time.Time      `json:"started_at"`
	StoppedAt        *time.Time      `json:"stopped_at"`
	EndedAt          *time.Time      `json:"ended_at"`
	EndReason        string          `json:"end_reason"`
	EndNote          string          `json:"end_note"`
	Revision         uint64          `json:"revision,string"`
	RequestKey       string          `json:"-"`
	RequestHash      string          `json:"-"`
	StartRequestKey  *string         `json:"-"`
	StartRequestHash *string         `json:"-"`
	StartReceipt     json.RawMessage `json:"-" gorm:"column:start_receipt_json"`
	HandoffReceipts  json.RawMessage `json:"-" gorm:"column:handoff_receipts_json"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func (Plan) TableName() string { return "publication_plan" }

type PlanBatchConfirmation struct {
	BatchID            uint64              `json:"batch_id,string"`
	ExpectedRevision   uint64              `json:"expected_revision,string"`
	IncludedRowKeys    []string            `json:"included_row_keys"`
	ExcludedRowKeys    []string            `json:"excluded_row_keys"`
	AllowShortfall     bool                `json:"allow_shortfall"`
	ExpectedShortfalls []ExpectedShortfall `json:"expected_shortfalls"`
}
type LatestManualVerification struct {
	VerifiedAt   time.Time `json:"verified_at"`
	Conclusion   Status    `json:"conclusion"`
	AttemptID    uint64    `json:"attempt_id,string"`
	TaskRevision uint64    `json:"task_revision,string"`
}
