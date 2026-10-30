package service

import (
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

func TestServiceCreatesPendingAccountWithinActorScope(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		CategoryIDs:    []identityservice.CategoryID{1},
		Platform:       PlatformBilibili,
		OriginalCookie: "secret-cookie",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.IdentificationStatus != IdentificationPending || !account.Enabled || account.PlatformLoginStatus != PlatformLoginUnknown {
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

func TestServiceAllowsLedgerAccountWithoutCategory(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	_, err := service.CreateAccount(actor, CreateAccountInput{Platform: PlatformBilibili, Remark: "先建台账后绑定类目"})
	if err != nil {
		t.Fatalf("CreateAccount(no category) error = %v", err)
	}
}

func TestServiceRejectsCrossUserScope(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := mediaActor(1, 10, identityservice.RoleSeniorOperator)

	_, err := service.CreateAccount(actor, CreateAccountInput{UserID: identityservice.UserID(2), CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-user error = %v, want ErrForbidden", err)
	}
}

func TestServiceAllowsEnabledPublicCategoriesAndNormalizesTheirSet(t *testing.T) {
	service := newTestServiceWithCategories(newMemoryStore(), fakeCategoryResolver{categories: map[identityservice.CategoryID]identityservice.Category{
		1: {ID: 1, Type: identityservice.CategoryTypeGame, Enabled: true},
		2: {ID: 2, Type: identityservice.CategoryTypeGame, Enabled: true},
	}})
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		CategoryIDs: []identityservice.CategoryID{2, 1, 2},
		Platform:    PlatformDouyin,
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if want := []identityservice.CategoryID{1, 2}; !slices.Equal(account.CategoryIDs, want) {
		t.Fatalf("CategoryIDs = %#v, want %#v", account.CategoryIDs, want)
	}
}

func TestServiceReplacesAndClearsEntireCategorySet(t *testing.T) {
	service := newTestServiceWithCategories(newMemoryStore(), fakeCategoryResolver{categories: map[identityservice.CategoryID]identityservice.Category{
		1: {ID: 1, Type: identityservice.CategoryTypeGame, Enabled: true},
		2: {ID: 2, Type: identityservice.CategoryTypeGame, Enabled: true},
		3: {ID: 3, Type: identityservice.CategoryTypeGame, Enabled: true},
	}})
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1, 2}, Platform: PlatformDouyin})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	updated, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{CategoryIDs: categoryIDsPtr([]identityservice.CategoryID{3, 2})})
	if err != nil {
		t.Fatalf("UpdateAccount(replace) error = %v", err)
	}

	updated, err = service.UpdateAccount(actor, account.ID, UpdateAccountInput{CategoryIDs: categoryIDsPtr([]identityservice.CategoryID{})})
	if err != nil {
		t.Fatalf("UpdateAccount(clear) error = %v", err)
	}
	if len(updated.CategoryIDs) != 0 {
		t.Fatalf("cleared CategoryIDs = %#v", updated.CategoryIDs)
	}
}

