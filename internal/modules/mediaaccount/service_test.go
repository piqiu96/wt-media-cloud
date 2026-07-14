package mediaaccount

import (
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func TestServiceCreatesPendingAccountWithinActorScope(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := identity.PublicUser{ID: "user-1", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, GameIDs: []string{"game-a"}}

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

func TestServiceRejectsCrossUserAndOutOfGameScope(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := identity.PublicUser{ID: "user-1", Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, GameIDs: []string{"game-a"}}

	_, err := service.CreateAccount(actor, CreateAccountInput{UserID: "user-2", GameID: "game-a", Platform: PlatformDouyin})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-user error = %v, want ErrForbidden", err)
	}
	_, err = service.CreateAccount(actor, CreateAccountInput{GameID: "game-b", Platform: PlatformDouyin})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("out-of-game error = %v, want ErrForbidden", err)
	}
}

func TestServiceTechnicianCanAssignGlobalAccount(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := identity.PublicUser{ID: "tech-1", Role: identity.RoleTechnician, Status: identity.UserStatusEnabled}

	account, err := service.CreateAccount(actor, CreateAccountInput{UserID: "user-2", GameID: "game-z", Platform: PlatformBaijiahao})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.UserID != "user-2" || account.GameID != "game-z" {
		t.Fatalf("account = %#v", account)
	}
}

func TestServiceIdentifiesAccountAndMarksUserScopedDuplicate(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := identity.PublicUser{ID: "user-1", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, GameIDs: []string{"game-a"}}

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
	tech := identity.PublicUser{ID: "tech-1", Role: identity.RoleTechnician, Status: identity.UserStatusEnabled}

	for _, userID := range []string{"user-1", "user-2"} {
		account, err := service.CreateAccount(tech, CreateAccountInput{UserID: userID, GameID: "game-a", Platform: PlatformDouyin})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.IdentifyAccount(tech, account.ID, IdentifyAccountInput{PlatformAccountID: "same-platform-id", LoginStatus: LoginNormal}); err != nil {
			t.Fatalf("user %s identify error = %v", userID, err)
		}
	}
}

func TestServiceValidatesBusinessAndLoginStatusesIndependently(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := identity.PublicUser{ID: "user-1", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, GameIDs: []string{"game-a"}}
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
	actor := identity.PublicUser{ID: "user-1", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, GameIDs: []string{"game-a"}}
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
	tech := identity.PublicUser{ID: "tech", Role: identity.RoleTechnician, Status: identity.UserStatusEnabled}
	account, _ := service.CreateAccount(tech, CreateAccountInput{UserID: "user-2", GameID: "game-a", Platform: PlatformDouyin})
	actor := identity.PublicUser{ID: "user-1", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, GameIDs: []string{"game-a"}}

	err := service.AddTags(actor, []string{account.ID}, []string{"forbidden"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AddTags() error = %v, want ErrForbidden", err)
	}
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
	return NewService(store, WithClock(func() time.Time { return now }), WithIDGenerator(func(prefix string) string {
		next++
		return prefix + "-" + string(rune('0'+next))
	}))
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

func (s *memoryStore) FindByIdentity(userID string, platform Platform, platformAccountID string) (AccountRecord, bool, error) {
	for _, record := range s.records {
		if record.UserID == userID && record.Platform == platform && record.PlatformAccountID == platformAccountID && record.IdentificationStatus == IdentificationIdentified {
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
		if query.UserID != "" && record.UserID != query.UserID {
			continue
		}
		if query.GameID != "" && record.GameID != query.GameID {
			continue
		}
		if query.Platform != "" && record.Platform != query.Platform {
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

func (s *memoryStore) AddTags(userID string, accountIDs, tags []string, createdAt time.Time) error {
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

func (s *memoryStore) RemoveTags(userID string, accountIDs, tags []string) error {
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
