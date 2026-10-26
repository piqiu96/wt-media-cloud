package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ComposeTaskDisplayName captures a short, immutable label at creation time.
// Legacy rows remain blank and are displayed with their task ID by clients.
// The material fallback is the bare id: `#` prefixes belong to no identifier
// in this product, and this string is what operators copy elsewhere.
func ComposeTaskDisplayName(materialTitle string, materialID int64, strategyName string) string {
	title := strings.TrimSpace(materialTitle)
	if title == "" {
		title = fmt.Sprintf("素材 %d", materialID)
	}
	name := strings.TrimSpace(strategyName)
	if name == "" {
		name = "合成策略"
	}
	shorten := func(value string, limit int) string {
		runes := []rune(value)
		if len(runes) > limit {
			return string(runes[:limit]) + "…"
		}
		return value
	}
	return shorten(title, 100) + " · " + shorten(name, 100)
}

// ComposeTaskBatchDisplayName applies an operator-supplied batch label when the
// command carries one: several materials are numbered in submission order, a
// single material keeps the plain name. A blank (or whitespace-only) label
// keeps the derived material/strategy name, so every existing caller is
// unchanged. The rule lives here and only here — the create drawer, the
// schedule path and any future client have to produce the same names.
func ComposeTaskBatchDisplayName(name string, index, total int, materialTitle string, materialID int64, strategyName string) string {
	label := strings.TrimSpace(name)
	if label == "" {
		return ComposeTaskDisplayName(materialTitle, materialID, strategyName)
	}
	if total <= 1 {
		return label
	}
	return fmt.Sprintf("%s-%d", label, index+1)
}

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
	DisplayName     string            `json:"display_name" gorm:"column:display_name"`
	TeamID          int64             `json:"team_id"`
	GameID          string            `json:"game_id"`
	OperatorID      *int64            `json:"operator_id"`
	OperatorName    string            `json:"operator_name" gorm:"-"`
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
	NextAttemptAt   *time.Time        `json:"-" gorm:"column:next_attempt_at"`
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

// Compose task execution steps. One entry is appended to the attempt row as
// each phase finishes; the detail page reads them back as the execution
// process. The keys are a closed set — an entry whose key is not here is
// dropped before it reaches a client.
const (
	ComposeStepValidateInput = "validate_input"
	ComposeStepPrepareInput  = "prepare_input"
	ComposeStepFetchGuide    = "fetch_guide"
	ComposeStepCut           = "cut"
	ComposeStepUpload        = "upload"
	ComposeStepCommit        = "commit"
)

const (
	ComposeStepOutcomeSuccess = "success"
	ComposeStepOutcomeFailed  = "failed"
)

// ComposeTaskAttemptStep is one recorded step of an execution attempt. The
// stored entry carries only server-authored labels and allowlisted metrics:
// nothing here is copied from an error string, a signed URL, a storage key or
// a local path. Normalize rebuilds the server-owned fields so the write path
// and the API projection use one implementation.
type ComposeTaskAttemptStep struct {
	Step       string                    `json:"step"`
	Label      string                    `json:"label"`
	Outcome    string                    `json:"outcome"`
	StartedAt  time.Time                 `json:"started_at"`
	FinishedAt time.Time                 `json:"finished_at"`
	DurationMS int64                     `json:"duration_ms"`
	ErrorCode  string                    `json:"error_code"`
	ErrorLabel string                    `json:"error_label"`
	Metrics    ComposeAttemptStepMetrics `json:"metrics"`
}

// ComposeAttemptStepMetrics holds counts and digests only: the staged file's
// size, the verified output's sha256, its duration and its row id.
type ComposeAttemptStepMetrics struct {
	SizeBytes        *int64 `json:"size_bytes,omitempty"`
	SHA256           string `json:"sha256,omitempty"`
	OutputDurationMS *int64 `json:"output_duration_ms,omitempty"`
	OutputID         *int64 `json:"output_id,omitempty"`
}

// ComposeTaskAttempt is one execution attempt of a compose task (M4-G). A
// manual retry opens its row at authorization time — source 'manual',
// outcome 'pending' — and the worker's claim transaction stamps started_at;
// the terminal paths (success, failure, requeue, cancel) close it. The
// (task_id, attempt) unique key lets the claim INSERT rely on absence.
//
// Steps is the attempt's process log. It rides in the steps_json column but is
// never written through gorm: the worker appends to it with an atomic
// JSON_ARRAY_APPEND and ListAttempts decodes it by hand, so the field is
// mapped out of gorm's view (`gorm:"-"`) on purpose.
type ComposeTaskAttempt struct {
	ID           int64                    `json:"id"`
	TaskID       int64                    `json:"compose_task_id" gorm:"column:task_id"`
	Attempt      int                      `json:"attempt" gorm:"column:attempt"`
	Source       string                   `json:"source" gorm:"column:source"`
	StartedAt    *time.Time               `json:"started_at" gorm:"column:started_at"`
	FinishedAt   *time.Time               `json:"finished_at" gorm:"column:finished_at"`
	Outcome      string                   `json:"outcome" gorm:"column:outcome"`
	Stage        string                   `json:"stage" gorm:"column:stage"`
	ErrorCode    string                   `json:"error_code" gorm:"column:error_code"`
	ErrorMessage string                   `json:"error_message" gorm:"column:error_message"`
	Steps        []ComposeTaskAttemptStep `json:"steps" gorm:"-"`
	CreatedAt    time.Time                `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time                `json:"updated_at" gorm:"column:updated_at"`
}

func (ComposeTaskAttempt) TableName() string { return "compose_task_attempts" }

// ComposeStagingCleanupCandidate names only an attempt whose execution has
// ended with a known result. The Worker derives the private staging key from
// these internal identifiers; no storage key is exposed through the API.
type ComposeStagingCleanupCandidate struct {
	TaskID  int64
	Attempt int
}

// ComposeUncertainReview records an administrator's explicit verification of
// one uncertain attempt. It is a human declaration, not an automatic claim
// that an expired lease proves the old FFmpeg process has stopped.
type ComposeUncertainReview struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"compose_task_id"`
	Attempt     int       `json:"attempt"`
	ActorUserID int64     `json:"actor_user_id"`
	Evidence    string    `json:"evidence"`
	CreatedAt   time.Time `json:"created_at"`
}

// CompositeOutput is the verified-output asset (engineering spec §3.5). The
// worker writes it inside the same transaction that marks the task success —
// a row here always has a verified object behind it. Publish facts live in
// the M6 publication domain; PublishStatus is only this row's projection.
type CompositeOutput struct {
	ID              int64           `json:"id"`
	TeamID          int64           `json:"team_id"`
	GameID          string          `json:"game_id"`
	OperatorID      *int64          `json:"operator_id"`
	OperatorName    string          `json:"operator_name" gorm:"-"`
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
