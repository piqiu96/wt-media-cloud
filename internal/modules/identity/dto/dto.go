// Package dto owns identity use-case input and output contracts.
package dto

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

type LoginResult struct {
	Token      string
	SessionID  string
	ExpiresAt  time.Time
	ClientType model.ClientType
	User       model.PublicUser
}

type LoginOptions struct {
	// ReplaceExisting is only meaningful for Desktop. Web logins always add a
	// session and never invalidate another Web session.
	ReplaceExisting bool
	// ReplaceSessionID is the active Desktop row the user confirmed replacing.
	// It must match the row found under the user lock; a missing or stale id
	// returns another conflict rather than silently replacing a newer login.
	ReplaceSessionID string
	// ClientType is the session type inferred server-side from the request
	// Origin (Tauri WebView -> desktop, otherwise web). It is not a
	// client-declared field; the handler fills it in before calling the service.
	ClientType model.ClientType
}

type AuthContext struct {
	User    model.PublicUser
	Session model.Session
}

type CreateUserInput struct {
	Username string
	Password string
	Role     model.Role
	TeamID   *model.TeamID
	GameIDs  []string
}
