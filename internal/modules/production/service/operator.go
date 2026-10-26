package service

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

var (
	ErrComposeOperatorInvalid   = errors.New("compose operator reference is invalid")
	ErrComposeOperatorForbidden = errors.New("compose operator directory is forbidden")

	errComposeScopeInvalid   = errors.New("compose list scope is invalid")
	errComposeScopeForbidden = errors.New("compose list scope is forbidden")
)

// ComposeOperatorOption is the assignment picker's row: enough to name a person
// without exposing identity fields the production page does not show.
type ComposeOperatorOption struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
}

// ListComposeOperators answers who may be assigned in this team, optionally
// narrowed to one game. Production owns the endpoint so the identity module's own /users
// permission stays unchanged: a senior names only their own team, an admin
// names the target team explicitly, and an ordinary operator has no directory.
func ListComposeOperators(actor identity.PublicUser, teamID int64, gameID string) ([]ComposeOperatorOption, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrComposeOperatorForbidden
	}
	switch actor.Role {
	case identity.RoleSeniorOperator:
		if actor.TeamID == nil || (teamID != 0 && teamID != int64(*actor.TeamID)) {
			return nil, ErrComposeOperatorForbidden
		}
		teamID = int64(*actor.TeamID)
	case identity.RoleAdmin:
		if teamID <= 0 {
			return nil, fmt.Errorf("%w: team_id is required", ErrComposeOperatorInvalid)
		}
	default:
		return nil, ErrComposeOperatorForbidden
	}
	gameID = strings.TrimSpace(gameID)
	users, err := identity.ListUsers()
	if err != nil {
		return nil, err
	}
	return composeOperatorOptions(users, teamID, gameID), nil
}

// composeOperatorOptions filters the team directory. An empty gameID skips the
// game check: the task and output pages narrow by team alone, while strategy
// assignment always names its exact game.
func composeOperatorOptions(users []identity.PublicUser, teamID int64, gameID string) []ComposeOperatorOption {
	options := make([]ComposeOperatorOption, 0)
	for _, user := range users {
		if composeOperatorTeamMember(user, teamID) && (gameID == "" || slices.Contains(user.GameIDs, gameID)) {
			options = append(options, ComposeOperatorOption{ID: int64(user.ID), Username: user.Username, Nickname: user.Nickname, DisplayName: composeOperatorDisplayName(user)})
		}
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].DisplayName != options[j].DisplayName {
			return options[i].DisplayName < options[j].DisplayName
		}
		return options[i].ID < options[j].ID
	})
	return options
}

// composeOperatorTeamMember is the role/team half of eligibility: an enabled
// operator or senior operator on that team.
func composeOperatorTeamMember(user identity.PublicUser, teamID int64) bool {
	if user.Status != identity.UserStatusEnabled || (user.Role != identity.RoleOperator && user.Role != identity.RoleSeniorOperator) {
		return false
	}
	return user.TeamID != nil && int64(*user.TeamID) == teamID
}

// composeOperatorMatches is the one definition of a valid assignee: a team
// member responsible for the strategy's game.
func composeOperatorMatches(user identity.PublicUser, teamID int64, gameID string) bool {
	return gameID != "" && composeOperatorTeamMember(user, teamID) && slices.Contains(user.GameIDs, gameID)
}

// resolveComposeOperator validates an operator id against the same rule as the
// picker. A nil directory means the caller did not wire identity (unit tests);
// nothing can be validated there, so the reference is refused rather than
// assumed valid.
func resolveComposeOperator(operators func() ([]identity.PublicUser, error), teamID int64, gameID string, operatorID int64) (identity.PublicUser, bool, error) {
	if operators == nil || operatorID <= 0 {
		return identity.PublicUser{}, false, nil
	}
	users, err := operators()
	if err != nil {
		return identity.PublicUser{}, false, err
	}
	for _, user := range users {
		if int64(user.ID) == operatorID {
			return user, composeOperatorMatches(user, teamID, gameID), nil
		}
	}
	return identity.PublicUser{}, false, nil
}

