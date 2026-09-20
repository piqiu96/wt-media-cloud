// Package model owns Media Account domain values.
package model

import (
	"errors"
	"time"

	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

var (
	ErrForbidden            = errors.New("media account operation is forbidden")
	ErrInvalidInput         = errors.New("media account input is invalid")
	ErrNotFound             = errors.New("media account was not found")
	ErrDuplicateAccount     = errors.New("media account already exists for this user and platform")
	ErrProfileUnavailable   = errors.New("browser profile is unavailable")
	ErrProfilePlatformTaken = errors.New("browser profile already has an account for this platform")
)

type Platform string

const (
	PlatformDouyin    Platform = "douyin"
	PlatformBilibili  Platform = "bilibili"
	PlatformBaijiahao Platform = "baijiahao"
)

type IdentificationStatus string

const (
	IdentificationPending    IdentificationStatus = "pending_identification"
	IdentificationIdentified IdentificationStatus = "identified"
	IdentificationDuplicate  IdentificationStatus = "duplicate"
)

type BusinessStatus string

const (
	BusinessEnabled  BusinessStatus = "enabled"
	BusinessDisabled BusinessStatus = "disabled"
)

type LoginStatus string

const (
	LoginUnknown            LoginStatus = "unknown"
	LoginNormal             LoginStatus = "normal"
	LoginNotLoggedIn        LoginStatus = "not_logged_in"
	LoginVerificationNeeded LoginStatus = "verification_needed"
	LoginExpired            LoginStatus = "expired"
	LoginRestricted         LoginStatus = "restricted"
	LoginAccountMismatch    LoginStatus = "account_mismatch"
	LoginEnvironmentError   LoginStatus = "environment_error"
)

type AccountCheckItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Account struct {
	ID                    string                `json:"id"`
	UserID                identitymodel.UserID  `json:"user_id"`
	TeamID                *identitymodel.TeamID `json:"team_id"`
	GameIDs               []string              `json:"game_ids"`
	Platform              Platform              `json:"platform"`
	PlatformAccountID     string                `json:"platform_account_id,omitempty"`
	Name                  string                `json:"name,omitempty"`
	AvatarURL             string                `json:"avatar_url,omitempty"`
	BrowserProfileID      string                `json:"browser_profile_id,omitempty"`
	Remark                string                `json:"remark,omitempty"`
	IdentificationStatus  IdentificationStatus  `json:"identification_status"`
	DuplicateOfAccountID  string                `json:"duplicate_of_account_id,omitempty"`
	BusinessStatus        BusinessStatus        `json:"business_status"`
	LoginStatus           LoginStatus           `json:"login_status"`
	CookieStatus          string                `json:"cookie_status,omitempty"`
	ActiveCookieUpdatedAt *time.Time            `json:"active_cookie_updated_at,omitempty"`
	LastCheckedAt         *time.Time            `json:"last_checked_at,omitempty"`
	CheckItems            []AccountCheckItem    `json:"check_items,omitempty"`
	Tags                  []string              `json:"tags"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
}

type AccountRecord struct {
	Account
	OriginalCookie string
	ActiveCookie   string
}

type AccountGroupFilters struct {
	GameIDs        []string       `json:"game_ids,omitempty"`
	Platform       Platform       `json:"platform,omitempty"`
	BusinessStatus BusinessStatus `json:"business_status,omitempty"`
	LoginStatus    LoginStatus    `json:"login_status,omitempty"`
	Search         string         `json:"search,omitempty"`
	AnyTags        []string       `json:"any_tags,omitempty"`
	AllTags        []string       `json:"all_tags,omitempty"`
	ExcludeTags    []string       `json:"exclude_tags,omitempty"`
}

type AccountGroup struct {
	ID        string                `json:"id"`
	UserID    identitymodel.UserID  `json:"user_id"`
	TeamID    *identitymodel.TeamID `json:"team_id,omitempty"`
	Name      string                `json:"name"`
	Filters   AccountGroupFilters   `json:"filters"`
	SortOrder int                   `json:"sort_order"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}
