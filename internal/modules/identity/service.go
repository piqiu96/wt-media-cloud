// Package identity owns Cloud users, roles, game scopes, and sessions.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleOperator       Role = "operator"
	RoleSeniorOperator Role = "senior_operator"
	RoleTechnician     Role = "technician"
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
	ErrBootstrapUnavailable = errors.New("initial technician cannot be created")
)

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         Role
	Status       UserStatus
	GameIDs      []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PublicUser is safe to return from an authenticated API. It has no password
// or session material.
type PublicUser struct {
	ID       string     `json:"id"`
	Username string     `json:"username"`
	Role     Role       `json:"role"`
	Status   UserStatus `json:"status"`
	GameIDs  []string   `json:"game_ids"`
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	InvalidAt *time.Time
}

type LoginResult struct {
	Token string
	User  PublicUser
}

type CreateUserInput struct {
	Username string
	Password string
	Role     Role
	GameIDs  []string
}

type Store interface {
	CountUsers() int
	CreateUser(User) error
	FindUser(id string) (User, bool)
	FindUserByUsername(username string) (User, bool)
	UpdateUser(User) error
	CreateSession(Session) error
	FindSessionByTokenHash(tokenHash string) (Session, bool)
	InvalidateUserSessions(userID string, at time.Time) error
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

func (s *Service) BootstrapTechnician(username, password string) (PublicUser, error) {
	if s.store.CountUsers() != 0 {
		return PublicUser{}, ErrBootstrapUnavailable
	}
	return s.createUser(CreateUserInput{Username: username, Password: password, Role: RoleTechnician})
}

func (s *Service) CreateUser(actorID string, input CreateUserInput) (PublicUser, error) {
	actor, ok := s.store.FindUser(actorID)
	if !ok || actor.Status != UserStatusEnabled || actor.Role != RoleTechnician {
		return PublicUser{}, ErrForbidden
	}
	return s.createUser(input)
}

func (s *Service) createUser(input CreateUserInput) (PublicUser, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || len(username) > 64 || len(input.Password) < 12 || !validRole(input.Role) {
		return PublicUser{}, ErrInvalidInput
	}
	if _, found := s.store.FindUserByUsername(username); found {
		return PublicUser{}, ErrUsernameTaken
	}
	hash, err := s.passwords.Hash([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return PublicUser{}, err
	}
	now := s.now()
	user := User{
		ID:           s.newID("user"),
		Username:     username,
		PasswordHash: string(hash),
		Role:         input.Role,
		Status:       UserStatusEnabled,
		GameIDs:      normalizeGameIDs(input.GameIDs),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if user.Role == RoleTechnician {
		user.GameIDs = nil
	}
	if err := s.store.CreateUser(user); err != nil {
		return PublicUser{}, err
	}
	return publicUser(user), nil
}

func (s *Service) Login(username, password string) (LoginResult, error) {
	user, ok := s.store.FindUserByUsername(strings.TrimSpace(username))
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
	return LoginResult{Token: token, User: publicUser(user)}, nil
}

func (s *Service) Authenticate(token string) (PublicUser, error) {
	if token == "" {
		return PublicUser{}, ErrSessionInvalid
	}
	session, ok := s.store.FindSessionByTokenHash(tokenHash(token))
	if !ok || session.InvalidAt != nil {
		return PublicUser{}, ErrSessionInvalid
	}
	user, ok := s.store.FindUser(session.UserID)
	if !ok || user.Status != UserStatusEnabled {
		return PublicUser{}, ErrSessionInvalid
	}
	return publicUser(user), nil
}

func (s *Service) SetUserStatus(actorID, userID string, status UserStatus) error {
	actor, ok := s.store.FindUser(actorID)
	if !ok || actor.Status != UserStatusEnabled || actor.Role != RoleTechnician {
		return ErrForbidden
	}
	user, ok := s.store.FindUser(userID)
	if !ok || !validStatus(status) {
		return ErrInvalidInput
	}
	user.Status = status
	user.UpdatedAt = s.now()
	if err := s.store.UpdateUser(user); err != nil {
		return err
	}
	if status == UserStatusDisabled {
		return s.store.InvalidateUserSessions(user.ID, user.UpdatedAt)
	}
	return nil
}

func (s *Service) CanAccessGame(userID, gameID string) bool {
	user, ok := s.store.FindUser(userID)
	if !ok || user.Status != UserStatusEnabled || strings.TrimSpace(gameID) == "" {
		return false
	}
	if user.Role == RoleTechnician {
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
	return PublicUser{ID: user.ID, Username: user.Username, Role: user.Role, Status: user.Status, GameIDs: append([]string(nil), user.GameIDs...)}
}

func validRole(role Role) bool {
	return role == RoleOperator || role == RoleSeniorOperator || role == RoleTechnician
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

func tokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

type memoryStore struct {
	mu       sync.RWMutex
	users    map[string]User
	byName   map[string]string
	sessions map[string]Session
}

func NewMemoryStore() Store {
	return &memoryStore{
		users:    make(map[string]User),
		byName:   make(map[string]string),
		sessions: make(map[string]Session),
	}
}

func (s *memoryStore) CountUsers() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

func (s *memoryStore) CreateUser(user User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byName[user.Username]; exists {
		return ErrUsernameTaken
	}
	s.users[user.ID] = cloneUser(user)
	s.byName[user.Username] = user.ID
	return nil
}

func (s *memoryStore) FindUser(id string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	return cloneUser(user), ok
}

func (s *memoryStore) FindUserByUsername(username string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byName[username]
	if !ok {
		return User{}, false
	}
	return cloneUser(s.users[id]), true
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

func (s *memoryStore) CreateSession(session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.TokenHash] = session
	return nil
}

func (s *memoryStore) FindSessionByTokenHash(tokenHash string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[tokenHash]
	return session, ok
}

func (s *memoryStore) InvalidateUserSessions(userID string, at time.Time) error {
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

func cloneUser(user User) User {
	user.GameIDs = append([]string(nil), user.GameIDs...)
	return user
}