func TestServiceRejectsDisabledPublicCategory(t *testing.T) {
	service := newTestServiceWithCategories(newMemoryStore(), fakeCategoryResolver{categories: map[identityservice.CategoryID]identityservice.Category{
		4: {ID: 4, Type: identityservice.CategoryTypeGame, Enabled: false},
	}})

	_, err := service.CreateAccount(mediaActor(1, 10, identityservice.RoleOperator), CreateAccountInput{
		CategoryIDs: []identityservice.CategoryID{4},
		Platform:    PlatformBilibili,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateAccount(disabled category) error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceAdminCanAssignGlobalAccount(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := identityservice.PublicUser{ID: identityservice.UserID(99), Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}

	account, err := service.CreateAccount(actor, CreateAccountInput{UserID: identityservice.UserID(2), CategoryIDs: []identityservice.CategoryID{3}, Platform: PlatformBaidu})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.UserID != identityservice.UserID(2) {
		t.Fatalf("account user = %d, want 2", account.UserID)
	}
}

func TestServiceSeniorCanManageButCannotCreateSameTeamCategoryAccountForAnotherUser(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	users := fakeUserResolver{users: map[identityservice.UserID]identityservice.PublicUser{
		2: {ID: 2, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}},
	}}
	service := newAccountService(store, withUserResolver(users), withCategoryResolver(defaultTestCategoryResolver()))
	actor := mediaActor(1, teamID, identityservice.RoleSeniorOperator)

	if _, err := service.CreateAccount(actor, CreateAccountInput{UserID: 2, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateAccount(same team/category) error=%v, want ErrForbidden", err)
	}
	store.records["account-2"] = AccountRecord{Account: Account{
		ID: "account-2", UserID: 2, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin,
	}}
	if _, err := service.UpdateAccount(actor, "account-2", UpdateAccountInput{Enabled: boolPtr(false)}); err != nil {
		t.Fatalf("UpdateAccount(same team/category) error=%v", err)
	}
}

func TestFullCookieRecordRejectsSeniorForAnotherUsersAccount(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	store.records["account-1"] = AccountRecord{
		Account:        Account{ID: "account-1", UserID: 2, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}},
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

	first, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili, OriginalCookie: "first-secret"})
	if err != nil {
		t.Fatal(err)
	}
	identified, err := service.IdentifyAccount(actor, first.ID, IdentifyAccountInput{PlatformAccountID: "platform-42", PlatformName: "main", PlatformAvatarURL: "https://example.invalid/a.png", PlatformLoginStatus: PlatformLoginNormal})
	if err != nil {
		t.Fatalf("IdentifyAccount() error = %v", err)
	}
	if identified.IdentificationStatus != IdentificationIdentified || identified.PlatformAccountID != "platform-42" {
		t.Fatalf("identified = %#v", identified)
	}

	candidate, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili, OriginalCookie: "candidate-secret"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.IdentifyAccount(actor, candidate.ID, IdentifyAccountInput{PlatformAccountID: "platform-42", PlatformName: "duplicate", PlatformLoginStatus: PlatformLoginNormal})
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
		account, err := service.CreateAccount(admin, CreateAccountInput{UserID: userID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.IdentifyAccount(admin, account.ID, IdentifyAccountInput{PlatformAccountID: "same-platform-id", PlatformLoginStatus: PlatformLoginNormal}); err != nil {
			t.Fatalf("user %d identify error = %v", userID, err)
		}
	}
}

func TestServiceValidatesBusinessAndPlatformLoginStatusesIndependently(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})

	updated, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{Enabled: boolPtr(false), PlatformLoginStatus: PlatformLoginVerificationNeeded})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.Enabled || updated.PlatformLoginStatus != PlatformLoginVerificationNeeded {
		t.Fatalf("updated = %#v", updated)
	}
	_, err = service.UpdateAccount(actor, account.ID, UpdateAccountInput{PlatformLoginStatus: PlatformLoginStatus("bad")})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid status error = %v", err)
	}
}

