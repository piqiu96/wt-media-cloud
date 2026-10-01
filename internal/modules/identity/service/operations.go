// Package-level operations preserve the existing Service behavior while callers use the controlled database registry.
package service

import "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"

func BootstrapAdmin(username, password string) (PublicUser, error) {
	return newService(mysqlStore{}).BootstrapAdmin(username, password)
}

func CreateUser(actorID UserID, input CreateUserInput) (PublicUser, error) {
	return newService(mysqlStore{}).CreateUser(actorID, input)
}

func Login(username, password string) (LoginResult, error) {
	return newService(mysqlStore{}).Login(username, password)
}

func LoginWithOptions(username, password string, options LoginOptions) (LoginResult, error) {
	return newService(mysqlStore{}).LoginWithOptions(username, password, options)
}

func Authenticate(token string) (PublicUser, error) {
	return newService(mysqlStore{}).Authenticate(token)
}

func AuthenticateContext(token string) (AuthContext, error) {
	return newService(mysqlStore{}).AuthenticateContext(token)
}

func Logout(token string) error {
	return newService(mysqlStore{}).Logout(token)
}

func SetUserStatus(actorID, userID UserID, status UserStatus) error {
	return newService(mysqlStore{}).SetUserStatus(actorID, userID, status)
}

func ChangeOwnPassword(userID UserID, currentPassword, newPassword string) error {
	return newService(mysqlStore{}).ChangeOwnPassword(userID, currentPassword, newPassword)
}

func UpdateOwnProfile(userID UserID, nickname, avatarID string) (PublicUser, error) {
	return newService(mysqlStore{}).UpdateOwnProfile(userID, nickname, avatarID)
}

func ResetPassword(actorID, userID UserID, newPassword string) error {
	return newService(mysqlStore{}).ResetPassword(actorID, userID, newPassword)
}

func ListUsers() ([]PublicUser, error) {
	return newService(mysqlStore{}).ListUsers()
}

func ResolveUser(userID UserID) (PublicUser, bool, error) {
	return newService(mysqlStore{}).ResolveUser(userID)
}

func ResolveGame(gameID string) (OperationGame, bool, error) {
	return newService(mysqlStore{}).ResolveGame(gameID)
}

func ListAuditLogs(limit int) ([]AuditEvent, error) {
	return newService(mysqlStore{}).ListAuditLogs(limit)
}

func CreateTeam(actorID UserID, name string) (OperationTeam, error) {
	return newService(mysqlStore{}).CreateTeam(actorID, name)
}

func ListTeams(actorID UserID) ([]OperationTeam, error) {
	return newService(mysqlStore{}).ListTeams(actorID)
}

func RenameTeam(actorID UserID, teamID TeamID, name string) (OperationTeam, error) {
	return newService(mysqlStore{}).RenameTeam(actorID, teamID, name)
}

func DeleteTeam(actorID UserID, teamID TeamID) error {
	return newService(mysqlStore{}).DeleteTeam(actorID, teamID)
}

func CreateGame(actorID UserID, id, name, remark string) (OperationGame, error) {
	return newService(mysqlStore{}).CreateGame(actorID, id, name, remark)
}

func ListGames(actorID UserID) ([]OperationGame, error) {
	return newService(mysqlStore{}).ListGames(actorID)
}

func GameReferences(actorID UserID, gameID string) (model.GameReferences, error) {
	return newService(mysqlStore{}).GameReferences(actorID, gameID)
}

func UpdateGame(actorID UserID, id, name string, status GameStatus, remark string) (OperationGame, error) {
	return newService(mysqlStore{}).UpdateGame(actorID, id, name, status, remark)
}

func DeleteGame(actorID UserID, id string) error {
	return newService(mysqlStore{}).DeleteGame(actorID, id)
}

func DeleteUser(actorID, userID UserID) error {
	return newService(mysqlStore{}).DeleteUser(actorID, userID)
}

func UpdateUserAccess(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string) (PublicUser, error) {
	return newService(mysqlStore{}).UpdateUserAccess(actorID, userID, role, teamID, gameIDs)
}

func UpdateUser(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string, status UserStatus) (PublicUser, error) {
	return newService(mysqlStore{}).UpdateUser(actorID, userID, role, teamID, gameIDs, status)
}

func CanAccessGame(userID UserID, gameID string) bool {
	return newService(mysqlStore{}).CanAccessGame(userID, gameID)
}
