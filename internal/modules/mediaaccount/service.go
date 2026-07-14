// Package mediaaccount owns Cloud media-account facts, assignment, and tags.
package mediaaccount

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type Platform string

const (
	PlatformDouyin    Platform = "douyin"
	PlatformBilibili  Platform = "bilibili"
	PlatformBaijiahao Platform = "baijiahao"
)

type IdentificationStatus string

const (
	IdentificationPending    IdentificationStatus = "pending_identification"
	IdentificationIdentified IdentificationStatus = "identified"
	IdentificationDuplicate  IdentificationStatus = "duplicate"
)

type BusinessStatus string

const (
	BusinessEnabled  BusinessStatus = "enabled"
	BusinessDisabled BusinessStatus = "disabled"
	BusinessRetired  BusinessStatus = "retired"
)

type LoginStatus string

const (
	LoginUnknown            LoginStatus = "unknown"
	LoginNormal             LoginStatus = "normal"
	LoginNotLoggedIn        LoginStatus = "not_logged_in"
	LoginVerificationNeeded LoginStatus = "verification_needed"
	LoginExpired            LoginStatus = "expired"
	LoginRestricted         LoginStatus = "restricted"
	LoginAccountMismatch    LoginStatus = "account_mismatch"
	LoginEnvironmentError   LoginStatus = "environment_error"
)

var (
	ErrForbidden        = errors.New("media account operation is forbidden")
	ErrInvalidInput     = errors.New("media account input is invalid")
	ErrNotFound         = errors.New("media account was not found")
	ErrDuplicateAccount = errors.New("media account already exists for this user and platform")
)

