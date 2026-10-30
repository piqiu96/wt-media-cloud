// Package model owns Profile Binding domain types.
package model

import (
	"errors"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrForbidden                  = errors.New("profile binding operation is forbidden")
	ErrInvalidInput               = errors.New("profile binding input is invalid")
	ErrIdentityUnverifiable       = errors.New("BitBrowser identity is unverifiable")
	ErrBitNotBound                = errors.New("BitBrowser main account is not bound")
	ErrBitBrowserOffline          = errors.New("BitBrowser observation is stale")
	ErrBitMainUserMismatch        = errors.New("BitBrowser main account does not match the bound account")
	ErrScanNotFound               = errors.New("profile scan was not found")
	ErrProfileProxyPending        = errors.New("profile proxy operation requires verification")
	ErrScanNotReady               = errors.New("profile scan is not ready")
	ErrScanExpired                = errors.New("profile scan has expired")
	ErrProfileNotFound            = errors.New("browser profile was not found")
	ErrProfileAssignedToOtherUser = errors.New("browser profile belongs to another user")
	ErrProfileInactive            = errors.New("browser profile is not active")
	ErrProfileReferenced          = errors.New("browser profile is still referenced")
	ErrProfileNotMissing          = errors.New("browser profile is not missing from BitBrowser")
)

// 主账号绑定只由 Local Agent 的 bind 端点写入，审计动作与绑定模型放在一起。
const (
	AuditMainAccountBind   = "bitbrowser.main_account.bind"
	AuditMainAccountRebind = "bitbrowser.main_account.rebind"
	AuditMainAccountVerify = "bitbrowser.main_account.verify"
)

type BitAccountStatus string

const BitAccountBound BitAccountStatus = "bound"

type ProfileLocalState string

const (
	ProfileActive       ProfileLocalState = "active"
	ProfileLocalMissing ProfileLocalState = "local_missing"
	ProfileArchived     ProfileLocalState = "archived"
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
	BitMainUserID  string                `json:"bit_main_user_id"`
	Status         BitAccountStatus      `json:"status"`
	BoundAt        *time.Time            `json:"bound_at,omitempty"`
	LastVerifiedAt *time.Time            `json:"last_verified_at,omitempty"`
}

type BrowserProfile struct {
	ID               string                 `json:"id"`
	UserID           sharedidentity.UserID  `json:"user_id"`
	TeamID           *sharedidentity.TeamID `json:"team_id"`
	BitProfileID     string                 `json:"bit_profile_id"`
	BitMainUserID    string                 `json:"bit_main_user_id"`
	BitUserID        string                 `json:"bit_user_id"`
	BitName          string                 `json:"bit_name"`
	BitSeq           int                    `json:"bit_seq"`
	BitGroupID       string                 `json:"bit_group_id,omitempty"`
	BitGroupName     string                 `json:"bit_group_name,omitempty"`
	BitProfileStatus string                 `json:"bit_profile_status,omitempty"`
	BitUpdatedAt     string                 `json:"bit_updated_at,omitempty"`
	BitProxyType     string                 `json:"bit_proxy_type,omitempty"`
	BitProxyHost     string                 `json:"bit_proxy_host,omitempty"`
	BitProxyPort     int                    `json:"bit_proxy_port,omitempty"`
	ProxyID          sharedidentity.ProxyID `json:"proxy_id,omitempty"`
	ReservedProxyID  sharedidentity.ProxyID `json:"reserved_proxy_id,omitempty"`
	ReservedSince    *time.Time             `json:"reserved_since,omitempty"`
	BitRemark        string                 `json:"bit_remark,omitempty"`
	Remark           string                 `json:"remark,omitempty"`
	Enabled          bool                   `json:"enabled"`
	LocalState       ProfileLocalState      `json:"local_state"`
	LastSyncedAt     time.Time              `json:"last_synced_at"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	// OwnershipConflict 是扫描时的证据，不是存储列：同一比特窗口已登记在其他
	// 运营名下，不能被最后扫描到的人认领。
	OwnershipConflict string `json:"ownership_conflict,omitempty"`
}

type ProfileDiff struct {
	Kind         DiffKind `json:"kind"`
	BitProfileID string   `json:"bit_profile_id"`
	ProfileID    string   `json:"profile_id,omitempty"`
	Fields       []string `json:"fields"`
}

type ProfileScan struct {
	ID            string                 `json:"id"`
	UserID        sharedidentity.UserID  `json:"user_id"`
	TeamID        *sharedidentity.TeamID `json:"team_id"`
	BitMainUserID string                 `json:"bit_main_user_id"`
	Status        ScanStatus             `json:"status"`
	Profiles      []BrowserProfile       `json:"profiles"`
	Diff          []ProfileDiff          `json:"diff"`
	CreatedAt     time.Time              `json:"created_at"`
	ExpiresAt     time.Time              `json:"expires_at"`
	ConfirmedAt   *time.Time             `json:"confirmed_at,omitempty"`
}
