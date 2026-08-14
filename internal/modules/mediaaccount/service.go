// Package mediaaccount owns Cloud media-account facts, assignment, and tags.
package mediaaccount

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard"
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
	ErrForbidden            = errors.New("media account operation is forbidden")
	ErrInvalidInput         = errors.New("media account input is invalid")
	ErrNotFound             = errors.New("media account was not found")
	ErrDuplicateAccount     = errors.New("media account already exists for this user and platform")
	ErrProfileUnavailable   = errors.New("browser profile is unavailable")
	ErrProfilePlatformTaken = errors.New("browser profile already has an account for this platform")
)

// Account is the API-safe representation. Cookie values are intentionally
// absent and must remain internal to AccountRecord.
type Account struct {
	ID                    string               `json:"id"`
	UserID                identity.UserID      `json:"user_id"`
	TeamID                *identity.TeamID     `json:"team_id"`
	GameID                string               `json:"game_id"`
	Platform              Platform             `json:"platform"`
	PlatformAccountID     string               `json:"platform_account_id,omitempty"`
	Name                  string               `json:"name,omitempty"`
	AvatarURL             string               `json:"avatar_url,omitempty"`
	BrowserProfileID      string               `json:"browser_profile_id,omitempty"`
	Remark                string               `json:"remark,omitempty"`
	IdentificationStatus  IdentificationStatus `json:"identification_status"`
	DuplicateOfAccountID  string               `json:"duplicate_of_account_id,omitempty"`
	BusinessStatus        BusinessStatus       `json:"business_status"`
	LoginStatus           LoginStatus          `json:"login_status"`
	CookieStatus          string               `json:"cookie_status,omitempty"`
	ActiveCookieUpdatedAt *time.Time           `json:"active_cookie_updated_at,omitempty"`
	LastCheckedAt         *time.Time           `json:"last_checked_at,omitempty"`
	CheckItems            []AccountCheckItem   `json:"check_items,omitempty"`
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
	UserID           identity.UserID
	GameID           string
	Name             string
	Platform         Platform
	OriginalCookie   string
	BrowserProfileID string
	Remark           string
	Tags             []string
}

type IdentifyAccountInput struct {
	PlatformAccountID string
	Name              string
	AvatarURL         string
	LoginStatus       LoginStatus
}

type AccountCheckStartInput struct {
	NodeID string
}

type AccountCheckStart struct {
	TaskID                    string      `json:"task_id"`
	AccountID                 string      `json:"account_id"`
	BrowserProfileID          string      `json:"browser_profile_id"`
	BitProfileID              string      `json:"bit_profile_id"`
	Platform                  Platform    `json:"platform"`
	ExpectedPlatformAccountID string      `json:"expected_platform_account_id,omitempty"`
	LoginStatus               LoginStatus `json:"login_status"`
}

type AccountCheckResultInput struct {
	TaskID            string
	PlatformAccountID string
	Name              string
	AvatarURL         string
	LoginStatus       LoginStatus
	Message           string
	CheckItems        []AccountCheckItem // Agent 返回的第 5-8 项；1-4 项由 Cloud 合成
}

type CookieReadStartInput struct {
	NodeID string
}

type CookieReadStart struct {
	TaskID           string   `json:"task_id"`
	AccountID        string   `json:"account_id"`
	BrowserProfileID string   `json:"browser_profile_id"`
	BitProfileID     string   `json:"bit_profile_id"`
	Platform         Platform `json:"platform"`
}

type CookieReadResultInput struct {
	TaskID  string
	Cookies []map[string]any
}

type UpdateAccountInput struct {
	BusinessStatus BusinessStatus
	LoginStatus    LoginStatus
	Remark         *string
	GameID         *string
	Name           *string
}

type AccountFilter struct {
	UserID         identity.UserID
	GameID         string
	Platform       Platform
	BusinessStatus BusinessStatus
	LoginStatus    LoginStatus
	Search         string
	ProfileSearch  string
	AnyTags        []string
	AllTags        []string
	ExcludeTags    []string
}

type AccountQuery = AccountFilter

