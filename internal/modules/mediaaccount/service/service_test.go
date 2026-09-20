package service

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	profileguardservice "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/service"
)

func TestServiceCreatesPendingAccountWithinActorScope(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		GameIDs:        []string{"game-a"},
		Platform:       PlatformBilibili,
		OriginalCookie: "secret-cookie",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.IdentificationStatus != IdentificationPending || account.BusinessStatus != BusinessEnabled || account.LoginStatus != LoginUnknown {
		t.Fatalf("account defaults = %#v", account)
	}
	stored, _, _ := store.Find(account.ID)
	if stored.OriginalCookie != "secret-cookie" {
		t.Fatalf("stored original cookie = %q", stored.OriginalCookie)
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || containsSecretField(string(encoded)) {
		t.Fatalf("public account exposed secret fields: %s", encoded)
	}
	auditJSON, err := json.Marshal(store.audits)
	if err != nil {
		t.Fatal(err)
	}
	if containsSecretField(string(auditJSON)) {
		t.Fatalf("audit exposed secret fields: %s", auditJSON)
	}
}

func TestServiceAllowsLedgerAccountWithoutGame(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	_, err := service.CreateAccount(actor, CreateAccountInput{Platform: PlatformBilibili, Remark: "先建台账后绑定游戏"})
	if err != nil {
		t.Fatalf("CreateAccount(no game) error = %v", err)
	}
}

func TestServiceRejectsCrossUserScope(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := mediaActor(1, 10, identityservice.RoleSeniorOperator)

	_, err := service.CreateAccount(actor, CreateAccountInput{UserID: identityservice.UserID(2), GameIDs: []string{"game-a"}, Platform: PlatformDouyin})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-user error = %v, want ErrForbidden", err)
	}
}

func TestServiceAllowsEnabledPublicGamesAndNormalizesTheirSet(t *testing.T) {
	service := newTestServiceWithGames(newMemoryStore(), fakeGameResolver{games: map[string]identityservice.OperationGame{
		"game-a": {ID: "game-a", Status: identityservice.GameStatusEnabled},
		"game-b": {ID: "game-b", Status: identityservice.GameStatusEnabled},
	}})
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		GameIDs:  []string{"game-b", "game-a", "game-b"},
		Platform: PlatformDouyin,
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if want := []string{"game-a", "game-b"}; strings.Join(account.GameIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("GameIDs = %#v, want %#v", account.GameIDs, want)
	}
}

func TestServiceReplacesAndClearsEntireGameSet(t *testing.T) {
	service := newTestServiceWithGames(newMemoryStore(), fakeGameResolver{games: map[string]identityservice.OperationGame{
		"game-a": {ID: "game-a", Status: identityservice.GameStatusEnabled},
		"game-b": {ID: "game-b", Status: identityservice.GameStatusEnabled},
		"game-z": {ID: "game-z", Status: identityservice.GameStatusEnabled},
	}})
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a", "game-b"}, Platform: PlatformDouyin})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	updated, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{GameIDs: gameIDsPtr([]string{"game-z", "game-b"})})
	if err != nil {
		t.Fatalf("UpdateAccount(replace) error = %v", err)
	}

	updated, err = service.UpdateAccount(actor, account.ID, UpdateAccountInput{GameIDs: gameIDsPtr([]string{})})
	if err != nil {
		t.Fatalf("UpdateAccount(clear) error = %v", err)
	}
	if len(updated.GameIDs) != 0 {
		t.Fatalf("cleared GameIDs = %#v", updated.GameIDs)
	}
}

