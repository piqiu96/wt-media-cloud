package identity

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewLoginInvalidatesPriorSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	tokens := []string{"session-first", "session-second"}
	service := NewService(
		NewMemoryStore(),
		WithClock(func() time.Time { return now }),
		WithTokenGenerator(func() string {
			token := tokens[0]
			tokens = tokens[1:]
			return token
		}),
	)

	if _, err := service.BootstrapTechnician("tech", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}

	first, err := service.Login("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	second, err := service.Login("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("second Login() error = %v", err)
	}
	if first.Token == second.Token {
		t.Fatalf("replacement login reused token %q", first.Token)
	}

	if _, err := service.Authenticate(first.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Authenticate(first token) error = %v, want ErrSessionInvalid", err)
	}
	actor, err := service.Authenticate(second.Token)
	if err != nil {
		t.Fatalf("Authenticate(second token) error = %v", err)
	}
	if actor.Username != "tech" || actor.Role != RoleTechnician {
		t.Fatalf("actor = %+v", actor)
	}
}

func TestAuthenticateContextReturnsServerSideSessionIdentity(t *testing.T) {
	service := NewService(NewMemoryStore(), WithTokenGenerator(func() string { return "session-token" }))
	if _, err := service.BootstrapTechnician("tech", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	login, err := service.Login("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	context, err := service.AuthenticateContext(login.Token)
	if err != nil {
		t.Fatalf("AuthenticateContext() error = %v", err)
	}
	if context.User.ID != login.User.ID || context.Session.ID == "" || context.Session.TokenHash == "" {
		t.Fatalf("context = %+v", context)
	}
}

func TestDisabledUserCannotAuthenticate(t *testing.T) {
	service := NewService(NewMemoryStore())
	technician, err := service.BootstrapTechnician("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	user, err := service.CreateUser(technician.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := service.SetUserStatus(technician.ID, user.ID, UserStatusDisabled); err != nil {
		t.Fatalf("SetUserStatus() error = %v", err)
	}

	if _, err := service.Login("operator", "a-long-operator-password"); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("Login() error = %v, want ErrAuthenticationFailed", err)
	}
}

func TestOnlyTechnicianCanCreateUsersAndAssignGameScopes(t *testing.T) {
	service := NewService(NewMemoryStore())
	technician, err := service.BootstrapTechnician("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	operator, err := service.CreateUser(technician.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := service.CreateUser(operator.ID, CreateUserInput{
		Username: "forbidden",
		Password: "a-long-forbidden-password",
		Role:     RoleOperator,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateUser(operator) error = %v, want ErrForbidden", err)
	}
	if !service.CanAccessGame(operator.ID, "game-a") || service.CanAccessGame(operator.ID, "game-b") {
		t.Fatalf("operator game scope is not enforced")
	}
	if !service.CanAccessGame(technician.ID, "any-game") {
		t.Fatalf("technician must have global game access")
	}
}

func TestUserCanChangeOwnPasswordAndTechnicianCanResetIt(t *testing.T) {
	service := NewService(NewMemoryStore())
	technician, err := service.BootstrapTechnician("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	operator, err := service.CreateUser(technician.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if err := service.ChangeOwnPassword(operator.ID, "a-long-operator-password", "a-new-operator-password"); err != nil {
		t.Fatalf("ChangeOwnPassword() error = %v", err)
	}
	if _, err := service.Login("operator", "a-long-operator-password"); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("old password Login() error = %v", err)
	}
	if _, err := service.Login("operator", "a-new-operator-password"); err != nil {
		t.Fatalf("new password Login() error = %v", err)
	}

	if err := service.ResetPassword(technician.ID, operator.ID, "a-reset-operator-password"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if _, err := service.Login("operator", "a-reset-operator-password"); err != nil {
		t.Fatalf("reset password Login() error = %v", err)
	}
}

func TestTechnicianCanUpdateRoleAndGameScopes(t *testing.T) {
	service := NewService(NewMemoryStore())
	technician, err := service.BootstrapTechnician("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	operator, err := service.CreateUser(technician.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	updated, err := service.UpdateUserAccess(technician.ID, operator.ID, RoleSeniorOperator, []string{"game-b"})
	if err != nil {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if updated.Role != RoleSeniorOperator || len(updated.GameIDs) != 1 || updated.GameIDs[0] != "game-b" {
		t.Fatalf("updated user = %+v", updated)
	}
	if service.CanAccessGame(operator.ID, "game-a") || !service.CanAccessGame(operator.ID, "game-b") {
		t.Fatalf("updated game scope is not enforced")
	}
}

func TestAuditEventsNeverContainPasswordOrSessionMaterial(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, WithTokenGenerator(func() string { return "raw-session-secret" }))
	technician, err := service.BootstrapTechnician("tech", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapTechnician() error = %v", err)
	}
	if _, err := service.CreateUser(technician.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
	}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := service.Login("tech", "a-long-initial-password"); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	for _, event := range store.AuditEvents() {
		encoded := event.Action + event.TargetID
		for key, value := range event.Summary {
			encoded += key + value
		}
		for _, secret := range []string{"a-long-initial-password", "a-long-operator-password", "raw-session-secret", "password_hash", "token_hash"} {
			if strings.Contains(encoded, secret) {
				t.Fatalf("audit event leaked %q: %+v", secret, event)
			}
		}
	}
}
