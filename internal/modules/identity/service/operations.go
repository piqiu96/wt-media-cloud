// Package-level operations preserve the existing Service behavior while callers use the controlled database registry.
package service

import "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"

func BootstrapAdmin(username, password string) (PublicUser, error) {
	return NewService(mysqlStore{}).BootstrapAdmin(username, password)
}

func CreateUser(actorID UserID, input CreateUserInput) (PublicUser, error) {
	return NewService(mysqlStore{}).CreateUser(actorID, input)
}

func Login(username, password string) (LoginResult, error) {
	return NewService(mysqlStore{}).Login(username, password)
}

func LoginWithOptions(username, password string, options LoginOptions) (LoginResult, error) {
	return NewService(mysqlStore{}).LoginWithOptions(username, password, options)
}

func Authenticate(token string) (PublicUser, error) {
	return NewService(mysqlStore{}).Authenticate(token)
}

func AuthenticateContext(token string) (AuthContext, error) {
	return NewService(mysqlStore{}).AuthenticateContext(token)
}

func Logout(token string) error {
	return NewService(mysqlStore{}).Logout(token)
}

func SetUserStatus(actorID, userID UserID, status UserStatus) error {
	return NewService(mysqlStore{}).SetUserStatus(actorID, userID, status)
}

func ChangeOwnPassword(userID UserID, currentPassword, newPassword string) error {
	return NewService(mysqlStore{}).ChangeOwnPassword(userID, currentPassword, newPassword)
}

func ResetPassword(actorID, userID UserID, newPassword string) error {
	return NewService(mysqlStore{}).ResetPassword(actorID, userID, newPassword)
}

func ListUsers() ([]PublicUser, error) {
	return NewService(mysqlStore{}).ListUsers()
}

func ResolveUser(userID UserID) (PublicUser, bool, error) {
	return NewService(mysqlStore{}).ResolveUser(userID)
}

func ResolveGame(gameID string) (OperationGame, bool, error) {
	return NewService(mysqlStore{}).ResolveGame(gameID)
}

func ListAuditLogs(limit int) ([]AuditEvent, error) {
	return NewService(mysqlStore{}).ListAuditLogs(limit)
}

func CreateTeam(actorID UserID, name string) (OperationTeam, error) {
	return NewService(mysqlStore{}).CreateTeam(actorID, name)
}

func ListTeams(actorID UserID) ([]OperationTeam, error) {
	return NewService(mysqlStore{}).ListTeams(actorID)
}

func RenameTeam(actorID UserID, teamID TeamID, name string) (OperationTeam, error) {
	return NewService(mysqlStore{}).RenameTeam(actorID, teamID, name)
}

func DeleteTeam(actorID UserID, teamID TeamID) error {
	return NewService(mysqlStore{}).DeleteTeam(actorID, teamID)
}

func CreateGame(actorID UserID, id, name, remark string) (OperationGame, error) {
	return NewService(mysqlStore{}).CreateGame(actorID, id, name, remark)
}

func ListGames(actorID UserID) ([]OperationGame, error) {
	return NewService(mysqlStore{}).ListGames(actorID)
}

func GameReferences(actorID UserID, gameID string) (model.GameReferences, error) {
	return NewService(mysqlStore{}).GameReferences(actorID, gameID)
}

func UpdateGame(actorID UserID, id, name string, status GameStatus, remark string) (OperationGame, error) {
	return NewService(mysqlStore{}).UpdateGame(actorID, id, name, status, remark)
}

func DeleteGame(actorID UserID, id string) error {
	return NewService(mysqlStore{}).DeleteGame(actorID, id)
}

func DeleteUser(actorID, userID UserID) error {
	return NewService(mysqlStore{}).DeleteUser(actorID, userID)
}

func UpdateUserAccess(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string) (PublicUser, error) {
	return NewService(mysqlStore{}).UpdateUserAccess(actorID, userID, role, teamID, gameIDs)
}

func UpdateUser(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string, status UserStatus) (PublicUser, error) {
	return NewService(mysqlStore{}).UpdateUser(actorID, userID, role, teamID, gameIDs, status)
}

func CanAccessGame(userID UserID, gameID string) bool {
	return NewService(mysqlStore{}).CanAccessGame(userID, gameID)
}
