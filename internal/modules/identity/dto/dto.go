// Package dto owns identity use-case input and output contracts.
package dto

import "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"

type LoginResult struct {
	Token string
	User  model.PublicUser
}

type LoginOptions struct {
	ReplaceExisting bool
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