func TestServiceBulkTagsAndFilters(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})
	second, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})

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
	account, _ := service.CreateAccount(admin, CreateAccountInput{UserID: identityservice.UserID(2), CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	err := service.AddTags(actor, []string{account.ID}, []string{"forbidden"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AddTags() error = %v, want ErrForbidden", err)
	}
}

func TestServiceBindsOnlySameUserActiveProfileAndOnePlatform(t *testing.T) {
	store := newMemoryStore()
	resolver := &fakeProfileResolver{profiles: map[string]identityservice.UserID{"profile-1": 1, "profile-2": 2}, inactive: map[string]bool{}, outOfScope: map[string]bool{}}
	service := newTestServiceWithProfiles(store, resolver)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})
	second, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformDouyin})

	bound, err := service.BindProfile(actor, first.ID, "profile-1")
	if err != nil || bound.BrowserProfileID != "profile-1" {
		t.Fatalf("BindProfile() account=%#v error=%v", bound, err)
	}
	storedFirst, _, _ := store.Find(first.ID)
	if storedFirst.PlatformLoginStatus != PlatformLoginUnknown || storedFirst.LastCheckAt != nil {
		t.Fatalf("binding must reset login check facts: %#v", storedFirst)
	}
	if _, err := service.BindProfile(actor, second.ID, "profile-1"); !errors.Is(err, ErrProfilePlatformTaken) {
		t.Fatalf("same platform bind error = %v", err)
	}
	if _, err := service.BindProfile(actor, second.ID, "profile-2"); !errors.Is(err, ErrProfileAssignedToOtherUser) {
		t.Fatalf("cross-user bind error = %v", err)
	}
	resolver.inactive["profile-3"] = true
	resolver.profiles["profile-3"] = 1
	if _, err := service.BindProfile(actor, second.ID, "profile-3"); !errors.Is(err, ErrProfileUnavailable) {
		t.Fatalf("inactive bind error = %v", err)
	}
	resolver.outOfScope["profile-4"] = true
	resolver.profiles["profile-4"] = 1
	if _, err := service.BindProfile(actor, second.ID, "profile-4"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("out-of-scope bind error = %v", err)
	}
	if _, err := service.BindProfile(actor, second.ID, "missing"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("missing bind error = %v", err)
	}
}

func TestServiceUnbindsProfileAndRejectsDisabledBinding(t *testing.T) {
	store := newMemoryStore()
	resolver := &fakeProfileResolver{profiles: map[string]identityservice.UserID{"profile-1": 1}, inactive: map[string]bool{}, outOfScope: map[string]bool{}}
	service := newTestServiceWithProfiles(store, resolver)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})
	bound, err := service.BindProfile(actor, account.ID, "profile-1")
	if err != nil {
		t.Fatalf("BindProfile() error=%v", err)
	}
	store.records[account.ID] = AccountRecord{Account: bound}
	unbound, err := service.UnbindProfile(actor, account.ID)
	if err != nil {
		t.Fatalf("UnbindProfile() error=%v", err)
	}
	if unbound.BrowserProfileID != "" || unbound.PlatformLoginStatus != PlatformLoginUnknown {
		t.Fatalf("unbound account = %#v", unbound)
	}
	store.records[account.ID] = AccountRecord{Account: Account{ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili, Enabled: false, PlatformLoginStatus: PlatformLoginUnknown}}
	if _, err := service.BindProfile(actor, account.ID, "profile-1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("disabled BindProfile() error=%v, want ErrInvalidInput", err)
	}
}

func TestServiceStartsLocalAccountCheckWithoutPersistentAuthorization(t *testing.T) {
	store := newMemoryStore()
	facts := &fakeProfileFacts{profiles: map[string]fakeProfileFact{
		"profile-1": {id: "profile-1", userID: 1, bitProfileID: "bit-profile-1", active: true},
	}}
	service := newTestServiceForAccountCheck(store, facts)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})
	store.records[account.ID] = AccountRecord{Account: Account{
		ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-1", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown,
	}}

	start, err := service.StartLocalAccountCheck(actor, account.ID, AccountCheckStartInput{})
	if err != nil {
		t.Fatalf("StartLocalAccountCheck() error=%v", err)
	}
	if start.BitProfileID != "bit-profile-1" || start.Platform != PlatformBilibili {
		t.Fatalf("start=%#v", start)
	}
	if start.OperationID == "" {
		t.Fatal("operation id is empty")
	}
}

