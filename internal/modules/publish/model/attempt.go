package model

import (
	"encoding/json"
	"time"
)

type ExecutionInput struct {
	PortraitCoverSHA256 string  `json:"portrait_cover_sha256"`
	PortraitCoverSize   int64   `json:"portrait_cover_size"`
	ProfileID           uint64  `json:"profile_id,string"`
	BitProfileID        string  `json:"bit_profile_id"`
	BitMainUserID       string  `json:"bit_main_user_id"`
	PlatformUID         string  `json:"platform_uid"`
	Content             Content `json:"content"`
	VideoSHA256         string  `json:"video_sha256"`
	VideoSize           int64   `json:"video_size"`
	CoverSHA256         string  `json:"cover_sha256"`
	CoverSize           int64   `json:"cover_size"`
}
type HumanContinuation struct {
	AttemptID        uint64 `json:"attempt_id,string"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
}

func (v HumanContinuation) Valid() bool { return v.AttemptID > 0 && v.ExpectedRevision > 0 }

type Attempt struct {
	AdmissionSlot      uint8           `json:"-" gorm:"default:1"`
	ID                 uint64          `json:"id,string"`
	PublicationID      uint64          `json:"publication_id,string"`
	AttemptNo          uint32          `json:"attempt_no"`
	ExecutorUserID     int64           `json:"executor_user_id,string"`
	ProfileID          uint64          `json:"profile_id,string" gorm:"column:browser_profile_id"`
	ClaimRequestKey    string          `json:"-"`
	Input              ExecutionInput  `json:"input_snapshot" gorm:"column:input_snapshot_json;serializer:json"`
	Status             AttemptStatus   `json:"status"`
	Stage              AttemptStage    `json:"stage"`
	PausedStage        *AttemptStage   `json:"-"`
	LastResumeRevision *uint64         `json:"-"`
	Revision           uint64          `json:"-" gorm:"default:1"`
	ReportSeq          uint64          `json:"report_seq"`
	StartedAt          time.Time       `json:"started_at"`
	HeartbeatAt        time.Time       `json:"heartbeat_at"`
	SubmitStartedAt    *time.Time      `json:"submit_started_at"`
	FinishedAt         *time.Time      `json:"finished_at"`
	ResultStatus       *Status         `json:"result_status"`
	ErrorCode          *string         `json:"error_code"`
	ErrorMessage       *string         `json:"error_message"`
	Steps              json.RawMessage `json:"steps" gorm:"column:steps_json"`
	Evidence           json.RawMessage `json:"evidence" gorm:"column:evidence_json"`
	ResultHash         *string         `json:"result_hash"`
	ResolvedBy         *int64          `json:"resolved_by,string"`
	ResolvedAt         *time.Time      `json:"resolved_at"`
	ResolutionNote     *string         `json:"resolution_note"`
}

func (Attempt) TableName() string { return "publication_attempt" }

type Claim struct {
	Task     Task    `json:"task"`
	Attempt  Attempt `json:"attempt"`
	Replayed bool    `json:"replayed"`
}
