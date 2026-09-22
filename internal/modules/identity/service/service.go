// Package identity owns Cloud users, roles, game scopes, and sessions.
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"golang.org/x/crypto/bcrypt"
)

const (
	RoleOperator       = model.RoleOperator
	RoleSeniorOperator = model.RoleSeniorOperator
	RoleAdmin          = model.RoleAdmin

	UserStatusEnabled  = model.UserStatusEnabled
	UserStatusDisabled = model.UserStatusDisabled

	GameStatusEnabled  = model.GameStatusEnabled
	GameStatusDisabled = model.GameStatusDisabled
)

type (
	Role                 = model.Role
	UserID               = model.UserID
	TeamID               = model.TeamID
	UserStatus           = model.UserStatus
	OperationTeam        = model.OperationTeam
	GameStatus           = model.GameStatus
	OperationGame        = model.OperationGame
	GameReferenceSummary = model.GameReferenceSummary
	GameReferenceUser    = model.GameReferenceUser
	GameReferenceAccount = model.GameReferenceAccount
	User                 = model.User
	PublicUser           = model.PublicUser
	Session              = model.Session
	AuditEvent           = model.AuditEvent
)

type (
	LoginResult     = dto.LoginResult
	LoginOptions    = dto.LoginOptions
	AuthContext     = dto.AuthContext
	CreateUserInput = dto.CreateUserInput
)

var (
	ErrAuthenticationFailed = model.ErrAuthenticationFailed
	ErrSessionInvalid       = model.ErrSessionInvalid
	ErrForbidden            = model.ErrForbidden
	ErrInvalidInput         = model.ErrInvalidInput
	ErrUsernameTaken        = model.ErrUsernameTaken
	ErrTeamNameTaken        = model.ErrTeamNameTaken
	ErrTeamInUse            = model.ErrTeamInUse
	ErrGameIDTaken          = model.ErrGameIDTaken
	ErrInvalidGameID        = model.ErrInvalidGameID
	ErrGameNameTaken        = model.ErrGameNameTaken
	ErrGameInUse            = model.ErrGameInUse
	ErrUserGameScopeInUse   = model.ErrUserGameScopeInUse
	ErrGameUnavailable      = model.ErrGameUnavailable
	ErrPasswordTooShort     = model.ErrPasswordTooShort
	ErrBootstrapUnavailable = model.ErrBootstrapUnavailable
	ErrSessionReplaceNeeded = model.ErrSessionReplaceNeeded
)

type Store interface {
	CountUsers() (int, error)
	CreateUser(User) (UserID, error)
	CreateUserWithAudit(User, AuditEvent) (UserID, error)
	FindUser(id UserID) (User, bool, error)
	FindUserByUsername(username string) (User, bool, error)
	ListUsers() ([]User, error)
	UpdateUser(User) error
	UpdateUserAndInvalidateSessions(User, AuditEvent) error
	DeleteUser(UserID) error
	DeleteUserWithAudit(UserID, AuditEvent) error
	CreateTeam(OperationTeam) (TeamID, error)
	CreateTeamWithAudit(OperationTeam, AuditEvent) (TeamID, error)
	FindTeam(TeamID) (OperationTeam, bool, error)
	ListTeams() ([]OperationTeam, error)
	UpdateTeam(OperationTeam) error
	UpdateTeamWithAudit(OperationTeam, AuditEvent) error
	DeleteTeam(TeamID) error
	DeleteTeamWithAudit(TeamID, AuditEvent) error
	TeamHasReferences(TeamID) (bool, error)
	CreateSession(Session) error
	FindSessionByTokenHash(tokenHash string) (Session, bool, error)
	HasActiveSession(userID UserID) (bool, error)
	InvalidateUserSessions(userID UserID, at time.Time) error
	AppendAudit(AuditEvent) error
	ListAuditLogs(limit int) ([]AuditEvent, error)
}

type operationGameStore interface {
	CreateGameWithAudit(OperationGame, AuditEvent) error
	FindGame(string) (OperationGame, bool, error)
	ListGames() ([]OperationGame, error)
	UpdateGameWithAudit(OperationGame, AuditEvent) error
	DeleteGameWithAudit(string, AuditEvent) error
	GameHasReferences(string) (bool, error)
}

type gameReferenceStore interface {
	ListGameReferenceSummaries() (map[string]GameReferenceSummary, error)
	GameReferences(string) (model.GameReferences, error)
	UserHasGameReferencesOutsideScope(UserID, []string) (bool, error)
}

