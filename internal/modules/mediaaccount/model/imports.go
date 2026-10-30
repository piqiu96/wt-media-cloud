package model

import (
	"errors"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"time"
)

var ErrImportConflict = errors.New("import_conflict")

type ImportItem struct {
	WindowName            string            `json:"window_name,omitempty"`
	RowNumber             int               `json:"row_number"`
	Status                string            `json:"status"`
	Step                  string            `json:"step"`
	Attempt               uint64            `json:"attempt"`
	OperationID           string            `json:"operation_id,omitempty"`
	InputCookie           string            `json:"input_cookie,omitempty"`
	BeforeCookie          *[]map[string]any `json:"before_cookie,omitempty"`
	Fingerprint           string            `json:"fingerprint,omitempty"`
	BrowserProfileID      string            `json:"browser_profile_id,omitempty"`
	BitProfileID          string            `json:"bit_profile_id,omitempty"`
	AccountID             string            `json:"account_id,omitempty"`
	SameIdentityAccountID string            `json:"same_identity_account_id,omitempty"`
	Name                  string            `json:"name,omitempty"`
	PlatformAccountID     string            `json:"platform_account_id,omitempty"`
	Message               string            `json:"message"`
	PreviousStatus        string            `json:"previous_status,omitempty"`
	CreationUnavailable   bool              `json:"creation_unavailable,omitempty"`
	LastResultStep        string            `json:"last_result_step,omitempty"`
	LastResultDigest      string            `json:"last_result_digest,omitempty"`
}
type ImportContent struct {
	CategoryIDs   []sharedidentity.CategoryID `json:"category_ids"`
	Tags          []string                    `json:"tags"`
	Remark        string                      `json:"remark"`
	GroupID       string                      `json:"group_id"`
	RequestDigest string                      `json:"request_digest,omitempty"`
	Items         []ImportItem                `json:"items"`
}
type ImportBatch struct {
	ID         string                `json:"id"`
	UserID     sharedidentity.UserID `json:"user_id"`
	RequestKey string                `json:"request_key"`
	Platform   Platform              `json:"platform"`
	Content    ImportContent         `json:"content_json"`
	Revision   uint64                `json:"revision"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// ImportResources is limited to the short batch transaction; no external calls.
type ImportResources interface {
	ReserveProfile(profileID, operationID string, platform Platform, marker string) (ImportTarget, error)
	ReleaseProfile(profileID, operationID string) error
	FindIdentity(platform Platform, uid string) (string, error)
	CreateAccount(AccountRecord) (string, error)
}

type ImportTarget struct {
	BitID string
	Name  string
}
