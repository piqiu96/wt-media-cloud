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

func AuthenticateAgentSession(sessionID string) (AuthContext, error) {
	return newService(mysqlStore{}).AuthenticateAgentSession(sessionID)
}

// AuditAgentTokenIssued records the issuance event only. The token itself is
// never persisted, so the audit summary must never contain it.
func AuditAgentTokenIssued(userID UserID, sessionID string) error {
	return newService(mysqlStore{}).audit(userID, "local_agent.agent_token.issue", "user", userID, map[string]string{
		"session_id": sessionID,
	})
}

func Logout(token string) error {
	return newService(mysqlStore{}).Logout(token)
}

func ForceLogout(actorID, userID UserID) error {
	return newService(mysqlStore{}).ForceLogout(actorID, userID)
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

func ResolveCategory(categoryID CategoryID) (Category, bool, error) {
	return newService(mysqlStore{}).ResolveCategory(categoryID)
}

func ListAuditLogs(limit int) ([]AuditEvent, error) {
	return newService(mysqlStore{}).ListAuditLogs(limit)
}

func CreateTeam(actorID UserID, name string) (OperationTeam, error) {
	return newService(mysqlStore{}).CreateTeam(actorID, name)
}

func ListTeams(actorID UserID, keyword string) ([]OperationTeam, error) {
	return newService(mysqlStore{}).ListTeams(actorID, keyword)
}

func RenameTeam(actorID UserID, teamID TeamID, name string) (OperationTeam, error) {
	return newService(mysqlStore{}).RenameTeam(actorID, teamID, name)
}

func DeleteTeam(actorID UserID, teamID TeamID) error {
	return newService(mysqlStore{}).DeleteTeam(actorID, teamID)
}

func CreateCategory(actorID UserID, categoryType CategoryType, name, remark string) (Category, error) {
	return newService(mysqlStore{}).CreateCategory(actorID, categoryType, name, remark)
}

func ListCategories(actorID UserID) ([]Category, error) {
	return newService(mysqlStore{}).ListCategories(actorID)
}

func ListAllCategories(actorID UserID) ([]Category, error) {
	return newService(mysqlStore{}).ListAllCategories(actorID)
}

func ListManagementCategories(actorID UserID) ([]Category, error) {
	return newService(mysqlStore{}).ListManagementCategories(actorID)
}

func CategoryReferences(actorID UserID, categoryID CategoryID) (model.CategoryReferences, error) {
	return newService(mysqlStore{}).CategoryReferences(actorID, categoryID)
}

func UpdateCategory(actorID UserID, id CategoryID, name string, enabled bool, remark string) (Category, error) {
	return newService(mysqlStore{}).UpdateCategory(actorID, id, name, enabled, remark)
}

func DeleteCategory(actorID UserID, id CategoryID) error {
	return newService(mysqlStore{}).DeleteCategory(actorID, id)
}

func DeleteUser(actorID, userID UserID) error {
	return newService(mysqlStore{}).DeleteUser(actorID, userID)
}

func UpdateUserAccess(actorID, userID UserID, role Role, teamID *TeamID, categoryIDs []CategoryID) (PublicUser, error) {
	return newService(mysqlStore{}).UpdateUserAccess(actorID, userID, role, teamID, categoryIDs)
}

func UpdateUser(actorID, userID UserID, role Role, teamID *TeamID, categoryIDs []CategoryID, status UserStatus) (PublicUser, error) {
	return newService(mysqlStore{}).UpdateUser(actorID, userID, role, teamID, categoryIDs, status)
}

func CanAccessCategory(userID UserID, categoryID CategoryID) bool {
	return newService(mysqlStore{}).CanAccessCategory(userID, categoryID)
}
