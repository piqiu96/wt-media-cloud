// Package dto owns Media Account input and query contracts.
package dto

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type CreateAccountInput struct {
	UserID           sharedidentity.UserID
	GameIDs          []string
	Name             string
	Platform         model.Platform
	OriginalCookie   string
	BrowserProfileID string
	Remark           string
	Tags             []string
}

type IdentifyAccountInput struct {
	PlatformAccountID string
	Name              string
	AvatarURL         string
	LoginStatus       model.LoginStatus
}

type AccountCheckStartInput struct{ NodeID string }

type AccountCheckStart struct {
	TaskID                    string            `json:"task_id"`
	AccountID                 string            `json:"account_id"`
	BrowserProfileID          string            `json:"browser_profile_id"`
	BitProfileID              string            `json:"bit_profile_id"`
	Platform                  model.Platform    `json:"platform"`
	ExpectedPlatformAccountID string            `json:"expected_platform_account_id,omitempty"`
	LoginStatus               model.LoginStatus `json:"login_status"`
}

type AccountCheckResultInput struct {
	TaskID            string
	PlatformAccountID string
	Name              string
	AvatarURL         string
	LoginStatus       model.LoginStatus
	Message           string
	CheckItems        []model.AccountCheckItem
}

type CookieReadStartInput struct{ NodeID string }

type CookieReadStart struct {
	TaskID           string         `json:"task_id"`
	AccountID        string         `json:"account_id"`
	BrowserProfileID string         `json:"browser_profile_id"`
	BitProfileID     string         `json:"bit_profile_id"`
	Platform         model.Platform `json:"platform"`
}

type CookieReadResultInput struct {
	TaskID  string
	Cookies []map[string]any
}

type UpdateAccountInput struct {
	BusinessStatus model.BusinessStatus
	LoginStatus    model.LoginStatus
	Remark         *string
	GameIDs        *[]string
	Name           *string
}

type AccountFilter struct {
	UserID         sharedidentity.UserID
	GameIDs        []string
	Platform       model.Platform
	BusinessStatus model.BusinessStatus
	LoginStatus    model.LoginStatus
	Search         string
	ProfileSearch  string
	AnyTags        []string
	AllTags        []string
	ExcludeTags    []string
}

type AccountGroupFilters = model.AccountGroupFilters

type CreateAccountGroupInput struct {
	Name    string              `json:"name"`
	Filters AccountGroupFilters `json:"filters"`
}

type UpdateAccountGroupInput struct {
	Name    *string              `json:"name"`
	Filters *AccountGroupFilters `json:"filters"`
}
