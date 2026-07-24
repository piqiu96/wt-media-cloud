package mediaaccount

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard"
)

func TestServiceCreatesPendingAccountWithinActorScope(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identity.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{
		GameID:         "game-a",
		Platform:       PlatformBilibili,
		OriginalCookie: "secret-cookie",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.UserID != actor.ID || account.GameID != "game-a" {
		t.Fatalf("account scope = %#v", account)
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
	actor := mediaActor(1, 10, identity.RoleOperator)

	account, err := service.CreateAccount(actor, CreateAccountInput{Platform: PlatformBilibili, Remark: "先建台账后绑定游戏"})
	if err != nil {
		t.Fatalf("CreateAccount(no game) error = %v", err)
	}
	if account.GameID != "" || account.BusinessStatus != BusinessEnabled || account.LoginStatus != LoginUnknown {
		t.Fatalf("account defaults = %#v", account)
	}
}

func TestServiceRejectsCrossUserAndOutOfGameScope(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := mediaActor(1, 10, identity.RoleSeniorOperator)

	_, err := service.CreateAccount(actor, CreateAccountInput{UserID: identity.UserID(2), GameID: "game-a", Platform: PlatformDouyin})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-user error = %v, want ErrForbidden", err)
	}
	_, err = service.CreateAccount(actor, CreateAccountInput{GameID: "game-b", Platform: PlatformDouyin})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("out-of-game error = %v, want ErrForbidden", err)
	}
}

func TestServiceAdminCanAssignGlobalAccount(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := identity.PublicUser{ID: identity.UserID(99), Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}

	account, err := service.CreateAccount(actor, CreateAccountInput{UserID: identity.UserID(2), GameID: "game-z", Platform: PlatformBaijiahao})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.UserID != identity.UserID(2) || account.GameID != "game-z" {
		t.Fatalf("account = %#v", account)
	}
}

func TestServiceSeniorCanManageButCannotCreateSameTeamGameAccountForAnotherUser(t *testing.T) {
	store := newMemoryStore()
	teamID := identity.TeamID(10)
	users := fakeUserResolver{users: map[identity.UserID]identity.PublicUser{
		2: {ID: 2, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a"}},
	}}
	service := NewService(store, WithUserResolver(users))
	actor := mediaActor(1, teamID, identity.RoleSeniorOperator)

	if _, err := service.CreateAccount(actor, CreateAccountInput{UserID: 2, GameID: "game-a", Platform: PlatformDouyin}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateAccount(same team/game) error=%v, want ErrForbidden", err)
	}
	store.records["account-2"] = AccountRecord{Account: Account{
		ID: "account-2", UserID: 2, TeamID: &teamID, GameID: "game-a", Platform: PlatformDouyin,
	}}
	if _, err := service.UpdateAccount(actor, "account-2", UpdateAccountInput{BusinessStatus: BusinessDisabled}); err != nil {
		t.Fatalf("UpdateAccount(same team/game) error=%v", err)
	}
}

func TestFullCookieRecordRejectsSeniorForAnotherUsersAccount(t *testing.T) {
	store := newMemoryStore()
	teamID := identity.TeamID(10)
	store.records["account-1"] = AccountRecord{
		Account:        Account{ID: "account-1", UserID: 2, TeamID: &teamID, GameID: "game-a"},
		OriginalCookie: "secret-cookie",
	}
	service := NewService(store)
	senior := mediaActor(1, teamID, identity.RoleSeniorOperator)
	admin := identity.PublicUser{ID: 99, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}

	if _, err := service.GetAccountRecord(senior, "account-1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetAccountRecord(senior) error=%v, want ErrForbidden", err)
	}
	if record, err := service.GetAccountRecord(admin, "account-1"); err != nil || record.OriginalCookie != "secret-cookie" {
		t.Fatalf("GetAccountRecord(admin) record=%+v error=%v", record, err)
	}
	owner := mediaActor(2, teamID, identity.RoleOperator)
	if _, err := service.GetOwnedAccountRecord(owner, "account-1"); err != nil {
		t.Fatalf("GetOwnedAccountRecord(owner) error=%v", err)
	}
	for name, actor := range map[string]identity.PublicUser{"senior": senior, "admin": admin} {
		if _, err := service.GetOwnedAccountRecord(actor, "account-1"); !errors.Is(err, ErrForbidden) {
			t.Fatalf("GetOwnedAccountRecord(%s) error=%v, want ErrForbidden", name, err)
		}
	}
}