func TestCheckAllowsUnclassifiedOwnedAccount(t *testing.T) {
	store := newMemoryStore()
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	store.records["unclassified"] = AccountRecord{Account: Account{ID: "unclassified", UserID: actor.ID, TeamID: actor.TeamID, Platform: PlatformBilibili, BrowserProfileID: "profile-1", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown}}
	facts := &fakeProfileFacts{profiles: map[string]fakeProfileFact{"profile-1": {id: "profile-1", userID: actor.ID, bitProfileID: "bit-profile-1", active: true}}}
	service := newTestServiceForAccountCheck(store, facts)
	if _, err := service.StartLocalAccountCheck(actor, "unclassified", AccountCheckStartInput{}); err != nil {
		t.Fatalf("unclassified account check: %v", err)
	}
}

// 2026-10-27：本地敏感操作按账号归属，而不只看角色。高级运营可以操作自己创建的
// 账号；同团队他人的账号看得见但不能本地执行；管理员即使拥有账号也不执行本地操作。
func TestLocalOperationsFollowAccountOwnership(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	senior := mediaActor(1, teamID, identityservice.RoleSeniorOperator)
	admin := identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}
	store.records["own"] = AccountRecord{Account: Account{
		ID: "own", UserID: senior.ID, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-senior", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown,
	}}
	store.records["teammate"] = AccountRecord{Account: Account{
		ID: "teammate", UserID: 2, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-teammate", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown,
	}}
	store.records["admin-own"] = AccountRecord{Account: Account{
		ID: "admin-own", UserID: admin.ID, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-admin", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown,
	}}
	facts := &fakeProfileFacts{profiles: map[string]fakeProfileFact{
		"profile-senior":   {id: "profile-senior", userID: senior.ID, bitProfileID: "bit-senior", active: true},
		"profile-teammate": {id: "profile-teammate", userID: 2, bitProfileID: "bit-teammate", active: true},
		"profile-admin":    {id: "profile-admin", userID: admin.ID, bitProfileID: "bit-admin", active: true},
	}}
	service := newTestServiceForAccountCheck(store, facts)

	if _, err := service.StartLocalAccountCheck(senior, "own", AccountCheckStartInput{}); err != nil {
		t.Fatalf("senior own StartLocalAccountCheck error=%v", err)
	}
	if _, err := service.StartCookieRead(senior, "own", CookieReadStartInput{}); err != nil {
		t.Fatalf("senior own StartCookieRead error=%v", err)
	}
	if _, err := service.StartLocalAccountCheck(senior, "teammate", AccountCheckStartInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("senior teammate StartLocalAccountCheck error=%v, want ErrForbidden", err)
	}
	if _, err := service.StartCookieRead(senior, "teammate", CookieReadStartInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("senior teammate StartCookieRead error=%v, want ErrForbidden", err)
	}
	for _, accountID := range []string{"own", "admin-own"} {
		if _, err := service.StartLocalAccountCheck(admin, accountID, AccountCheckStartInput{}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("admin %s StartLocalAccountCheck error=%v, want ErrForbidden", accountID, err)
		}
		if _, err := service.StartCookieRead(admin, accountID, CookieReadStartInput{}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("admin %s StartCookieRead error=%v, want ErrForbidden", accountID, err)
		}
	}
}

// ReadMediaAccountCookies (the async cloud-agent path) applies this exported
// gate in the handler; it must stay the same rule as the two synchronous starts.
func TestCanStartLocalOperationExcludesAdmins(t *testing.T) {
	for _, role := range []identityservice.Role{identityservice.RoleOperator, identityservice.RoleSeniorOperator} {
		if !CanStartLocalOperation(role) {
			t.Fatalf("CanStartLocalOperation(%s) = false, want true", role)
		}
	}
	if CanStartLocalOperation(identityservice.RoleAdmin) {
		t.Fatal("CanStartLocalOperation(admin) = true, want false")
	}
}