func TestServiceRejectsDisabledPublicGame(t *testing.T) {
	service := newTestServiceWithGames(newMemoryStore(), fakeGameResolver{games: map[string]identityservice.OperationGame{
		"game-disabled": {ID: "game-disabled", Status: identityservice.GameStatusDisabled},
	}})

	_, err := service.CreateAccount(mediaActor(1, 10, identityservice.RoleOperator), CreateAccountInput{
		GameIDs:  []string{"game-disabled"},
		Platform: PlatformBilibili,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateAccount(disabled game) error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceAdminCanAssignGlobalAccount(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := identityservice.PublicUser{ID: identityservice.UserID(99), Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}

	account, err := service.CreateAccount(actor, CreateAccountInput{UserID: identityservice.UserID(2), GameIDs: []string{"game-z"}, Platform: PlatformBaijiahao})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.UserID != identityservice.UserID(2) {
		t.Fatalf("account user = %d, want 2", account.UserID)
	}
}

func TestServiceSeniorCanManageButCannotCreateSameTeamGameAccountForAnotherUser(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	users := fakeUserResolver{users: map[identityservice.UserID]identityservice.PublicUser{
		2: {ID: 2, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a"}},
	}}
	service := newAccountService(store, withUserResolver(users), withGameResolver(defaultTestGameResolver()))
	actor := mediaActor(1, teamID, identityservice.RoleSeniorOperator)

	if _, err := service.CreateAccount(actor, CreateAccountInput{UserID: 2, GameIDs: []string{"game-a"}, Platform: PlatformDouyin}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateAccount(same team/game) error=%v, want ErrForbidden", err)
	}
	store.records["account-2"] = AccountRecord{Account: Account{
		ID: "account-2", UserID: 2, TeamID: &teamID, GameIDs: []string{"game-a"}, Platform: PlatformDouyin,
	}}
	if _, err := service.UpdateAccount(actor, "account-2", UpdateAccountInput{BusinessStatus: BusinessDisabled}); err != nil {
		t.Fatalf("UpdateAccount(same team/game) error=%v", err)
	}
}

func TestFullCookieRecordRejectsSeniorForAnotherUsersAccount(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	store.records["account-1"] = AccountRecord{
		Account:        Account{ID: "account-1", UserID: 2, TeamID: &teamID, GameIDs: []string{"game-a"}},
		OriginalCookie: "secret-cookie",
	}
	service := newAccountService(store)
	senior := mediaActor(1, teamID, identityservice.RoleSeniorOperator)
	admin := identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}

	if _, err := service.GetAccountRecord(senior, "account-1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetAccountRecord(senior) error=%v, want ErrForbidden", err)
	}
	if record, err := service.GetAccountRecord(admin, "account-1"); err != nil || record.OriginalCookie != "secret-cookie" {
		t.Fatalf("GetAccountRecord(admin) record=%+v error=%v", record, err)
	}
	owner := mediaActor(2, teamID, identityservice.RoleOperator)
	if _, err := service.GetOwnedAccountRecord(owner, "account-1"); err != nil {
		t.Fatalf("GetOwnedAccountRecord(owner) error=%v", err)
	}
	for name, actor := range map[string]identityservice.PublicUser{"senior": senior, "admin": admin} {
		if _, err := service.GetOwnedAccountRecord(actor, "account-1"); !errors.Is(err, ErrForbidden) {
			t.Fatalf("GetOwnedAccountRecord(%s) error=%v, want ErrForbidden", name, err)
		}
	}
}

func TestServiceIdentifiesAccountAndMarksUserScopedDuplicate(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	first, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili, OriginalCookie: "first-secret"})
	if err != nil {
		t.Fatal(err)
	}
	identified, err := service.IdentifyAccount(actor, first.ID, IdentifyAccountInput{PlatformAccountID: "platform-42", Name: "main", AvatarURL: "https://example.invalid/a.png", LoginStatus: LoginNormal})
	if err != nil {
		t.Fatalf("IdentifyAccount() error = %v", err)
	}
	if identified.IdentificationStatus != IdentificationIdentified || identified.PlatformAccountID != "platform-42" {
		t.Fatalf("identified = %#v", identified)
	}

	candidate, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili, OriginalCookie: "candidate-secret"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.IdentifyAccount(actor, candidate.ID, IdentifyAccountInput{PlatformAccountID: "platform-42", Name: "duplicate", LoginStatus: LoginNormal})
	if !errors.Is(err, ErrDuplicateAccount) {
		t.Fatalf("duplicate error = %v", err)
	}
	if duplicate.IdentificationStatus != IdentificationDuplicate || duplicate.DuplicateOfAccountID != first.ID {
		t.Fatalf("duplicate = %#v", duplicate)
	}
	storedFirst, _, _ := store.Find(first.ID)
	storedCandidate, _, _ := store.Find(candidate.ID)
	if storedFirst.OriginalCookie != "first-secret" || storedCandidate.OriginalCookie != "candidate-secret" {
		t.Fatal("duplicate identification overwrote cookie facts")
	}
}

func TestServiceIdentityUniquenessIsPerUser(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	admin := identityservice.PublicUser{ID: identityservice.UserID(99), Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}

	for _, userID := range []identityservice.UserID{1, 2} {
		account, err := service.CreateAccount(admin, CreateAccountInput{UserID: userID, GameIDs: []string{"game-a"}, Platform: PlatformDouyin})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.IdentifyAccount(admin, account.ID, IdentifyAccountInput{PlatformAccountID: "same-platform-id", LoginStatus: LoginNormal}); err != nil {
			t.Fatalf("user %d identify error = %v", userID, err)
		}
	}
}

