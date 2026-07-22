// Package identity owns Cloud users, roles, game scopes, and sessions.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"golang.org/x/crypto/bcrypt"
)

type Role string

type UserID int64

type TeamID int64

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

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrSessionInvalid       = errors.New("session is invalid")
	ErrForbidden            = errors.New("operation is forbidden")
	ErrInvalidInput         = errors.New("identity input is invalid")
	ErrUsernameTaken        = errors.New("username is already in use")
	ErrTeamNameTaken        = errors.New("team name is already in use")
	ErrTeamInUse            = errors.New("team is still referenced")
	ErrBootstrapUnavailable = errors.New("initial admin cannot be created")
)

type OperationTeam struct {
	ID        TeamID    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type User struct {
	ID           UserID
	Username     string
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
	Role     Role       `json:"role"`
	Status   UserStatus `json:"status"`
	TeamID   *TeamID    `json:"team_id"`
	TeamName string     `json:"team_name"`
	GameIDs  []string   `json:"game_ids"`
}

type Session struct {
	ID        string
	UserID    UserID
	TokenHash string
	CreatedAt time.Time
	InvalidAt *time.Time
}

type LoginResult struct {
	Token string
	User  PublicUser
}

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

// AuthContext exposes server-side session identity to trusted Cloud modules
// without returning the raw session token to an API consumer.
type AuthContext struct {
	User    PublicUser
	Session Session
}

type CreateUserInput struct {
	Username string
	Password string
	Role     Role
	TeamID   *TeamID
	GameIDs  []string
}

type AuditEvent struct {
	ID          string
	ActorUserID UserID
	Action      string
	TargetType  string
	TargetID    string
	Summary     map[string]string
	CreatedAt   time.Time
}

type Store interface {
	CountUsers() (int, error)
	CreateUser(User) (UserID, error)
	FindUser(id UserID) (User, bool, error)
	FindUserByUsername(username string) (User, bool, error)
	ListUsers() ([]User, error)
	UpdateUser(User) error
	DeleteUser(UserID) error
	CreateTeam(OperationTeam) (TeamID, error)
	FindTeam(TeamID) (OperationTeam, bool, error)
	ListTeams() ([]OperationTeam, error)
	UpdateTeam(OperationTeam) error
	DeleteTeam(TeamID) error
	TeamHasReferences(TeamID) (bool, error)
	CreateSession(Session) error
	FindSessionByTokenHash(tokenHash string) (Session, bool, error)
	InvalidateUserSessions(userID UserID, at time.Time) error
	AppendAudit(AuditEvent) error
	ListAuditLogs(limit int) ([]AuditEvent, error)
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

func NewService(store Store, options ...Option) *Service {
	service := &Service{
		store:     store,
		now:       func() time.Time { return time.Now().UTC() },
		newID:     common.NewID,
		newToken:  common.NewToken,
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
	user, err := s.createUser(CreateUserInput{Username: username, Password: password, Role: RoleAdmin})
	if err != nil {
		return PublicUser{}, err
	}
	if err := s.audit(user.ID, "user.bootstrap", "user", user.ID, map[string]string{"role": string(user.Role)}); err != nil {
		return PublicUser{}, err
	}
	return user, nil
}

func (s *Service) CreateUser(actorID UserID, input CreateUserInput) (PublicUser, error) {
	actor, _, err := s.store.FindUser(actorID)
	if err != nil {
		return PublicUser{}, err
	}
	if !isAdmin(actor) {
		return PublicUser{}, ErrForbidden
	}
	user, err := s.createUser(input)
	if err != nil {
		return PublicUser{}, err
	}
	if err := s.audit(actor.ID, "user.create", "user", user.ID, map[string]string{"role": string(user.Role), "username": user.Username}); err != nil {
		return PublicUser{}, err
	}
	return user, nil
}

func (s *Service) createUser(input CreateUserInput) (PublicUser, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || len(username) > 64 || len(input.Password) < 12 || !validRole(input.Role) {
		return PublicUser{}, ErrInvalidInput
	}
	_, found, err := s.store.FindUserByUsername(username)
	if err != nil {
		return PublicUser{}, err
	}
	if found {
		return PublicUser{}, ErrUsernameTaken
	}
	hash, err := s.passwords.Hash([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return PublicUser{}, err
	}
	gameIDs := normalizeGameIDs(input.GameIDs)
	if input.Role == RoleAdmin {
		if input.TeamID != nil || len(gameIDs) != 0 {
			return PublicUser{}, ErrInvalidInput
		}
	} else {
		if input.TeamID == nil || *input.TeamID <= 0 || len(gameIDs) == 0 {
			return PublicUser{}, ErrInvalidInput
		}
		team, found, err := s.store.FindTeam(*input.TeamID)
		if err != nil {
			return PublicUser{}, err
		}
		if !found {
			return PublicUser{}, ErrInvalidInput
		}
		_ = team
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
	userID, err := s.store.CreateUser(user)
	if err != nil {
		return PublicUser{}, err
	}
	user.ID = userID
	if user.TeamID != nil {
		team, _, err := s.store.FindTeam(*user.TeamID)
		if err != nil {
			return PublicUser{}, err
		}
		user.TeamName = team.Name
	}
	return publicUser(user), nil
}

func (s *Service) Login(username, password string) (LoginResult, error) {
	user, ok, err := s.store.FindUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || user.Status != UserStatusEnabled || s.passwords.Compare([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginResult{}, ErrAuthenticationFailed
	}
	now := s.now()
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
	if err := s.audit(user.ID, "user.login", "user", user.ID, map[string]string{"username": user.Username}); err != nil {
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
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return err
	}
	if !isAdmin(actor) || actorID == userID {
		return ErrForbidden
	}
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return err
	}
	if !ok || !validStatus(status) {
		return ErrInvalidInput
	}
	user.Status = status
	user.UpdatedAt = s.now()
	if err := s.store.UpdateUser(user); err != nil {
		return err
	}
	if status == UserStatusDisabled {
		if err := s.store.InvalidateUserSessions(user.ID, user.UpdatedAt); err != nil {
			return err
		}
	}
	return s.audit(actor.ID, "user.status.update", "user", user.ID, map[string]string{"status": string(status)})
}

func (s *Service) ChangeOwnPassword(userID UserID, currentPassword, newPassword string) error {
	user, ok, err := s.store.FindUser(userID)
	if err != nil {
		return err
	}
	if !ok || user.Status != UserStatusEnabled || s.passwords.Compare([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return ErrAuthenticationFailed
	}
	if err := s.updatePassword(user, newPassword); err != nil {
		return err
	}
	return s.audit(user.ID, "user.password.change", "user", user.ID, map[string]string{"result": "changed"})
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
	if err := s.updatePassword(user, newPassword); err != nil {
		return err
	}
	return s.audit(actor.ID, "user.password.reset", "user", user.ID, map[string]string{"result": "reset"})
}

func (s *Service) updatePassword(user User, newPassword string) error {
	if len(newPassword) < 12 {
		return ErrInvalidInput
	}
	hash, err := s.passwords.Hash([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	user.UpdatedAt = s.now()
	if err := s.store.UpdateUser(user); err != nil {
		return err
	}
	return s.store.InvalidateUserSessions(user.ID, user.UpdatedAt)
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
	teamID, err := s.store.CreateTeam(team)
	if err != nil {
		return OperationTeam{}, err
	}
	team.ID = teamID
	if err := s.audit(actor.ID, "operation_team.create", "operation_team", UserID(team.ID), map[string]string{"name": team.Name}); err != nil {
		return OperationTeam{}, err
	}
	return team, nil
}

func (s *Service) ListTeams(actorID UserID) ([]OperationTeam, error) {
	actor, ok, err := s.store.FindUser(actorID)
	if err != nil {
		return nil, err
	}
	if !ok || !isAdmin(actor) {
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
	if err := s.store.UpdateTeam(team); err != nil {
		return OperationTeam{}, err
	}
	if err := s.audit(actor.ID, "operation_team.rename", "operation_team", UserID(team.ID), map[string]string{"name": team.Name}); err != nil {
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
	if err := s.store.DeleteTeam(teamID); err != nil {
		return err
	}
	return s.audit(actor.ID, "operation_team.delete", "operation_team", UserID(teamID), map[string]string{"result": "deleted"})
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
	if err := s.store.DeleteUser(userID); err != nil {
		return err
	}
	return s.audit(actor.ID, "user.delete", "user", userID, map[string]string{"result": "deleted"})
}

func (s *Service) UpdateUserAccess(actorID, userID UserID, role Role, teamID *TeamID, gameIDs []string) (PublicUser, error) {
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
	normalizedGameIDs := normalizeGameIDs(gameIDs)
	if role == RoleAdmin {
		if teamID != nil || len(normalizedGameIDs) != 0 {
			return PublicUser{}, ErrInvalidInput
		}
	} else {
		if teamID == nil || *teamID <= 0 || len(normalizedGameIDs) == 0 {
			return PublicUser{}, ErrInvalidInput
		}
		if _, found, err := s.store.FindTeam(*teamID); err != nil {
			return PublicUser{}, err
		} else if !found {
			return PublicUser{}, ErrInvalidInput
		}
	}
	user.Role = role
	user.TeamID = cloneTeamID(teamID)
	user.GameIDs = normalizedGameIDs
	user.UpdatedAt = s.now()
	if err := s.store.UpdateUser(user); err != nil {
		return PublicUser{}, err
	}
	if err := s.audit(actor.ID, "user.access.update", "user", user.ID, map[string]string{"role": string(role)}); err != nil {
		return PublicUser{}, err
	}
	if err := s.store.InvalidateUserSessions(user.ID, user.UpdatedAt); err != nil {
		return PublicUser{}, err
	}
	if user.TeamID != nil {
		team, _, err := s.store.FindTeam(*user.TeamID)
		if err != nil {
			return PublicUser{}, err
		}
		user.TeamName = team.Name
	} else {
		user.TeamName = ""
	}
	return publicUser(user), nil
}

func (s *Service) audit(actorID UserID, action, targetType string, targetID UserID, summary map[string]string) error {
	return s.store.AppendAudit(AuditEvent{
		ID:          s.newID("audit"),
		ActorUserID: actorID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    strconv.FormatInt(int64(targetID), 10),
		Summary:     summary,
		CreatedAt:   s.now(),
	})
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

func validStatus(status UserStatus) bool {
	return status == UserStatusEnabled || status == UserStatusDisabled
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