func TestServiceAppliesLocalAccountCheckMismatchAsBusinessResult(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})
	second, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})
	store.records[first.ID] = AccountRecord{Account: Account{
		ID: first.ID, UserID: actor.ID, TeamID: actor.TeamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		PlatformAccountID: "uid-1", IdentificationStatus: IdentificationIdentified, Enabled: true, PlatformLoginStatus: PlatformLoginNormal,
	}}
	store.records[second.ID] = AccountRecord{Account: Account{
		ID: second.ID, UserID: actor.ID, TeamID: actor.TeamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		BrowserProfileID: "profile-2", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown,
	}}

	updated, err := service.ApplyLocalAccountCheckResult(actor, second.ID, AccountCheckResultInput{
		OperationID: "task-1", PlatformAccountID: "uid-1", PlatformLoginStatus: PlatformLoginNormal,
	})
	if err != nil {
		t.Fatalf("ApplyLocalAccountCheckResult() error=%v", err)
	}
	if updated.PlatformLoginStatus != PlatformLoginAccountMismatch || updated.IdentificationStatus != IdentificationDuplicate || updated.DuplicateOfAccountID != first.ID {
		t.Fatalf("updated=%#v", updated)
	}
}

func TestCheckPreservesIdentityOnMismatchAndRefreshesSameUID(t *testing.T) {
	for _, tc := range []struct {
		name, observed, wantUID, wantName string
		wantStatus                        PlatformLoginStatus
	}{
		{"same UID", "uid-1", "uid-1", "new name", PlatformLoginNormal},
		{"different UID", "uid-2", "uid-1", "old name", PlatformLoginAccountMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemoryStore()
			actor := mediaActor(1, 10, identityservice.RoleOperator)
			store.records["1"] = AccountRecord{Account: Account{ID: "1", UserID: actor.ID, TeamID: actor.TeamID, Platform: PlatformBilibili, PlatformAccountID: "uid-1", PlatformName: "old name", Enabled: true, PlatformLoginStatus: PlatformLoginNormal}}
			updated, err := newTestService(store).ApplyLocalAccountCheckResult(actor, "1", AccountCheckResultInput{OperationID: "check-1", PlatformAccountID: tc.observed, PlatformName: "new name", PlatformLoginStatus: PlatformLoginNormal})
			if err != nil {
				t.Fatal(err)
			}
			if updated.PlatformAccountID != tc.wantUID || updated.PlatformName != tc.wantName || updated.PlatformLoginStatus != tc.wantStatus {
				t.Fatalf("unexpected result: %#v", updated)
			}
		})
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
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", enabled: true, lastCheckResult: "ok", expiresAt: &future, bound: true},
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
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", enabled: true, lastCheckResult: "", expiresAt: &future, bound: true},
			wantProxyOK:    "fail",
			wantExpiration: "pass",
		},
		{
			name:           "paused proxy fails both checks",
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", enabled: false, lastCheckResult: "ok", expiresAt: &future, bound: true},
			wantProxyOK:    "fail",
			wantExpiration: "fail",
		},
		{
			name:           "expired proxy fails both checks",
			proxyFact:      fakeProfileProxyFact{proxyID: "proxy-1", enabled: true, lastCheckResult: "ok", expiresAt: &past, bound: true},
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
			service := newTestServiceForAccountCheck(store, facts)
			account, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})
			if err != nil {
				t.Fatal(err)
			}
			store.records[account.ID] = AccountRecord{Account: Account{
				ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
				BrowserProfileID: "profile-1", Enabled: true, PlatformLoginStatus: PlatformLoginUnknown,
			}}

			updated, err := service.ApplyLocalAccountCheckResult(actor, account.ID, AccountCheckResultInput{
				OperationID: "task-1", PlatformAccountID: "platform-account-1", PlatformLoginStatus: PlatformLoginNormal,
			})
			if err != nil {
				t.Fatalf("ApplyLocalAccountCheckResult() error=%v", err)
			}
			if got := accountCheckStatus(t, updated.LastCheckItems, "proxy_ok"); got != tc.wantProxyOK {
				t.Fatalf("proxy_ok=%q, want %q; checks=%#v", got, tc.wantProxyOK, updated.LastCheckItems)
			}
			if got := accountCheckStatus(t, updated.LastCheckItems, "proxy_expired"); got != tc.wantExpiration {
				t.Fatalf("proxy_expired=%q, want %q; checks=%#v", got, tc.wantExpiration, updated.LastCheckItems)
			}
		})
	}
}