func TestServiceValidatesBusinessAndLoginStatusesIndependently(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformDouyin})

	updated, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{BusinessStatus: BusinessDisabled, LoginStatus: LoginVerificationNeeded})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.BusinessStatus != BusinessDisabled || updated.LoginStatus != LoginVerificationNeeded {
		t.Fatalf("updated = %#v", updated)
	}
	_, err = service.UpdateAccount(actor, account.ID, UpdateAccountInput{BusinessStatus: BusinessStatus("bad")})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid status error = %v", err)
	}
}

func TestServiceBulkTagsAndFilters(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformDouyin})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})

	if err := service.AddTags(actor, []string{first.ID, second.ID}, []string{" launch ", "vip", "vip"}); err != nil {
		t.Fatalf("AddTags() error = %v", err)
	}
	if err := service.AddTags(actor, []string{first.ID}, []string{"douyin"}); err != nil {
		t.Fatal(err)
	}

	assertAccountIDs(t, service, actor, AccountFilter{AnyTags: []string{"douyin", "missing"}}, first.ID)
	assertAccountIDs(t, service, actor, AccountFilter{AllTags: []string{"launch", "vip"}}, first.ID, second.ID)
	assertAccountIDs(t, service, actor, AccountFilter{ExcludeTags: []string{"douyin"}}, second.ID)

	if err := service.RemoveTags(actor, []string{second.ID}, []string{"vip"}); err != nil {
		t.Fatalf("RemoveTags() error = %v", err)
	}
	assertAccountIDs(t, service, actor, AccountFilter{AllTags: []string{"launch", "vip"}}, first.ID)
}

func TestServiceRejectsTaggingAnotherUsersAccount(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	admin := identityservice.PublicUser{ID: identityservice.UserID(99), Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}
	account, _ := service.CreateAccount(admin, CreateAccountInput{UserID: identityservice.UserID(2), GameIDs: []string{"game-a"}, Platform: PlatformDouyin})
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	err := service.AddTags(actor, []string{account.ID}, []string{"forbidden"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AddTags() error = %v, want ErrForbidden", err)
	}
}

func TestServiceBindsOnlySameUserActiveProfileAndOnePlatform(t *testing.T) {
	store := newMemoryStore()
	resolver := &fakeProfileResolver{profiles: map[string]identityservice.UserID{"profile-1": 1, "profile-2": 2}, inactive: map[string]bool{}}
	service := newTestServiceWithProfiles(store, resolver)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformDouyin})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformDouyin})

	bound, err := service.BindProfile(actor, first.ID, "profile-1")
	if err != nil || bound.BrowserProfileID != "profile-1" {
		t.Fatalf("BindProfile() account=%#v error=%v", bound, err)
	}
	storedFirst, _, _ := store.Find(first.ID)
	if storedFirst.LoginStatus != LoginUnknown || storedFirst.LastCheckedAt != nil {
		t.Fatalf("binding must reset login check facts: %#v", storedFirst)
	}
	if _, err := service.BindProfile(actor, second.ID, "profile-1"); !errors.Is(err, ErrProfilePlatformTaken) {
		t.Fatalf("same platform bind error = %v", err)
	}
	if _, err := service.BindProfile(actor, second.ID, "profile-2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-user bind error = %v", err)
	}
	resolver.inactive["profile-3"] = true
	resolver.profiles["profile-3"] = 1
	if _, err := service.BindProfile(actor, second.ID, "profile-3"); !errors.Is(err, ErrProfileUnavailable) {
		t.Fatalf("inactive bind error = %v", err)
	}
}

func TestServiceUnbindsProfileAndRejectsDisabledBinding(t *testing.T) {
	store := newMemoryStore()
	resolver := &fakeProfileResolver{profiles: map[string]identityservice.UserID{"profile-1": 1}, inactive: map[string]bool{}}
	service := newTestServiceWithProfiles(store, resolver)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})
	bound, err := service.BindProfile(actor, account.ID, "profile-1")
	if err != nil {
		t.Fatalf("BindProfile() error=%v", err)
	}
	store.records[account.ID] = AccountRecord{Account: bound}
	unbound, err := service.UnbindProfile(actor, account.ID)
	if err != nil {
		t.Fatalf("UnbindProfile() error=%v", err)
	}
	if unbound.BrowserProfileID != "" || unbound.LoginStatus != LoginUnknown {
		t.Fatalf("unbound account = %#v", unbound)
	}
	store.records[account.ID] = AccountRecord{Account: Account{ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, GameIDs: []string{"game-a"}, Platform: PlatformBilibili, BusinessStatus: BusinessDisabled, LoginStatus: LoginUnknown}}
	if _, err := service.BindProfile(actor, account.ID, "profile-1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("disabled BindProfile() error=%v, want ErrInvalidInput", err)
	}
}

