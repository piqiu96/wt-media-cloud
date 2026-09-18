// Package dto owns identity use-case input and output contracts.
package dto

import "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"

type LoginResult struct {
	Token string
	User  model.PublicUser
}

type LoginOptions struct {
	ReplaceExisting bool
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