func TestServiceCreateAccountStoresName(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili, PlatformName: "  测试昵称  ",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.PlatformName != "测试昵称" {
		t.Fatalf("account.PlatformName = %q, want %q", account.PlatformName, "测试昵称")
	}
	stored, _, _ := store.Find(account.ID)
	if stored.PlatformName != "测试昵称" {
		t.Fatalf("stored.PlatformName = %q, want %q", stored.PlatformName, "测试昵称")
	}
}

func TestServiceUpdateAccountName(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili, PlatformName: "原名"})

	updated, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{PlatformName: strPtr("  新名  ")})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.PlatformName != "新名" {
		t.Fatalf("updated.PlatformName = %q, want %q", updated.PlatformName, "新名")
	}

	cleared, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{PlatformName: strPtr("")})
	if err != nil {
		t.Fatalf("UpdateAccount(clear) error = %v", err)
	}
	if cleared.PlatformName != "" {
		t.Fatalf("cleared.PlatformName = %q, want empty", cleared.PlatformName)
	}
}

func TestNonAdminCannotUseCategoryOutsideAssignedScope(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	actor.CategoryIDs = []identityservice.CategoryID{1}

	if _, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{2}, Platform: PlatformBilibili}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateAccount(outside scope) error = %v, want ErrForbidden", err)
	}
	account, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})
	if err != nil {
		t.Fatalf("CreateAccount(in scope) error = %v", err)
	}
	if _, err := service.UpdateAccount(actor, account.ID, UpdateAccountInput{CategoryIDs: categoryIDsPtr([]identityservice.CategoryID{2})}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("UpdateAccount(outside scope) error = %v, want ErrForbidden", err)
	}
	if _, err := service.ListAccounts(actor, AccountFilter{CategoryIDs: []identityservice.CategoryID{2}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListAccounts(outside scope) error = %v, want ErrForbidden", err)
	}
}

func TestServiceApplyCheckResultUpdatesObservedName(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili})

	// name 为空时回填
	updated, err := service.ApplyLocalAccountCheckResult(actor, account.ID, AccountCheckResultInput{
		OperationID: "task-1", PlatformAccountID: "uid-1", PlatformName: "回填昵称", PlatformLoginStatus: PlatformLoginNormal,
	})
	if err != nil {
		t.Fatalf("ApplyLocalAccountCheckResult() error = %v", err)
	}
	if updated.PlatformName != "回填昵称" {
		t.Fatalf("backfilled name = %q, want %q", updated.PlatformName, "回填昵称")
	}

	// 同一 UID 的昵称随平台信息更新
	store.records[account.ID] = AccountRecord{Account: Account{
		ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili,
		PlatformAccountID: "uid-1", PlatformName: "已有名", IdentificationStatus: IdentificationIdentified,
		Enabled: true, PlatformLoginStatus: PlatformLoginNormal,
	}}
	kept, err := service.ApplyLocalAccountCheckResult(actor, account.ID, AccountCheckResultInput{
		OperationID: "task-2", PlatformAccountID: "uid-1", PlatformName: "新名", PlatformLoginStatus: PlatformLoginNormal,
	})
	if err != nil {
		t.Fatalf("ApplyLocalAccountCheckResult(keep) error = %v", err)
	}
	if kept.PlatformName != "新名" {
		t.Fatalf("updated name = %q", kept.PlatformName)
	}
}