type Service struct {
	store     Store
	now       func() time.Time
	newID     func(string) string
	newToken  func() string
	passwords passwordHasher
}

type passwordHasher interface {
	Hash(password []byte, cost int) ([]byte, error)
	Compare(hash, password []byte) error
}

type bcryptHasher struct{}

func (bcryptHasher) Hash(password []byte, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, cost)
}

func (bcryptHasher) Compare(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(service *Service) { service.now = now }
}

func WithTokenGenerator(newToken func() string) Option {
	return func(service *Service) { service.newToken = newToken }
}

func newService(store Store, options ...Option) *Service {
	service := &Service{
		store:     store,
		now:       func() time.Time { return time.Now().UTC() },
		newID:     id.NewID,
		newToken:  id.NewToken,
		passwords: bcryptHasher{},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) BootstrapAdmin(username, password string) (PublicUser, error) {
	count, err := s.store.CountUsers()
	if err != nil {
		return PublicUser{}, err
	}
	if count != 0 {
		return PublicUser{}, ErrBootstrapUnavailable
	}
	user, teamName, err := s.prepareUser(CreateUserInput{Username: username, Password: password, Role: RoleAdmin})
	if err != nil {
		return PublicUser{}, err
	}
	event := s.newAuditEvent(0, "user.bootstrap", "user", 0, map[string]string{"role": string(user.Role)})
	userID, err := s.store.CreateUserWithAudit(user, event)
	if err != nil {
		return PublicUser{}, err
	}
	user.ID, user.TeamName = userID, teamName
	return publicUser(user), nil
}

func (s *Service) CreateUser(actorID UserID, input CreateUserInput) (PublicUser, error) {
	actor, _, err := s.store.FindUser(actorID)
	if err != nil {
		return PublicUser{}, err
	}
	if !isAdmin(actor) {
		return PublicUser{}, ErrForbidden
	}
	user, teamName, err := s.prepareUser(input)
	if err != nil {
		return PublicUser{}, err
	}
	event := s.newAuditEvent(actor.ID, "user.create", "user", 0, map[string]string{
		"role": string(user.Role), "username": user.Username, "team_id": formatTeamID(user.TeamID), "game_ids": strings.Join(user.GameIDs, ","),
	})
	userID, err := s.store.CreateUserWithAudit(user, event)
	if err != nil {
		return PublicUser{}, err
	}
	user.ID, user.TeamName = userID, teamName
	return publicUser(user), nil
}

func (s *Service) prepareUser(input CreateUserInput) (User, string, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || len(username) > 64 || !validRole(input.Role) {
		return User{}, "", ErrInvalidInput
	}
	if len(input.Password) < 6 {
		return User{}, "", ErrPasswordTooShort
	}
	_, found, err := s.store.FindUserByUsername(username)
	if err != nil {
		return User{}, "", err
	}
	if found {
		return User{}, "", ErrUsernameTaken
	}
	hash, err := s.passwords.Hash([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, "", err
	}
	gameIDs := normalizeGameIDs(input.GameIDs)
	teamName := ""
	if input.Role == RoleAdmin {
		if input.TeamID != nil || len(gameIDs) != 0 {
			return User{}, "", ErrInvalidInput
		}
	} else {
		if input.TeamID == nil || *input.TeamID <= 0 || len(gameIDs) == 0 {
			return User{}, "", ErrInvalidInput
		}
		team, found, err := s.store.FindTeam(*input.TeamID)
		if err != nil {
			return User{}, "", err
		}
		if !found {
			return User{}, "", ErrInvalidInput
		}
		teamName = team.Name
		if err := s.ensureGamesAssignable(gameIDs); err != nil {
			return User{}, "", err
		}
	}
	now := s.now()
	user := User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         input.Role,
		Status:       UserStatusEnabled,
		TeamID:       cloneTeamID(input.TeamID),
		GameIDs:      gameIDs,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return user, teamName, nil
}

func (s *Service) Login(username, password string) (LoginResult, error) {
	return s.LoginWithOptions(username, password, LoginOptions{ReplaceExisting: true})
}

func (s *Service) LoginWithOptions(username, password string, options LoginOptions) (LoginResult, error) {
	user, ok, err := s.store.FindUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || user.Status != UserStatusEnabled || s.passwords.Compare([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginResult{}, ErrAuthenticationFailed
	}
	now := s.now()
	hasActive, err := s.store.HasActiveSession(user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if hasActive && !options.ReplaceExisting {
		return LoginResult{}, ErrSessionReplaceNeeded
	}
	if err := s.store.InvalidateUserSessions(user.ID, now); err != nil {
		return LoginResult{}, err
	}
	token := s.newToken()
	if err := s.store.CreateSession(Session{
		ID:        s.newID("session"),
		UserID:    user.ID,
		TokenHash: tokenHash(token),
		CreatedAt: now,
	}); err != nil {
		return LoginResult{}, err
	}
	if err := s.audit(user.ID, "user.login", "user", user.ID, map[string]string{"username": user.Username, "replace_existing": strconv.FormatBool(hasActive)}); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: publicUser(user)}, nil
}

func (s *Service) Authenticate(token string) (PublicUser, error) {
	context, err := s.AuthenticateContext(token)
	if err != nil {
		return PublicUser{}, err
	}
	return context.User, nil
}

func (s *Service) AuthenticateContext(token string) (AuthContext, error) {
	if token == "" {
		return AuthContext{}, ErrSessionInvalid
	}
	session, ok, err := s.store.FindSessionByTokenHash(tokenHash(token))
	if err != nil {
		return AuthContext{}, err
	}
	if !ok || session.InvalidAt != nil {
		return AuthContext{}, ErrSessionInvalid
	}
	user, ok, err := s.store.FindUser(session.UserID)
	if err != nil {
		return AuthContext{}, err
	}
	if !ok || user.Status != UserStatusEnabled {
		return AuthContext{}, ErrSessionInvalid
	}
	return AuthContext{User: publicUser(user), Session: session}, nil
}

func (s *Service) Logout(token string) error {
	if token == "" {
		return ErrSessionInvalid
	}
	session, ok, err := s.store.FindSessionByTokenHash(tokenHash(token))
	if err != nil {
		return err
	}
	if !ok || session.InvalidAt != nil {
		return ErrSessionInvalid
	}
	return s.store.InvalidateUserSessions(session.UserID, s.now())
}

func (s *Service) SetUserStatus(actorID, userID UserID, status UserStatus) error {
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidInput
	}
	_, err = s.updateUser(actorID, userID, user.Role, user.TeamID, user.GameIDs, status, "user.status.update")
	return err
}

func (s *Service) ChangeOwnPassword(userID UserID, currentPassword, newPassword string) error {
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return err
	}
	if !ok || user.Status != UserStatusEnabled || s.passwords.Compare([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return ErrAuthenticationFailed
	}
	updated, err := s.passwordUpdatedUser(user, newPassword)
	if err != nil {
		return err
	}
	return s.store.UpdateUserAndInvalidateSessions(updated, s.newAuditEvent(user.ID, "user.password.change", "user", user.ID, map[string]string{"result": "changed"}))
}

func (s *Service) ResetPassword(actorID, userID UserID, newPassword string) error {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return err
	}
	if !isAdmin(actor) {
		return ErrForbidden
	}
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidInput
	}
	updated, err := s.passwordUpdatedUser(user, newPassword)
	if err != nil {
		return err
	}
	return s.store.UpdateUserAndInvalidateSessions(updated, s.newAuditEvent(actor.ID, "user.password.reset", "user", user.ID, map[string]string{"result": "reset"}))
}

