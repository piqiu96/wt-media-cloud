package service

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

func TestOwnProfileDefaultsAndChoiceSurviveNewLogin(t *testing.T) {
	s := NewService(NewMemoryStore())
	user, err := s.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	if user.Nickname != "admin" || user.AvatarID != "sky" {
		t.Fatalf("defaults = %+v", user)
	}
	if _, err := s.UpdateOwnProfile(user.ID, "", "ocean"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty nickname = %v", err)
	}
	if _, err := s.UpdateOwnProfile(user.ID, "小明", "external-url"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("external avatar = %v", err)
	}
	updated, err := s.UpdateOwnProfile(user.ID, " 小明 ", "ocean")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "小明" || updated.AvatarID != "ocean" || updated.Username != "admin" {
		t.Fatalf("updated = %+v", updated)
	}
	logged, err := s.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	if logged.User.Nickname != "小明" || logged.User.AvatarID != "ocean" {
		t.Fatalf("login user = %+v", logged.User)
	}
}

func TestLoginReportsDistinctInputErrorsBeforeCredentialCheck(t *testing.T) {
	service := NewService(NewMemoryStore())
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	cases := []struct {
		name     string
		username string
		password string
		want     error
	}{
		{"empty username", "", "a-long-initial-password", ErrMissingCredentials},
		{"empty password", "admin", "", ErrMissingCredentials},
		{"username over 64 bytes", strings.Repeat("u", 65), "a-long-initial-password", ErrLoginInputInvalid},
		{"password under 6 bytes", "admin", "12345", ErrLoginInputInvalid},
		{"password over 72 bytes", "admin", strings.Repeat("p", 73), ErrLoginInputInvalid},
		{"in-range wrong password", "admin", "a-wrong-but-valid-password", ErrAuthenticationFailed},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := service.LoginWithOptions(testCase.username, testCase.password, LoginOptions{}); !errors.Is(err, testCase.want) {
				t.Fatalf("LoginWithOptions(%q, %q) error = %v, want %v", testCase.username, testCase.password, err, testCase.want)
			}
		})
	}
}

