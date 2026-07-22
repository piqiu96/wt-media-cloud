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

	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}

	first, err := service.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	second, err := service.Login("admin", "a-long-initial-password")
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
	if actor.Username != "admin" || actor.Role != RoleAdmin {
		t.Fatalf("actor = %+v", actor)
	}
}

func TestAuthenticateContextReturnsServerSideSessionIdentity(t *testing.T) {
	service := NewService(NewMemoryStore(), WithTokenGenerator(func() string { return "session-token" }))
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	login, err := service.Login("admin", "a-long-initial-password")
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
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	team := mustCreateTeam(t, service, admin.ID, "停用测试组")
	user, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := service.SetUserStatus(admin.ID, user.ID, UserStatusDisabled); err != nil {
		t.Fatalf("SetUserStatus() error = %v", err)
	}

	if _, err := service.Login("operator", "a-long-operator-password"); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("Login() error = %v, want ErrAuthenticationFailed", err)
	}
}

func TestOnlyAdminCanCreateUsersAndAssignGameScopes(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	team := mustCreateTeam(t, service, admin.ID, "权限测试组")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := service.CreateUser(operator.ID, CreateUserInput{
		Username: "forbidden",
		Password: "a-long-forbidden-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateUser(operator) error = %v, want ErrForbidden", err)
	}
	if !service.CanAccessGame(operator.ID, "game-a") || service.CanAccessGame(operator.ID, "game-b") {
		t.Fatalf("operator game scope is not enforced")
	}
	if !service.CanAccessGame(admin.ID, "any-game") {
		t.Fatalf("admin must have global game access")
	}
}

func TestUserCanChangeOwnPasswordAndAdminCanResetIt(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	team := mustCreateTeam(t, service, admin.ID, "密码测试组")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
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

	if err := service.ResetPassword(admin.ID, operator.ID, "a-reset-operator-password"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if _, err := service.Login("operator", "a-reset-operator-password"); err != nil {
		t.Fatalf("reset password Login() error = %v", err)
	}
}

func TestAdminCanUpdateRoleAndGameScopes(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	team, err := service.CreateTeam(admin.ID, "测试组")
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	updated, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &team.ID, []string{"game-b"})
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
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	team := mustCreateTeam(t, service, admin.ID, "审计测试组")
	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := service.Login("admin", "a-long-initial-password"); err != nil {
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

func mustCreateTeam(t *testing.T, service *Service, adminID UserID, name string) OperationTeam {
	t.Helper()
	team, err := service.CreateTeam(adminID, name)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	return team
}

func TestAdminCreatesTeamsAndTeamScopedUsers(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if admin.ID <= 0 || admin.Role != RoleAdmin || admin.TeamID != nil || len(admin.GameIDs) != 0 {
		t.Fatalf("admin = %+v", admin)
	}

	team, err := service.CreateTeam(admin.ID, "火影组")
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a",
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if operator.TeamID == nil || *operator.TeamID != team.ID || operator.TeamName != "火影组" {
		t.Fatalf("operator = %+v", operator)
	}

	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "missing-team",
		Password: "a-long-operator-password",
		Role:     RoleSeniorOperator,
		GameIDs:  []string{"game-a"},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateUser(missing team) error = %v, want ErrInvalidInput", err)
	}
	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "admin-with-team",
		Password: "a-long-admin-password",
		Role:     RoleAdmin,
		TeamID:   &team.ID,
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateUser(admin with team) error = %v, want ErrInvalidInput", err)
	}
}

func TestOnlyAdminManagesTeamsAndReferencedTeamCannotBeDeleted(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team, _ := service.CreateTeam(admin.ID, "火影组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, GameIDs: []string{"game-a"},
	})

	if _, err := service.CreateTeam(operator.ID, "越权组"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateTeam(operator) error = %v, want ErrForbidden", err)
	}
	if err := service.DeleteTeam(admin.ID, team.ID); !errors.Is(err, ErrTeamInUse) {
		t.Fatalf("DeleteTeam(referenced) error = %v, want ErrTeamInUse", err)
	}

	empty, err := service.CreateTeam(admin.ID, "空组")
	if err != nil {
		t.Fatalf("CreateTeam(empty) error = %v", err)
	}
	if err := service.DeleteTeam(admin.ID, empty.ID); err != nil {
		t.Fatalf("DeleteTeam(empty) error = %v", err)
	}
}

func TestUpdateUserAccessTransfersCurrentTeamAndInvalidatesSession(t *testing.T) {
	service := NewService(NewMemoryStore(), WithTokenGenerator(func() string { return "operator-session" }))
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	oldTeam, _ := service.CreateTeam(admin.ID, "旧组")
	newTeam, _ := service.CreateTeam(admin.ID, "新组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &oldTeam.ID, GameIDs: []string{"game-a"},
	})
	login, err := service.Login("operator-a", "a-long-operator-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	updated, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []string{"game-b"})
	if err != nil {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if updated.TeamID == nil || *updated.TeamID != newTeam.ID || updated.Role != RoleSeniorOperator {
		t.Fatalf("updated = %+v", updated)
	}
	if _, err := service.Authenticate(login.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Authenticate(old session) error = %v, want ErrSessionInvalid", err)
	}
}

func TestPublicUserCanAccessUsesRoleTeamAndGameIntersection(t *testing.T) {
	teamA := TeamID(10)
	teamB := TeamID(20)
	tests := []struct {
		name    string
		actor   PublicUser
		ownerID UserID
		teamID  *TeamID
		gameID  string
		want    bool
	}{
		{name: "admin all", actor: PublicUser{ID: 1, Role: RoleAdmin, Status: UserStatusEnabled}, ownerID: 99, teamID: &teamB, gameID: "game-z", want: true},
		{name: "senior same team and game", actor: PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA, GameIDs: []string{"game-a"}}, ownerID: 99, teamID: &teamA, gameID: "game-a", want: true},
		{name: "senior cross team", actor: PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA, GameIDs: []string{"game-a"}}, ownerID: 99, teamID: &teamB, gameID: "game-a", want: false},
		{name: "senior cross game", actor: PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA, GameIDs: []string{"game-a"}}, ownerID: 99, teamID: &teamA, gameID: "game-b", want: false},
		{name: "operator self and game", actor: PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA, GameIDs: []string{"game-a"}}, ownerID: 3, teamID: &teamA, gameID: "game-a", want: true},
		{name: "operator other user", actor: PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA, GameIDs: []string{"game-a"}}, ownerID: 4, teamID: &teamA, gameID: "game-a", want: false},
		{name: "operator cross game", actor: PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA, GameIDs: []string{"game-a"}}, ownerID: 3, teamID: &teamA, gameID: "game-b", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actor.CanAccess(tt.ownerID, tt.teamID, tt.gameID); got != tt.want {
				t.Fatalf("CanAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}