func TestServiceStartsLocalAccountCheckByCreatingSensitiveAuthorization(t *testing.T) {
	store := newMemoryStore()
	facts := &fakeProfileFacts{profiles: map[string]fakeProfileFact{
		"profile-1": {id: "profile-1", userID: 1, bitProfileID: "bit-profile-1", active: true},
	}}
	tasks := &fakeSensitiveTasks{}
	service := newTestServiceForAccountCheck(store, facts, tasks)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})
	store.records[account.ID] = AccountRecord{Account: Account{
		ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, GameIDs: []string{"game-a"}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-1", BusinessStatus: BusinessEnabled, LoginStatus: LoginUnknown,
	}}

	start, err := service.StartLocalAccountCheck(actor, account.ID, AccountCheckStartInput{NodeID: "node-1"})
	if err != nil {
		t.Fatalf("StartLocalAccountCheck() error=%v", err)
	}
	if start.BitProfileID != "bit-profile-1" || start.Platform != PlatformBilibili {
		t.Fatalf("start=%#v", start)
	}
	if len(tasks.tasks) != 1 {
		t.Fatalf("created sensitive tasks=%d, want 1", len(tasks.tasks))
	}
	task := tasks.tasks[0]
	if task.Operation != profileguardservice.OperationAuthenticatedAccountCheck || task.NodeID != "node-1" || task.BitProfileID != "bit-profile-1" {
		t.Fatalf("task=%#v", task)
	}
}

func TestServiceAppliesLocalAccountCheckMismatchAsBusinessResult(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})
	store.records[first.ID] = AccountRecord{Account: Account{
		ID: first.ID, UserID: actor.ID, TeamID: actor.TeamID, GameIDs: []string{"game-a"}, Platform: PlatformBilibili,
		PlatformAccountID: "uid-1", IdentificationStatus: IdentificationIdentified, BusinessStatus: BusinessEnabled, LoginStatus: LoginNormal,
	}}
	store.records[second.ID] = AccountRecord{Account: Account{
		ID: second.ID, UserID: actor.ID, TeamID: actor.TeamID, GameIDs: []string{"game-a"}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-2", BusinessStatus: BusinessEnabled, LoginStatus: LoginUnknown,
	}}

	updated, err := service.ApplyLocalAccountCheckResult(actor, second.ID, AccountCheckResultInput{
		TaskID: "task-1", PlatformAccountID: "uid-1", LoginStatus: LoginNormal,
	})
	if err != nil {
		t.Fatalf("ApplyLocalAccountCheckResult() error=%v", err)
	}
	if updated.LoginStatus != LoginAccountMismatch || updated.IdentificationStatus != IdentificationDuplicate || updated.DuplicateOfAccountID != first.ID {
		t.Fatalf("updated=%#v", updated)
	}
}

func TestServiceAccountCheckReportsBoundProxyFacts(t *testing.T) {
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)
	cases := []struct {
		name           string
		proxyFact      fakeProfileProxyFact
		wantProxyOK    string
		wantExpiration string
	}{
		{
			name:           "checked active proxy passes both checks",
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", businessStatus: "active", lastCheckResult: "ok", expiresAt: &future, bound: true},
			wantProxyOK:    "pass",
			wantExpiration: "pass",
		},
		{
			name:           "missing proxy fails connectivity and skips expiry",
			proxyFact:      fakeProfileProxyFact{},
			wantProxyOK:    "fail",
			wantExpiration: "skip",
		},
		{
			name:           "unchecked proxy fails connectivity without claiming expiry",
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", businessStatus: "active", lastCheckResult: "", expiresAt: &future, bound: true},
			wantProxyOK:    "fail",
			wantExpiration: "pass",
		},
		{
			name:           "paused proxy fails both checks",
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", businessStatus: "paused", lastCheckResult: "ok", expiresAt: &future, bound: true},
			wantProxyOK:    "fail",
			wantExpiration: "fail",
		},
		{
			name:           "expired proxy fails both checks",
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", businessStatus: "active", lastCheckResult: "ok", expiresAt: &past, bound: true},
			wantProxyOK:    "fail",
			wantExpiration: "fail",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemoryStore()
			facts := &fakeProfileFacts{
				profiles: map[string]fakeProfileFact{
					"profile-1": {id: "profile-1", userID: actor.ID, bitProfileID: "bit-profile-1", active: true},
				},
				proxyFacts: map[string]fakeProfileProxyFact{"profile-1": tc.proxyFact},
			}
			service := newTestServiceForAccountCheck(store, facts, &fakeSensitiveTasks{})
			account, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})
			if err != nil {
				t.Fatal(err)
			}
			store.records[account.ID] = AccountRecord{Account: Account{
				ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, GameIDs: []string{"game-a"}, Platform: PlatformBilibili,
				BrowserProfileID: "profile-1", BusinessStatus: BusinessEnabled, LoginStatus: LoginUnknown,
			}}

			updated, err := service.ApplyLocalAccountCheckResult(actor, account.ID, AccountCheckResultInput{
				TaskID: "task-1", PlatformAccountID: "platform-account-1", LoginStatus: LoginNormal,
			})
			if err != nil {
				t.Fatalf("ApplyLocalAccountCheckResult() error=%v", err)
			}
			if got := accountCheckStatus(t, updated.CheckItems, "proxy_ok"); got != tc.wantProxyOK {
				t.Fatalf("proxy_ok=%q, want %q; checks=%#v", got, tc.wantProxyOK, updated.CheckItems)
			}
			if got := accountCheckStatus(t, updated.CheckItems, "proxy_expired"); got != tc.wantExpiration {
				t.Fatalf("proxy_expired=%q, want %q; checks=%#v", got, tc.wantExpiration, updated.CheckItems)
			}
		})
	}
}