func TestServiceFiltersByStatusAndSearch(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili, Remark: "重点账号"})
	second, _ := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBaidu, Remark: "普通账号"})
	firstRecord, _, _ := store.Find(first.ID)
	firstRecord.Enabled = false
	firstRecord.PlatformLoginStatus = PlatformLoginExpired
	store.records[first.ID] = firstRecord
	secondRecord, _, _ := store.Find(second.ID)
	secondRecord.PlatformLoginStatus = PlatformLoginNormal
	store.records[second.ID] = secondRecord

	assertAccountIDs(t, service, actor, AccountFilter{Enabled: boolPtr(false), PlatformLoginStatus: PlatformLoginExpired, Search: "重点"}, first.ID)
	assertAccountIDs(t, service, actor, AccountFilter{PlatformLoginStatus: PlatformLoginNormal, Search: "普通"}, second.ID)
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
		Name: "重点账号", Filters: AccountGroupFilters{Platform: PlatformBilibili, Enabled: boolPtr(true)},
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

	if _, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBilibili}); err != nil {
		t.Fatalf("create bilibili: %v", err)
	}
	if _, err := service.CreateAccount(actor, CreateAccountInput{CategoryIDs: []identityservice.CategoryID{1}, Platform: PlatformBaidu}); err != nil {
		t.Fatalf("create baidu: %v", err)
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

func categoryIDsPtr(value []identityservice.CategoryID) *[]identityservice.CategoryID {
	return &value
}

func newTestService(store Store) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store, withUserResolver(testUserResolver()), withCategoryResolver(defaultTestCategoryResolver()), withClock(func() time.Time { return now }), withIDGenerator(func(prefix string) string {
		next++
		return prefix + "-" + string(rune('0'+next))
	}))
}

func newTestServiceWithCategories(store Store, categories categoryResolver) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store, withUserResolver(testUserResolver()), withCategoryResolver(categories), withClock(func() time.Time { return now }), withIDGenerator(func(prefix string) string {
		next++
		return prefix + "-categories-test-" + string(rune('0'+next))
	}))
}

func newTestServiceWithProfiles(store Store, resolver profileResolver) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store, withProfileResolver(resolver), withUserResolver(testUserResolver()), withCategoryResolver(defaultTestCategoryResolver()), withClock(func() time.Time { return now }), withIDGenerator(func(prefix string) string {
		next++
		return prefix + "-profile-test-" + string(rune('0'+next))
	}))
}