func TestNewDesktopLoginRequiresConfirmationBeforeInvalidatingPriorSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	tokens := []string{"session-first", "session-conflict", "session-second"}
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

	first, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatalf("first LoginWithOptions() error = %v", err)
	}
	if _, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop}); !errors.Is(err, ErrDesktopLoginConflict) {
		t.Fatalf("unconfirmed LoginWithOptions() error = %v, want ErrDesktopLoginConflict", err)
	}
	stillActive, err := service.Authenticate(first.Token)
	if err != nil {
		t.Fatalf("unconfirmed replacement invalidated first token: %v", err)
	}
	if stillActive.Username != "admin" {
		t.Fatalf("stillActive = %+v", stillActive)
	}
	second, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
		ClientType:       ClientTypeDesktop,
		ReplaceExisting:  true,
		ReplaceSessionID: first.SessionID,
	})
	if err != nil {
		t.Fatalf("confirmed LoginWithOptions() error = %v", err)
	}
	if first.Token == second.Token {
		t.Fatalf("replacement login reused token %q", first.Token)
	}

	if _, err := service.Authenticate(first.Token); !errors.Is(err, ErrDesktopReplaced) {
		t.Fatalf("Authenticate(first token) error = %v, want ErrDesktopReplaced", err)
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
		Username:    "operator",
		Password:    "a-long-operator-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
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

func TestOnlyAdminCanCreateUsersAndAssignCategoryScopes(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	team := mustCreateTeam(t, service, admin.ID, "权限测试组")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username:    "operator",
		Password:    "a-long-operator-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := service.CreateUser(operator.ID, CreateUserInput{
		Username:    "forbidden",
		Password:    "a-long-forbidden-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateUser(operator) error = %v, want ErrForbidden", err)
	}
	if !service.CanAccessCategory(operator.ID, 1) || service.CanAccessCategory(operator.ID, 2) {
		t.Fatalf("operator category scope is not enforced")
	}
	if !service.CanAccessCategory(admin.ID, 999) {
		t.Fatalf("admin must have global category access")
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
		Username:    "operator",
		Password:    "a-long-operator-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
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

func TestAdminCanUpdateRoleAndCategoryScopes(t *testing.T) {
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
		Username:    "operator",
		Password:    "a-long-operator-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	updated, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &team.ID, []CategoryID{2})
	if err != nil {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if updated.Role != RoleSeniorOperator || len(updated.CategoryIDs) != 1 || updated.CategoryIDs[0] != 2 {
		t.Fatalf("updated user = %+v", updated)
	}
	if service.CanAccessCategory(operator.ID, 1) || !service.CanAccessCategory(operator.ID, 2) {
		t.Fatalf("updated category scope is not enforced")
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
		Username:    "operator",
		Password:    "a-long-operator-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
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
	if admin.ID <= 0 || admin.Role != RoleAdmin || admin.TeamID != nil || len(admin.CategoryIDs) != 0 {
		t.Fatalf("admin = %+v", admin)
	}

	team, err := service.CreateTeam(admin.ID, "火影组")
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username:    "operator-a",
		Password:    "a-long-operator-password",
		Role:        RoleOperator,
		TeamID:      &team.ID,
		CategoryIDs: []CategoryID{1},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if operator.TeamID == nil || *operator.TeamID != team.ID || operator.TeamName != "火影组" {
		t.Fatalf("operator = %+v", operator)
	}

	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username:    "missing-team",
		Password:    "a-long-operator-password",
		Role:        RoleSeniorOperator,
		CategoryIDs: []CategoryID{1},
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
		TeamID: &team.ID, CategoryIDs: []CategoryID{1},
	})

	teams, err := service.ListTeams(operator.ID, "")
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

func TestReferencedCategoryCannotBeDisabledOrDeleted(t *testing.T) {
	store := newCategoryMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team, _ := service.CreateTeam(admin.ID, "火影组")
	category, err := service.CreateCategory(admin.ID, CategoryTypeGame, "火影忍者", "")
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if _, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, CategoryIDs: []CategoryID{category.ID},
	}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := service.UpdateCategory(admin.ID, category.ID, category.Name, false, ""); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("UpdateCategory(disable referenced) error = %v, want ErrCategoryInUse", err)
	}
	if err := service.DeleteCategory(admin.ID, category.ID); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("DeleteCategory(referenced) error = %v, want ErrCategoryInUse", err)
	}

	empty, err := service.CreateCategory(admin.ID, CategoryTypeGame, "三角洲", "")
	if err != nil {
		t.Fatalf("CreateCategory(empty) error = %v", err)
	}
	if _, err := service.UpdateCategory(admin.ID, empty.ID, empty.Name, false, ""); err != nil {
		t.Fatalf("UpdateCategory(disable empty) error = %v", err)
	}
	if err := service.DeleteCategory(admin.ID, empty.ID); err != nil {
		t.Fatalf("DeleteCategory(empty) error = %v", err)
	}
}

