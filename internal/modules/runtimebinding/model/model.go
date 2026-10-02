// Package model owns runtime-binding domain types.
package model

import (
	"errors"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrDeviceNotBound = errors.New("no device is bound")
	ErrDeviceMismatch = errors.New("another device is bound")
)

type BindingTicket struct {
	ID        string
	UserID    sharedidentity.UserID
	SessionID string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// AgentNode is the execution-presence layer (contract v2): it binds by
// device_id, never by a login session, so session invalidation cannot revoke a
// node's credential. Registration still requires a live session, but only
// through the binding ticket, which carries its own session_id.
type AgentNode struct {
	ID                   string                `json:"id"`
	AgentID              string                `json:"agent_id"`
	DeviceID             string                `json:"device_id"`
	DevicePublicKey      []byte                `json:"-"`
	DeviceName           string                `json:"-"`
	BindDevice           bool                  `json:"-"`
	UserID               sharedidentity.UserID `json:"user_id"`
	Mode                 string                `json:"mode"`
	AgentVersion         string                `json:"agent_version"`
	ContractMajorVersion string                `json:"contract_major_version"`
	ContractRevision     string                `json:"contract_revision"`
	Status               string                `json:"status"`
	CredentialHash       string                `json:"-"`
	RegisteredAt         time.Time             `json:"registered_at"`
	LastHeartbeatAt      time.Time             `json:"last_heartbeat_at"`
}

type DeviceBinding struct {
	Bound               bool       `json:"bound"`
	DeviceID            string     `json:"device_id,omitempty"`
	DeviceName          string     `json:"device_name,omitempty"`
	BoundAt             *time.Time `json:"bound_at,omitempty"`
	LastVerifiedAt      *time.Time `json:"last_verified_at,omitempty"`
	BitAccountBound     bool       `json:"bit_account_bound"`
	BitMainUserIDMasked string     `json:"bit_main_user_id_masked,omitempty"`
}

type DependencyFact struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

type DiskFact struct {
	Status        string `json:"status"`
	FreeMegabytes int64  `json:"free_megabytes"`
}