func TestServiceCreateAccountStoresName(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		GameIDs: []string{"game-a"}, Platform: PlatformBilibili, Name: "  测试昵称  ",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.Name != "测试昵称" {
		t.Fatalf("account.Name = %q, want %q", account.Name, "测试昵称")
	}
	stored, _, _ := store.Find(account.ID)
	if stored.Name != "测试昵称" {
		t.Fatalf("stored.Name = %q, want %q", stored.Name, "测试昵称")
	}
}

func TestServiceUpdateAccountName(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili, Name: "原名"})

	updated, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{Name: strPtr("  新名  ")})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.Name != "新名" {
		t.Fatalf("updated.Name = %q, want %q", updated.Name, "新名")
	}

	cleared, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{Name: strPtr("")})
	if err != nil {
		t.Fatalf("UpdateAccount(clear) error = %v", err)
	}
	if cleared.Name != "" {
		t.Fatalf("cleared.Name = %q, want empty", cleared.Name)
	}
}

func TestNonAdminCannotUseGameOutsideAssignedScope(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	actor.GameIDs = []string{"game-a"}

	if _, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-b"}, Platform: PlatformBilibili}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateAccount(outside scope) error = %v, want ErrForbidden", err)
	}
	account, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})
	if err != nil {
		t.Fatalf("CreateAccount(in scope) error = %v", err)
	}
	if _, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{GameIDs: gameIDsPtr([]string{"game-b"})}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("UpdateAccount(outside scope) error = %v, want ErrForbidden", err)
	}
	if _, err := service.ListAccounts(actor, AccountFilter{GameIDs: []string{"game-b"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListAccounts(outside scope) error = %v, want ErrForbidden", err)
	}
}

func TestServiceApplyCheckResultBackfillsNameOnlyWhenEmpty(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili})

	// name 为空时回填
	updated, err := service.ApplyLocalAccountCheckResult(actor, account.ID, AccountCheckResultInput{
		TaskID: "task-1", PlatformAccountID: "uid-1", Name: "回填昵称", LoginStatus: LoginNormal,
	})
	if err != nil {
		t.Fatalf("ApplyLocalAccountCheckResult() error = %v", err)
	}
	if updated.Name != "回填昵称" {
		t.Fatalf("backfilled name = %q, want %q", updated.Name, "回填昵称")
	}

	// 已有 name 不被覆盖
	store.records[account.ID] = AccountRecord{Account: Account{
		ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, GameIDs: []string{"game-a"}, Platform: PlatformBilibili,
		PlatformAccountID: "uid-1", Name: "已有名", IdentificationStatus: IdentificationIdentified,
		BusinessStatus: BusinessEnabled, LoginStatus: LoginNormal,
	}}
	kept, err := service.ApplyLocalAccountCheckResult(actor, account.ID, AccountCheckResultInput{
		TaskID: "task-2", PlatformAccountID: "uid-1", Name: "新名", LoginStatus: LoginNormal,
	})
	if err != nil {
		t.Fatalf("ApplyLocalAccountCheckResult(keep) error = %v", err)
	}
	if kept.Name != "已有名" {
		t.Fatalf("existing name overwritten = %q", kept.Name)
	}
}

