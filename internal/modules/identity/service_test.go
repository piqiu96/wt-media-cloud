package identity

import (
	"errors"
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
