// Package mediaaccount owns Cloud media-account facts, assignment, and tags.
package service

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	profileguardservice "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	Platform             = model.Platform
	IdentificationStatus = model.IdentificationStatus
	BusinessStatus       = model.BusinessStatus
	LoginStatus          = model.LoginStatus
	Account              = model.Account
	AccountRecord        = model.AccountRecord
	AccountCheckItem     = model.AccountCheckItem
	AccountGroup         = model.AccountGroup

	CreateAccountInput      = dto.CreateAccountInput
	IdentifyAccountInput    = dto.IdentifyAccountInput
	AccountCheckStartInput  = dto.AccountCheckStartInput
	AccountCheckStart       = dto.AccountCheckStart
	AccountCheckResultInput = dto.AccountCheckResultInput
	CookieReadStartInput    = dto.CookieReadStartInput
	CookieReadStart         = dto.CookieReadStart
	CookieReadResultInput   = dto.CookieReadResultInput
	UpdateAccountInput      = dto.UpdateAccountInput
	AccountFilter           = dto.AccountFilter
	AccountQuery            = dto.AccountFilter
	AccountGroupFilters     = dto.AccountGroupFilters
	CreateAccountGroupInput = dto.CreateAccountGroupInput
	UpdateAccountGroupInput = dto.UpdateAccountGroupInput
)

const (
	PlatformDouyin           = model.PlatformDouyin
	PlatformBilibili         = model.PlatformBilibili
	PlatformBaijiahao        = model.PlatformBaijiahao
	IdentificationPending    = model.IdentificationPending
	IdentificationIdentified = model.IdentificationIdentified
	IdentificationDuplicate  = model.IdentificationDuplicate
	BusinessEnabled          = model.BusinessEnabled
	BusinessDisabled         = model.BusinessDisabled
	LoginUnknown             = model.LoginUnknown
	LoginNormal              = model.LoginNormal
	LoginNotLoggedIn         = model.LoginNotLoggedIn
	LoginVerificationNeeded  = model.LoginVerificationNeeded
	LoginExpired             = model.LoginExpired
	LoginRestricted          = model.LoginRestricted
	LoginAccountMismatch     = model.LoginAccountMismatch
	LoginEnvironmentError    = model.LoginEnvironmentError
)

var (
	ErrForbidden            = model.ErrForbidden
	ErrInvalidInput         = model.ErrInvalidInput
	ErrNotFound             = model.ErrNotFound
	ErrDuplicateAccount     = model.ErrDuplicateAccount
	ErrProfileUnavailable   = model.ErrProfileUnavailable
	ErrProfilePlatformTaken = model.ErrProfilePlatformTaken
)

type Store interface {
	Create(record AccountRecord) (string, error)
	Find(id string) (AccountRecord, bool, error)
	FindByIdentity(userID identityservice.UserID, platform Platform, platformAccountID string) (AccountRecord, bool, error)
	FindByProfilePlatform(profileID string, platform Platform) (AccountRecord, bool, error)
	Update(AccountRecord, *[]string) error
	List(AccountQuery) ([]AccountRecord, error)
	AddTags(userID identityservice.UserID, accountIDs, tags []string, createdAt time.Time) error
	RemoveTags(userID identityservice.UserID, accountIDs, tags []string) error
	ListTags(accountIDs []string) (map[string][]string, error)
	AppendAudit(identityservice.AuditEvent) error
	CreateGroup(group AccountGroup) (string, error)
	FindGroup(id string) (AccountGroup, bool, error)
	ListGroups(userID identityservice.UserID) ([]AccountGroup, error)
	UpdateGroup(AccountGroup) error
	DeleteGroup(id string) error
}

type accountService struct {
	store          Store
	profiles       profileResolver
	profileFacts   profileFactResolver
	sensitiveTasks sensitiveTaskCreator
	users          userResolver
	games          gameResolver
	now            func() time.Time
	newID          func(string) string
}

type profileResolver interface {
	ResolveProfile(profileID string) (userID identityservice.UserID, active bool, found bool, err error)
}

type profileFactResolver interface {
	ResolveProfileForAccountCheck(profileID string) (id string, userID identityservice.UserID, bitProfileID string, active bool, found bool, err error)
	ResolveProxyForAccountCheck(profileID string) (proxyID string, businessStatus string, lastCheckResult string, expiresAt *time.Time, bound bool, err error)
}

type sensitiveTaskCreator interface {
	CreateAuthorizedTask(profileguardservice.SensitiveTask) error
}

type accountCheckProxyFact struct {
	proxyID         string
	businessStatus  string
	lastCheckResult string
	expiresAt       *time.Time
	bound           bool
}