func TestServiceFiltersByStatusAndSearch(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili, Remark: "重点账号"})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBaijiahao, Remark: "普通账号"})
	firstRecord, _, _ := store.Find(first.ID)
	firstRecord.BusinessStatus = BusinessDisabled
	firstRecord.LoginStatus = LoginExpired
	store.records[first.ID] = firstRecord
	secondRecord, _, _ := store.Find(second.ID)
	secondRecord.LoginStatus = LoginNormal
	store.records[second.ID] = secondRecord

	assertAccountIDs(t, service, actor, AccountFilter{BusinessStatus: BusinessDisabled, LoginStatus: LoginExpired, Search: "重点"}, first.ID)
	assertAccountIDs(t, service, actor, AccountFilter{LoginStatus: LoginNormal, Search: "普通"}, second.ID)
}

func assertAccountIDs(t *testing.T, service *accountService, actor identityservice.PublicUser, filter AccountFilter, want ...string) {
	t.Helper()
	accounts, err := service.ListAccounts(actor, filter)
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	got := make([]string, 0, len(accounts))
	for _, account := range accounts {
		got = append(got, account.ID)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("account IDs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("account IDs = %v, want %v", got, want)
		}
	}
}

func TestServiceAccountGroupCRUD(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	group, err := service.CreateAccountGroup(actor, CreateAccountGroupInput{
		Name: "重点账号", Filters: AccountGroupFilters{Platform: PlatformBilibili, BusinessStatus: BusinessEnabled},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if group.ID == "" || group.Name != "重点账号" {
		t.Fatalf("group = %#v", group)
	}
	groups, err := service.ListAccountGroups(actor)
	if err != nil || len(groups) != 1 {
		t.Fatalf("list groups = %#v err=%v", groups, err)
	}

	other := mediaActor(2, 10, identityservice.RoleOperator)
	if _, err := service.UpdateAccountGroup(other, group.ID, UpdateAccountGroupInput{Name: strPtr("抢改")}); err == nil {
		t.Fatal("other user should not update group")
	}
	updated, err := service.UpdateAccountGroup(actor, group.ID, UpdateAccountGroupInput{Name: strPtr("改名")})
	if err != nil || updated.Name != "改名" {
		t.Fatalf("update = %#v err=%v", updated, err)
	}
	if err := service.DeleteAccountGroup(other, group.ID); err == nil {
		t.Fatal("other user should not delete group")
	}
	if err := service.DeleteAccountGroup(actor, group.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestServiceAccountGroupApplyFilters(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	if _, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBilibili}); err != nil {
		t.Fatalf("create bilibili: %v", err)
	}
	if _, err := service.CreateAccount(actor, CreateAccountInput{GameIDs: []string{"game-a"}, Platform: PlatformBaijiahao}); err != nil {
		t.Fatalf("create baijiahao: %v", err)
	}
	group, err := service.CreateAccountGroup(actor, CreateAccountGroupInput{Name: "仅B站", Filters: AccountGroupFilters{Platform: PlatformBilibili}})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	accounts, err := service.ListAccountsByGroup(actor, group.ID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Platform != PlatformBilibili {
		t.Fatalf("apply accounts = %#v", accounts)
	}
	if _, err := service.ListAccountsByGroup(mediaActor(2, 10, identityservice.RoleOperator), group.ID); err == nil {
		t.Fatal("other user should not apply group")
	}
}

func strPtr(value string) *string {
	return &value
}

func gameIDsPtr(value []string) *[]string {
	return &value
}

func newTestService(store Store) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store, withUserResolver(testUserResolver()), withGameResolver(defaultTestGameResolver()), withClock(func() time.Time { return now }), withIDGenerator(func(prefix string) string {
		next++
		return prefix + "-" + string(rune('0'+next))
	}))
}

func newTestServiceWithGames(store Store, games gameResolver) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store, withUserResolver(testUserResolver()), withGameResolver(games), withClock(func() time.Time { return now }), withIDGenerator(func(prefix string) string {
		next++
		return prefix + "-games-test-" + string(rune('0'+next))
	}))
}

func newTestServiceWithProfiles(store Store, resolver profileResolver) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store, withProfileResolver(resolver), withUserResolver(testUserResolver()), withGameResolver(defaultTestGameResolver()), withClock(func() time.Time { return now }), withIDGenerator(func(prefix string) string {
		next++
		return prefix + "-profile-test-" + string(rune('0'+next))
	}))
}

func newTestServiceForAccountCheck(store Store, facts profileFactResolver, tasks sensitiveTaskCreator) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store,
		withUserResolver(testUserResolver()),
		withGameResolver(defaultTestGameResolver()),
		withProfileFactResolver(facts),
		withSensitiveTaskCreator(tasks),
		withClock(func() time.Time { return now }),
		withIDGenerator(func(prefix string) string {
			next++
			return prefix + "-check-test-" + string(rune('0'+next))
		}),
	)
}