func TestAdminCategoryListIncludesReferenceSummaryAndDetails(t *testing.T) {
	store := newCategoryMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team := mustCreateTeam(t, service, admin.ID, "火影组")
	category, err := service.CreateCategory(admin.ID, CategoryTypeGame, "火影忍者", "")
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, CategoryIDs: []CategoryID{category.ID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	store.accountReferences[category.ID] = []CategoryReferenceAccount{{AccountID: "8", Name: "B站账号", Platform: "bilibili", UserID: operator.ID, Username: operator.Username}}

	categories, err := service.ListCategories(admin.ID)
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(categories) != 1 || categories[0].ReferenceSummary == nil || categories[0].ReferenceSummary.UserScopeCount != 1 || categories[0].ReferenceSummary.MediaAccountCount != 1 || categories[0].ReferenceSummary.TotalCount != 2 {
		t.Fatalf("ListCategories() = %#v, want user=1 account=1 total=2", categories)
	}
	references, err := service.CategoryReferences(admin.ID, category.ID)
	if err != nil {
		t.Fatalf("CategoryReferences() error = %v", err)
	}
	if len(references.Users) != 1 || references.Users[0].Username != "operator-a" || len(references.MediaAccounts) != 1 || references.MediaAccounts[0].AccountID != "8" {
		t.Fatalf("CategoryReferences() = %#v", references)
	}
	if _, err := service.CategoryReferences(operator.ID, category.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CategoryReferences(operator) error = %v, want ErrForbidden", err)
	}
}

func TestListAllCategoriesReturnsEnabledCatalogForOperators(t *testing.T) {
	store := newCategoryMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team := mustCreateTeam(t, service, admin.ID, "火影组")
	naruto, err := service.CreateCategory(admin.ID, CategoryTypeGame, "火影忍者", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateCategory(admin.ID, CategoryTypeGame, "三角洲", ""); err != nil {
		t.Fatal(err)
	}
	retired, err := service.CreateCategory(admin.ID, CategoryTypeGame, "停用类目", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateCategory(admin.ID, retired.ID, retired.Name, false, ""); err != nil {
		t.Fatal(err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-catalog", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, CategoryIDs: []CategoryID{naruto.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	assigned, err := service.ListCategories(operator.ID)
	if err != nil || len(assigned) != 1 || assigned[0].ID != naruto.ID {
		t.Fatalf("assigned categories = %#v, err=%v", assigned, err)
	}
	catalog, err := service.ListAllCategories(operator.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 {
		t.Fatalf("catalog = %#v, want two enabled categories", catalog)
	}
	for _, category := range catalog {
		if category.ID == retired.ID || !category.Enabled {
			t.Fatalf("catalog leaked a disabled category: %#v", category)
		}
	}
	adminCatalog, err := service.ListAllCategories(admin.ID)
	if err != nil || len(adminCatalog) != 3 {
		t.Fatalf("admin catalog = %#v, err=%v, want all three categories", adminCatalog, err)
	}
}

func TestListManagementCategoriesShowsEveryStatusWithoutReferenceSummaries(t *testing.T) {
	store := newCategoryMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team := mustCreateTeam(t, service, admin.ID, "火影组")
	naruto, err := service.CreateCategory(admin.ID, CategoryTypeGame, "火影忍者", "")
	if err != nil {
		t.Fatal(err)
	}
	retired, err := service.CreateCategory(admin.ID, CategoryTypeGame, "停用类目", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateCategory(admin.ID, retired.ID, retired.Name, false, ""); err != nil {
		t.Fatal(err)
	}
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-mgmt", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, CategoryIDs: []CategoryID{naruto.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	senior, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "senior-mgmt", Password: "a-long-operator-password", Role: RoleSeniorOperator,
		TeamID: &team.ID, CategoryIDs: []CategoryID{naruto.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The admin list carries summaries; the management list must not, or a
	// senior would read counts for a reference drawer they cannot open.
	categories, err := service.ListManagementCategories(senior.ID)
	if err != nil {
		t.Fatalf("ListManagementCategories(senior) error = %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("management categories = %#v, want both enabled and disabled", categories)
	}
	for _, category := range categories {
		if category.ReferenceSummary != nil {
			t.Fatalf("management categories leaked a reference summary: %#v", category)
		}
	}
	if _, err := service.ListManagementCategories(operator.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListManagementCategories(operator) error = %v, want ErrForbidden", err)
	}
}

func TestListTeamsReportsMembershipCount(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	alpha := mustCreateTeam(t, service, admin.ID, "甲组")
	beta := mustCreateTeam(t, service, admin.ID, "乙组")
	for _, input := range []CreateUserInput{
		{Username: "alpha-a", Password: "a-long-operator-password", Role: RoleOperator, TeamID: &alpha.ID, CategoryIDs: []CategoryID{1}},
		{Username: "alpha-b", Password: "a-long-operator-password", Role: RoleSeniorOperator, TeamID: &alpha.ID, CategoryIDs: []CategoryID{1}},
		{Username: "beta-a", Password: "a-long-operator-password", Role: RoleOperator, TeamID: &beta.ID, CategoryIDs: []CategoryID{1}},
	} {
		if _, err := service.CreateUser(admin.ID, input); err != nil {
			t.Fatal(err)
		}
	}
	teams, err := service.ListTeams(admin.ID, "")
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	counts := make(map[TeamID]int64, len(teams))
	for _, team := range teams {
		counts[team.ID] = team.UserCount
	}
	if counts[alpha.ID] != 2 || counts[beta.ID] != 1 {
		t.Fatalf("team user counts = %#v, want 甲组=2 乙组=1", counts)
	}
	if filtered, err := service.ListTeams(admin.ID, "甲"); err != nil || len(filtered) != 1 || filtered[0].UserCount != 2 {
		t.Fatalf("filtered teams = %#v, err=%v", filtered, err)
	}
}

func TestUserCategoryScopeCannotDropReferencedAccountCategory(t *testing.T) {
	store := newCategoryMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team := mustCreateTeam(t, service, admin.ID, "火影组")
	naruto, _ := service.CreateCategory(admin.ID, CategoryTypeGame, "火影忍者", "")
	delta, _ := service.CreateCategory(admin.ID, CategoryTypeGame, "三角洲", "")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &team.ID, CategoryIDs: []CategoryID{naruto.ID, delta.ID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	store.accountReferences[naruto.ID] = []CategoryReferenceAccount{{AccountID: "8", UserID: operator.ID}}

	if _, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleOperator, &team.ID, []CategoryID{delta.ID}); !errors.Is(err, ErrUserCategoryScopeInUse) {
		t.Fatalf("UpdateUserAccess(drop referenced category) error = %v, want ErrUserCategoryScopeInUse", err)
	}
}

func TestServiceResolveCategoryReturnsStoredCategoryWithoutActorScope(t *testing.T) {
	store := newCategoryMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	category, err := service.CreateCategory(admin.ID, CategoryTypeGame, "火影忍者", "")
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}

	resolved, found, err := service.ResolveCategory(category.ID)
	if err != nil || !found || resolved.ID != category.ID || !resolved.Enabled {
		t.Fatalf("ResolveCategory() = %#v, found=%v, err=%v", resolved, found, err)
	}
}

func TestUpdateUserAccessTransfersCurrentTeamAndInvalidatesSession(t *testing.T) {
	service := NewService(NewMemoryStore(), WithTokenGenerator(func() string { return "operator-session" }))
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	oldTeam, _ := service.CreateTeam(admin.ID, "旧组")
	newTeam, _ := service.CreateTeam(admin.ID, "新组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &oldTeam.ID, CategoryIDs: []CategoryID{1},
	})
	login, err := service.Login("operator-a", "a-long-operator-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	updated, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []CategoryID{2})
	if err != nil {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if updated.TeamID == nil || *updated.TeamID != newTeam.ID || updated.Role != RoleSeniorOperator {
		t.Fatalf("updated = %+v", updated)
	}
	if _, err := service.Authenticate(login.Token); !errors.Is(err, ErrSessionInvalidated) {
		t.Fatalf("Authenticate(old session) error = %v, want ErrSessionInvalidated", err)
	}
}

func TestUpdateUserRejectsInvalidStatusWithoutChangingAccess(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	oldTeam, _ := service.CreateTeam(admin.ID, "旧组")
	newTeam, _ := service.CreateTeam(admin.ID, "新组")
	operator, _ := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &oldTeam.ID, CategoryIDs: []CategoryID{1},
	})

	if _, err := service.UpdateUser(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []CategoryID{2}, UserStatus("invalid")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateUser() error = %v, want ErrInvalidInput", err)
	}
	stored, _, _ := service.store.FindUser(operator.ID)
	if stored.Role != RoleOperator || stored.Status != UserStatusEnabled || stored.TeamID == nil || *stored.TeamID != oldTeam.ID || len(stored.CategoryIDs) != 1 || stored.CategoryIDs[0] != 1 {
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
		TeamID: &oldTeam.ID, CategoryIDs: []CategoryID{1},
	})
	login, _ := service.Login("operator-a", "a-long-operator-password")

	updated, err := service.UpdateUser(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []CategoryID{2}, UserStatusDisabled)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.Role != RoleSeniorOperator || updated.Status != UserStatusDisabled || updated.TeamID == nil || *updated.TeamID != newTeam.ID {
		t.Fatalf("updated = %+v", updated)
	}
	if _, err := service.Authenticate(login.Token); !errors.Is(err, ErrSessionInvalidated) {
		t.Fatalf("Authenticate(old session) error = %v, want ErrSessionInvalidated", err)
	}
}

func TestPublicUserCanAccessUsesRoleTeamAndCategoryIntersection(t *testing.T) {
	teamA := TeamID(10)
	teamB := TeamID(20)
	tests := []struct {
		name       string
		actor      PublicUser
		ownerID    UserID
		teamID     *TeamID
		categoryID CategoryID
		want       bool
	}{
		{name: "admin all", actor: PublicUser{ID: 1, Role: RoleAdmin, Status: UserStatusEnabled}, ownerID: 99, teamID: &teamB, categoryID: 99, want: true},
		{name: "senior same team and category", actor: PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA, CategoryIDs: []CategoryID{1}}, ownerID: 99, teamID: &teamA, categoryID: 1, want: true},
		{name: "senior cross team", actor: PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA, CategoryIDs: []CategoryID{1}}, ownerID: 99, teamID: &teamB, categoryID: 1, want: false},
		{name: "senior cross category", actor: PublicUser{ID: 2, Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamA, CategoryIDs: []CategoryID{1}}, ownerID: 99, teamID: &teamA, categoryID: 2, want: false},
		{name: "operator self and category", actor: PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA, CategoryIDs: []CategoryID{1}}, ownerID: 3, teamID: &teamA, categoryID: 1, want: true},
		{name: "operator other user", actor: PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA, CategoryIDs: []CategoryID{1}}, ownerID: 4, teamID: &teamA, categoryID: 1, want: false},
		{name: "operator cross category", actor: PublicUser{ID: 3, Role: RoleOperator, Status: UserStatusEnabled, TeamID: &teamA, CategoryIDs: []CategoryID{1}}, ownerID: 3, teamID: &teamA, categoryID: 2, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actor.CanAccess(tt.ownerID, tt.teamID, tt.categoryID); got != tt.want {
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
		t.Fatal("admin cannot access non-category resource")
	}
	if !senior.CanAccessOwnedResource(99, &teamA) || senior.CanAccessOwnedResource(99, &teamB) {
		t.Fatal("senior non-category resource scope is not limited to own team")
	}
	if !operator.CanAccessOwnedResource(operator.ID, &teamA) || operator.CanAccessOwnedResource(99, &teamA) {
		t.Fatal("operator non-category resource scope is not limited to self")
	}
}

func TestAccessUpdatesAuditTeamAndCategories(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	teamA, _ := service.CreateTeam(admin.ID, "旧组")
	teamB, _ := service.CreateTeam(admin.ID, "新组")
	operator, err := service.CreateUser(admin.ID, CreateUserInput{
		Username: "operator-a", Password: "a-long-operator-password", Role: RoleOperator,
		TeamID: &teamA.ID, CategoryIDs: []CategoryID{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &teamB.ID, []CategoryID{3, 2}); err != nil {
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
		if summary["team_id"] == "" || summary["category_ids"] == "" {
			t.Fatalf("%s audit does not trace team and category scope: %#v", name, summary)
		}
	}
}

type failingIdentityStore struct {
	Store
	failAudit      bool
	failInvalidate bool
}

type categoryMemoryStore struct {
	*memoryStore
	categories        map[CategoryID]Category
	accountReferences map[CategoryID][]CategoryReferenceAccount
	nextCategoryID    CategoryID
}

func newCategoryMemoryStore() *categoryMemoryStore {
	return &categoryMemoryStore{memoryStore: NewMemoryStore(), categories: make(map[CategoryID]Category), accountReferences: make(map[CategoryID][]CategoryReferenceAccount)}
}

func (s *categoryMemoryStore) CreateCategoryWithAudit(category Category, event AuditEvent) (CategoryID, error) {
	for _, existing := range s.categories {
		if existing.Name == category.Name {
			return 0, ErrCategoryNameTaken
		}
	}
	s.nextCategoryID++
	category.ID = s.nextCategoryID
	s.categories[category.ID] = category
	return category.ID, s.AppendAudit(event)
}

func (s *categoryMemoryStore) FindCategory(id CategoryID) (Category, bool, error) {
	category, ok := s.categories[id]
	return category, ok, nil
}

func (s *categoryMemoryStore) ListCategories() ([]Category, error) {
	result := make([]Category, 0, len(s.categories))
	for _, category := range s.categories {
		result = append(result, category)
	}
	return result, nil
}

func (s *categoryMemoryStore) UpdateCategoryWithAudit(category Category, event AuditEvent) error {
	if _, exists := s.categories[category.ID]; !exists {
		return ErrInvalidInput
	}
	for _, existing := range s.categories {
		if existing.ID != category.ID && existing.Name == category.Name {
			return ErrCategoryNameTaken
		}
	}
	s.categories[category.ID] = category
	return s.AppendAudit(event)
}

func (s *categoryMemoryStore) DeleteCategoryWithAudit(id CategoryID, event AuditEvent) error {
	if _, exists := s.categories[id]; !exists {
		return ErrInvalidInput
	}
	delete(s.categories, id)
	return s.AppendAudit(event)
}

func (s *categoryMemoryStore) CategoryHasReferences(id CategoryID) (bool, error) {
	users, err := s.ListUsers()
	if err != nil {
		return false, err
	}
	for _, user := range users {
		for _, categoryID := range user.CategoryIDs {
			if categoryID == id {
				return true, nil
			}
		}
	}
	return len(s.accountReferences[id]) > 0, nil
}

func (s *categoryMemoryStore) ListCategoryReferenceSummaries() (map[CategoryID]CategoryReferenceSummary, error) {
	summaries := make(map[CategoryID]CategoryReferenceSummary, len(s.categories))
	for categoryID := range s.categories {
		summaries[categoryID] = CategoryReferenceSummary{MediaAccountCount: len(s.accountReferences[categoryID])}
	}
	users, err := s.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		for _, categoryID := range user.CategoryIDs {
			summary := summaries[categoryID]
			summary.UserScopeCount++
			summaries[categoryID] = summary
		}
	}
	return summaries, nil
}

func (s *categoryMemoryStore) CategoryReferences(categoryID CategoryID) (model.CategoryReferences, error) {
	references := model.CategoryReferences{CategoryID: categoryID, Users: []model.CategoryReferenceUser{}, MediaAccounts: append([]model.CategoryReferenceAccount(nil), s.accountReferences[categoryID]...)}
	users, err := s.ListUsers()
	if err != nil {
		return model.CategoryReferences{}, err
	}
	for _, user := range users {
		if containsCategory(user.CategoryIDs, categoryID) {
			references.Users = append(references.Users, CategoryReferenceUser{UserID: user.ID, Username: user.Username, Role: user.Role, TeamID: user.TeamID, TeamName: user.TeamName})
		}
	}
	return references, nil
}

func (s *categoryMemoryStore) UserHasCategoryReferencesOutsideScope(userID UserID, categoryIDs []CategoryID) (bool, error) {
	for categoryID, references := range s.accountReferences {
		if containsCategory(categoryIDs, categoryID) {
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

func (s *failingIdentityStore) InvalidateUserSessions(userID UserID, at time.Time, reason string) error {
	if s.failInvalidate {
		return errors.New("session invalidation unavailable")
	}
	return s.Store.InvalidateUserSessions(userID, at, reason)
}

func (s *failingIdentityStore) UpdateUserAndInvalidateSessions(user User, event AuditEvent, reason string) error {
	if s.failAudit {
		return errors.New("audit unavailable")
	}
	if s.failInvalidate {
		return errors.New("session invalidation unavailable")
	}
	return s.Store.UpdateUserAndInvalidateSessions(user, event, reason)
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
		TeamID: &oldTeam.ID, CategoryIDs: []CategoryID{1},
	})
	login, _ := service.Login("operator-a", "a-long-operator-password")
	store.failAudit = true

	if _, err := service.UpdateUserAccess(admin.ID, operator.ID, RoleSeniorOperator, &newTeam.ID, []CategoryID{2}); err == nil {
		t.Fatal("UpdateUserAccess() error = nil")
	}
	stored, _, _ := base.FindUser(operator.ID)
	if stored.Role != RoleOperator || stored.TeamID == nil || *stored.TeamID != oldTeam.ID || len(stored.CategoryIDs) != 1 || stored.CategoryIDs[0] != 1 {
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
		TeamID: &team.ID, CategoryIDs: []CategoryID{1},
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
	teams, _ := base.ListTeams("")
	if len(teams) != 0 {
		t.Fatalf("failed audited team create persisted teams: %+v", teams)
	}
}

func TestListTeamsFiltersByKeywordAndID(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	first, _ := service.CreateTeam(admin.ID, "火影组")
	service.CreateTeam(admin.ID, "海贼组")

	byName, err := service.ListTeams(admin.ID, "海贼")
	if err != nil || len(byName) != 1 || byName[0].Name != "海贼组" {
		t.Fatalf("ListTeams(keyword) = %#v, err = %v", byName, err)
	}
	byID, err := service.ListTeams(admin.ID, strconv.FormatInt(int64(first.ID), 10))
	if err != nil || len(byID) != 1 || byID[0].ID != first.ID {
		t.Fatalf("ListTeams(id) = %#v, err = %v", byID, err)
	}
	all, err := service.ListTeams(admin.ID, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListTeams(empty) = %#v, err = %v", all, err)
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
		TeamID: &team.ID, CategoryIDs: []CategoryID{1},
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