func TestServiceIdentifiesAccountAndMarksUserScopedDuplicate(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identity.RoleOperator)

	first, err := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili, OriginalCookie: "first-secret"})
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

	candidate, err := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili, OriginalCookie: "candidate-secret"})
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
	admin := identity.PublicUser{ID: identity.UserID(99), Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}

	for _, userID := range []identity.UserID{1, 2} {
		account, err := service.CreateAccount(admin, CreateAccountInput{UserID: userID, GameID: "game-a", Platform: PlatformDouyin})
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
	actor := mediaActor(1, 10, identity.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformDouyin})

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
	actor := mediaActor(1, 10, identity.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformDouyin})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili})

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
	admin := identity.PublicUser{ID: identity.UserID(99), Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	account, _ := service.CreateAccount(admin, CreateAccountInput{UserID: identity.UserID(2), GameID: "game-a", Platform: PlatformDouyin})
	actor := mediaActor(1, 10, identity.RoleOperator)

	err := service.AddTags(actor, []string{account.ID}, []string{"forbidden"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AddTags() error = %v, want ErrForbidden", err)
	}
}

func TestServiceBindsOnlySameUserActiveProfileAndOnePlatform(t *testing.T) {
	store := newMemoryStore()
	resolver := &fakeProfileResolver{profiles: map[string]identity.UserID{"profile-1": 1, "profile-2": 2}, inactive: map[string]bool{}}
	service := newTestServiceWithProfiles(store, resolver)
	actor := mediaActor(1, 10, identity.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformDouyin})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformDouyin})

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
	resolver := &fakeProfileResolver{profiles: map[string]identity.UserID{"profile-1": 1}, inactive: map[string]bool{}}
	service := newTestServiceWithProfiles(store, resolver)
	actor := mediaActor(1, 10, identity.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili})
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
	store.records[account.ID] = AccountRecord{Account: Account{ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, GameID: "game-a", Platform: PlatformBilibili, BusinessStatus: BusinessDisabled, LoginStatus: LoginUnknown}}
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
	actor := mediaActor(1, 10, identity.RoleOperator)
	account, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili})
	store.records[account.ID] = AccountRecord{Account: Account{
		ID: account.ID, UserID: actor.ID, TeamID: actor.TeamID, GameID: "game-a", Platform: PlatformBilibili,
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
	if task.Operation != profileguard.OperationAuthenticatedAccountCheck || task.NodeID != "node-1" || task.BitProfileID != "bit-profile-1" {
		t.Fatalf("task=%#v", task)
	}
}

func TestServiceAppliesLocalAccountCheckMismatchAsBusinessResult(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identity.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili})
	store.records[first.ID] = AccountRecord{Account: Account{
		ID: first.ID, UserID: actor.ID, TeamID: actor.TeamID, GameID: "game-a", Platform: PlatformBilibili,
		PlatformAccountID: "uid-1", IdentificationStatus: IdentificationIdentified, BusinessStatus: BusinessEnabled, LoginStatus: LoginNormal,
	}}
	store.records[second.ID] = AccountRecord{Account: Account{
		ID: second.ID, UserID: actor.ID, TeamID: actor.TeamID, GameID: "game-a", Platform: PlatformBilibili,
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

func TestServiceFiltersByStatusAndSearch(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := mediaActor(1, 10, identity.RoleOperator)
	first, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBilibili, Remark: "重点账号"})
	second, _ := service.CreateAccount(actor, CreateAccountInput{GameID: "game-a", Platform: PlatformBaijiahao, Remark: "普通账号"})
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

func assertAccountIDs(t *testing.T, service *Service, actor identity.PublicUser, filter AccountFilter, want ...string) {
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

func newTestService(store Store) *Service {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return NewService(store, WithUserResolver(testUserResolver()), WithClock(func() time.Time { return now }), WithIDGenerator(func(prefix string) string {
		next++
		return prefix + "-" + string(rune('0'+next))
	}))
}

func newTestServiceWithProfiles(store Store, resolver ProfileResolver) *Service {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return NewService(store, WithProfileResolver(resolver), WithUserResolver(testUserResolver()), WithClock(func() time.Time { return now }), WithIDGenerator(func(prefix string) string {
		next++
		return prefix + "-profile-test-" + string(rune('0'+next))
	}))
}

func newTestServiceForAccountCheck(store Store, facts ProfileFactResolver, tasks SensitiveTaskCreator) *Service {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	next := 0
	return NewService(store,
		WithUserResolver(testUserResolver()),
		WithProfileFactResolver(facts),
		WithSensitiveTaskCreator(tasks),
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func(prefix string) string {
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
	records map[string]AccountRecord
	tags    map[string]map[string]struct{}
	audits  []identity.AuditEvent
}

func newMemoryStore() *memoryStore {
	return &memoryStore{records: map[string]AccountRecord{}, tags: map[string]map[string]struct{}{}}
}

func (s *memoryStore) Create(record AccountRecord) error {
	s.records[record.ID] = record
	return nil
}

func (s *memoryStore) Find(id string) (AccountRecord, bool, error) {
	record, ok := s.records[id]
	return record, ok, nil
}

func (s *memoryStore) FindByIdentity(userID identity.UserID, platform Platform, platformAccountID string) (AccountRecord, bool, error) {
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

func (s *memoryStore) Update(record AccountRecord) error {
	s.records[record.ID] = record
	return nil
}

func (s *memoryStore) List(query AccountQuery) ([]AccountRecord, error) {
	var records []AccountRecord
	for _, record := range s.records {
		if query.UserID > 0 && record.UserID != query.UserID {
			continue
		}
		if query.GameID != "" && record.GameID != query.GameID {
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

func recordMatchesSearch(record AccountRecord, search string) bool {
	search = strings.ToLower(search)
	for _, value := range []string{record.ID, record.PlatformAccountID, record.Name, record.Remark} {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}

func (s *memoryStore) AddTags(userID identity.UserID, accountIDs, tags []string, createdAt time.Time) error {
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

func (s *memoryStore) RemoveTags(userID identity.UserID, accountIDs, tags []string) error {
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

func (s *memoryStore) AppendAudit(event identity.AuditEvent) error {
	s.audits = append(s.audits, event)
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
	profiles map[string]identity.UserID
	inactive map[string]bool
}

func (r *fakeProfileResolver) ResolveProfile(profileID string) (userID identity.UserID, active bool, found bool, err error) {
	userID, found = r.profiles[profileID]
	return userID, !r.inactive[profileID], found, nil
}

type fakeProfileFact struct {
	id           string
	userID       identity.UserID
	bitProfileID string
	active       bool
}

type fakeProfileFacts struct {
	profiles map[string]fakeProfileFact
}

func (r *fakeProfileFacts) ResolveProfileForAccountCheck(profileID string) (string, identity.UserID, string, bool, bool, error) {
	profile, found := r.profiles[profileID]
	return profile.id, profile.userID, profile.bitProfileID, profile.active, found, nil
}

type fakeSensitiveTasks struct {
	tasks []profileguard.SensitiveTask
}

func (s *fakeSensitiveTasks) CreateAuthorizedTask(task profileguard.SensitiveTask) error {
	s.tasks = append(s.tasks, task)
	return nil
}

type fakeUserResolver struct {
	users map[identity.UserID]identity.PublicUser
}

func (r fakeUserResolver) ResolveUser(userID identity.UserID) (identity.PublicUser, bool, error) {
	user, found := r.users[userID]
	return user, found, nil
}

func testUserResolver() UserResolver {
	return fakeUserResolver{users: map[identity.UserID]identity.PublicUser{
		1: mediaActor(1, 10, identity.RoleOperator),
		2: mediaActor(2, 20, identity.RoleOperator),
	}}
}

func mediaActor(userID identity.UserID, teamID identity.TeamID, role identity.Role) identity.PublicUser {
	return identity.PublicUser{ID: userID, Role: role, Status: identity.UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a"}}
}