func containsSecretField(value string) bool {
	for _, needle := range []string{"secret-cookie", "original_cookie", "active_cookie"} {
		if len(value) >= len(needle) {
			for i := 0; i+len(needle) <= len(value); i++ {
				if value[i:i+len(needle)] == needle {
					return true
				}
			}
		}
	}
	return false
}

type memoryStore struct {
	records    map[string]AccountRecord
	gameIDs    map[string][]string
	tags       map[string]map[string]struct{}
	audits     []identityservice.AuditEvent
	groups     map[string]AccountGroup
	groupSeq   int64
	accountSeq int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{records: map[string]AccountRecord{}, gameIDs: map[string][]string{}, tags: map[string]map[string]struct{}{}, groups: map[string]AccountGroup{}}
}

func (s *memoryStore) Create(record AccountRecord) (string, error) {
	s.accountSeq++
	record.ID = strconv.FormatInt(s.accountSeq, 10)
	s.gameIDs[record.ID] = append([]string(nil), record.GameIDs...)
	record.GameIDs = nil
	s.records[record.ID] = record
	return record.ID, nil
}

func (s *memoryStore) Find(id string) (AccountRecord, bool, error) {
	record, ok := s.records[id]
	return s.withGameIDs(record), ok, nil
}

func (s *memoryStore) FindByIdentity(userID identityservice.UserID, platform Platform, platformAccountID string) (AccountRecord, bool, error) {
	for _, record := range s.records {
		if record.UserID == userID && record.Platform == platform && record.PlatformAccountID == platformAccountID && record.IdentificationStatus == IdentificationIdentified {
			return record, true, nil
		}
	}
	return AccountRecord{}, false, nil
}

func (s *memoryStore) FindByProfilePlatform(profileID string, platform Platform) (AccountRecord, bool, error) {
	for _, record := range s.records {
		if record.BrowserProfileID == profileID && record.Platform == platform {
			return record, true, nil
		}
	}
	return AccountRecord{}, false, nil
}

func (s *memoryStore) Update(record AccountRecord, replaceGameIDs *[]string) error {
	if replaceGameIDs != nil {
		s.gameIDs[record.ID] = append([]string(nil), (*replaceGameIDs)...)
	}
	record.GameIDs = nil
	s.records[record.ID] = record
	return nil
}

