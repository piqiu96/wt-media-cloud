// Package dto owns Media Account input and query contracts.
package dto

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type CreateAccountInput struct {
	UserID           sharedidentity.UserID
	CategoryIDs      []sharedidentity.CategoryID
	PlatformName     string
	Platform         model.Platform
	OriginalCookie   string
	BrowserProfileID string
	Remark           string
	Tags             []string
}

type AccountCheckStartInput struct{}

type AccountCheckStart struct {
	OperationID               string                    `json:"operation_id"`
	AccountID                 string                    `json:"account_id"`
	BrowserProfileID          string                    `json:"browser_profile_id"`
	BitProfileID              string                    `json:"bit_profile_id"`
	Platform                  model.Platform            `json:"platform"`
	ExpectedPlatformAccountID string                    `json:"expected_platform_account_id,omitempty"`
	PlatformLoginStatus       model.PlatformLoginStatus `json:"platform_login_status"`
}

type AccountCheckResultInput struct {
	ExecutionStatus     string `json:"execution_status"`
	OperationID         string
	PlatformAccountID   string
	PlatformName        string
	PlatformAvatarURL   string
	PlatformLoginStatus model.PlatformLoginStatus
	Message             string
	LastCheckItems      []model.AccountCheckItem
}

type CookieReadStartInput struct{}

type CookieReadStart struct {
	OperationID      string         `json:"operation_id"`
	AccountID        string         `json:"account_id"`
	BrowserProfileID string         `json:"browser_profile_id"`
	BitProfileID     string         `json:"bit_profile_id"`
	Platform         model.Platform `json:"platform"`
}

type CookieReadResultInput struct {
	OperationID string
	Cookies     []map[string]any
}

type UpdateAccountInput struct {
	Tags        *[]string
	Enabled     *bool
	Remark      *string
	CategoryIDs *[]sharedidentity.CategoryID
}

type AccountFilter struct {
	View                string
	TeamID              sharedidentity.TeamID
	CategoryType        string
	Offset              int
	Limit               int
	UserID              sharedidentity.UserID
	CategoryIDs         []sharedidentity.CategoryID
	Platform            model.Platform
	Enabled             *bool
	PlatformLoginStatus model.PlatformLoginStatus
	Search              string
	ProfileSearch       string
	AnyTags             []string
	AllTags             []string
	ExcludeTags         []string
}

type AccountPage struct {
	Stats    model.AccountCounts `json:"stats"`
	Items    []model.Account     `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}