func (s *Service) passwordUpdatedUser(user User, newPassword string) (User, error) {
	if len(newPassword) < 6 {
		return User{}, ErrPasswordTooShort
	}
	hash, err := s.passwords.Hash([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	user.PasswordHash = string(hash)
	user.UpdatedAt = s.now()
	return user, nil
}

func (s *Service) ensureGamesAssignable(gameIDs []string) error {
	gameStore, ok := s.store.(operationGameStore)
	if !ok {
		return nil
	}
	for _, gameID := range gameIDs {
		game, found, err := gameStore.FindGame(gameID)
		if err != nil {
			return err
		}
		if !found || game.Status != GameStatusEnabled {
			return ErrGameUnavailable
		}
	}
	return nil
}

func (s *Service) ListUsers() ([]PublicUser, error) {
	users, err := s.store.ListUsers()
	if err != nil {
		return nil, err
	}
	result := make([]PublicUser, 0, len(users))
	for _, u := range users {
		result = append(result, publicUser(u))
	}
	return result, nil
}

// ResolveUser exposes a secret-free user record to trusted in-process modules.
func (s *Service) ResolveUser(userID UserID) (PublicUser, bool, error) {
	user, found, err := s.store.FindUser(userID)
	if err != nil || !found {
		return PublicUser{}, found, err
	}
	return publicUser(user), true, nil
}

// ResolveGame exposes an operation-game fact to trusted in-process modules.
// It intentionally does not apply the caller's user game-scope permissions.
func (s *Service) ResolveGame(gameID string) (OperationGame, bool, error) {
	gameStore, ok := s.store.(operationGameStore)
	if !ok {
		return OperationGame{}, false, nil
	}
	return gameStore.FindGame(strings.TrimSpace(gameID))
}

func (s *Service) ListAuditLogs(limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListAuditLogs(limit)
}

func (s *Service) CreateTeam(actorID UserID, name string) (OperationTeam, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return OperationTeam{}, err
	}
	name = strings.TrimSpace(name)
	if !ok || !isAdmin(actor) {
		return OperationTeam{}, ErrForbidden
	}
	if name == "" || len(name) > 64 {
		return OperationTeam{}, ErrInvalidInput
	}
	now := s.now()
	team := OperationTeam{Name: name, CreatedAt: now, UpdatedAt: now}
	event := s.newAuditEvent(actor.ID, "operation_team.create", "operation_team", 0, map[string]string{"name": team.Name})
	teamID, err := s.store.CreateTeamWithAudit(team, event)
	if err != nil {
		return OperationTeam{}, err
	}
	team.ID = teamID
	return team, nil
}

func (s *Service) ListTeams(actorID UserID) ([]OperationTeam, error) {
	_, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	return s.store.ListTeams()
}

func (s *Service) RenameTeam(actorID UserID, teamID TeamID, name string) (OperationTeam, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return OperationTeam{}, err
	}
	name = strings.TrimSpace(name)
	if !ok || !isAdmin(actor) {
		return OperationTeam{}, ErrForbidden
	}
	if teamID <= 0 || name == "" || len(name) > 64 {
		return OperationTeam{}, ErrInvalidInput
	}
	team, found, err := s.store.FindTeam(teamID)
	if err != nil {
		return OperationTeam{}, err
	}
	if !found {
		return OperationTeam{}, ErrInvalidInput
	}
	team.Name = name
	team.UpdatedAt = s.now()
	event := s.newAuditEvent(actor.ID, "operation_team.rename", "operation_team", UserID(team.ID), map[string]string{"name": team.Name})
	if err := s.store.UpdateTeamWithAudit(team, event); err != nil {
		return OperationTeam{}, err
	}
	return team, nil
}