func newTestServiceForAccountCheck(store Store, facts profileFactResolver) *accountService {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return newAccountService(store,
		withUserResolver(testUserResolver()),
		withCategoryResolver(defaultTestCategoryResolver()),
		withProfileFactResolver(facts),
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
	records     map[string]AccountRecord
	categoryIDs map[string][]identityservice.CategoryID
	tags        map[string]map[string]struct{}
	audits      []identityservice.AuditEvent
	groups      map[string]AccountGroup
	groupSeq    int64
	accountSeq  int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{records: map[string]AccountRecord{}, categoryIDs: map[string][]identityservice.CategoryID{}, tags: map[string]map[string]struct{}{}, groups: map[string]AccountGroup{}}
}

func (s *memoryStore) Create(record AccountRecord) (string, error) {
	s.accountSeq++
	record.ID = strconv.FormatInt(s.accountSeq, 10)
	s.categoryIDs[record.ID] = append([]identityservice.CategoryID(nil), record.CategoryIDs...)
	record.CategoryIDs = nil
	s.records[record.ID] = record
	return record.ID, nil
}

func (s *memoryStore) Find(id string) (AccountRecord, bool, error) {
	record, ok := s.records[id]
	return s.withCategoryIDs(record), ok, nil
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

func (s *memoryStore) Update(record AccountRecord, replaceCategoryIDs *[]identityservice.CategoryID) error {
	if replaceCategoryIDs != nil {
		s.categoryIDs[record.ID] = append([]identityservice.CategoryID(nil), (*replaceCategoryIDs)...)
	}
	record.CategoryIDs = nil
	s.records[record.ID] = record
	return nil
}

func (s *memoryStore) List(query AccountQuery) ([]AccountRecord, error) {
	var records []AccountRecord
	for _, record := range s.records {
		if query.UserID > 0 && record.UserID != query.UserID {
			continue
		}
		record = s.withCategoryIDs(record)
		if len(query.CategoryIDs) > 0 && !intersects(record.CategoryIDs, query.CategoryIDs) {
			continue
		}
		if query.Platform != "" && record.Platform != query.Platform {
			continue
		}
		if query.Enabled != nil && record.Enabled != *query.Enabled {
			continue
		}
		if query.PlatformLoginStatus != "" && record.PlatformLoginStatus != query.PlatformLoginStatus {
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

func (s *memoryStore) withCategoryIDs(record AccountRecord) AccountRecord {
	record.CategoryIDs = append([]identityservice.CategoryID(nil), s.categoryIDs[record.ID]...)
	return record
}

func intersects(values, wanted []identityservice.CategoryID) bool {
	for _, value := range values {
		if contains(wanted, value) {
			return true
		}
	}
	return false
}

func recordMatchesSearch(record AccountRecord, search string) bool {
	search = strings.ToLower(search)
	for _, value := range []string{record.ID, record.PlatformAccountID, record.PlatformName, record.Remark} {
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
	profiles   map[string]identityservice.UserID
	inactive   map[string]bool
	outOfScope map[string]bool
}

func (r *fakeProfileResolver) ResolveProfile(profileID string) (userID identityservice.UserID, active bool, inScope bool, found bool, err error) {
	userID, found = r.profiles[profileID]
	return userID, !r.inactive[profileID], !r.outOfScope[profileID], found, nil
}

type fakeProfileFact struct {
	id           string
	userID       identityservice.UserID
	bitProfileID string
	active       bool
	outOfScope   bool
}

type fakeProfileProxyFact struct {
	proxyID         string
	enabled         bool
	lastCheckResult string
	expiresAt       *time.Time
	bound           bool
}

type fakeProfileFacts struct {
	profiles   map[string]fakeProfileFact
	proxyFacts map[string]fakeProfileProxyFact
}

func (r *fakeProfileFacts) ResolveProfileForAccountCheck(profileID string) (string, identityservice.UserID, string, bool, bool, bool, error) {
	profile, found := r.profiles[profileID]
	return profile.id, profile.userID, profile.bitProfileID, !profile.outOfScope, profile.active, found, nil
}

func (r *fakeProfileFacts) ResolveProxyForAccountCheck(profileID string) (string, bool, string, *time.Time, bool, error) {
	proxy, found := r.proxyFacts[profileID]
	if !found {
		return "", false, "", nil, false, nil
	}
	return proxy.proxyID, proxy.enabled, proxy.lastCheckResult, proxy.expiresAt, proxy.bound, nil
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

type fakeUserResolver struct {
	users map[identityservice.UserID]identityservice.PublicUser
}

type fakeCategoryResolver struct {
	categories map[identityservice.CategoryID]identityservice.Category
}

func defaultTestCategoryResolver() fakeCategoryResolver {
	return fakeCategoryResolver{categories: map[identityservice.CategoryID]identityservice.Category{
		1: {ID: 1, Type: identityservice.CategoryTypeGame, Enabled: true},
		2: {ID: 2, Type: identityservice.CategoryTypeGame, Enabled: true},
		3: {ID: 3, Type: identityservice.CategoryTypeGame, Enabled: true},
	}}
}

func (r fakeCategoryResolver) ResolveCategory(categoryID identityservice.CategoryID) (identityservice.Category, bool, error) {
	category, found := r.categories[categoryID]
	return category, found, nil
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
	return identityservice.PublicUser{ID: userID, Role: role, Status: identityservice.UserStatusEnabled, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1, 2, 3}}
}

func boolPtr(value bool) *bool { return &value }
