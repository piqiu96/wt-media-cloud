package service

import (
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"strings"
	"testing"
	"time"
)

func TestOwnProfileDefaultsAndChoiceSurviveNewLogin(t *testing.T) {
	s := NewService(NewMemoryStore())
	user, err := s.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil { t.Fatal(err) }
	if user.Nickname != "admin" || user.AvatarID != "sky" { t.Fatalf("defaults = %+v", user) }
	if _, err := s.UpdateOwnProfile(user.ID, "", "ocean"); !errors.Is(err, ErrInvalidInput) { t.Fatalf("empty nickname = %v", err) }
	if _, err := s.UpdateOwnProfile(user.ID, "小明", "external-url"); !errors.Is(err, ErrInvalidInput) { t.Fatalf("external avatar = %v", err) }
	updated, err := s.UpdateOwnProfile(user.ID, " 小明 ", "ocean")
	if err != nil { t.Fatal(err) }
	if updated.Nickname != "小明" || updated.AvatarID != "ocean" || updated.Username != "admin" { t.Fatalf("updated = %+v", updated) }
	logged, err := s.Login("admin", "a-long-initial-password")
	if err != nil { t.Fatal(err) }
	if logged.User.Nickname != "小明" || logged.User.AvatarID != "ocean" { t.Fatalf("login user = %+v", logged.User) }
}