func (s *Service) DeleteTeam(actorID UserID, teamID TeamID) error {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return err
	}
	if !ok || !isAdmin(actor) {
		return ErrForbidden
	}
	if _, found, err := s.store.FindTeam(teamID); err != nil {
		return err
	} else if !found {
		return ErrInvalidInput
	}
	referenced, err := s.store.TeamHasReferences(teamID)
	if err != nil {
		return err
	}
	if referenced {
		return ErrTeamInUse
	}
	event := s.newAuditEvent(actor.ID, "operation_team.delete", "operation_team", UserID(teamID), map[string]string{"result": "deleted"})
	return s.store.DeleteTeamWithAudit(teamID, event)
}

func (s *Service) CreateGame(actorID UserID, id, name, remark string) (OperationGame, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return OperationGame{}, err
	}
	gameStore, okStore := s.store.(operationGameStore)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	remark = strings.TrimSpace(remark)
	if !ok || !isAdmin(actor) || !okStore || name == "" || len(name) > 128 || len(remark) > 255 {
		return OperationGame{}, ErrInvalidInput
	}
	if !validGameID(id) {
		return OperationGame{}, ErrInvalidGameID
	}
	if _, exists, err := gameStore.FindGame(id); err != nil {
		return OperationGame{}, err
	} else if exists {
		return OperationGame{}, ErrGameIDTaken
	}
	now := s.now()
	game := OperationGame{ID: id, Name: name, Status: GameStatusEnabled, Remark: remark, CreatedAt: now, UpdatedAt: now}
	event := s.newAuditEvent(actor.ID, "operation_game.create", "operation_game", 0, map[string]string{"game_id": id, "name": name})
	if err := gameStore.CreateGameWithAudit(game, event); err != nil {
		return OperationGame{}, err
	}
	return game, nil
}

func (s *Service) ListGames(actorID UserID) ([]OperationGame, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	gameStore, okStore := s.store.(operationGameStore)
	if !okStore {
		return nil, ErrForbidden
	}
	games, err := gameStore.ListGames()
	if err != nil {
		return nil, err
	}
	if !isAdmin(actor) {
		enabled := make([]OperationGame, 0, len(games))
		for _, game := range games {
			if game.Status == GameStatusEnabled && containsGame(actor.GameIDs, game.ID) {
				enabled = append(enabled, game)
			}
		}
		return enabled, nil
	}
	if references, ok := s.store.(gameReferenceStore); ok {
		summaries, err := references.ListGameReferenceSummaries()
		if err != nil {
			return nil, err
		}
		for index := range games {
			summary := summaries[games[index].ID]
			summary.TotalCount = summary.UserScopeCount + summary.MediaAccountCount
			games[index].ReferenceSummary = &summary
		}
	}
	return games, nil
}