func composeOperatorDisplayName(user identity.PublicUser) string {
	if strings.TrimSpace(user.Nickname) != "" {
		return strings.TrimSpace(user.Nickname)
	}
	return user.Username
}

// composeOperatorNameLookup resolves the names for one page in one directory
// read. Unassigned rows keep an empty name; the page renders them as 未记录
// rather than inventing a person.
func composeOperatorNameLookup(operators func() ([]identity.PublicUser, error), ids []int64) (map[int64]string, error) {
	if operators == nil || len(ids) == 0 {
		return nil, nil
	}
	wanted := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	users, err := operators()
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(wanted))
	for _, user := range users {
		if _, ok := wanted[int64(user.ID)]; ok {
			names[int64(user.ID)] = composeOperatorDisplayName(user)
		}
	}
	return names, nil
}

// composeListScope is the resolved operator visibility of a task or output
// list. IncludeUnownedCreatedBy is non-zero only for "mine": historical rows
// without an operator stay visible to their creator, but they are never
// attributed to them.
type composeListScope struct {
	TeamID         int64
	OperatorID     int64
	IncludeUnowned int64
}

// resolveComposeListScope applies one rule set to both production lists.
// Ordinary operators are pinned to themselves and may not ask for anything
// else; seniors default to their whole team and may narrow to one named
// operator; admins default to everything and may narrow by team or operator.
func resolveComposeListScope(actor identity.PublicUser, teamID, operatorID int64, scope string) (composeListScope, error) {
	if actor.Status != identity.UserStatusEnabled {
		return composeListScope{}, errComposeScopeForbidden
	}
	if teamID < 0 || operatorID < 0 {
		return composeListScope{}, errComposeScopeInvalid
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		switch actor.Role {
		case identity.RoleAdmin:
			scope = "all"
		case identity.RoleSeniorOperator:
			scope = "team"
		default:
			scope = "mine"
		}
	}
	switch scope {
	case "mine", "team", "all":
	default:
		return composeListScope{}, errComposeScopeInvalid
	}
	switch actor.Role {
	case identity.RoleOperator:
		if actor.TeamID == nil {
			return composeListScope{}, errComposeScopeForbidden
		}
		if scope != "mine" {
			return composeListScope{}, errComposeScopeForbidden
		}
		if operatorID != 0 && operatorID != int64(actor.ID) {
			return composeListScope{}, errComposeScopeForbidden
		}
		if teamID != 0 && teamID != int64(*actor.TeamID) {
			return composeListScope{}, errComposeScopeForbidden
		}
		return composeListScope{TeamID: int64(*actor.TeamID), OperatorID: int64(actor.ID), IncludeUnowned: int64(actor.ID)}, nil
	case identity.RoleSeniorOperator:
		if actor.TeamID == nil {
			return composeListScope{}, errComposeScopeForbidden
		}
		if teamID != 0 && teamID != int64(*actor.TeamID) {
			return composeListScope{}, errComposeScopeForbidden
		}
		if scope == "all" {
			return composeListScope{}, errComposeScopeInvalid
		}
		result := composeListScope{TeamID: int64(*actor.TeamID)}
		switch {
		case operatorID != 0:
			result.OperatorID = operatorID
		case scope == "mine":
			result.OperatorID = int64(actor.ID)
			result.IncludeUnowned = int64(actor.ID)
		}
		return result, nil
	case identity.RoleAdmin:
		if scope == "team" && teamID <= 0 {
			return composeListScope{}, errComposeScopeInvalid
		}
		result := composeListScope{TeamID: teamID}
		switch {
		case operatorID != 0:
			result.OperatorID = operatorID
		case scope == "mine":
			result.OperatorID = int64(actor.ID)
			result.IncludeUnowned = int64(actor.ID)
		}
		return result, nil
	default:
		return composeListScope{}, errComposeScopeForbidden
	}
}
