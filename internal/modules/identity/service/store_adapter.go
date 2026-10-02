package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/repository"
)

type mysqlStore struct{}

func (mysqlStore) CountUsers() (int, error) { return repository.CountUsers() }
func (mysqlStore) CreateUser(user model.User) (model.UserID, error) {
	return repository.CreateUser(user)
}
func (mysqlStore) CreateUserWithAudit(user model.User, event model.AuditEvent) (model.UserID, error) {
	return repository.CreateUserWithAudit(user, event)
}
func (mysqlStore) FindUser(id model.UserID) (model.User, bool, error) {
	return repository.FindUser(id)
}
func (mysqlStore) FindUserByUsername(username string) (model.User, bool, error) {
	return repository.FindUserByUsername(username)
}

func (mysqlStore) UpdateOwnProfile(userID model.UserID, nickname, avatarID string, at time.Time, event model.AuditEvent) error {
	return repository.UpdateOwnProfile(userID, nickname, avatarID, at, event)
}
func (mysqlStore) ListUsers() ([]model.User, error) { return repository.ListUsers() }
func (mysqlStore) UpdateUser(user model.User) error { return repository.UpdateUser(user) }
func (mysqlStore) UpdateUserAndInvalidateSessions(user model.User, event model.AuditEvent) error {
	return repository.UpdateUserAndInvalidateSessions(user, event)
}
func (mysqlStore) DeleteUser(userID model.UserID) error { return repository.DeleteUser(userID) }
func (mysqlStore) DeleteUserWithAudit(userID model.UserID, event model.AuditEvent) error {
	return repository.DeleteUserWithAudit(userID, event)
}
func (mysqlStore) CreateTeam(team model.OperationTeam) (model.TeamID, error) {
	return repository.CreateTeam(team)
}
func (mysqlStore) CreateTeamWithAudit(team model.OperationTeam, event model.AuditEvent) (model.TeamID, error) {
	return repository.CreateTeamWithAudit(team, event)
}
func (mysqlStore) FindTeam(teamID model.TeamID) (model.OperationTeam, bool, error) {
	return repository.FindTeam(teamID)
}
func (mysqlStore) ListTeams() ([]model.OperationTeam, error) { return repository.ListTeams() }
func (mysqlStore) UpdateTeam(team model.OperationTeam) error { return repository.UpdateTeam(team) }
func (mysqlStore) UpdateTeamWithAudit(team model.OperationTeam, event model.AuditEvent) error {
	return repository.UpdateTeamWithAudit(team, event)
}
func (mysqlStore) DeleteTeam(teamID model.TeamID) error { return repository.DeleteTeam(teamID) }
func (mysqlStore) DeleteTeamWithAudit(teamID model.TeamID, event model.AuditEvent) error {
	return repository.DeleteTeamWithAudit(teamID, event)
}
func (mysqlStore) TeamHasReferences(teamID model.TeamID) (bool, error) {
	return repository.TeamHasReferences(teamID)
}
func (mysqlStore) CreateGameWithAudit(game model.OperationGame, event model.AuditEvent) error {
	return repository.CreateGameWithAudit(game, event)
}
func (mysqlStore) FindGame(id string) (model.OperationGame, bool, error) {
	return repository.FindGame(id)
}
func (mysqlStore) ListGames() ([]model.OperationGame, error) { return repository.ListGames() }
func (mysqlStore) UpdateGameWithAudit(game model.OperationGame, event model.AuditEvent) error {
	return repository.UpdateGameWithAudit(game, event)
}
func (mysqlStore) DeleteGameWithAudit(id string, event model.AuditEvent) error {
	return repository.DeleteGameWithAudit(id, event)
}
func (mysqlStore) GameHasReferences(id string) (bool, error) {
	return repository.GameHasReferences(id)
}
func (mysqlStore) ListGameReferenceSummaries() (map[string]model.GameReferenceSummary, error) {
	return repository.ListGameReferenceSummaries()
}
func (mysqlStore) GameReferences(gameID string) (model.GameReferences, error) {
	return repository.GameReferences(gameID)
}
func (mysqlStore) UserHasGameReferencesOutsideScope(userID model.UserID, gameIDs []string) (bool, error) {
	return repository.UserHasGameReferencesOutsideScope(userID, gameIDs)
}
func (mysqlStore) CreateSession(session model.Session) error {
	return repository.CreateSession(session)
}
func (mysqlStore) FindSessionByTokenHash(tokenHash string) (model.Session, bool, error) {
	return repository.FindSessionByTokenHash(tokenHash)
}
func (mysqlStore) HasActiveSessionForClientType(userID model.UserID, clientType model.ClientType) (bool, error) {
	return repository.HasActiveSessionForClientType(userID, clientType)
}
func (mysqlStore) InvalidateUserSessionsForClientType(userID model.UserID, clientType model.ClientType, at time.Time) error {
	return repository.InvalidateUserSessionsForClientType(userID, clientType, at)
}
func (mysqlStore) InvalidateSessionByID(sessionID string, at time.Time) error {
	return repository.InvalidateSessionByID(sessionID, at)
}
func (mysqlStore) AppendAudit(event model.AuditEvent) error { return repository.AppendAudit(event) }
func (mysqlStore) ListAuditLogs(limit int) ([]model.AuditEvent, error) {
	return repository.ListAuditLogs(limit)
}