func (s *Service) GameReferences(actorID UserID, gameID string) (model.GameReferences, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return model.GameReferences{}, err
	}
	if !ok || !isAdmin(actor) {
		return model.GameReferences{}, ErrForbidden
	}
	gameStore, ok := s.store.(operationGameStore)
	if !ok {
		return model.GameReferences{}, ErrForbidden
	}
	if _, found, err := gameStore.FindGame(strings.TrimSpace(gameID)); err != nil {
		return model.GameReferences{}, err
	} else if !found {
		return model.GameReferences{}, ErrInvalidInput
	}
	references, ok := s.store.(gameReferenceStore)
	if !ok {
		return model.GameReferences{}, ErrForbidden
	}
	return references.GameReferences(strings.TrimSpace(gameID))
}

func (s *Service) UpdateGame(actorID UserID, id, name string, status GameStatus, remark string) (OperationGame, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return OperationGame{}, err
	}
	gameStore, okStore := s.store.(operationGameStore)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	remark = strings.TrimSpace(remark)
	if !ok || !isAdmin(actor) || !okStore || id == "" || name == "" || len(name) > 128 || len(remark) > 255 || !validGameStatus(status) {
		return OperationGame{}, ErrInvalidInput
	}
	game, found, err := gameStore.FindGame(id)
	if err != nil {
		return OperationGame{}, err
	}
	if !found {
		return OperationGame{}, ErrInvalidInput
	}
	if game.Status != GameStatusDisabled && status == GameStatusDisabled {
		referenced, err := gameStore.GameHasReferences(id)
		if err != nil {
			return OperationGame{}, err
		}
		if referenced {
			return OperationGame{}, ErrGameInUse
		}
	}
	game.Name = name
	game.Status = status
	game.Remark = remark
	game.UpdatedAt = s.now()
	event := s.newAuditEvent(actor.ID, "operation_game.update", "operation_game", 0, map[string]string{"game_id": id, "status": string(status)})
	if err := gameStore.UpdateGameWithAudit(game, event); err != nil {
		return OperationGame{}, err
	}
	return game, nil
}

func (s *Service) DeleteGame(actorID UserID, id string) error {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return err
	}
	gameStore, okStore := s.store.(operationGameStore)
	id = strings.TrimSpace(id)
	if !ok || !isAdmin(actor) || !okStore || id == "" {
		return ErrForbidden
	}
	if _, found, err := gameStore.FindGame(id); err != nil {
		return err
	} else if !found {
		return ErrInvalidInput
	}
	referenced, err := gameStore.GameHasReferences(id)
	if err != nil {
		return err
	}
	if referenced {
		return ErrGameInUse
	}
	event := s.newAuditEvent(actor.ID, "operation_game.delete", "operation_game", 0, map[string]string{"game_id": id})
	return gameStore.DeleteGameWithAudit(id, event)
}

func (s *Service) DeleteUser(actorID, userID UserID) error {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return err
	}
	if !ok || !isAdmin(actor) || actorID == userID {
		return ErrForbidden
	}
	if _, found, err := s.store.FindUser(userID); err != nil {
		return err
	} else if !found {
		return ErrInvalidInput
	}
	event := s.newAuditEvent(actor.ID, "user.delete", "user", userID, map[string]string{"result": "deleted"})
	return s.store.DeleteUserWithAudit(userID, event)
}

func (s *Service) UpdateUserAccess(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string) (PublicUser, error) {
	return s.updateUser(actorID, userID, role, teamID, gameIDs, "", "user.access.update")
}

// UpdateUser applies the complete admin form as one validated transaction so
// callers never observe a partial role/team/game/status update.
func (s *Service) UpdateUser(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string, status UserStatus) (PublicUser, error) {
	return s.updateUser(actorID, userID, role, teamID, gameIDs, status, "user.update")
}

