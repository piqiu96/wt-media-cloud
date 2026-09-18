// Package model owns runtime-binding domain types.
package model

import (
	"time"

	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

type BindingTicket struct {
	ID        string
	UserID    identitymodel.UserID
	SessionID string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

type AgentNode struct {
	ID                   string               `json:"id"`
	AgentID              string               `json:"agent_id"`
	DeviceID             string               `json:"device_id"`
	UserID               identitymodel.UserID `json:"user_id"`
	SessionID            string               `json:"-"`
	Mode                 string               `json:"mode"`
	AgentVersion         string               `json:"agent_version"`
	ContractMajorVersion string               `json:"contract_major_version"`
	ContractRevision     string               `json:"contract_revision"`
	Status               string               `json:"status"`
	CredentialHash       string               `json:"-"`
	RegisteredAt         time.Time            `json:"registered_at"`
	LastHeartbeatAt      time.Time            `json:"last_heartbeat_at"`
}

type DependencyFact struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

type DiskFact struct {
	Status        string `json:"status"`
	FreeMegabytes int64  `json:"free_megabytes"`
}
