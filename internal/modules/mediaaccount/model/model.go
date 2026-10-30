// Package model owns Media Account domain values.
package model

import (
	"errors"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrForbidden                  = errors.New("media account operation is forbidden")
	ErrInvalidInput               = errors.New("media account input is invalid")
	ErrNotFound                   = errors.New("media account was not found")
	ErrDuplicateAccount           = errors.New("media account already exists for this user and platform")
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
	IdentificationDuplicate  IdentificationStatus = "duplicate"
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

type Account struct {
	ID     string                `json:"id"`
	UserID sharedidentity.UserID `json:"user_id"`
	// UserName is the account owner's display name (nickname, falling back to
	// username), resolved by the read query. It is a projection, never written.
	UserName             string                      `json:"user_name"`
	TeamID               *sharedidentity.TeamID      `json:"team_id"`
	CategoryIDs          []sharedidentity.CategoryID `json:"category_ids"`
	Platform             Platform                    `json:"platform"`
	PlatformAccountID    string                      `json:"platform_account_id,omitempty"`
	PlatformName         string                      `json:"platform_name,omitempty"`
	PlatformAvatarURL    string                      `json:"platform_avatar_url,omitempty"`
	BrowserProfileID     string                      `json:"browser_profile_id,omitempty"`
	Remark               string                      `json:"remark,omitempty"`
	IdentificationStatus IdentificationStatus        `json:"identification_status"`
	DuplicateOfAccountID string                      `json:"duplicate_of_account_id,omitempty"`
	Enabled              bool                        `json:"enabled"`
	PlatformLoginStatus  PlatformLoginStatus         `json:"platform_login_status"`
	CookieStatus         string                      `json:"cookie_status,omitempty"`
	CookieUpdatedAt      *time.Time                  `json:"cookie_updated_at,omitempty"`
	LastCheckAt          *time.Time                  `json:"last_check_at,omitempty"`
	LastCheckItems       []AccountCheckItem          `json:"last_check_items,omitempty"`
	Tags                 []string                    `json:"tags"`
	CreatedAt            time.Time                   `json:"created_at"`
	UpdatedAt            time.Time                   `json:"updated_at"`
}

type AccountRecord struct {
	Account
	OriginalCookie string
	ActiveCookie   string
}

type AccountGroupFilters struct {
	CategoryIDs         []sharedidentity.CategoryID `json:"category_ids,omitempty"`
	Platform            Platform                    `json:"platform,omitempty"`
	Enabled             *bool                       `json:"enabled,omitempty"`
	PlatformLoginStatus PlatformLoginStatus         `json:"platform_login_status,omitempty"`
	Search              string                      `json:"search,omitempty"`
	AnyTags             []string                    `json:"any_tags,omitempty"`
	AllTags             []string                    `json:"all_tags,omitempty"`
	ExcludeTags         []string                    `json:"exclude_tags,omitempty"`
}

type AccountGroup struct {
	ID        string                 `json:"id"`
	UserID    sharedidentity.UserID  `json:"user_id"`
	TeamID    *sharedidentity.TeamID `json:"team_id,omitempty"`
	Name      string                 `json:"name"`
	Filters   AccountGroupFilters    `json:"filters"`
	SortOrder int                    `json:"sort_order"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}