func (s *Service) updateUser(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string, status UserStatus, action string) (PublicUser, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return PublicUser{}, err
	}
	if !isAdmin(actor) {
		return PublicUser{}, ErrForbidden
	}
	if !validRole(role) {
		return PublicUser{}, ErrInvalidInput
	}
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return PublicUser{}, err
	}
	if !ok {
		return PublicUser{}, ErrInvalidInput
	}
	if status == "" {
		status = user.Status
	}
	if !validStatus(status) || (actorID == userID && status != user.Status) {
		return PublicUser{}, ErrInvalidInput
	}
	normalizedGameIDs := normalizeGameIDs(gameIDs)
	teamName := ""
	if role == RoleAdmin {
		if teamID != nil || len(normalizedGameIDs) != 0 {
			return PublicUser{}, ErrInvalidInput
		}
	} else {
		if teamID == nil || *teamID <= 0 || len(normalizedGameIDs) == 0 {
			return PublicUser{}, ErrInvalidInput
		}
		if team, found, err := s.store.FindTeam(*teamID); err != nil {
			return PublicUser{}, err
		} else if !found {
			return PublicUser{}, ErrInvalidInput
		} else {
			teamName = team.Name
		}
		if err := s.ensureGamesAssignable(normalizedGameIDs); err != nil {
			return PublicUser{}, err
		}
		if references, ok := s.store.(gameReferenceStore); ok {
			inUse, err := references.UserHasGameReferencesOutsideScope(user.ID, normalizedGameIDs)
			if err != nil {
				return PublicUser{}, err
			}
			if inUse {
				return PublicUser{}, ErrUserGameScopeInUse
			}
		}
	}
	user.Role = role
	user.Status = status
	user.TeamID = cloneTeamID(teamID)
	user.GameIDs = normalizedGameIDs
	user.UpdatedAt = s.now()
	event := s.newAuditEvent(actor.ID, action, "user", user.ID, map[string]string{
		"role": string(role), "status": string(status), "team_id": formatTeamID(user.TeamID), "game_ids": strings.Join(user.GameIDs, ","),
	})
	if err := s.store.UpdateUserAndInvalidateSessions(user, event); err != nil {
		return PublicUser{}, err
	}
	if user.TeamID != nil {
		user.TeamName = teamName
	} else {
		user.TeamName = ""
	}
	return publicUser(user), nil
}

func (s *Service) audit(actorID UserID, action, targetType string, targetID UserID, summary map[string]string) error {
	return s.store.AppendAudit(s.newAuditEvent(actorID, action, targetType, targetID, summary))
}

func (s *Service) newAuditEvent(actorID UserID, action, targetType string, targetID UserID, summary map[string]string) AuditEvent {
	return AuditEvent{
		ID:          s.newID("audit"),
		ActorUserID: actorID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    strconv.FormatInt(int64(targetID), 10),
		Summary:     summary,
		CreatedAt:   s.now(),
	}
}

func (s *Service) CanAccessGame(userID UserID, gameID string) bool {
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return false
	}
	if !ok || user.Status != UserStatusEnabled || strings.TrimSpace(gameID) == "" {
		return false
	}
	if user.Role == RoleAdmin {
		return true
	}
	for _, assigned := range user.GameIDs {
		if assigned == gameID {
			return true
		}
	}
	return false
}

func publicUser(user User) PublicUser {
	return PublicUser{ID: user.ID, Username: user.Username, Role: user.Role, Status: user.Status, TeamID: cloneTeamID(user.TeamID), TeamName: user.TeamName, GameIDs: append([]string(nil), user.GameIDs...)}
}

func validRole(role Role) bool {
	return role == RoleOperator || role == RoleSeniorOperator || role == RoleAdmin
}

func isAdmin(user User) bool {
	return user.Status == UserStatusEnabled && user.Role == RoleAdmin
}

func cloneTeamID(value *TeamID) *TeamID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func formatTeamID(value *TeamID) string {
	if value == nil {
		return "none"
	}
	return strconv.FormatInt(int64(*value), 10)
}

func validStatus(status UserStatus) bool {
	return status == UserStatusEnabled || status == UserStatusDisabled
}

func validGameStatus(status GameStatus) bool {
	return status == GameStatusEnabled || status == GameStatusDisabled
}

func validGameID(id string) bool {
	if id == "" || len(id) > 32 {
		return false
	}
	for _, ch := range id {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		return false
	}
	return true
}