func TestNewLoginRequiresConfirmationBeforeInvalidatingPriorSession(t *testing.T) {
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
	if _, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{}); !errors.Is(err, ErrSessionReplaceNeeded) {
		t.Fatalf("unconfirmed LoginWithOptions() error = %v, want ErrSessionReplaceNeeded", err)
	}
	stillActive, err := service.Authenticate(first.Token)
	if err != nil {
		t.Fatalf("unconfirmed replacement invalidated first token: %v", err)
	}
	if stillActive.Username != "admin" {
		t.Fatalf("stillActive = %+v", stillActive)
	}
	second, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ReplaceExisting: true})
	if err != nil {
		t.Fatalf("confirmed LoginWithOptions() error = %v", err)
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

	teams, err := service.ListTeams(operator.ID)
	if err != nil || len(teams) != 1 || teams[0].ID != team.ID || teams[0].Name != "火影组" {
		t.Fatalf("ListTeams(operator) = %#v, err = %v", teams, err)
	}
	if _, err := service.CreateTeam(operator.ID, "越权组"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateTeam(operator) error = %v, want ErrForbidden", err)
	}
	if _, err := service.RenameTeam(operator.ID, team.ID, "越权重命名"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RenameTeam(operator) error = %v, want ErrForbidden", err)
	}
	if err := service.DeleteTeam(operator.ID, team.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DeleteTeam(operator) error = %v, want ErrForbidden", err)
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

func TestReferencedGameCannotBeDisabledOrDeleted(t *testing.T) {
	store := newGameMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team, _ := service.CreateTeam(admin.ID, "火影组")
	game, err := service.CreateGame(admin.ID, "naruto", "火影忍者", "")
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, GameIDs: []string{game.ID},
	}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := service.UpdateGame(admin.ID, game.ID, game.Name, GameStatusDisabled, ""); !errors.Is(err, ErrGameInUse) {
		t.Fatalf("UpdateGame(disable referenced) error = %v, want ErrGameInUse", err)
	}
	if err := service.DeleteGame(admin.ID, game.ID); !errors.Is(err, ErrGameInUse) {
		t.Fatalf("DeleteGame(referenced) error = %v, want ErrGameInUse", err)
	}

	empty, err := service.CreateGame(admin.ID, "delta", "三角洲", "")
	if err != nil {
		t.Fatalf("CreateGame(empty) error = %v", err)
	}
	if _, err := service.UpdateGame(admin.ID, empty.ID, empty.Name, GameStatusDisabled, ""); err != nil {
		t.Fatalf("UpdateGame(disable empty) error = %v", err)
	}
	if err := service.DeleteGame(admin.ID, empty.ID); err != nil {
		t.Fatalf("DeleteGame(empty) error = %v", err)
	}
}

func TestAdminGameListIncludesReferenceSummaryAndDetails(t *testing.T) {
	store := newGameMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team := mustCreateTeam(t, service, admin.ID, "火影组")
	game, err := service.CreateGame(admin.ID, "naruto", "火影忍者", "")
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, GameIDs: []string{game.ID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	store.accountReferences[game.ID] = []GameReferenceAccount{{AccountID: "8", Name: "B站账号", Platform: "bilibili", UserID: operator.ID, Username: operator.Username}}

	games, err := service.ListGames(admin.ID)
	if err != nil {
		t.Fatalf("ListGames() error = %v", err)
	}
	if len(games) != 1 || games[0].ReferenceSummary == nil || games[0].ReferenceSummary.UserScopeCount != 1 || games[0].ReferenceSummary.MediaAccountCount != 1 || games[0].ReferenceSummary.TotalCount != 2 {
		t.Fatalf("ListGames() = %#v, want user=1 account=1 total=2", games)
	}
	references, err := service.GameReferences(admin.ID, game.ID)
	if err != nil {
		t.Fatalf("GameReferences() error = %v", err)
	}
	if len(references.Users) != 1 || references.Users[0].Username != "operator-a" || len(references.MediaAccounts) != 1 || references.MediaAccounts[0].AccountID != "8" {
		t.Fatalf("GameReferences() = %#v", references)
	}
	if _, err := service.GameReferences(operator.ID, game.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("GameReferences(operator) error = %v, want ErrForbidden", err)
	}
}

func TestUserGameScopeCannotDropReferencedAccountGame(t *testing.T) {
	store := newGameMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team := mustCreateTeam(t, service, admin.ID, "火影组")
	naruto, _ := service.CreateGame(admin.ID, "naruto", "火影忍者", "")
	delta, _ := service.CreateGame(admin.ID, "delta", "三角洲", "")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, GameIDs: []string{naruto.ID, delta.ID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	store.accountReferences[naruto.ID] = []GameReferenceAccount{{AccountID: "8", UserID: operator.ID}}

	if _, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleOperator, &team.ID, []string{delta.ID}); !errors.Is(err, ErrUserGameScopeInUse) {
		t.Fatalf("UpdateUserAccess(drop referenced game) error = %v, want ErrUserGameScopeInUse", err)
	}
}

func TestServiceResolveGameReturnsStoredGameWithoutActorScope(t *testing.T) {
	store := newGameMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	game, err := service.CreateGame(admin.ID, "naruto", "火影忍者", "")
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	resolved, found, err := service.ResolveGame(game.ID)
	if err != nil || !found || resolved.ID != game.ID || resolved.Status != GameStatusEnabled {
		t.Fatalf("ResolveGame() = %#v, found=%v, err=%v", resolved, found, err)
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

func TestUpdateUserRejectsInvalidStatusWithoutChangingAccess(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	oldTeam, _ := service.CreateTeam(admin.ID, "旧组")
	newTeam, _ := service.CreateTeam(admin.ID, "新组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &oldTeam.ID, GameIDs: []string{"game-a"},
	})

	if _, err := service.UpdateUser(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []string{"game-b"}, UserStatus("invalid")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateUser() error = %v, want ErrInvalidInput", err)
	}
	stored, _, _ := service.store.FindUser(operator.ID)
	if stored.Role != RoleOperator || stored.Status != UserStatusEnabled || stored.TeamID == nil || *stored.TeamID != oldTeam.ID || len(stored.GameIDs) != 1 || stored.GameIDs[0] != "game-a" {
		t.Fatalf("invalid combined update changed persisted user: %+v", stored)
	}
}

func TestUpdateUserCommitsAccessAndStatusTogetherAndInvalidatesSession(t *testing.T) {
	service := NewService(NewMemoryStore(), WithTokenGenerator(func() string { return "operator-session" }))
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	oldTeam, _ := service.CreateTeam(admin.ID, "旧组")
	newTeam, _ := service.CreateTeam(admin.ID, "新组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &oldTeam.ID, GameIDs: []string{"game-a"},
	})
	login, _ := service.Login("operator-a", "a-long-operator-password")

	updated, err := service.UpdateUser(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []string{"game-b"}, UserStatusDisabled)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.Role != RoleSeniorOperator || updated.Status != UserStatusDisabled || updated.TeamID == nil || *updated.TeamID != newTeam.ID {
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

func TestPublicUserCanAccessOwnedResourceUsesRoleAndTeamOnly(t *testing.T) {
	teamA := TeamID(10)
	teamB := TeamID(20)
	admin := PublicUser{ID: 1, Role: RoleAdmin, Status: UserStatusEnabled}
	senior := PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA}
	operator := PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA}

	if !admin.CanAccessOwnedResource(99, &teamB) {
		t.Fatal("admin cannot access non-game resource")
	}
	if !senior.CanAccessOwnedResource(99, &teamA) || senior.CanAccessOwnedResource(99, &teamB) {
		t.Fatal("senior non-game resource scope is not limited to own team")
	}
	if !operator.CanAccessOwnedResource(operator.ID, &teamA) || operator.CanAccessOwnedResource(99, &teamA) {
		t.Fatal("operator non-game resource scope is not limited to self")
	}
}

func TestAccessUpdatesAuditTeamAndGames(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	teamA, _ := service.CreateTeam(admin.ID, "旧组")
	teamB, _ := service.CreateTeam(admin.ID, "新组")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &teamA.ID, GameIDs: []string{"game-a", "game-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &teamB.ID, []string{"game-c", "game-b"}); err != nil {
		t.Fatal(err)
	}

	var createSummary, updateSummary map[string]string
	for _, event := range store.AuditEvents() {
		switch event.Action {
		case "user.create":
			createSummary = event.Summary
		case "user.access.update":
			updateSummary = event.Summary
		}
	}
	for name, summary := range map[string]map[string]string{"create": createSummary, "update": updateSummary} {
		if summary["team_id"] == "" || summary["game_ids"] == "" {
			t.Fatalf("%s audit does not trace team and game scope: %#v", name, summary)
		}
	}
}

type failingIdentityStore struct {
	Store
	failAudit      bool
	failInvalidate bool
}

type gameMemoryStore struct {
	*memoryStore
	games             map[string]OperationGame
	accountReferences map[string][]GameReferenceAccount
}

func newGameMemoryStore() *gameMemoryStore {
	return &gameMemoryStore{memoryStore: NewMemoryStore(), games: make(map[string]OperationGame), accountReferences: make(map[string][]GameReferenceAccount)}
}

func (s *gameMemoryStore) CreateGameWithAudit(game OperationGame, event AuditEvent) error {
	if _, exists := s.games[game.ID]; exists {
		return ErrGameIDTaken
	}
	for _, existing := range s.games {
		if existing.Name == game.Name {
			return ErrGameNameTaken
		}
	}
	s.games[game.ID] = game
	return s.AppendAudit(event)
}

func (s *gameMemoryStore) FindGame(id string) (OperationGame, bool, error) {
	game, ok := s.games[id]
	return game, ok, nil
}

func (s *gameMemoryStore) ListGames() ([]OperationGame, error) {
	result := make([]OperationGame, 0, len(s.games))
	for _, game := range s.games {
		result = append(result, game)
	}
	return result, nil
}

func (s *gameMemoryStore) UpdateGameWithAudit(game OperationGame, event AuditEvent) error {
	if _, exists := s.games[game.ID]; !exists {
		return ErrInvalidInput
	}
	for _, existing := range s.games {
		if existing.ID != game.ID && existing.Name == game.Name {
			return ErrGameNameTaken
		}
	}
	s.games[game.ID] = game
	return s.AppendAudit(event)
}

func (s *gameMemoryStore) DeleteGameWithAudit(id string, event AuditEvent) error {
	if _, exists := s.games[id]; !exists {
		return ErrInvalidInput
	}
	delete(s.games, id)
	return s.AppendAudit(event)
}

func (s *gameMemoryStore) GameHasReferences(id string) (bool, error) {
	users, err := s.ListUsers()
	if err != nil {
		return false, err
	}
	for _, user := range users {
		for _, gameID := range user.GameIDs {
			if gameID == id {
				return true, nil
			}
		}
	}
	return len(s.accountReferences[id]) > 0, nil
}

func (s *gameMemoryStore) ListGameReferenceSummaries() (map[string]GameReferenceSummary, error) {
	summaries := make(map[string]GameReferenceSummary, len(s.games))
	for gameID := range s.games {
		summaries[gameID] = GameReferenceSummary{MediaAccountCount: len(s.accountReferences[gameID])}
	}
	users, err := s.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		for _, gameID := range user.GameIDs {
			summary := summaries[gameID]
			summary.UserScopeCount++
			summaries[gameID] = summary
		}
	}
	return summaries, nil
}

func (s *gameMemoryStore) GameReferences(gameID string) (model.GameReferences, error) {
	references := model.GameReferences{GameID: gameID, Users: []model.GameReferenceUser{}, MediaAccounts: append([]model.GameReferenceAccount(nil), s.accountReferences[gameID]...)}
	users, err := s.ListUsers()
	if err != nil {
		return model.GameReferences{}, err
	}
	for _, user := range users {
		if containsGame(user.GameIDs, gameID) {
			references.Users = append(references.Users, GameReferenceUser{UserID: user.ID, Username: user.Username, Role: user.Role, TeamID: user.TeamID, TeamName: user.TeamName})
		}
	}
	return references, nil
}

func (s *gameMemoryStore) UserHasGameReferencesOutsideScope(userID UserID, gameIDs []string) (bool, error) {
	for gameID, references := range s.accountReferences {
		if containsGame(gameIDs, gameID) {
			continue
		}
		for _, reference := range references {
			if reference.UserID == userID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *failingIdentityStore) CreateUserWithAudit(user User, event AuditEvent) (UserID, error) {
	if s.failAudit {
		return 0, errors.New("audit unavailable")
	}
	return s.Store.CreateUserWithAudit(user, event)
}

func (s *failingIdentityStore) DeleteUserWithAudit(userID UserID, event AuditEvent) error {
	if s.failAudit {
		return errors.New("audit unavailable")
	}
	return s.Store.DeleteUserWithAudit(userID, event)
}

func (s *failingIdentityStore) CreateTeamWithAudit(team OperationTeam, event AuditEvent) (TeamID, error) {
	if s.failAudit {
		return 0, errors.New("audit unavailable")
	}
	return s.Store.CreateTeamWithAudit(team, event)
}

func (s *failingIdentityStore) UpdateTeamWithAudit(team OperationTeam, event AuditEvent) error {
	if s.failAudit {
		return errors.New("audit unavailable")
	}
	return s.Store.UpdateTeamWithAudit(team, event)
}

func (s *failingIdentityStore) DeleteTeamWithAudit(teamID TeamID, event AuditEvent) error {
	if s.failAudit {
		return errors.New("audit unavailable")
	}
	return s.Store.DeleteTeamWithAudit(teamID, event)
}

func (s *failingIdentityStore) AppendAudit(event AuditEvent) error {
	if s.failAudit {
		return errors.New("audit unavailable")
	}
	return s.Store.AppendAudit(event)
}

func (s *failingIdentityStore) InvalidateUserSessionsForClientType(userID UserID, clientType ClientType, at time.Time) error {
	if s.failInvalidate {
		return errors.New("session invalidation unavailable")
	}
	return s.Store.InvalidateUserSessionsForClientType(userID, clientType, at)
}

func (s *failingIdentityStore) UpdateUserAndInvalidateSessions(user User, event AuditEvent) error {
	if s.failAudit {
		return errors.New("audit unavailable")
	}
	if s.failInvalidate {
		return errors.New("session invalidation unavailable")
	}
	return s.Store.UpdateUserAndInvalidateSessions(user, event)
}

func TestUpdateUserAccessFailureDoesNotLeaveChangedAccessWithLiveSession(t *testing.T) {
	base := NewMemoryStore()
	store := &failingIdentityStore{Store: base}
	service := NewService(store, WithTokenGenerator(func() string { return "operator-session" }))
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	oldTeam, _ := service.CreateTeam(admin.ID, "旧组")
	newTeam, _ := service.CreateTeam(admin.ID, "新组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &oldTeam.ID, GameIDs: []string{"game-a"},
	})
	login, _ := service.Login("operator-a", "a-long-operator-password")
	store.failAudit = true

	if _, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []string{"game-b"}); err == nil {
		t.Fatal("UpdateUserAccess() error = nil")
	}
	stored, _, _ := base.FindUser(operator.ID)
	if stored.Role != RoleOperator || stored.TeamID == nil || *stored.TeamID != oldTeam.ID || len(stored.GameIDs) != 1 || stored.GameIDs[0] != "game-a" {
		t.Fatalf("failed access update changed persisted scope: %+v", stored)
	}
	if _, err := service.Authenticate(login.Token); err != nil {
		t.Fatalf("failed access update invalidated unchanged session: %v", err)
	}
}

func TestCreateUserAuditFailureDoesNotPersistUser(t *testing.T) {
	base := NewMemoryStore()
	store := &failingIdentityStore{Store: base}
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team, _ := service.CreateTeam(admin.ID, "运营组")
	store.failAudit = true

	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, GameIDs: []string{"game-a"},
	}); err == nil {
		t.Fatal("CreateUser() error = nil")
	}
	if _, found, _ := base.FindUserByUsername("operator-a"); found {
		t.Fatal("failed audited create persisted user")
	}
}

