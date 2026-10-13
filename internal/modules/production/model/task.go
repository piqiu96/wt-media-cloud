package model

import (
	"encoding/json"
	"time"
)

type ComposeTaskStatus string

const (
	ComposeTaskPending   ComposeTaskStatus = "pending"
	ComposeTaskRunning   ComposeTaskStatus = "running"
	ComposeTaskSuccess   ComposeTaskStatus = "success"
	ComposeTaskFailed    ComposeTaskStatus = "failed"
	ComposeTaskCancelled ComposeTaskStatus = "cancelled"
)

// ComposeTask is one production run (engineering spec §3.4). The strategy
// binding columns — StrategyID, TemplateCode, TemplateVersion and
// ConfigSnapshot — are creation facts (ADR-0021): later strategy edits and
// template versions never reach an existing row, and the M4-C5 executor reads
// only the snapshot. Claim/lease/heartbeat columns exist from day one but only
// the worker lifecycle (M4-C5) writes them.
type ComposeTask struct {
	ID              int64             `json:"id"`
	TeamID          int64             `json:"team_id"`
	GameID          string            `json:"game_id"`
	TriggerType     string            `json:"trigger_type"`
	MaterialID      int64             `json:"material_id"`
	MaterialUsageID int64             `json:"material_usage_id" gorm:"column:material_usage_id"`
	StrategyID      int64             `json:"compose_strategy_id" gorm:"column:compose_strategy_id"`
	TemplateCode    string            `json:"template_code" gorm:"column:template_code"`
	TemplateVersion uint32            `json:"template_version" gorm:"column:template_version"`
	ConfigSnapshot  json.RawMessage   `json:"config_snapshot" gorm:"column:config_snapshot_json;type:json"`
	Status          ComposeTaskStatus `json:"status"`
	CancelRequested bool              `json:"cancel_requested" gorm:"column:cancel_requested"`
	RequestKey      string            `json:"-" gorm:"column:request_key"`
	CreatedBy       int64             `json:"created_by"`
	ClaimedNodeID   *int64            `json:"-" gorm:"column:claimed_node_id"`
	LeaseExpiresAt  *time.Time        `json:"-" gorm:"column:lease_expires_at"`
	HeartbeatAt     *time.Time        `json:"-" gorm:"column:heartbeat_at"`
	AttemptCount    int               `json:"attempt_count" gorm:"column:attempt_count"`
	MaxAttempts     int               `json:"-" gorm:"column:max_attempts"`
	StartedAt       *time.Time        `json:"started_at" gorm:"column:started_at"`
	FinishedAt      *time.Time        `json:"finished_at" gorm:"column:finished_at"`
	Stage           string            `json:"stage"`
	Progress        int               `json:"progress"`
	ErrorCode       string            `json:"error_code"`
	ErrorMessage    string            `json:"error_message"`
	OutputObjectKey string            `json:"-" gorm:"column:output_object_key"`
	CompositeOutput *int64            `json:"composite_output_id" gorm:"column:composite_output_id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

func (ComposeTask) TableName() string { return "compose_tasks" }

// ComposeTaskAttempt is one execution attempt of a compose task (M4-G). A
// manual retry opens its row at authorization time — source 'manual',
// outcome 'pending' — and the worker's claim transaction stamps started_at;
// the terminal paths (success, failure, requeue, cancel) close it. The
// (task_id, attempt) unique key lets the claim INSERT rely on absence.
type ComposeTaskAttempt struct {
	ID           int64      `json:"id"`
	TaskID       int64      `json:"compose_task_id" gorm:"column:task_id"`
	Attempt      int        `json:"attempt" gorm:"column:attempt"`
	Source       string     `json:"source" gorm:"column:source"`
	StartedAt    *time.Time `json:"started_at" gorm:"column:started_at"`
	FinishedAt   *time.Time `json:"finished_at" gorm:"column:finished_at"`
	Outcome      string     `json:"outcome" gorm:"column:outcome"`
	Stage        string     `json:"stage" gorm:"column:stage"`
	ErrorCode    string     `json:"error_code" gorm:"column:error_code"`
	ErrorMessage string     `json:"error_message" gorm:"column:error_message"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"column:updated_at"`
}

func (ComposeTaskAttempt) TableName() string { return "compose_task_attempts" }

// CompositeOutput is the verified-output asset (engineering spec §3.5). The
// worker writes it inside the same transaction that marks the task success —
// a row here always has a verified object behind it. Publish facts live in
// the M6 publication domain; PublishStatus is only this row's projection.
type CompositeOutput struct {
	ID              int64           `json:"id"`
	TeamID          int64           `json:"team_id"`
	GameID          string          `json:"game_id"`
	MaterialID      int64           `json:"material_id"`
	MaterialUsageID int64           `json:"material_usage_id" gorm:"column:material_usage_id"`
	TaskID          int64           `json:"compose_task_id" gorm:"column:compose_task_id"`
	StrategyID      int64           `json:"compose_strategy_id" gorm:"column:compose_strategy_id"`
	TemplateCode    string          `json:"template_code" gorm:"column:template_code"`
	TemplateVersion uint32          `json:"template_version" gorm:"column:template_version"`
	ObjectKey       string          `json:"-" gorm:"column:object_key"`
	FileSizeBytes   int64           `json:"file_size_bytes" gorm:"column:file_size_bytes"`
	SHA256          string          `json:"sha256" gorm:"column:sha256"`
	Media           json.RawMessage `json:"media" gorm:"column:media_json;type:json"`
	PublishStatus   string          `json:"publish_status" gorm:"column:publish_status"`
	CreatedBy       int64           `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (CompositeOutput) TableName() string { return "composite_outputs" }

// ComposeOutputCommit reports the ids written by the success transaction.
type ComposeOutputCommit struct {
	OutputID int64
	TaskID   int64
}