func normalizeGameIDs(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			unique[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func containsGame(gameIDs []string, gameID string) bool {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return false
	}
	for _, assigned := range gameIDs {
		if assigned == gameID {
			return true
		}
	}
	return false
}

func tokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

type memoryStore struct {
	mu         sync.RWMutex
	users      map[UserID]User
	byName     map[string]UserID
	teams      map[TeamID]OperationTeam
	teamNames  map[string]TeamID
	nextUserID UserID
	nextTeamID TeamID
	sessions   map[string]Session
	auditLogs  []AuditEvent
}

func NewMemoryStore() *memoryStore {
	return &memoryStore{
		users:      make(map[UserID]User),
		byName:     make(map[string]UserID),
		teams:      make(map[TeamID]OperationTeam),
		teamNames:  make(map[string]TeamID),
		nextUserID: 1,
		nextTeamID: 1,
		sessions:   make(map[string]Session),
	}
}

func (s *memoryStore) CountUsers() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users), nil
}

func (s *memoryStore) CreateUser(user User) (UserID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byName[user.Username]; exists {
		return 0, ErrUsernameTaken
	}
	user.ID = s.nextUserID
	s.nextUserID++
	if user.TeamID != nil {
		team, ok := s.teams[*user.TeamID]
		if !ok {
			return 0, ErrInvalidInput
		}
		user.TeamName = team.Name
	}
	s.users[user.ID] = cloneUser(user)
	s.byName[user.Username] = user.ID
	return user.ID, nil
}

func (s *memoryStore) CreateUserWithAudit(user User, event AuditEvent) (UserID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byName[user.Username]; exists {
		return 0, ErrUsernameTaken
	}
	user.ID = s.nextUserID
	s.nextUserID++
	if user.TeamID != nil {
		team, ok := s.teams[*user.TeamID]
		if !ok {
			return 0, ErrInvalidInput
		}
		user.TeamName = team.Name
	}
	if event.ActorUserID == 0 {
		event.ActorUserID = user.ID
	}
	if event.TargetID == "" || event.TargetID == "0" {
		event.TargetID = strconv.FormatInt(int64(user.ID), 10)
	}
	s.users[user.ID] = cloneUser(user)
	s.byName[user.Username] = user.ID
	s.auditLogs = append(s.auditLogs, cloneAuditEvent(event))
	return user.ID, nil
}

func (s *memoryStore) FindUser(id UserID) (User, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	return cloneUser(user), ok, nil
}

func (s *memoryStore) FindUserByUsername(username string) (User, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byName[username]
	if !ok {
		return User{}, false, nil
	}
	return cloneUser(s.users[id]), true, nil
}

func (s *memoryStore) ListUsers() ([]User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]User, 0, len(s.users))
	for _, u := range s.users {
		if u.TeamID != nil {
			u.TeamName = s.teams[*u.TeamID].Name
		}
		result = append(result, u)
	}
	return result, nil
}

func (s *memoryStore) UpdateUser(user User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[user.ID]; !exists {
		return ErrInvalidInput
	}
	s.users[user.ID] = cloneUser(user)
	return nil
}

func (s *memoryStore) UpdateUserAndInvalidateSessions(user User, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.users[user.ID]
	if !ok {
		return ErrInvalidInput
	}
	if current.Username != user.Username {
		if _, exists := s.byName[user.Username]; exists {
			return ErrUsernameTaken
		}
		delete(s.byName, current.Username)
		s.byName[user.Username] = user.ID
	}
	user.GameIDs = append([]string(nil), user.GameIDs...)
	s.users[user.ID] = user
	for hash, session := range s.sessions {
		if session.UserID == user.ID && session.InvalidAt == nil {
			at := user.UpdatedAt
			session.InvalidAt = &at
			s.sessions[hash] = session
		}
	}
	s.auditLogs = append(s.auditLogs, event)
	return nil
}

func (s *memoryStore) DeleteUser(userID UserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, exists := s.users[userID]
	if !exists {
		return ErrInvalidInput
	}
	for _, session := range s.sessions {
		if session.UserID == userID {
			return ErrTeamInUse
		}
	}
	delete(s.users, userID)
	delete(s.byName, user.Username)
	return nil
}

func (s *memoryStore) DeleteUserWithAudit(userID UserID, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, exists := s.users[userID]
	if !exists {
		return ErrInvalidInput
	}
	for _, session := range s.sessions {
		if session.UserID == userID {
			return ErrTeamInUse
		}
	}
	delete(s.users, userID)
	delete(s.byName, user.Username)
	s.auditLogs = append(s.auditLogs, cloneAuditEvent(event))
	return nil
}

func (s *memoryStore) CreateTeam(team OperationTeam) (TeamID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.teamNames[team.Name]; exists {
		return 0, ErrTeamNameTaken
	}
	team.ID = s.nextTeamID
	s.nextTeamID++
	s.teams[team.ID] = team
	s.teamNames[team.Name] = team.ID
	return team.ID, nil
}

