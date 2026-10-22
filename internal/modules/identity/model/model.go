// Package model owns identity domain types.
package model

import (
	"errors"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrSessionInvalid       = errors.New("session is invalid")
	ErrSessionInvalidated   = errors.New("session has been invalidated")
	ErrSessionExpired       = errors.New("session has expired")
	ErrDesktopReplaced      = errors.New("desktop session was replaced by another login")
	ErrDesktopLoginRequired = errors.New("a desktop login is required")
	ErrDesktopLoginConflict = errors.New("desktop login requires replacement confirmation")
	// ErrSessionReplaceNeeded is the pre-068 name for the same conflict. It is
	// kept only so existing internal callers keep compiling; no path treats it
	// as a separate outcome.
	ErrSessionReplaceNeeded = ErrDesktopLoginConflict
	ErrSessionStateInvalid  = errors.New("multiple active desktop sessions exist")
	ErrMissingCredentials   = errors.New("login username or password is empty")
	ErrLoginInputInvalid    = errors.New("login username or password violates length limits")
	ErrForbidden            = errors.New("operation is forbidden")
	ErrInvalidInput         = errors.New("identity input is invalid")
	ErrUsernameTaken        = errors.New("username is already in use")
	ErrTeamNameTaken        = errors.New("team name is already in use")
	ErrTeamInUse            = errors.New("team is still referenced")
	ErrGameIDTaken          = errors.New("game id is already in use")
	ErrInvalidGameID        = errors.New("game id is invalid")
	ErrGameNameTaken        = errors.New("game name is already in use")
	ErrGameInUse            = errors.New("game is still referenced")
	ErrUserGameScopeInUse   = errors.New("user game scope is still referenced by media accounts")
	ErrGameUnavailable      = errors.New("game is not available")
	ErrPasswordTooShort     = errors.New("password is too short")
	ErrBootstrapUnavailable = errors.New("initial admin cannot be created")
)

// DesktopLoginConflictError carries the currently active Desktop session so
// the client can resubmit with the exact row it is replacing. A stale id is
// rejected the same way, which is what makes two concurrent confirmations
// resolve to one winner without a version counter on users.
type DesktopLoginConflictError struct {
	ActiveSessionID string
}

func (e *DesktopLoginConflictError) Error() string {
	return ErrDesktopLoginConflict.Error()
}

func (e *DesktopLoginConflictError) Is(target error) bool {
	return target == ErrDesktopLoginConflict
}

type Role string

type UserID = sharedidentity.UserID

type TeamID = sharedidentity.TeamID

const (
	RoleOperator       Role = "operator"
	RoleSeniorOperator Role = "senior_operator"
	RoleAdmin          Role = "admin"
)

type UserStatus string

const (
	UserStatusEnabled  UserStatus = "enabled"
	UserStatusDisabled UserStatus = "disabled"
)

type OperationTeam struct {
	ID        TeamID    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GameStatus string

const (
	GameStatusEnabled  GameStatus = "enabled"
	GameStatusDisabled GameStatus = "disabled"
)

type OperationGame struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Status           GameStatus            `json:"status"`
	Remark           string                `json:"remark"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
	ReferenceSummary *GameReferenceSummary `json:"reference_summary,omitempty"`
}

type GameReferenceSummary struct {
	UserScopeCount    int `json:"user_scope_count"`
	MediaAccountCount int `json:"media_account_count"`
	TotalCount        int `json:"total_count"`
}

type GameReferenceUser struct {
	UserID   UserID  `json:"user_id"`
	Username string  `json:"username"`
	Role     Role    `json:"role"`
	TeamID   *TeamID `json:"team_id"`
	TeamName string  `json:"team_name"`
}

type GameReferenceAccount struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	UserID    UserID `json:"user_id"`
	Username  string `json:"username"`
}

type GameReferences struct {
	GameID        string                 `json:"game_id"`
	Users         []GameReferenceUser    `json:"users"`
	MediaAccounts []GameReferenceAccount `json:"media_accounts"`
}

type User struct {
	ID           UserID
	Username     string
	Nickname     string
	AvatarID     string
	PasswordHash string
	Role         Role
	Status       UserStatus
	TeamID       *TeamID
	TeamName     string
	GameIDs      []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PublicUser is safe to return from an authenticated API. It has no password
// or session material.
type PublicUser struct {
	ID       UserID     `json:"id"`
	Username string     `json:"username"`
	Nickname string     `json:"nickname"`
	AvatarID string     `json:"avatar_id"`
	Role     Role       `json:"role"`
	Status   UserStatus `json:"status"`
	TeamID   *TeamID    `json:"team_id"`
	TeamName string     `json:"team_name"`
	GameIDs  []string   `json:"game_ids"`
}

type ClientType string

const (
	ClientTypeWeb     ClientType = "web"
	ClientTypeDesktop ClientType = "desktop"
)

const (
	SessionInvalidReasonLogout          = "logout"
	SessionInvalidReasonPasswordChanged = "password_changed"
	SessionInvalidReasonAdminReset      = "admin_reset"
	SessionInvalidReasonUserDisabled    = "user_disabled"
	SessionInvalidReasonForceLogout     = "force_logout"
	SessionInvalidReasonDesktopReplaced = "desktop_replaced"
	SessionInvalidReasonSuperseded      = "superseded"
)

type Session struct {
	ID                string
	UserID            UserID
	TokenHash         string
	ClientType        ClientType
	CreatedAt         time.Time
	ExpiresAt         time.Time
	InvalidAt         *time.Time
	InvalidatedReason string
}

// SessionCreateResult reports whether the atomic session insert replaced an
// active Desktop row, for audit and caller diagnostics.
type SessionCreateResult struct {
	ReplacedSessionID string
}

type AuditEvent = sharedidentity.AuditEvent

// CanAccess applies the single Cloud data-scope rule shared by business modules.
func (u PublicUser) CanAccess(ownerID UserID, teamID *TeamID, gameID string) bool {
	if u.Status != UserStatusEnabled {
		return false
	}
	if u.Role == RoleAdmin {
		return true
	}
	if !containsGame(u.GameIDs, gameID) {
		return false
	}
	if u.Role == RoleSeniorOperator {
		return u.TeamID != nil && teamID != nil && *u.TeamID == *teamID
	}
	return u.Role == RoleOperator && u.ID == ownerID
}

// CanAccessOwnedResource applies the shared scope rule for resources, such as
// Browser Profiles, that do not belong to a game.
func (u PublicUser) CanAccessOwnedResource(ownerID UserID, teamID *TeamID) bool {
	if u.Status != UserStatusEnabled {
		return false
	}
	if u.Role == RoleAdmin {
		return true
	}
	if u.Role == RoleSeniorOperator {
		return u.TeamID != nil && teamID != nil && *u.TeamID == *teamID
	}
	return u.Role == RoleOperator && u.ID == ownerID
}

func containsGame(gameIDs []string, gameID string) bool {
	for _, existing := range gameIDs {
		if existing == gameID {
			return true
		}
	}
	return false
}
