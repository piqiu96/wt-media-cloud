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
	RequestKey      string            `json:"-" gorm:"column:request_key"`
	CreatedBy       int64             `json:"created_by"`
	ClaimedNodeID   *int64            `json:"-" gorm:"column:claimed_node_id"`
	LeaseExpiresAt  *time.Time        `json:"-" gorm:"column:lease_expires_at"`
	HeartbeatAt     *time.Time        `json:"-" gorm:"column:heartbeat_at"`
	AttemptCount    int               `json:"attempt_count" gorm:"column:attempt_count"`
	MaxAttempts     int               `json:"-" gorm:"column:max_attempts"`
	StartedAt       *time.Time        `json:"-" gorm:"column:started_at"`
	FinishedAt      *time.Time        `json:"-" gorm:"column:finished_at"`
	Stage           string            `json:"stage"`
	Progress        int               `json:"progress"`
	ErrorCode       string            `json:"error_code"`
	ErrorMessage    string            `json:"error_message"`
	OutputObjectKey string            `json:"-" gorm:"column:output_object_key"`
	CompositeOutput *int64            `json:"-" gorm:"column:composite_output_id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

func (ComposeTask) TableName() string { return "compose_tasks" }
