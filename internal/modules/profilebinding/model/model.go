// Package model owns Profile Binding domain types.
package model

import (
	"errors"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrForbidden            = errors.New("profile binding operation is forbidden")
	ErrInvalidInput         = errors.New("profile binding input is invalid")
	ErrIdentityUnverifiable = errors.New("BitBrowser identity is unverifiable")
	ErrIdentityMismatch     = errors.New("BitBrowser identity does not match the bound user")
	ErrScanNotFound         = errors.New("profile scan was not found")
	ErrScanNotReady         = errors.New("profile scan is not ready")
	ErrScanExpired          = errors.New("profile scan has expired")
	ErrProfileNotFound      = errors.New("browser profile was not found")
	ErrProfileInactive      = errors.New("browser profile is not active")
	ErrProfileReferenced    = errors.New("browser profile is still referenced")
	ErrProfileNotDisabled   = errors.New("browser profile is not disabled")
)

type BitAccountStatus string

const BitAccountBound BitAccountStatus = "bound"

type ProfileLocalStatus string

const (
	ProfileActive       ProfileLocalStatus = "active"
	ProfileLocalMissing ProfileLocalStatus = "local_missing"
	ProfileArchived     ProfileLocalStatus = "archived"
)

type ProfileBusinessStatus string

const (
	ProfileBusinessEnabled  ProfileBusinessStatus = "enabled"
	ProfileBusinessDisabled ProfileBusinessStatus = "disabled"
)

type ScanStatus string

const (
	ScanReady     ScanStatus = "ready"
	ScanConfirmed ScanStatus = "confirmed"
)

type DiffKind string

const (
	DiffAdded   DiffKind = "added"
	DiffChanged DiffKind = "changed"
	DiffMissing DiffKind = "missing"
)

type BitAccountBinding struct {
	UserID         sharedidentity.UserID `json:"user_id"`
	MainUserID     string                `json:"main_user_id"`
	Status         BitAccountStatus      `json:"status"`
	BoundAt        *time.Time            `json:"bound_at,omitempty"`
	LastVerifiedAt *time.Time            `json:"last_verified_at,omitempty"`
}

type BrowserProfile struct {
	ID             string                 `json:"id"`
	UserID         sharedidentity.UserID  `json:"user_id"`
	TeamID         *sharedidentity.TeamID `json:"team_id"`
	BitProfileID   string                 `json:"bit_profile_id"`
	MainUserID     string                 `json:"main_user_id"`
	ProfileUserID  string                 `json:"profile_user_id"`
	Name           string                 `json:"name"`
	Seq            int                    `json:"seq,omitempty"`
	GroupID        string                 `json:"group_id,omitempty"`
	GroupName      string                 `json:"group_name,omitempty"`
	BitStatus      string                 `json:"bit_status,omitempty"`
	BitUpdatedAt   string                 `json:"bit_updated_at,omitempty"`
	ProxyType      string                 `json:"proxy_type,omitempty"`
	ProxyHost      string                 `json:"proxy_host,omitempty"`
	ProxyPort      int                    `json:"proxy_port,omitempty"`
	ProxyID        string                 `json:"proxy_id,omitempty"`
	Remark         string                 `json:"remark,omitempty"`
	CloudRemark    string                 `json:"cloud_remark,omitempty"`
	BusinessStatus ProfileBusinessStatus  `json:"business_status"`
	LocalStatus    ProfileLocalStatus     `json:"local_status"`
	LastSyncedAt   time.Time              `json:"last_synced_at"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type ProfileDiff struct {
	Kind         DiffKind `json:"kind"`
	BitProfileID string   `json:"bit_profile_id"`
	ProfileID    string   `json:"profile_id,omitempty"`
	Fields       []string `json:"fields"`
}

type ProfileScan struct {
	ID          string                 `json:"id"`
	UserID      sharedidentity.UserID  `json:"user_id"`
	TeamID      *sharedidentity.TeamID `json:"team_id"`
	MainUserID  string                 `json:"main_user_id"`
	Status      ScanStatus             `json:"status"`
	Profiles    []BrowserProfile       `json:"profiles"`
	Diff        []ProfileDiff          `json:"diff"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   time.Time              `json:"expires_at"`
	ConfirmedAt *time.Time             `json:"confirmed_at,omitempty"`
}