func (s *memoryStore) CreateTeamWithAudit(team OperationTeam, event AuditEvent) (TeamID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.teamNames[team.Name]; exists {
		return 0, ErrTeamNameTaken
	}
	team.ID = s.nextTeamID
	s.nextTeamID++
	if event.TargetID == "" || event.TargetID == "0" {
		event.TargetID = strconv.FormatInt(int64(team.ID), 10)
	}
	s.teams[team.ID] = team
	s.teamNames[team.Name] = team.ID
	s.auditLogs = append(s.auditLogs, cloneAuditEvent(event))
	return team.ID, nil
}

func (s *memoryStore) FindTeam(teamID TeamID) (OperationTeam, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	team, ok := s.teams[teamID]
	return team, ok, nil
}

func (s *memoryStore) ListTeams() ([]OperationTeam, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]OperationTeam, 0, len(s.teams))
	for _, team := range s.teams {
		result = append(result, team)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *memoryStore) UpdateTeam(team OperationTeam) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, exists := s.teams[team.ID]
	if !exists {
		return ErrInvalidInput
	}
	if existingID, exists := s.teamNames[team.Name]; exists && existingID != team.ID {
		return ErrTeamNameTaken
	}
	delete(s.teamNames, previous.Name)
	s.teams[team.ID] = team
	s.teamNames[team.Name] = team.ID
	return nil
}

func (s *memoryStore) UpdateTeamWithAudit(team OperationTeam, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, exists := s.teams[team.ID]
	if !exists {
		return ErrInvalidInput
	}
	if existingID, exists := s.teamNames[team.Name]; exists && existingID != team.ID {
		return ErrTeamNameTaken
	}
	delete(s.teamNames, previous.Name)
	s.teams[team.ID] = team
	s.teamNames[team.Name] = team.ID
	s.auditLogs = append(s.auditLogs, cloneAuditEvent(event))
	return nil
}

func (s *memoryStore) DeleteTeam(teamID TeamID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	team, exists := s.teams[teamID]
	if !exists {
		return ErrInvalidInput
	}
	delete(s.teams, teamID)
	delete(s.teamNames, team.Name)
	return nil
}

func (s *memoryStore) DeleteTeamWithAudit(teamID TeamID, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	team, exists := s.teams[teamID]
	if !exists {
		return ErrInvalidInput
	}
	for _, user := range s.users {
		if user.TeamID != nil && *user.TeamID == teamID {
			return ErrTeamInUse
		}
	}
	delete(s.teams, teamID)
	delete(s.teamNames, team.Name)
	s.auditLogs = append(s.auditLogs, cloneAuditEvent(event))
	return nil
}

func (s *memoryStore) TeamHasReferences(teamID TeamID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if user.TeamID != nil && *user.TeamID == teamID {
			return true, nil
		}
	}
	return false, nil
}

func (s *memoryStore) CreateSession(session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.TokenHash] = session
	return nil
}

func (s *memoryStore) FindSessionByTokenHash(tokenHash string) (Session, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[tokenHash]
	return session, ok, nil
}

func (s *memoryStore) HasActiveSession(userID UserID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, session := range s.sessions {
		if session.UserID == userID && session.InvalidAt == nil {
			return true, nil
		}
	}
	return false, nil
}

func (s *memoryStore) InvalidateUserSessions(userID UserID, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, session := range s.sessions {
		if session.UserID == userID && session.InvalidAt == nil {
			invalidAt := at
			session.InvalidAt = &invalidAt
			s.sessions[hash] = session
		}
	}
	return nil
}

func (s *memoryStore) AppendAudit(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLogs = append(s.auditLogs, cloneAuditEvent(event))
	return nil
}

func (s *memoryStore) ListAuditLogs(limit int) ([]AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.auditLogs)
	if limit > n {
		limit = n
	}
	result := make([]AuditEvent, limit)
	for i := 0; i < limit; i++ {
		result[i] = s.auditLogs[n-1-i]
	}
	return result, nil
}

func (s *memoryStore) AuditEvents() []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AuditEvent, len(s.auditLogs))
	for i, event := range s.auditLogs {
		result[i] = cloneAuditEvent(event)
	}
	return result
}

func cloneUser(user User) User {
	user.TeamID = cloneTeamID(user.TeamID)
	user.GameIDs = append([]string(nil), user.GameIDs...)
	return user
}

func cloneAuditEvent(event AuditEvent) AuditEvent {
	summary := make(map[string]string, len(event.Summary))
	for key, value := range event.Summary {
		summary[key] = value
	}
	event.Summary = summary
	return event
}