func (s *memoryStore) List(query AccountQuery) ([]AccountRecord, error) {
	var records []AccountRecord
	for _, record := range s.records {
		if query.UserID > 0 && record.UserID != query.UserID {
			continue
		}
		record = s.withGameIDs(record)
		if len(query.GameIDs) > 0 && !intersects(record.GameIDs, query.GameIDs) {
			continue
		}
		if query.Platform != "" && record.Platform != query.Platform {
			continue
		}
		if query.BusinessStatus != "" && record.BusinessStatus != query.BusinessStatus {
			continue
		}
		if query.LoginStatus != "" && record.LoginStatus != query.LoginStatus {
			continue
		}
		if query.Search != "" && !recordMatchesSearch(record, query.Search) {
			continue
		}
		if !matchesTags(s.tags[record.ID], query.AnyTags, query.AllTags, query.ExcludeTags) {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records, nil
}

func (s *memoryStore) withGameIDs(record AccountRecord) AccountRecord {
	record.GameIDs = append([]string(nil), s.gameIDs[record.ID]...)
	return record
}

func intersects(values, wanted []string) bool {
	for _, value := range values {
		if contains(wanted, value) {
			return true
		}
	}
	return false
}

func recordMatchesSearch(record AccountRecord, search string) bool {
	search = strings.ToLower(search)
	for _, value := range []string{record.ID, record.PlatformAccountID, record.Name, record.Remark} {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}

func (s *memoryStore) AddTags(userID identityservice.UserID, accountIDs, tags []string, createdAt time.Time) error {
	for _, accountID := range accountIDs {
		if s.tags[accountID] == nil {
			s.tags[accountID] = map[string]struct{}{}
		}
		for _, tag := range tags {
			s.tags[accountID][tag] = struct{}{}
		}
	}
	return nil
}

func (s *memoryStore) RemoveTags(userID identityservice.UserID, accountIDs, tags []string) error {
	for _, accountID := range accountIDs {
		for _, tag := range tags {
			delete(s.tags[accountID], tag)
		}
	}
	return nil
}

func (s *memoryStore) ListTags(accountIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(accountIDs))
	for _, accountID := range accountIDs {
		for tag := range s.tags[accountID] {
			result[accountID] = append(result[accountID], tag)
		}
		sort.Strings(result[accountID])
	}
	return result, nil
}

func (s *memoryStore) AppendAudit(event identityservice.AuditEvent) error {
	s.audits = append(s.audits, event)
	return nil
}

func (s *memoryStore) CreateGroup(group AccountGroup) (string, error) {
	s.groupSeq++
	group.ID = strconv.FormatInt(s.groupSeq, 10)
	s.groups[group.ID] = group
	return group.ID, nil
}

func (s *memoryStore) FindGroup(id string) (AccountGroup, bool, error) {
	group, found := s.groups[id]
	return group, found, nil
}

func (s *memoryStore) ListGroups(userID identityservice.UserID) ([]AccountGroup, error) {
	groups := []AccountGroup{}
	for _, group := range s.groups {
		if group.UserID == userID {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (s *memoryStore) UpdateGroup(group AccountGroup) error {
	s.groups[group.ID] = group
	return nil
}

func (s *memoryStore) DeleteGroup(id string) error {
	delete(s.groups, id)
	return nil
}

func matchesTags(accountTags map[string]struct{}, anyTags, allTags, excludeTags []string) bool {
	if len(anyTags) > 0 {
		matched := false
		for _, tag := range anyTags {
			if _, ok := accountTags[tag]; ok {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	for _, tag := range allTags {
		if _, ok := accountTags[tag]; !ok {
			return false
		}
	}
	for _, tag := range excludeTags {
		if _, ok := accountTags[tag]; ok {
			return false
		}
	}
	return true
}

type fakeProfileResolver struct {
	profiles map[string]identityservice.UserID
	inactive map[string]bool
}

func (r *fakeProfileResolver) ResolveProfile(profileID string) (userID identityservice.UserID, active bool, found bool, err error) {
	userID, found = r.profiles[profileID]
	return userID, !r.inactive[profileID], found, nil
}

type fakeProfileFact struct {
	id           string
	userID       identityservice.UserID
	bitProfileID string
	active       bool
}

type fakeProfileProxyFact struct {
	proxyID         string
	businessStatus  string
	lastCheckResult string
	expiresAt       *time.Time
	bound           bool
}

type fakeProfileFacts struct {
	profiles   map[string]fakeProfileFact
	proxyFacts map[string]fakeProfileProxyFact
}

func (r *fakeProfileFacts) ResolveProfileForAccountCheck(profileID string) (string, identityservice.UserID, string, bool, bool, error) {
	profile, found := r.profiles[profileID]
	return profile.id, profile.userID, profile.bitProfileID, profile.active, found, nil
}

func (r *fakeProfileFacts) ResolveProxyForAccountCheck(profileID string) (string, string, string, *time.Time, bool, error) {
	proxy, found := r.proxyFacts[profileID]
	if !found {
		return "", "", "", nil, false, nil
	}
	return proxy.proxyID, proxy.businessStatus, proxy.lastCheckResult, proxy.expiresAt, proxy.bound, nil
}

func accountCheckStatus(t *testing.T, items []AccountCheckItem, key string) string {
	t.Helper()
	for _, item := range items {
		if item.Key == key {
			return item.Status
		}
	}
	t.Fatalf("missing check item %q: %#v", key, items)
	return ""
}

type fakeSensitiveTasks struct {
	tasks []profileguardservice.SensitiveTask
}

func (s *fakeSensitiveTasks) CreateAuthorizedTask(task profileguardservice.SensitiveTask) error {
	s.tasks = append(s.tasks, task)
	return nil
}

type fakeUserResolver struct {
	users map[identityservice.UserID]identityservice.PublicUser
}

type fakeGameResolver struct {
	games map[string]identityservice.OperationGame
}

func defaultTestGameResolver() fakeGameResolver {
	return fakeGameResolver{games: map[string]identityservice.OperationGame{
		"game-a": {ID: "game-a", Status: identityservice.GameStatusEnabled},
		"game-b": {ID: "game-b", Status: identityservice.GameStatusEnabled},
		"game-z": {ID: "game-z", Status: identityservice.GameStatusEnabled},
	}}
}

func (r fakeGameResolver) ResolveGame(gameID string) (identityservice.OperationGame, bool, error) {
	game, found := r.games[gameID]
	return game, found, nil
}

func (r fakeUserResolver) ResolveUser(userID identityservice.UserID) (identityservice.PublicUser, bool, error) {
	user, found := r.users[userID]
	return user, found, nil
}

func testUserResolver() userResolver {
	return fakeUserResolver{users: map[identityservice.UserID]identityservice.PublicUser{
		1: mediaActor(1, 10, identityservice.RoleOperator),
		2: mediaActor(2, 20, identityservice.RoleOperator),
	}}
}

func mediaActor(userID identityservice.UserID, teamID identityservice.TeamID, role identityservice.Role) identityservice.PublicUser {
	return identityservice.PublicUser{ID: userID, Role: role, Status: identityservice.UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a", "game-b", "game-z"}}
}
