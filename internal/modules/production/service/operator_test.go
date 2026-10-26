package service

import (
	"errors"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

func TestComposeOperatorOptionsKeepOnlyEnabledTeamResponsiblePeople(t *testing.T) {
	team := identity.TeamID(7)
	otherTeam := identity.TeamID(8)
	users := []identity.PublicUser{
		{ID: 9, Username: "op9", Nickname: "小王", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}},
		{ID: 11, Username: "senior11", Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta", "gamma"}},
		{ID: 10, Username: "op10", Role: identity.RoleOperator, Status: identity.UserStatusDisabled, TeamID: &team, GameIDs: []string{"delta"}},
		{ID: 12, Username: "op12", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &otherTeam, GameIDs: []string{"delta"}},
		{ID: 13, Username: "admin13", Role: identity.RoleAdmin, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}},
		{ID: 14, Username: "op14", Nickname: "阿李", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"gamma"}},
	}
	options := composeOperatorOptions(users, 7, "delta")
	if len(options) != 2 || options[0].ID != 11 || options[1].ID != 9 {
		// Eligibility: 9 and 11 only; 14 is responsible for another game.
		// Sorted by display name, ASCII before CJK.
		t.Fatalf("options = %+v", options)
	}
	got := map[int64]string{}
	for _, option := range options {
		got[option.ID] = option.DisplayName
	}
	if got[9] != "小王" || got[11] != "senior11" {
		t.Fatalf("display names = %+v", got)
	}
	if _, ok := got[14]; ok {
		t.Fatalf("an operator responsible for another game must not be offered: %+v", options)
	}
	teamWide := composeOperatorOptions(users, 7, "")
	if len(teamWide) != 3 || teamWide[0].ID != 11 || teamWide[1].ID != 9 || teamWide[2].ID != 14 {
		t.Fatalf("team directory without a game = %+v", teamWide)
	}
}

func TestListComposeOperatorsRefusesBeforeTouchingIdentity(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	if _, err := ListComposeOperators(operator, 7, "delta"); !errors.Is(err, ErrComposeOperatorForbidden) {
		t.Fatalf("ordinary operator error = %v", err)
	}
	senior := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	if _, err := ListComposeOperators(senior, 8, "delta"); !errors.Is(err, ErrComposeOperatorForbidden) {
		t.Fatalf("senior other-team error = %v", err)
	}
	admin := identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	if _, err := ListComposeOperators(admin, 0, "delta"); !errors.Is(err, ErrComposeOperatorInvalid) {
		t.Fatalf("admin missing-team error = %v", err)
	}
}

func TestComposeListScopePerRole(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	scope, err := resolveComposeListScope(operator, 0, 0, "")
	if err != nil || scope.TeamID != 7 || scope.OperatorID != 9 || scope.IncludeUnowned != 9 {
		t.Fatalf("operator default scope=%+v err=%v", scope, err)
	}
	if _, err := resolveComposeListScope(operator, 0, 0, "team"); !errors.Is(err, errComposeScopeForbidden) {
		t.Fatalf("operator team scope error = %v", err)
	}
	if _, err := resolveComposeListScope(operator, 0, 10, ""); !errors.Is(err, errComposeScopeForbidden) {
		t.Fatalf("operator foreign operator error = %v", err)
	}

	senior := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	scope, err = resolveComposeListScope(senior, 0, 0, "")
	if err != nil || scope.TeamID != 7 || scope.OperatorID != 11 || scope.IncludeUnowned != 11 {
		t.Fatalf("senior default scope=%+v err=%v", scope, err)
	}
	scope, err = resolveComposeListScope(senior, 0, 0, "team")
	if err != nil || scope.OperatorID != 0 || scope.IncludeUnowned != 0 || scope.TeamID != 7 {
		t.Fatalf("senior team scope=%+v err=%v", scope, err)
	}
	scope, err = resolveComposeListScope(senior, 0, 9, "team")
	if err != nil || scope.OperatorID != 9 || scope.IncludeUnowned != 0 {
		t.Fatalf("senior named scope=%+v err=%v", scope, err)
	}
	if _, err := resolveComposeListScope(senior, 8, 0, ""); !errors.Is(err, errComposeScopeForbidden) {
		t.Fatalf("senior other-team error = %v", err)
	}
	if _, err := resolveComposeListScope(senior, 0, 0, "all"); !errors.Is(err, errComposeScopeInvalid) {
		t.Fatalf("senior all error = %v", err)
	}

	admin := identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	scope, err = resolveComposeListScope(admin, 0, 0, "")
	if err != nil || scope.TeamID != 0 || scope.OperatorID != 0 {
		t.Fatalf("admin default scope=%+v err=%v", scope, err)
	}
	scope, err = resolveComposeListScope(admin, 7, 9, "team")
	if err != nil || scope.TeamID != 7 || scope.OperatorID != 9 {
		t.Fatalf("admin narrowed scope=%+v err=%v", scope, err)
	}
	if _, err := resolveComposeListScope(admin, 0, 0, "team"); !errors.Is(err, errComposeScopeInvalid) {
		t.Fatalf("admin team without team_id error = %v", err)
	}
}