// Account is the API-safe representation. Cookie values are intentionally
// absent and must remain internal to AccountRecord.
type Account struct {
	ID                    string               `json:"id"`
	UserID                string               `json:"user_id"`
	GameID                string               `json:"game_id"`
	Platform              Platform             `json:"platform"`
	PlatformAccountID     string               `json:"platform_account_id,omitempty"`
	Name                  string               `json:"name,omitempty"`
	AvatarURL             string               `json:"avatar_url,omitempty"`
	BrowserProfileID      string               `json:"browser_profile_id,omitempty"`
	IdentificationStatus  IdentificationStatus `json:"identification_status"`
	DuplicateOfAccountID  string               `json:"duplicate_of_account_id,omitempty"`
	BusinessStatus        BusinessStatus       `json:"business_status"`
	LoginStatus           LoginStatus          `json:"login_status"`
	CookieStatus          string               `json:"cookie_status,omitempty"`
	ActiveCookieUpdatedAt *time.Time           `json:"active_cookie_updated_at,omitempty"`
	LastCheckedAt         *time.Time           `json:"last_checked_at,omitempty"`
	Tags                  []string             `json:"tags"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
}

type AccountRecord struct {
	Account
	OriginalCookie string
	ActiveCookie   string
}

type CreateAccountInput struct {
	UserID         string
	GameID         string
	Platform       Platform
	OriginalCookie string
}

type IdentifyAccountInput struct {
	PlatformAccountID string
	Name              string
	AvatarURL         string
	LoginStatus       LoginStatus
}

type UpdateAccountInput struct {
	BusinessStatus BusinessStatus
	LoginStatus    LoginStatus
}

type AccountFilter struct {
	UserID      string
	GameID      string
	Platform    Platform
	AnyTags     []string
	AllTags     []string
	ExcludeTags []string
}

type AccountQuery = AccountFilter

type Store interface {
	Create(AccountRecord) error
	Find(id string) (AccountRecord, bool, error)
	FindByIdentity(userID string, platform Platform, platformAccountID string) (AccountRecord, bool, error)
	Update(AccountRecord) error
	List(AccountQuery) ([]AccountRecord, error)
	AddTags(userID string, accountIDs, tags []string, createdAt time.Time) error
	RemoveTags(userID string, accountIDs, tags []string) error
	ListTags(accountIDs []string) (map[string][]string, error)
	AppendAudit(identity.AuditEvent) error
}

type Service struct {
	store Store
	now   func() time.Time
	newID func(string) string
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(service *Service) { service.now = now }
}

func WithIDGenerator(newID func(string) string) Option {
	return func(service *Service) { service.newID = newID }
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{store: store, now: func() time.Time { return time.Now().UTC() }, newID: common.NewID}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) CreateAccount(actor identity.PublicUser, input CreateAccountInput) (Account, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		userID = actor.ID
	}
	gameID := strings.TrimSpace(input.GameID)
	platform := Platform(strings.ToLower(strings.TrimSpace(string(input.Platform))))
	if !validActor(actor) || gameID == "" || len(gameID) > 128 || !validPlatform(platform) {
		return Account{}, ErrInvalidInput
	}
	if !canAccess(actor, userID, gameID) {
		return Account{}, ErrForbidden
	}
	now := s.now()
	record := AccountRecord{
		Account: Account{
			ID:                   s.newID("media_account"),
			UserID:               userID,
			GameID:               gameID,
			Platform:             platform,
			IdentificationStatus: IdentificationPending,
			BusinessStatus:       BusinessEnabled,
			LoginStatus:          LoginUnknown,
			Tags:                 []string{},
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		OriginalCookie: input.OriginalCookie,
	}
	if err := s.store.Create(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.create", record.ID, map[string]string{
		"user_id": userID, "game_id": gameID, "platform": string(platform),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *Service) GetAccount(actor identity.PublicUser, accountID string) (Account, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	accounts, err := s.attachTags([]AccountRecord{record})
	if err != nil {
		return Account{}, err
	}
	return accounts[0], nil
}

func (s *Service) ListAccounts(actor identity.PublicUser, filter AccountFilter) ([]Account, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.GameID = strings.TrimSpace(filter.GameID)
	if actor.Role != identity.RoleTechnician {
		if filter.UserID != "" && filter.UserID != actor.ID {
			return nil, ErrForbidden
		}
		filter.UserID = actor.ID
		if filter.GameID != "" && !contains(actor.GameIDs, filter.GameID) {
			return nil, ErrForbidden
		}
	}
	if filter.Platform != "" {
		filter.Platform = Platform(strings.ToLower(strings.TrimSpace(string(filter.Platform))))
		if !validPlatform(filter.Platform) {
			return nil, ErrInvalidInput
		}
	}
	var err error
	if filter.AnyTags, err = normalizeTags(filter.AnyTags); err != nil {
		return nil, err
	}
	if filter.AllTags, err = normalizeTags(filter.AllTags); err != nil {
		return nil, err
	}
	if filter.ExcludeTags, err = normalizeTags(filter.ExcludeTags); err != nil {
		return nil, err
	}
	records, err := s.store.List(AccountQuery(filter))
	if err != nil {
		return nil, err
	}
	visible := records[:0]
	for _, record := range records {
		if canAccess(actor, record.UserID, record.GameID) {
			visible = append(visible, record)
		}
	}
	return s.attachTags(visible)
}

func (s *Service) UpdateAccount(actor identity.PublicUser, accountID string, input UpdateAccountInput) (Account, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if input.BusinessStatus != "" {
		if !validBusinessStatus(input.BusinessStatus) {
			return Account{}, ErrInvalidInput
		}
		record.BusinessStatus = input.BusinessStatus
	}
	if input.LoginStatus != "" {
		if !validLoginStatus(input.LoginStatus) {
			return Account{}, ErrInvalidInput
		}
		record.LoginStatus = input.LoginStatus
	}
	if input.BusinessStatus == "" && input.LoginStatus == "" {
		return Account{}, ErrInvalidInput
	}
	record.UpdatedAt = s.now()
	if err := s.store.Update(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.status.update", record.ID, map[string]string{
		"business_status": string(record.BusinessStatus), "login_status": string(record.LoginStatus),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *Service) IdentifyAccount(actor identity.PublicUser, accountID string, input IdentifyAccountInput) (Account, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	platformAccountID := strings.TrimSpace(input.PlatformAccountID)
	if platformAccountID == "" || len(platformAccountID) > 255 || !validLoginStatus(input.LoginStatus) {
		return Account{}, ErrInvalidInput
	}
	existing, found, err := s.store.FindByIdentity(record.UserID, record.Platform, platformAccountID)
	if err != nil {
		return Account{}, err
	}
	if found && existing.ID != record.ID {
		record.IdentificationStatus = IdentificationDuplicate
		record.DuplicateOfAccountID = existing.ID
		record.UpdatedAt = s.now()
		if err := s.store.Update(record); err != nil {
			return Account{}, err
		}
		if err := s.audit(actor.ID, "media_account.identify.duplicate", record.ID, map[string]string{"duplicate_of": existing.ID}); err != nil {
			return Account{}, err
		}
		return record.Account, ErrDuplicateAccount
	}
	record.PlatformAccountID = platformAccountID
	record.Name = strings.TrimSpace(input.Name)
	record.AvatarURL = strings.TrimSpace(input.AvatarURL)
	record.LoginStatus = input.LoginStatus
	record.IdentificationStatus = IdentificationIdentified
	record.DuplicateOfAccountID = ""
	record.UpdatedAt = s.now()
	checkedAt := record.UpdatedAt
	record.LastCheckedAt = &checkedAt
	if err := s.store.Update(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.identify", record.ID, map[string]string{
		"platform": string(record.Platform), "platform_account_id": record.PlatformAccountID,
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *Service) AddTags(actor identity.PublicUser, accountIDs, tags []string) error {
	return s.changeTags(actor, accountIDs, tags, true)
}

func (s *Service) RemoveTags(actor identity.PublicUser, accountIDs, tags []string) error {
	return s.changeTags(actor, accountIDs, tags, false)
}

func (s *Service) changeTags(actor identity.PublicUser, accountIDs, tags []string, add bool) error {
	accountIDs = normalizeStrings(accountIDs)
	normalizedTags, err := normalizeTags(tags)
	if err != nil || len(accountIDs) == 0 || len(normalizedTags) == 0 {
		return ErrInvalidInput
	}
	byUser := map[string][]string{}
	for _, accountID := range accountIDs {
		record, err := s.authorizedRecord(actor, accountID)
		if err != nil {
			return err
		}
		byUser[record.UserID] = append(byUser[record.UserID], record.ID)
	}
	for userID, ids := range byUser {
		if add {
			err = s.store.AddTags(userID, ids, normalizedTags, s.now())
		} else {
			err = s.store.RemoveTags(userID, ids, normalizedTags)
		}
		if err != nil {
			return err
		}
	}
	action := "media_account.tags.remove"
	if add {
		action = "media_account.tags.add"
	}
	for _, accountID := range accountIDs {
		if err := s.audit(actor.ID, action, accountID, map[string]string{"tags": strings.Join(normalizedTags, ",")}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) authorizedRecord(actor identity.PublicUser, accountID string) (AccountRecord, error) {
	if !validActor(actor) || strings.TrimSpace(accountID) == "" {
		return AccountRecord{}, ErrForbidden
	}
	record, found, err := s.store.Find(accountID)
	if err != nil {
		return AccountRecord{}, err
	}
	if !found {
		return AccountRecord{}, ErrNotFound
	}
	if !canAccess(actor, record.UserID, record.GameID) {
		return AccountRecord{}, ErrForbidden
	}
	return record, nil
}

func (s *Service) attachTags(records []AccountRecord) ([]Account, error) {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	tagsByAccount, err := s.store.ListTags(ids)
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, 0, len(records))
	for _, record := range records {
		account := record.Account
		account.Tags = tagsByAccount[record.ID]
		if account.Tags == nil {
			account.Tags = []string{}
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (s *Service) audit(actorID, action, accountID string, summary map[string]string) error {
	return s.store.AppendAudit(identity.AuditEvent{
		ID: s.newID("audit"), ActorUserID: actorID, Action: action,
		TargetType: "media_account", TargetID: accountID, Summary: summary, CreatedAt: s.now(),
	})
}

func validActor(actor identity.PublicUser) bool {
	if actor.ID == "" || actor.Status != identity.UserStatusEnabled {
		return false
	}
	return actor.Role == identity.RoleOperator || actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleTechnician
}

func canAccess(actor identity.PublicUser, userID, gameID string) bool {
	if !validActor(actor) {
		return false
	}
	if actor.Role == identity.RoleTechnician {
		return true
	}
	return actor.ID == userID && contains(actor.GameIDs, gameID)
}

func validPlatform(platform Platform) bool {
	return platform == PlatformDouyin || platform == PlatformBilibili || platform == PlatformBaijiahao
}

func validBusinessStatus(status BusinessStatus) bool {
	return status == BusinessEnabled || status == BusinessDisabled || status == BusinessRetired
}

func validLoginStatus(status LoginStatus) bool {
	switch status {
	case LoginUnknown, LoginNormal, LoginNotLoggedIn, LoginVerificationNeeded, LoginExpired, LoginRestricted, LoginAccountMismatch, LoginEnvironmentError:
		return true
	default:
		return false
	}
}

func normalizeTags(tags []string) ([]string, error) {
	normalized := normalizeStrings(tags)
	for _, tag := range normalized {
		if len(tag) > 64 {
			return nil, ErrInvalidInput
		}
	}
	return normalized, nil
}

func normalizeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