type userResolver interface {
	ResolveUser(userID identityservice.UserID) (identityservice.PublicUser, bool, error)
}

// gameResolver exposes enabled-game facts to the media-account module without
// copying the identity store or applying user game-scope permissions.
type gameResolver interface {
	ResolveGame(gameID string) (identityservice.OperationGame, bool, error)
}

type option func(*accountService)

func withClock(now func() time.Time) option {
	return func(service *accountService) { service.now = now }
}

func withIDGenerator(newID func(string) string) option {
	return func(service *accountService) { service.newID = newID }
}

func withProfileResolver(resolver profileResolver) option {
	return func(service *accountService) { service.profiles = resolver }
}

func withProfileFactResolver(resolver profileFactResolver) option {
	return func(service *accountService) { service.profileFacts = resolver }
}

func withSensitiveTaskCreator(creator sensitiveTaskCreator) option {
	return func(service *accountService) { service.sensitiveTasks = creator }
}

func withUserResolver(resolver userResolver) option {
	return func(service *accountService) { service.users = resolver }
}

func withGameResolver(resolver gameResolver) option {
	return func(service *accountService) { service.games = resolver }
}

func newAccountService(store Store, options ...option) *accountService {
	service := &accountService{store: store, now: func() time.Time { return time.Now().UTC() }, newID: id.NewID}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *accountService) CreateAccount(actor identityservice.PublicUser, input CreateAccountInput) (Account, error) {
	userID := input.UserID
	if userID <= 0 {
		userID = actor.ID
	}
	gameIDs, err := s.resolveCreateGameIDs(input.GameIDs)
	if err != nil {
		return Account{}, err
	}
	platform := Platform(strings.ToLower(strings.TrimSpace(string(input.Platform))))
	remark := strings.TrimSpace(input.Remark)
	name := strings.TrimSpace(input.Name)
	if !validActor(actor) || !validPlatform(platform) || len(remark) > 500 || len(name) > 128 {
		return Account{}, ErrInvalidInput
	}
	target := actor
	teamID := actor.TeamID
	if userID != actor.ID {
		if actor.Role != identityservice.RoleAdmin {
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
	if err := validateGameScope(target, gameIDs); err != nil {
		return Account{}, err
	}
	now := s.now()
	record := AccountRecord{
		Account: Account{
			UserID:               userID,
			TeamID:               teamID,
			GameIDs:              gameIDs,
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
		"user_id": strconv.FormatInt(int64(userID), 10), "game_ids": strings.Join(gameIDs, ","), "platform": string(platform),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

// GetAccountRecord returns the full account record including cookies.
// Intended only for trusted operations like cookie export.
func (s *accountService) GetAccountRecord(actor identityservice.PublicUser, accountID string) (AccountRecord, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return AccountRecord{}, err
	}
	if actor.ID != record.UserID && actor.Role != identityservice.RoleAdmin {
		return AccountRecord{}, ErrForbidden
	}
	return record, nil
}

// GetOwnedAccountRecord authorizes a local sensitive operation. Even admins
// and senior operators must not operate another user's Desktop resources.
func (s *accountService) GetOwnedAccountRecord(actor identityservice.PublicUser, accountID string) (AccountRecord, error) {
	record, err := s.authorizedRecord(actor, accountID)
	if err != nil {
		return AccountRecord{}, err
	}
	if actor.ID != record.UserID {
		return AccountRecord{}, ErrForbidden
	}
	return record, nil
}

func (s *accountService) GetAccount(actor identityservice.PublicUser, accountID string) (Account, error) {
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

func (s *accountService) ListAccounts(actor identityservice.PublicUser, filter AccountFilter) ([]Account, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	filter.GameIDs = normalizeGameIDs(filter.GameIDs)
	if err := s.validateGameIDs(filter.GameIDs); err != nil {
		return nil, err
	}
	if err := validateGameScope(actor, filter.GameIDs); err != nil {
		return nil, err
	}
	if actor.Role == identityservice.RoleOperator {
		if filter.UserID > 0 && filter.UserID != actor.ID {
			return nil, ErrForbidden
		}
		filter.UserID = actor.ID
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
		if actor.CanAccessOwnedResource(record.UserID, record.TeamID) {
			visible = append(visible, record)
		}
	}
	return s.attachTags(visible)
}

func (s *accountService) UpdateAccount(actor identityservice.PublicUser, accountID string, input UpdateAccountInput) (Account, error) {
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
	replaceGameIDs, err := s.resolveUpdateGameIDs(input.GameIDs)
	if err != nil {
		return Account{}, err
	}
	if replaceGameIDs != nil {
		record.GameIDs = *replaceGameIDs
	}
	owner := actor
	if record.UserID != actor.ID {
		if s.users == nil {
			return Account{}, ErrForbidden
		}
		resolved, found, err := s.users.ResolveUser(record.UserID)
		if err != nil {
			return Account{}, err
		}
		if !found {
			return Account{}, ErrNotFound
		}
		owner = resolved
	}
	if err := validateGameScope(owner, record.GameIDs); err != nil {
		return Account{}, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if len(name) > 128 {
			return Account{}, ErrInvalidInput
		}
		record.Name = name
	}
	if input.BusinessStatus == "" && input.LoginStatus == "" && input.Remark == nil && input.GameIDs == nil && input.Name == nil {
		return Account{}, ErrInvalidInput
	}
	record.UpdatedAt = s.now()
	if err := s.store.Update(record, replaceGameIDs); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.status.update", record.ID, map[string]string{
		"business_status": string(record.BusinessStatus), "login_status": string(record.LoginStatus),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *accountService) IdentifyAccount(actor identityservice.PublicUser, accountID string, input IdentifyAccountInput) (Account, error) {
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
		if err := s.store.Update(record, nil); err != nil {
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
	if err := s.store.Update(record, nil); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.identify", record.ID, map[string]string{
		"platform": string(record.Platform), "platform_account_id": record.PlatformAccountID,
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *accountService) StartLocalAccountCheck(actor identityservice.PublicUser, accountID string, input AccountCheckStartInput) (AccountCheckStart, error) {
	if actor.Role != identityservice.RoleOperator {
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
	if err := validateGameScope(actor, record.GameIDs); err != nil {
		return AccountCheckStart{}, err
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
	task := profileguardservice.SensitiveTask{
		ID: taskID, UserID: actor.ID, ProfileID: profileID, BitProfileID: bitProfileID, NodeID: nodeID,
		Operation: profileguardservice.OperationAuthenticatedAccountCheck, Status: profileguardservice.TaskAuthorized,
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

func (s *accountService) StartCookieRead(actor identityservice.PublicUser, accountID string, input CookieReadStartInput) (CookieReadStart, error) {
	if actor.Role != identityservice.RoleOperator {
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
	task := profileguardservice.SensitiveTask{
		ID: taskID, UserID: actor.ID, ProfileID: profileID, BitProfileID: bitProfileID, NodeID: nodeID,
		Operation: profileguardservice.OperationCookieRead, Status: profileguardservice.TaskAuthorized,
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

func (s *accountService) ApplyCookieReadResult(actor identityservice.PublicUser, accountID string, input CookieReadResultInput) (Account, error) {
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
	if err := s.store.Update(record, nil); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.cookie_read.result", record.ID, map[string]string{
		"task_id": strings.TrimSpace(input.TaskID), "cookie_count": strconv.Itoa(len(input.Cookies)),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *accountService) ApplyLocalAccountCheckResult(actor identityservice.PublicUser, accountID string, input AccountCheckResultInput) (Account, error) {
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if strings.TrimSpace(input.TaskID) == "" || !validLoginStatus(input.LoginStatus) {
		return Account{}, ErrInvalidInput
	}
	now := s.now()
	proxyFact, err := s.resolveAccountCheckProxyFact(record.BrowserProfileID)
	if err != nil {
		return Account{}, err
	}
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
			record.CheckItems = mergeCheckItems(input.CheckItems, proxyFact, now)
			record.UpdatedAt = now
			record.LastCheckedAt = &now
			if err := s.store.Update(record, nil); err != nil {
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
	// 3/4 由已读回的 Profile—代理正式关系派生；5-8 来自 Agent 返回。
	record.CheckItems = mergeCheckItems(input.CheckItems, proxyFact, now)
	record.UpdatedAt = now
	record.LastCheckedAt = &now
	if err := s.store.Update(record, nil); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.check.result", record.ID, map[string]string{
		"task_id": strings.TrimSpace(input.TaskID), "login_status": string(record.LoginStatus), "message": strings.TrimSpace(input.Message),
	}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *accountService) AddTags(actor identityservice.PublicUser, accountIDs, tags []string) error {
	return s.changeTags(actor, accountIDs, tags, true)
}

func (s *accountService) RemoveTags(actor identityservice.PublicUser, accountIDs, tags []string) error {
	return s.changeTags(actor, accountIDs, tags, false)
}

func (s *accountService) BindProfile(actor identityservice.PublicUser, accountID, profileID string) (Account, error) {
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
	if err := s.store.Update(record, nil); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.profile.bind", record.ID, map[string]string{"browser_profile_id": profileID}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *accountService) UnbindProfile(actor identityservice.PublicUser, accountID string) (Account, error) {
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
	if err := s.store.Update(record, nil); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.profile.unbind", record.ID, map[string]string{}); err != nil {
		return Account{}, err
	}
	return record.Account, nil
}

func (s *accountService) changeTags(actor identityservice.PublicUser, accountIDs, tags []string, add bool) error {
	accountIDs = normalizeStrings(accountIDs)
	normalizedTags, err := normalizeTags(tags)
	if err != nil || len(accountIDs) == 0 || len(normalizedTags) == 0 {
		return ErrInvalidInput
	}
	byUser := map[identityservice.UserID][]string{}
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

func (s *accountService) CreateAccountGroup(actor identityservice.PublicUser, input CreateAccountGroupInput) (AccountGroup, error) {
	if !validActor(actor) {
		return AccountGroup{}, ErrForbidden
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 128 {
		return AccountGroup{}, ErrInvalidInput
	}
	filters, err := s.normalizeGroupFilters(input.Filters)
	if err != nil {
		return AccountGroup{}, ErrInvalidInput
	}
	// 主键一律自增，由 DB 分配；CreateGroup 返回新 id
	userID := actor.ID
	now := s.now()
	group := AccountGroup{
		UserID: userID, TeamID: actor.TeamID,
		Name: name, Filters: filters, CreatedAt: now, UpdatedAt: now,
	}
	newID, err := s.store.CreateGroup(group)
	if err != nil {
		return AccountGroup{}, err
	}
	group.ID = newID
	return group, nil
}

func (s *accountService) ListAccountGroups(actor identityservice.PublicUser) ([]AccountGroup, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	return s.store.ListGroups(actor.ID)
}

func (s *accountService) UpdateAccountGroup(actor identityservice.PublicUser, groupID string, input UpdateAccountGroupInput) (AccountGroup, error) {
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
	if group.UserID != actor.ID && actor.Role != identityservice.RoleAdmin {
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
		filters, err := s.normalizeGroupFilters(*input.Filters)
		if err != nil {
			return AccountGroup{}, ErrInvalidInput
		}
		group.Filters = filters
	}
	group.UpdatedAt = s.now()
	if err := s.store.UpdateGroup(group); err != nil {
		return AccountGroup{}, err
	}
	return group, nil
}

func (s *accountService) DeleteAccountGroup(actor identityservice.PublicUser, groupID string) error {
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
	if group.UserID != actor.ID && actor.Role != identityservice.RoleAdmin {
		return ErrForbidden
	}
	return s.store.DeleteGroup(groupID)
}

func (s *accountService) ListAccountsByGroup(actor identityservice.PublicUser, groupID string) ([]Account, error) {
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
	if group.UserID != actor.ID && actor.Role != identityservice.RoleAdmin {
		return nil, ErrForbidden
	}
	filter := AccountFilter{UserID: actor.ID}
	filter.GameIDs = append([]string(nil), group.Filters.GameIDs...)
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

func (s *accountService) resolveAccountCheckProxyFact(profileID string) (accountCheckProxyFact, error) {
	if s.profileFacts == nil || strings.TrimSpace(profileID) == "" {
		return accountCheckProxyFact{}, nil
	}
	proxyID, businessStatus, lastCheckResult, expiresAt, bound, err := s.profileFacts.ResolveProxyForAccountCheck(profileID)
	if err != nil {
		return accountCheckProxyFact{}, err
	}
	return accountCheckProxyFact{
		proxyID:         strings.TrimSpace(proxyID),
		businessStatus:  strings.TrimSpace(businessStatus),
		lastCheckResult: strings.TrimSpace(lastCheckResult),
		expiresAt:       expiresAt,
		bound:           bound && strings.TrimSpace(proxyID) != "",
	}, nil
}

// mergeCheckItems combines account-check facts 1-4 with Agent items 5-8.
// Proxy checks intentionally never use na: an unbound Profile is a failed
// connectivity precondition and has no expiry to evaluate.
func mergeCheckItems(agentItems []AccountCheckItem, proxyFact accountCheckProxyFact, now time.Time) []AccountCheckItem {
	fixed := []AccountCheckItem{
		{Key: "identity_match", Label: "比特浏览器账号匹配", Status: "pass"},
		{Key: "profile_exists", Label: "绑定窗口存在", Status: "pass"},
		proxyCheckItem(proxyFact, now),
		proxyExpiryCheckItem(proxyFact, now),
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

func proxyCheckItem(proxyFact accountCheckProxyFact, now time.Time) AccountCheckItem {
	item := AccountCheckItem{Key: "proxy_ok", Label: "窗口代理正常"}
	if !proxyFact.bound {
		item.Status, item.Message = "fail", "绑定窗口未配置正式代理关系"
		return item
	}
	if proxyFact.businessStatus != "active" {
		item.Status, item.Message = "fail", "绑定代理已停用或已过期"
		return item
	}
	if proxyFact.expiresAt != nil && !proxyFact.expiresAt.After(now) {
		item.Status, item.Message = "fail", "绑定代理已过期"
		return item
	}
	if proxyFact.lastCheckResult != "ok" {
		item.Status, item.Message = "fail", "绑定代理尚未检测正常"
		return item
	}
	item.Status = "pass"
	return item
}

func proxyExpiryCheckItem(proxyFact accountCheckProxyFact, now time.Time) AccountCheckItem {
	item := AccountCheckItem{Key: "proxy_expired", Label: "代理到期/停用"}
	if !proxyFact.bound {
		item.Status, item.Message = "skip", "绑定窗口未配置代理"
		return item
	}
	if proxyFact.businessStatus != "active" {
		item.Status, item.Message = "fail", "绑定代理已停用或已过期"
		return item
	}
	if proxyFact.expiresAt != nil && !proxyFact.expiresAt.After(now) {
		item.Status, item.Message = "fail", "绑定代理已过期"
		return item
	}
	item.Status = "pass"
	return item
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

func (s *accountService) normalizeGroupFilters(filters AccountGroupFilters) (AccountGroupFilters, error) {
	if !validGroupFilters(filters) {
		return AccountGroupFilters{}, ErrInvalidInput
	}
	filters.GameIDs = normalizeGameIDs(filters.GameIDs)
	if err := s.validateGameIDs(filters.GameIDs); err != nil {
		return AccountGroupFilters{}, err
	}
	return filters, nil
}

func (s *accountService) resolveCreateGameIDs(gameIDs []string) ([]string, error) {
	gameIDs = normalizeGameIDs(gameIDs)
	if err := s.validateGameIDs(gameIDs); err != nil {
		return nil, err
	}
	return gameIDs, nil
}

func (s *accountService) resolveUpdateGameIDs(gameIDs *[]string) (*[]string, error) {
	if gameIDs == nil {
		return nil, nil
	}
	resolved, err := s.resolveCreateGameIDs(*gameIDs)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func (s *accountService) validateGameIDs(gameIDs []string) error {
	if s.games == nil && len(gameIDs) > 0 {
		return ErrInvalidInput
	}
	for _, gameID := range gameIDs {
		if len(gameID) > 32 {
			return ErrInvalidInput
		}
		game, found, err := s.games.ResolveGame(gameID)
		if err != nil {
			return err
		}
		if !found || game.Status != identityservice.GameStatusEnabled {
			return ErrInvalidInput
		}
	}
	return nil
}

func validateGameScope(user identityservice.PublicUser, gameIDs []string) error {
	if user.Role == identityservice.RoleAdmin {
		return nil
	}
	for _, gameID := range gameIDs {
		found := false
		for _, allowedID := range user.GameIDs {
			if allowedID == gameID {
				found = true
				break
			}
		}
		if !found {
			return ErrForbidden
		}
	}
	return nil
}

func normalizeGameIDs(gameIDs []string) []string {
	return normalizeStrings(gameIDs)
}

// NormalizeGameIDs canonicalizes game identifiers at HTTP and persistence boundaries.
func NormalizeGameIDs(gameIDs []string) []string { return normalizeGameIDs(gameIDs) }

func (s *accountService) authorizedRecord(actor identityservice.PublicUser, accountID string) (AccountRecord, error) {
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
	if !actor.CanAccessOwnedResource(record.UserID, record.TeamID) {
		return AccountRecord{}, ErrForbidden
	}
	return record, nil
}

func (s *accountService) attachTags(records []AccountRecord) ([]Account, error) {
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

func (s *accountService) audit(actorID identityservice.UserID, action, accountID string, summary map[string]string) error {
	return s.store.AppendAudit(identityservice.AuditEvent{
		ID: s.newID("audit"), ActorUserID: actorID, Action: action,
		TargetType: "media_account", TargetID: accountID, Summary: summary, CreatedAt: s.now(),
	})
}

func validActor(actor identityservice.PublicUser) bool {
	if actor.ID <= 0 || actor.Status != identityservice.UserStatusEnabled {
		return false
	}
	return actor.Role == identityservice.RoleOperator || actor.Role == identityservice.RoleSeniorOperator || actor.Role == identityservice.RoleAdmin
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