func TestCreateTeamAuditFailureDoesNotPersistTeam(t *testing.T) {
	base := NewMemoryStore()
	store := &failingIdentityStore{Store: base}
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	store.failAudit = true

	if _, err := service.CreateTeam(admin.ID, "失败组"); err == nil {
		t.Fatal("CreateTeam() error = nil")
	}
	teams, _ := base.ListTeams()
	if len(teams) != 0 {
		t.Fatalf("failed audited team create persisted teams: %+v", teams)
	}
}

func TestResetPasswordFailureDoesNotLeaveNewPasswordWithLiveOldSession(t *testing.T) {
	base := NewMemoryStore()
	store := &failingIdentityStore{Store: base}
	service := NewService(store, WithTokenGenerator(func() string { return "operator-session" }))
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team, _ := service.CreateTeam(admin.ID, "密码组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, GameIDs: []string{"game-a"},
	})
	login, _ := service.Login("operator-a", "a-long-operator-password")
	store.failInvalidate = true

	if err := service.ResetPassword(admin.ID, operator.ID, "a-reset-operator-password"); err == nil {
		t.Fatal("ResetPassword() error = nil")
	}
	stored, _, _ := base.FindUser(operator.ID)
	if service.passwords.Compare([]byte(stored.PasswordHash), []byte("a-long-operator-password")) != nil {
		t.Fatal("failed reset changed persisted password")
	}
	if _, err := service.Authenticate(login.Token); err != nil {
		t.Fatalf("failed reset invalidated unchanged session: %v", err)
	}
}
