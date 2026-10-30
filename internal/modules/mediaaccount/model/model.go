// Package model owns Media Account domain values.
package model

import (
	"errors"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrCookieConflict             = errors.New("cookie operation conflicts with current target or receipt")
	ErrStaleCheck                 = errors.New("media account check is stale")
	ErrForbidden                  = errors.New("media account operation is forbidden")
	ErrInvalidInput               = errors.New("media account input is invalid")
	ErrNotFound                   = errors.New("media account was not found")
	ErrProfileUnavailable         = errors.New("browser profile is unavailable")
	ErrProfileNotFound            = errors.New("browser profile was not found")
	ErrProfileAssignedToOtherUser = errors.New("browser profile belongs to another user")
	ErrProfilePlatformTaken       = errors.New("browser profile already has an account for this platform")
)

type Platform string

const (
	PlatformDouyin   Platform = "douyin"
	PlatformBilibili Platform = "bilibili"
	PlatformBaidu    Platform = "baidu"
)

type IdentificationStatus string

const (
	IdentificationPending    IdentificationStatus = "pending_identification"
	IdentificationIdentified IdentificationStatus = "identified"
)

type PlatformLoginStatus string

const (
	PlatformLoginUnknown            PlatformLoginStatus = "unknown"
	PlatformLoginNormal             PlatformLoginStatus = "normal"
	PlatformLoginNotLoggedIn        PlatformLoginStatus = "not_logged_in"
	PlatformLoginVerificationNeeded PlatformLoginStatus = "verification_needed"
	PlatformLoginExpired            PlatformLoginStatus = "expired"
	PlatformLoginRestricted         PlatformLoginStatus = "restricted"
	PlatformLoginAccountMismatch    PlatformLoginStatus = "account_mismatch"
	PlatformLoginEnvironmentError   PlatformLoginStatus = "environment_error"
)

type AccountCheckItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type CheckIdentity struct {
	PlatformAccountID string `json:"platform_account_id"`
	PlatformName      string `json:"platform_name"`
	PlatformAvatarURL string `json:"platform_avatar_url"`
}

type Account struct {
	ID     string                `json:"id"`
	UserID sharedidentity.UserID `json:"user_id"`
	// UserName is the account owner's display name (nickname, falling back to
	// username), resolved by the read query. It is a projection, never written.
	UserName              string                      `json:"user_name"`
	TeamID                *sharedidentity.TeamID      `json:"team_id"`
	CategoryIDs           []sharedidentity.CategoryID `json:"category_ids"`
	Platform              Platform                    `json:"platform"`
	PlatformAccountID     string                      `json:"platform_account_id,omitempty"`
	PlatformName          string                      `json:"platform_name,omitempty"`
	PlatformAvatarURL     string                      `json:"platform_avatar_url,omitempty"`
	BrowserProfileID      string                      `json:"browser_profile_id,omitempty"`
	Remark                string                      `json:"remark,omitempty"`
	IdentificationStatus  IdentificationStatus        `json:"identification_status"`
	SameIdentityAccountID string                      `json:"same_identity_account_id,omitempty"`
	Enabled               bool                        `json:"enabled"`
	PlatformLoginStatus   PlatformLoginStatus         `json:"platform_login_status"`
	CookieStatus          string                      `json:"cookie_status,omitempty"`
	CookieUpdatedAt       *time.Time                  `json:"cookie_updated_at,omitempty"`
	LastCheckStatus       string                      `json:"last_check_status"`
	LastCheckMessage      string                      `json:"last_check_message"`
	LastCheckIdentity     *CheckIdentity              `json:"last_check_identity,omitempty"`
	CheckOperationID      string                      `json:"-"`
	CheckProfileID        string                      `json:"-"`
	CheckStartedAt        *time.Time                  `json:"-"`
	LastCheckAt           *time.Time                  `json:"last_check_at,omitempty"`
	LastCheckItems        []AccountCheckItem          `json:"last_check_items,omitempty"`
	Tags                  []string                    `json:"tags"`
	CreatedAt             time.Time                   `json:"created_at"`
	UpdatedAt             time.Time                   `json:"updated_at"`
}

type AccountRecord struct {
	Account
	OriginalCookie string
	ActiveCookie   string
	BeforeCookie   string
	CookieUpdate   *CookieUpdate
}

type AccountCounts struct {
	Total    int `json:"total"`
	Normal   int `json:"normal"`
	Abnormal int `json:"abnormal"`
	Unbound  int `json:"unbound"`
}

// CookieUpdate is private execution state, never part of Account projections.
type CookieUpdate struct {
	RequestKey                string   `json:"request_key"`
	RequestDigest             string   `json:"request_digest"`
	OperationID               string   `json:"operation_id"`
	BrowserProfileID          string   `json:"browser_profile_id"`
	BitProfileID              string   `json:"bit_profile_id"`
	Platform                  Platform `json:"platform"`
	ExpectedPlatformAccountID string   `json:"expected_platform_account_id"`
	InputCookie               string   `json:"input_cookie,omitempty"`
	Fingerprint               string   `json:"fingerprint,omitempty"`
	Status                    string   `json:"status"`
	Step                      string   `json:"step"`
	Message                   string   `json:"message"`
}