// AccountGroupFilters 是账号组保存的筛选条件（AccountFilter 子集，均可为空=不筛选）。
type AccountGroupFilters struct {
	GameID         string         `json:"game_id,omitempty"`
	Platform       Platform       `json:"platform,omitempty"`
	BusinessStatus BusinessStatus `json:"business_status,omitempty"`
	LoginStatus    LoginStatus    `json:"login_status,omitempty"`
	Search         string         `json:"search,omitempty"`
	AnyTags        []string       `json:"any_tags,omitempty"`
	AllTags        []string       `json:"all_tags,omitempty"`
	ExcludeTags    []string       `json:"exclude_tags,omitempty"`
}

// AccountCheckItem 是账号检查 8 项中的一项结果（PRD 3.3.10）。
// Status: pass=通过 | fail=不通过 | skip=跳过 | na=不适用（未接入/延后）。
type AccountCheckItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type AccountGroup struct {
	ID        string              `json:"id"`
	UserID    identity.UserID     `json:"user_id"`
	TeamID    *identity.TeamID    `json:"team_id,omitempty"`
	Name      string              `json:"name"`
	Filters   AccountGroupFilters `json:"filters"`
	SortOrder int                 `json:"sort_order"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type CreateAccountGroupInput struct {
	Name    string              `json:"name"`
	Filters AccountGroupFilters `json:"filters"`
}

type UpdateAccountGroupInput struct {
	Name    *string             `json:"name"`
	Filters *AccountGroupFilters `json:"filters"`
}

type Store interface {
	Create(record AccountRecord) (string, error)
	Find(id string) (AccountRecord, bool, error)
	FindByIdentity(userID identity.UserID, platform Platform, platformAccountID string) (AccountRecord, bool, error)
	FindByProfilePlatform(profileID string, platform Platform) (AccountRecord, bool, error)
	Update(AccountRecord) error
	List(AccountQuery) ([]AccountRecord, error)
	AddTags(userID identity.UserID, accountIDs, tags []string, createdAt time.Time) error
	RemoveTags(userID identity.UserID, accountIDs, tags []string) error
	ListTags(accountIDs []string) (map[string][]string, error)
	AppendAudit(identity.AuditEvent) error
	CreateGroup(group AccountGroup) (string, error)
	FindGroup(id string) (AccountGroup, bool, error)
	ListGroups(userID identity.UserID) ([]AccountGroup, error)
	UpdateGroup(AccountGroup) error
	DeleteGroup(id string) error
}

type Service struct {
	store          Store
	profiles       ProfileResolver
	profileFacts   ProfileFactResolver
	sensitiveTasks SensitiveTaskCreator
	users          UserResolver
	now            func() time.Time
	newID          func(string) string
}

type ProfileResolver interface {
	ResolveProfile(profileID string) (userID identity.UserID, active bool, found bool, err error)
}

type ProfileFactResolver interface {
	ResolveProfileForAccountCheck(profileID string) (id string, userID identity.UserID, bitProfileID string, active bool, found bool, err error)
}

type SensitiveTaskCreator interface {
	CreateAuthorizedTask(profileguard.SensitiveTask) error
}

type UserResolver interface {
	ResolveUser(userID identity.UserID) (identity.PublicUser, bool, error)
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(service *Service) { service.now = now }
}

func WithIDGenerator(newID func(string) string) Option {
	return func(service *Service) { service.newID = newID }
}

func WithProfileResolver(resolver ProfileResolver) Option {
	return func(service *Service) { service.profiles = resolver }
}

func WithProfileFactResolver(resolver ProfileFactResolver) Option {
	return func(service *Service) { service.profileFacts = resolver }
}

func WithSensitiveTaskCreator(creator SensitiveTaskCreator) Option {
	return func(service *Service) { service.sensitiveTasks = creator }
}

func WithUserResolver(resolver UserResolver) Option {
	return func(service *Service) { service.users = resolver }
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{store: store, now: func() time.Time { return time.Now().UTC() }, newID: common.NewID}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) CreateAccount(actor identity.PublicUser, input CreateAccountInput) (Account, error) {
	userID := input.UserID
	if userID <= 0 {
		userID = actor.ID
	}
	gameID := strings.TrimSpace(input.GameID)
	platform := Platform(strings.ToLower(strings.TrimSpace(string(input.Platform))))
	remark := strings.TrimSpace(input.Remark)
	name := strings.TrimSpace(input.Name)
	if !validActor(actor) || len(gameID) > 128 || !validPlatform(platform) || len(remark) > 500 || len(name) > 128 {
		return Account{}, ErrInvalidInput
	}
	teamID := actor.TeamID
	if userID != actor.ID {
		if actor.Role != identity.RoleAdmin {
			return Account{}, ErrForbidden
		}
		if s.users == nil {
			return Account{}, ErrForbidden
		}
		target, found, err := s.users.ResolveUser(userID)
		if err != nil {
			return Account{}, err
		}
		if !found {
			return Account{}, ErrInvalidInput
		}
		teamID = target.TeamID
	}
	if gameID != "" && !actor.CanAccess(userID, teamID, gameID) {
		return Account{}, ErrForbidden
	}
	now := s.now()
	record := AccountRecord{
		Account: Account{
			UserID:               userID,
			TeamID:               teamID,
			GameID:               gameID,
			Platform:             platform,
			Name:                 name,
			Remark:               remark,
			IdentificationStatus: IdentificationPending,
			BusinessStatus:       BusinessEnabled,
			LoginStatus:          LoginUnknown,
			Tags:                 []string{},
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		OriginalCookie: input.OriginalCookie,
	}
	// 主键一律自增，由 DB 分配；Create 返回新 id
	newID, err := s.store.Create(record)
	if err != nil {
		return Account{}, err
	}
	record.ID = newID
	if strings.TrimSpace(input.BrowserProfileID) != "" {
		account, err := s.BindProfile(actor, record.ID, input.BrowserProfileID)
		if err != nil {
			return Account{}, err
		}
		record.Account = account
	}
	if len(input.Tags) > 0 {
		if err := s.AddTags(actor, []string{record.ID}, input.Tags); err != nil {
			return Account{}, err
		}
	}
	if err := s.audit(actor.ID, "media_account.create", record.ID, map[string]string{
		"user_id": strconv.FormatInt(int64(userID), 10), "game_id": gameID, "platform": string(platform),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

// GetAccountRecord returns the full account record including cookies.
// Intended only for trusted operations like cookie export.
func (s *Service) GetAccountRecord(actor identity.PublicUser, accountID string) (AccountRecord, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return AccountRecord{}, err
	}
	if actor.ID != record.UserID && actor.Role != identity.RoleAdmin {
		return AccountRecord{}, ErrForbidden
	}
	return record, nil
}

// GetOwnedAccountRecord authorizes a local sensitive operation. Even admins
// and senior operators must not operate another user's Desktop resources.
func (s *Service) GetOwnedAccountRecord(actor identity.PublicUser, accountID string) (AccountRecord, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return AccountRecord{}, err
	}
	if actor.ID != record.UserID {
		return AccountRecord{}, ErrForbidden
	}
	return record, nil
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
	filter.GameID = strings.TrimSpace(filter.GameID)
	if actor.Role == identity.RoleOperator {
		if filter.UserID > 0 && filter.UserID != actor.ID {
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
	if filter.BusinessStatus != "" && !validBusinessStatus(filter.BusinessStatus) {
		return nil, ErrInvalidInput
	}
	if filter.LoginStatus != "" && !validLoginStatus(filter.LoginStatus) {
		return nil, ErrInvalidInput
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len(filter.Search) > 128 {
		return nil, ErrInvalidInput
	}
	filter.ProfileSearch = strings.TrimSpace(filter.ProfileSearch)
	if len(filter.ProfileSearch) > 128 {
		return nil, ErrInvalidInput
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
		if actor.CanAccess(record.UserID, record.TeamID, record.GameID) {
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
	if input.Remark != nil {
		remark := strings.TrimSpace(*input.Remark)
		if len(remark) > 500 {
			return Account{}, ErrInvalidInput
		}
		record.Remark = remark
	}
	if input.GameID != nil {
		gameID := strings.TrimSpace(*input.GameID)
		if len(gameID) > 128 {
			return Account{}, ErrInvalidInput
		}
		if gameID != "" && !actor.CanAccess(record.UserID, record.TeamID, gameID) {
			return Account{}, ErrForbidden
		}
		record.GameID = gameID
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if len(name) > 128 {
			return Account{}, ErrInvalidInput
		}
		record.Name = name
	}
	if input.BusinessStatus == "" && input.LoginStatus == "" && input.Remark == nil && input.GameID == nil && input.Name == nil {
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

func (s *Service) StartLocalAccountCheck(actor identity.PublicUser, accountID string, input AccountCheckStartInput) (AccountCheckStart, error) {
	if actor.Role != identity.RoleOperator {
		return AccountCheckStart{}, ErrForbidden
	}
	if s.profileFacts == nil || s.sensitiveTasks == nil {
		return AccountCheckStart{}, ErrProfileUnavailable
	}
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return AccountCheckStart{}, err
	}
	if record.BusinessStatus != BusinessEnabled {
		return AccountCheckStart{}, ErrInvalidInput
	}
	if strings.TrimSpace(record.BrowserProfileID) == "" {
		return AccountCheckStart{}, ErrProfileUnavailable
	}
	nodeID := strings.TrimSpace(input.NodeID)
	if nodeID == "" {
		return AccountCheckStart{}, ErrInvalidInput
	}
	profileID, profileUserID, bitProfileID, profileActive, found, err := s.profileFacts.ResolveProfileForAccountCheck(record.BrowserProfileID)
	if err != nil {
		return AccountCheckStart{}, err
	}
	if !found || !profileActive || profileUserID != actor.ID || bitProfileID == "" {
		return AccountCheckStart{}, ErrProfileUnavailable
	}
	now := s.now()
	taskID := s.newID("sensitive-account-check")
	task := profileguard.SensitiveTask{
		ID: taskID, UserID: actor.ID, ProfileID: profileID, BitProfileID: bitProfileID, NodeID: nodeID,
		Operation: profileguard.OperationAuthenticatedAccountCheck, Status: profileguard.TaskAuthorized,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.sensitiveTasks.CreateAuthorizedTask(task); err != nil {
		return AccountCheckStart{}, err
	}
	if err := s.audit(actor.ID, "media_account.check.start", record.ID, map[string]string{
		"task_id": taskID, "browser_profile_id": record.BrowserProfileID, "bit_profile_id": bitProfileID,
	}); err != nil {
		return AccountCheckStart{}, err
	}
	return AccountCheckStart{
		TaskID: taskID, AccountID: record.ID, BrowserProfileID: record.BrowserProfileID, BitProfileID: bitProfileID,
		Platform: record.Platform, ExpectedPlatformAccountID: record.PlatformAccountID, LoginStatus: record.LoginStatus,
	}, nil
}

func (s *Service) StartCookieRead(actor identity.PublicUser, accountID string, input CookieReadStartInput) (CookieReadStart, error) {
	if actor.Role != identity.RoleOperator {
		return CookieReadStart{}, ErrForbidden
	}
	if s.profileFacts == nil || s.sensitiveTasks == nil {
		return CookieReadStart{}, ErrProfileUnavailable
	}
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return CookieReadStart{}, err
	}
	if record.BusinessStatus != BusinessEnabled {
		return CookieReadStart{}, ErrInvalidInput
	}
	if strings.TrimSpace(record.BrowserProfileID) == "" {
		return CookieReadStart{}, ErrProfileUnavailable
	}
	nodeID := strings.TrimSpace(input.NodeID)
	if nodeID == "" {
		return CookieReadStart{}, ErrInvalidInput
	}
	profileID, profileUserID, bitProfileID, profileActive, found, err := s.profileFacts.ResolveProfileForAccountCheck(record.BrowserProfileID)
	if err != nil {
		return CookieReadStart{}, err
	}
	if !found || !profileActive || profileUserID != actor.ID || bitProfileID == "" {
		return CookieReadStart{}, ErrProfileUnavailable
	}
	now := s.now()
	taskID := s.newID("sensitive-cookie-read")
	task := profileguard.SensitiveTask{
		ID: taskID, UserID: actor.ID, ProfileID: profileID, BitProfileID: bitProfileID, NodeID: nodeID,
		Operation: profileguard.OperationCookieRead, Status: profileguard.TaskAuthorized,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.sensitiveTasks.CreateAuthorizedTask(task); err != nil {
		return CookieReadStart{}, err
	}
	if err := s.audit(actor.ID, "media_account.cookie_read.start", record.ID, map[string]string{
		"task_id": taskID, "browser_profile_id": record.BrowserProfileID, "bit_profile_id": bitProfileID,
	}); err != nil {
		return CookieReadStart{}, err
	}
	return CookieReadStart{
		TaskID: taskID, AccountID: record.ID, BrowserProfileID: record.BrowserProfileID, BitProfileID: bitProfileID, Platform: record.Platform,
	}, nil
}

func (s *Service) ApplyCookieReadResult(actor identity.PublicUser, accountID string, input CookieReadResultInput) (Account, error) {
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if strings.TrimSpace(input.TaskID) == "" {
		return Account{}, ErrInvalidInput
	}
	now := s.now()
	serialized, err := json.Marshal(input.Cookies)
	if err != nil {
		return Account{}, ErrInvalidInput
	}
	record.ActiveCookie = string(serialized)
	record.CookieStatus = "active"
	record.ActiveCookieUpdatedAt = &now
	record.UpdatedAt = now
	if err := s.store.Update(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.cookie_read.result", record.ID, map[string]string{
		"task_id": strings.TrimSpace(input.TaskID), "cookie_count": strconv.Itoa(len(input.Cookies)),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *Service) ApplyLocalAccountCheckResult(actor identity.PublicUser, accountID string, input AccountCheckResultInput) (Account, error) {
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if strings.TrimSpace(input.TaskID) == "" || !validLoginStatus(input.LoginStatus) {
		return Account{}, ErrInvalidInput
	}
	now := s.now()
	platformAccountID := strings.TrimSpace(input.PlatformAccountID)
	if input.LoginStatus == LoginNormal {
		if platformAccountID == "" {
			return Account{}, ErrInvalidInput
		}
		existing, found, err := s.store.FindByIdentity(record.UserID, record.Platform, platformAccountID)
		if err != nil {
			return Account{}, err
		}
		if found && existing.ID != record.ID {
			record.IdentificationStatus = IdentificationDuplicate
			record.DuplicateOfAccountID = existing.ID
			record.LoginStatus = LoginAccountMismatch
			record.UpdatedAt = now
			record.LastCheckedAt = &now
			if err := s.store.Update(record); err != nil {
				return Account{}, err
			}
			if err := s.audit(actor.ID, "media_account.check.result", record.ID, map[string]string{
				"task_id": strings.TrimSpace(input.TaskID), "login_status": string(record.LoginStatus), "message": "duplicate:" + existing.ID,
			}); err != nil {
				return Account{}, err
			}
			return record.Account, nil
		}
		record.PlatformAccountID = platformAccountID
		if record.Name == "" {
			record.Name = strings.TrimSpace(input.Name)
		}
		record.AvatarURL = strings.TrimSpace(input.AvatarURL)
		record.IdentificationStatus = IdentificationIdentified
		record.DuplicateOfAccountID = ""
	} else if platformAccountID != "" && record.PlatformAccountID != "" && platformAccountID != record.PlatformAccountID {
		record.LoginStatus = LoginAccountMismatch
	} else {
		record.LoginStatus = input.LoginStatus
	}
	if record.LoginStatus != LoginAccountMismatch {
		record.LoginStatus = input.LoginStatus
	}
	// 合成 8 项检查明细：1/2 来自本检查前置（能执行到此处说明前置通过）；
	// 3/4 代理项延后 M2-C（na）；5-8 来自 Agent 返回。
	record.CheckItems = mergeCheckItems(input.CheckItems)
	record.UpdatedAt = now
	record.LastCheckedAt = &now
	if err := s.store.Update(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.check.result", record.ID, map[string]string{
		"task_id": strings.TrimSpace(input.TaskID), "login_status": string(record.LoginStatus), "message": strings.TrimSpace(input.Message),
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

func (s *Service) BindProfile(actor identity.PublicUser, accountID, profileID string) (Account, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if record.BusinessStatus != BusinessEnabled {
		return Account{}, ErrInvalidInput
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || s.profiles == nil {
		return Account{}, ErrProfileUnavailable
	}
	if record.BrowserProfileID == profileID {
		return record.Account, nil
	}
	userID, active, found, err := s.profiles.ResolveProfile(profileID)
	if err != nil {
		return Account{}, err
	}
	if !found || !active {
		return Account{}, ErrProfileUnavailable
	}
	if userID != record.UserID {
		return Account{}, ErrForbidden
	}
	existing, found, err := s.store.FindByProfilePlatform(profileID, record.Platform)
	if err != nil {
		return Account{}, err
	}
	if found && existing.ID != record.ID {
		return Account{}, ErrProfilePlatformTaken
	}
	record.BrowserProfileID = profileID
	record.LoginStatus = LoginUnknown
	record.LastCheckedAt = nil
	record.UpdatedAt = s.now()
	if err := s.store.Update(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.profile.bind", record.ID, map[string]string{"browser_profile_id": profileID}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *Service) UnbindProfile(actor identity.PublicUser, accountID string) (Account, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if record.BusinessStatus != BusinessEnabled {
		return Account{}, ErrInvalidInput
	}
	if record.BrowserProfileID == "" {
		return record.Account, nil
	}
	record.BrowserProfileID = ""
	record.LoginStatus = LoginUnknown
	record.LastCheckedAt = nil
	record.UpdatedAt = s.now()
	if err := s.store.Update(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.profile.unbind", record.ID, map[string]string{}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *Service) changeTags(actor identity.PublicUser, accountIDs, tags []string, add bool) error {
	accountIDs = normalizeStrings(accountIDs)
	normalizedTags, err := normalizeTags(tags)
	if err != nil || len(accountIDs) == 0 || len(normalizedTags) == 0 {
		return ErrInvalidInput
	}
	byUser := map[identity.UserID][]string{}
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

func (s *Service) CreateAccountGroup(actor identity.PublicUser, input CreateAccountGroupInput) (AccountGroup, error) {
	if !validActor(actor) {
		return AccountGroup{}, ErrForbidden
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 128 {
		return AccountGroup{}, ErrInvalidInput
	}
	if !validGroupFilters(input.Filters) {
		return AccountGroup{}, ErrInvalidInput
	}
	// 主键一律自增，由 DB 分配；CreateGroup 返回新 id
	userID := actor.ID
	now := s.now()
	group := AccountGroup{
		UserID: userID, TeamID: actor.TeamID,
		Name: name, Filters: input.Filters, CreatedAt: now, UpdatedAt: now,
	}
	newID, err := s.store.CreateGroup(group)
	if err != nil {
		return AccountGroup{}, err
	}
	group.ID = newID
	return group, nil
}

func (s *Service) ListAccountGroups(actor identity.PublicUser) ([]AccountGroup, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	return s.store.ListGroups(actor.ID)
}

func (s *Service) UpdateAccountGroup(actor identity.PublicUser, groupID string, input UpdateAccountGroupInput) (AccountGroup, error) {
	if !validActor(actor) || strings.TrimSpace(groupID) == "" {
		return AccountGroup{}, ErrForbidden
	}
	group, found, err := s.store.FindGroup(groupID)
	if err != nil {
		return AccountGroup{}, err
	}
	if !found {
		return AccountGroup{}, ErrNotFound
	}
	if group.UserID != actor.ID && actor.Role != identity.RoleAdmin {
		return AccountGroup{}, ErrForbidden
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len(name) > 128 {
			return AccountGroup{}, ErrInvalidInput
		}
		group.Name = name
	}
	if input.Filters != nil {
		if !validGroupFilters(*input.Filters) {
			return AccountGroup{}, ErrInvalidInput
		}
		group.Filters = *input.Filters
	}
	group.UpdatedAt = s.now()
	if err := s.store.UpdateGroup(group); err != nil {
		return AccountGroup{}, err
	}
	return group, nil
}

func (s *Service) DeleteAccountGroup(actor identity.PublicUser, groupID string) error {
	if !validActor(actor) || strings.TrimSpace(groupID) == "" {
		return ErrForbidden
	}
	group, found, err := s.store.FindGroup(groupID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if group.UserID != actor.ID && actor.Role != identity.RoleAdmin {
		return ErrForbidden
	}
	return s.store.DeleteGroup(groupID)
}

func (s *Service) ListAccountsByGroup(actor identity.PublicUser, groupID string) ([]Account, error) {
	if !validActor(actor) || strings.TrimSpace(groupID) == "" {
		return nil, ErrForbidden
	}
	group, found, err := s.store.FindGroup(groupID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	if group.UserID != actor.ID && actor.Role != identity.RoleAdmin {
		return nil, ErrForbidden
	}
	filter := AccountFilter{UserID: actor.ID}
	if group.Filters.GameID != "" {
		filter.GameID = group.Filters.GameID
	}
	if group.Filters.Platform != "" {
		filter.Platform = group.Filters.Platform
	}
	if group.Filters.BusinessStatus != "" {
		filter.BusinessStatus = group.Filters.BusinessStatus
	}
	if group.Filters.LoginStatus != "" {
		filter.LoginStatus = group.Filters.LoginStatus
	}
	filter.Search = group.Filters.Search
	filter.AnyTags = group.Filters.AnyTags
	filter.AllTags = group.Filters.AllTags
	filter.ExcludeTags = group.Filters.ExcludeTags
	return s.ListAccounts(actor, filter)
}

// mergeCheckItems 合成账号检查 8 项明细：固定 1-4 项 + Agent 返回的 5-8 项（缺失补 na）。
func mergeCheckItems(agentItems []AccountCheckItem) []AccountCheckItem {
	fixed := []AccountCheckItem{
		{Key: "identity_match", Label: "比特浏览器账号匹配", Status: "pass"},
		{Key: "profile_exists", Label: "绑定窗口存在", Status: "pass"},
		{Key: "proxy_ok", Label: "窗口代理正常", Status: "na", Message: "代理管理未接入（M2-C）"},
		{Key: "proxy_expired", Label: "代理到期/停用", Status: "na", Message: "代理管理未接入（M2-C）"},
	}
	merged := make([]AccountCheckItem, 0, 8)
	merged = append(merged, fixed...)
	merged = append(merged, agentItems...)
	keys := make(map[string]bool, len(merged))
	for _, item := range merged {
		keys[item.Key] = true
	}
	defaults := []AccountCheckItem{
		{Key: "platform_login", Label: "平台登录状态", Status: "na", Message: "本次未获取到平台身份"},
		{Key: "account_match", Label: "登录账号与台账一致", Status: "na", Message: "未校验"},
		{Key: "verification_needed", Label: "需验证码/安全验证", Status: "na", Message: "需真实受限账号样本对齐"},
		{Key: "account_restricted", Label: "账号限制/封号", Status: "na", Message: "需真实受限账号样本对齐"},
	}
	for _, item := range defaults {
		if !keys[item.Key] {
			merged = append(merged, item)
		}
	}
	return merged
}

func validGroupFilters(filters AccountGroupFilters) bool {
	if filters.Platform != "" && !validPlatform(filters.Platform) {
		return false
	}
	if filters.BusinessStatus != "" && !validBusinessStatus(filters.BusinessStatus) {
		return false
	}
	if filters.LoginStatus != "" && !validLoginStatus(filters.LoginStatus) {
		return false
	}
	return true
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
	if !actor.CanAccess(record.UserID, record.TeamID, record.GameID) {
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

func (s *Service) audit(actorID identity.UserID, action, accountID string, summary map[string]string) error {
	return s.store.AppendAudit(identity.AuditEvent{
		ID: s.newID("audit"), ActorUserID: actorID, Action: action,
		TargetType: "media_account", TargetID: accountID, Summary: summary, CreatedAt: s.now(),
	})
}

func validActor(actor identity.PublicUser) bool {
	if actor.ID <= 0 || actor.Status != identity.UserStatusEnabled {
		return false
	}
	return actor.Role == identity.RoleOperator || actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin
}

func validPlatform(platform Platform) bool {
	return platform == PlatformDouyin || platform == PlatformBilibili || platform == PlatformBaijiahao
}

func validBusinessStatus(status BusinessStatus) bool {
	return status == BusinessEnabled || status == BusinessDisabled
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
