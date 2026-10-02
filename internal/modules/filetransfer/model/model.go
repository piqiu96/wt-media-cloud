// Package model owns cross-domain file-transfer facts and state transitions.
package model

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type AssetType string

const (
	AssetMaterial AssetType = "material"
)

type Purpose string

const (
	PurposeComposeInputPrepare Purpose = "compose_input_prepare"
	PurposeUserDownload        Purpose = "user_download"
)

type ExecutionScope string

const (
	ExecutionCloud      ExecutionScope = "cloud"
	ExecutionLocalAgent ExecutionScope = "local_agent"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Task is executor-neutral: production can request a Cloud prepare task, and
// a Local Agent can execute a user download without inheriting production code.
//
// It is also self-contained. The executor that leases a task runs later, on
// another machine, and this module may not import `production` (see
// architecture_test.go), so the facts a lease needs — the title to name the file
// with, the object to grant, the size and hash to verify against — are copied
// onto the task when it is created rather than read from `materials` when it is
// leased.
type Task struct {
	ID                string          `json:"id"`
	TeamID            identity.TeamID `json:"team_id"`
	AssetType         AssetType       `json:"asset_type"`
	AssetID           int64           `json:"asset_id"`
	AssetTitle        string          `json:"asset_title,omitempty"`
	GameName          string          `json:"game_name,omitempty"`
	PublishedAt       *time.Time      `json:"published_at,omitempty"`
	SourceObjectKey   string          `json:"-"`
	Purpose           Purpose         `json:"purpose"`
	ExecutionScope    ExecutionScope  `json:"execution_scope"`
	Status            Status          `json:"status"`
	RequestedBy       identity.UserID `json:"requested_by"`
	AssignedNodeID    string          `json:"assigned_node_id,omitempty"`
	AssignedDeviceID  string          `json:"assigned_device_id,omitempty"`
	ClaimedByNodeID   string          `json:"claimed_by_node_id,omitempty"`
	DependencyTaskID  string          `json:"dependency_task_id,omitempty"`
	TotalBytes        int64           `json:"total_bytes,omitempty"`
	TransferredBytes  int64           `json:"transferred_bytes,omitempty"`
	SpeedBytesPerSec  int64           `json:"speed_bytes_per_sec,omitempty"`
	ETASeconds        *int64          `json:"eta_seconds,omitempty"`
	AttemptCount      int             `json:"attempt_count"`
	MaxAttempts       int             `json:"max_attempts"`
	LeaseExpiresAt    *time.Time      `json:"lease_expires_at,omitempty"`
	HeartbeatAt       *time.Time      `json:"heartbeat_at,omitempty"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	FinishedAt        *time.Time      `json:"finished_at,omitempty"`
	CancelRequestedAt *time.Time      `json:"cancel_requested_at,omitempty"`
	ExpectedSHA256    string          `json:"-"`
	FileName          string          `json:"file_name,omitempty"`
	ErrorCode         string          `json:"error_code,omitempty"`
	ErrorMessage      string          `json:"error_message,omitempty"`
	IntegritySHA256   string          `json:"integrity_sha256,omitempty"`
	IntegrityBytes    *int64          `json:"integrity_bytes,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
