// Package mediaaccount owns Cloud media-account facts, assignment, and tags.
package service

import (
	"sort"
	"strconv"
	"strings"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	Platform             = model.Platform
	IdentificationStatus = model.IdentificationStatus
	PlatformLoginStatus  = model.PlatformLoginStatus
	Account              = model.Account
	AccountRecord        = model.AccountRecord
	AccountCheckItem     = model.AccountCheckItem

	CreateAccountInput      = dto.CreateAccountInput
	AccountCheckStartInput  = dto.AccountCheckStartInput
	AccountCheckStart       = dto.AccountCheckStart
	AccountCheckResultInput = dto.AccountCheckResultInput
	UpdateAccountInput      = dto.UpdateAccountInput
	AccountFilter           = dto.AccountFilter
	AccountQuery            = dto.AccountFilter
)

const (
	PlatformDouyin                  = model.PlatformDouyin
	PlatformBilibili                = model.PlatformBilibili
	PlatformBaidu                   = model.PlatformBaidu
	IdentificationPending           = model.IdentificationPending
	IdentificationIdentified        = model.IdentificationIdentified
	IdentificationDuplicate         = model.IdentificationDuplicate
	PlatformLoginUnknown            = model.PlatformLoginUnknown
	PlatformLoginNormal             = model.PlatformLoginNormal
	PlatformLoginNotLoggedIn        = model.PlatformLoginNotLoggedIn
	PlatformLoginVerificationNeeded = model.PlatformLoginVerificationNeeded
	PlatformLoginExpired            = model.PlatformLoginExpired
	PlatformLoginRestricted         = model.PlatformLoginRestricted
	PlatformLoginAccountMismatch    = model.PlatformLoginAccountMismatch
	PlatformLoginEnvironmentError   = model.PlatformLoginEnvironmentError
)

var (
	ErrStaleCheck                 = model.ErrStaleCheck
	ErrForbidden                  = model.ErrForbidden
	ErrInvalidInput               = model.ErrInvalidInput
	ErrNotFound                   = model.ErrNotFound
	ErrDuplicateAccount           = model.ErrDuplicateAccount
	ErrProfileUnavailable         = model.ErrProfileUnavailable
	ErrProfileNotFound            = model.ErrProfileNotFound
	ErrProfileAssignedToOtherUser = model.ErrProfileAssignedToOtherUser
	ErrProfilePlatformTaken       = model.ErrProfilePlatformTaken
)

type Store interface {
	ListPage(AccountQuery) ([]AccountRecord, model.AccountCounts, error)
	StartCheck(AccountRecord) error
	CompleteCheck(AccountRecord) error
	Create(record AccountRecord) (string, error)
	Find(id string) (AccountRecord, bool, error)
	FindByIdentity(userID identityservice.UserID, platform Platform, platformAccountID string) (AccountRecord, bool, error)
	FindByProfilePlatform(profileID string, platform Platform) (AccountRecord, bool, error)
	UpdateMetadata(AccountRecord, UpdateAccountInput) error
	SetProfile(AccountRecord) error
	List(AccountQuery) ([]AccountRecord, error)
	ListTags(accountIDs []string) (map[string][]string, error)
	AppendAudit(identityservice.AuditEvent) error
}

type accountService struct {
	store        Store
	profiles     profileResolver
	profileFacts profileFactResolver
	users        userResolver
	categories   categoryResolver
	now          func() time.Time
	newID        func(string) string
}

type profileResolver interface {
	ResolveProfile(profileID string) (userID identityservice.UserID, active bool, inScope bool, found bool, err error)
}

type profileFactResolver interface {
	ResolveProfileForAccountCheck(profileID string) (id string, userID identityservice.UserID, bitProfileID string, inScope bool, active bool, found bool, err error)
	ResolveProxyForAccountCheck(profileID string) (proxyID string, enabled bool, lastCheckResult string, expiresAt *time.Time, bound bool, err error)
}

type accountCheckProxyFact struct {
	proxyID         string
	enabled         bool
	lastCheckResult string
	expiresAt       *time.Time
	bound           bool
}

type userResolver interface {
	ResolveUser(userID identityservice.UserID) (identityservice.PublicUser, bool, error)
}

// categoryResolver exposes enabled-category facts to the media-account module without
// copying the identity store or applying user category-scope permissions.
type categoryResolver interface {
	ResolveCategory(categoryID identityservice.CategoryID) (identityservice.Category, bool, error)
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

func withUserResolver(resolver userResolver) option {
	return func(service *accountService) { service.users = resolver }
}

func withCategoryResolver(resolver categoryResolver) option {
	return func(service *accountService) { service.categories = resolver }
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
	categoryIDs, err := s.resolveCreateCategoryIDs(input.CategoryIDs)
	if err != nil {
		return Account{}, err
	}
	platform := Platform(strings.ToLower(strings.TrimSpace(string(input.Platform))))
	remark := strings.TrimSpace(input.Remark)
	name := strings.TrimSpace(input.PlatformName)
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
	if err := validateCategoryScope(target, categoryIDs); err != nil {
		return Account{}, err
	}
	tags, err := normalizeTags(input.Tags)
	if err != nil {
		return Account{}, err
	}
	now := s.now()
	record := AccountRecord{
		Account: Account{
			UserID:               userID,
			UserName:             displayName(target),
			TeamID:               teamID,
			CategoryIDs:          categoryIDs,
			Platform:             platform,
			PlatformName:         name,
			Remark:               remark,
			IdentificationStatus: IdentificationPending,
			Enabled:              true,
			PlatformLoginStatus:  PlatformLoginUnknown,
			Tags:                 tags,
			LastCheckStatus:      "never",
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
	if err := s.audit(actor.ID, "media_account.create", record.ID, map[string]string{
		"user_id": strconv.FormatInt(int64(userID), 10), "category_ids": formatCategoryIDs(categoryIDs), "platform": string(platform),
	}); err != nil {
		return Account{}, err
	}
	return normalizedAccount(record.Account), nil
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

func (s *accountService) scopedAccountFilter(actor identityservice.PublicUser, filter AccountFilter) (AccountFilter, error) {
	if !validActor(actor) {
		return AccountFilter{}, ErrForbidden
	}
	filter.CategoryIDs = normalizeCategoryIDs(filter.CategoryIDs)
	if err := s.validateCategoryIDs(filter.CategoryIDs); err != nil {
		return AccountFilter{}, err
	}
	if err := validateCategoryScope(actor, filter.CategoryIDs); err != nil {
		return AccountFilter{}, err
	}
	if actor.Role == identityservice.RoleOperator {
		if filter.UserID > 0 && filter.UserID != actor.ID {
			return AccountFilter{}, ErrForbidden
		}
		filter.UserID = actor.ID
	}
	if filter.Platform != "" {
		filter.Platform = Platform(strings.ToLower(strings.TrimSpace(string(filter.Platform))))
		if !validPlatform(filter.Platform) {
			return AccountFilter{}, ErrInvalidInput
		}
	}
	if filter.PlatformLoginStatus != "" && !validPlatformLoginStatus(filter.PlatformLoginStatus) {
		return AccountFilter{}, ErrInvalidInput
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len(filter.Search) > 128 {
		return AccountFilter{}, ErrInvalidInput
	}
	filter.ProfileSearch = strings.TrimSpace(filter.ProfileSearch)
	if len(filter.ProfileSearch) > 128 {
		return AccountFilter{}, ErrInvalidInput
	}
	var err error
	if filter.AnyTags, err = normalizeTags(filter.AnyTags); err != nil {
		return AccountFilter{}, err
	}
	if filter.AllTags, err = normalizeTags(filter.AllTags); err != nil {
		return AccountFilter{}, err
	}
	if filter.ExcludeTags, err = normalizeTags(filter.ExcludeTags); err != nil {
		return AccountFilter{}, err
	}
	if filter.View != "" && filter.View != "normal" && filter.View != "attention" && filter.View != "unbound" {
		return AccountFilter{}, ErrInvalidInput
	}
	if filter.CategoryType != "" && filter.CategoryType != "game" && filter.CategoryType != "ai" && filter.CategoryType != "anime" {
		return AccountFilter{}, ErrInvalidInput
	}
	if actor.Role == identityservice.RoleSeniorOperator {
		if actor.TeamID != nil {
			filter.TeamID = *actor.TeamID
		} else {
			return AccountFilter{}, ErrForbidden
		}
	}
	return filter, nil
}
func (s *accountService) ListAccounts(actor identityservice.PublicUser, filter AccountFilter) ([]Account, error) {
	filter, err := s.scopedAccountFilter(actor, filter)
	if err != nil {
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
	if input.Enabled != nil {
		record.Enabled = *input.Enabled
	}
	if input.Enabled != nil && (input.Tags != nil || input.Remark != nil || input.CategoryIDs != nil) {
		return Account{}, ErrInvalidInput
	}
	if input.Tags != nil {
		tags, err := normalizeTags(*input.Tags)
		if err != nil {
			return Account{}, err
		}
		record.Tags = tags
		input.Tags = &tags
	}
	if input.Remark != nil {
		remark := strings.TrimSpace(*input.Remark)
		if len(remark) > 500 {
			return Account{}, ErrInvalidInput
		}
		record.Remark = remark
	}
	replaceCategoryIDs, err := s.resolveUpdateCategoryIDs(input.CategoryIDs)
	if err != nil {
		return Account{}, err
	}
	if replaceCategoryIDs != nil {
		record.CategoryIDs = *replaceCategoryIDs
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
	if err := validateCategoryScope(owner, record.CategoryIDs); err != nil {
		return Account{}, err
	}
	if input.Enabled == nil && input.Tags == nil && input.Remark == nil && input.CategoryIDs == nil {
		return Account{}, ErrInvalidInput
	}
	record.UpdatedAt = s.now()
	if err := s.store.UpdateMetadata(record, input); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.status.update", record.ID, map[string]string{
		"enabled": strconv.FormatBool(record.Enabled), "platform_login_status": string(record.PlatformLoginStatus),
	}); err != nil {
		return Account{}, err
	}
	return s.GetAccount(actor, record.ID)
}

func canStartLocalOperation(role identityservice.Role) bool {
	return role == identityservice.RoleOperator || role == identityservice.RoleSeniorOperator
}

// CanStartLocalOperation exposes the role half so HTTP handlers that start a
// local task without going through StartLocalAccountCheck apply the same gate.
func CanStartLocalOperation(role identityservice.Role) bool {
	return canStartLocalOperation(role)
}

func (s *accountService) StartLocalAccountCheck(actor identityservice.PublicUser, accountID string, input AccountCheckStartInput) (AccountCheckStart, error) {
	if !canStartLocalOperation(actor.Role) {
		return AccountCheckStart{}, ErrForbidden
	}
	if s.profileFacts == nil {
		return AccountCheckStart{}, ErrProfileUnavailable
	}
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return AccountCheckStart{}, err
	}
	if record.CookieUpdate != nil && (record.CookieUpdate.Status == "running" || record.CookieUpdate.Status == "uncertain") {
		return AccountCheckStart{}, ErrCookieConflict
	}
	if !record.Enabled {
		return AccountCheckStart{}, ErrInvalidInput
	}
	if err := validateCategoryScope(actor, record.CategoryIDs); err != nil {
		return AccountCheckStart{}, err
	}
	if strings.TrimSpace(record.BrowserProfileID) == "" {
		return AccountCheckStart{}, ErrProfileUnavailable
	}
	_, profileUserID, bitProfileID, profileInScope, profileActive, found, err := s.profileFacts.ResolveProfileForAccountCheck(record.BrowserProfileID)
	if err != nil {
		return AccountCheckStart{}, err
	}
	if !found || !profileInScope {
		return AccountCheckStart{}, ErrProfileNotFound
	}
	if profileUserID != actor.ID {
		return AccountCheckStart{}, ErrProfileAssignedToOtherUser
	}
	if !profileActive || bitProfileID == "" {
		return AccountCheckStart{}, ErrProfileUnavailable
	}
	operationID := s.newID("account-check")
	record.CheckOperationID = operationID
	record.CheckProfileID = record.BrowserProfileID
	now := s.now()
	record.CheckStartedAt = &now
	if err := s.store.StartCheck(record); err != nil {
		return AccountCheckStart{}, err
	}
	if err := s.audit(actor.ID, "media_account.check.start", record.ID, map[string]string{
		"operation_id": operationID, "browser_profile_id": record.BrowserProfileID, "bit_profile_id": bitProfileID,
	}); err != nil {
		return AccountCheckStart{}, err
	}
	return AccountCheckStart{
		OperationID: operationID, AccountID: record.ID, BrowserProfileID: record.BrowserProfileID, BitProfileID: bitProfileID,
		Platform: record.Platform, ExpectedPlatformAccountID: record.PlatformAccountID, PlatformLoginStatus: record.PlatformLoginStatus,
	}, nil
}

func (s *accountService) ApplyLocalAccountCheckResult(actor identityservice.PublicUser, accountID string, input AccountCheckResultInput) (Account, error) {
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if input.OperationID == "" || input.OperationID != record.CheckOperationID || record.CheckProfileID != record.BrowserProfileID {
		return Account{}, ErrStaleCheck
	}
	if record.LastCheckStatus == "completed" || record.LastCheckStatus == "failed" {
		return s.GetAccount(actor, record.ID)
	}
	if record.LastCheckStatus != "running" {
		return Account{}, ErrStaleCheck
	}
	if len(input.Message) > 500 || len(input.PlatformAccountID) > 255 || len(input.PlatformName) > 128 || len(input.PlatformAvatarURL) > 2048 {
		return Account{}, ErrInvalidInput
	}
	if input.ExecutionStatus != "completed" && input.ExecutionStatus != "failed" {
		return Account{}, ErrInvalidInput
	}
	if input.ExecutionStatus == "failed" {
		record.LastCheckStatus = "failed"
		record.LastCheckMessage = strings.TrimSpace(input.Message)
		record.UpdatedAt = s.now()
		if err := s.store.CompleteCheck(record); err != nil {
			return s.settledCheck(actor, record, err)
		}
		return s.GetAccount(actor, record.ID)
	}
	if !validPlatformLoginStatus(input.PlatformLoginStatus) || input.PlatformLoginStatus == PlatformLoginEnvironmentError {
		return Account{}, ErrInvalidInput
	}
	record.LastCheckStatus = "completed"
	record.LastCheckMessage = strings.TrimSpace(input.Message)
	record.LastCheckIdentity = &model.CheckIdentity{PlatformAccountID: strings.TrimSpace(input.PlatformAccountID), PlatformName: strings.TrimSpace(input.PlatformName), PlatformAvatarURL: strings.TrimSpace(input.PlatformAvatarURL)}
	now := s.now()
	proxyFact, err := s.resolveAccountCheckProxyFact(record.BrowserProfileID)
	if err != nil {
		return Account{}, err
	}
	platformAccountID := strings.TrimSpace(input.PlatformAccountID)
	if input.PlatformLoginStatus == PlatformLoginNormal && platformAccountID != "" && record.PlatformAccountID != "" && platformAccountID != record.PlatformAccountID {
		input.PlatformLoginStatus = PlatformLoginAccountMismatch
	}
	record.PlatformLoginStatus = input.PlatformLoginStatus
	if input.PlatformLoginStatus == PlatformLoginNormal {
		if platformAccountID == "" {
			return Account{}, ErrInvalidInput
		}
		existing, found, err := s.store.FindByIdentity(record.UserID, record.Platform, platformAccountID)
		if err != nil {
			return Account{}, err
		}
		if found && existing.ID != record.ID {
			record.IdentificationStatus = IdentificationDuplicate
			record.LastCheckMessage = "识别出的账号已存在，请查看已有账号"
			record.DuplicateOfAccountID = existing.ID
			record.PlatformLoginStatus = PlatformLoginAccountMismatch
			record.LastCheckItems = mergeLastCheckItems(input.LastCheckItems, proxyFact, now)
			record.UpdatedAt = now
			record.LastCheckAt = &now
			if err := s.store.CompleteCheck(record); err != nil {
				return s.settledCheck(actor, record, err)
			}
			if err := s.audit(actor.ID, "media_account.check.result", record.ID, map[string]string{
				"operation_id": strings.TrimSpace(input.OperationID), "login_status": string(record.PlatformLoginStatus), "message": "duplicate:" + existing.ID,
			}); err != nil {
				return Account{}, err
			}
			return s.GetAccount(actor, record.ID)
		}
		record.PlatformAccountID = platformAccountID
		record.PlatformName = strings.TrimSpace(input.PlatformName)
		record.PlatformAvatarURL = strings.TrimSpace(input.PlatformAvatarURL)
		record.IdentificationStatus = IdentificationIdentified
		record.DuplicateOfAccountID = ""
	} else if platformAccountID != "" && record.PlatformAccountID != "" && platformAccountID != record.PlatformAccountID {
		record.PlatformLoginStatus = PlatformLoginAccountMismatch
	} else {
		record.PlatformLoginStatus = input.PlatformLoginStatus
	}
	// 合成 8 项检查明细：1/2 来自本检查前置（能执行到此处说明前置通过）；
	// 3/4 由已读回的 Profile—代理正式关系派生；5-8 来自 Agent 返回。
	record.LastCheckItems = mergeLastCheckItems(input.LastCheckItems, proxyFact, now)
	record.UpdatedAt = now
	record.LastCheckAt = &now
	if err := s.store.CompleteCheck(record); err != nil {
		return s.settledCheck(actor, record, err)
	}
	if err := s.audit(actor.ID, "media_account.check.result", record.ID, map[string]string{
		"operation_id": strings.TrimSpace(input.OperationID), "login_status": string(record.PlatformLoginStatus), "message": strings.TrimSpace(input.Message),
	}); err != nil {
		return Account{}, err
	}
	return s.GetAccount(actor, record.ID)
}

func (s *accountService) BindProfile(actor identityservice.PublicUser, accountID, profileID string) (Account, error) {
	if !canStartLocalOperation(actor.Role) {
		return Account{}, ErrForbidden
	}
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if !record.Enabled {
		return Account{}, ErrInvalidInput
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || s.profiles == nil {
		return Account{}, ErrProfileUnavailable
	}
	if record.BrowserProfileID == profileID {
		return record.Account, nil
	}
	userID, active, inScope, found, err := s.profiles.ResolveProfile(profileID)
	if err != nil {
		return Account{}, err
	}
	if !found || !inScope {
		return Account{}, ErrProfileNotFound
	}
	if userID != record.UserID {
		return Account{}, ErrProfileAssignedToOtherUser
	}
	if !active {
		return Account{}, ErrProfileUnavailable
	}
	existing, found, err := s.store.FindByProfilePlatform(profileID, record.Platform)
	if err != nil {
		return Account{}, err
	}
	if found && existing.ID != record.ID {
		return Account{}, ErrProfilePlatformTaken
	}
	record.BrowserProfileID = profileID
	record.CheckOperationID = ""
	record.LastCheckStatus = "never"
	record.UpdatedAt = s.now()
	if err := s.store.SetProfile(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.profile.bind", record.ID, map[string]string{"browser_profile_id": profileID}); err != nil {
		return Account{}, err
	}
	return s.GetAccount(actor, record.ID)
}

func (s *accountService) UnbindProfile(actor identityservice.PublicUser, accountID string) (Account, error) {
	if !canStartLocalOperation(actor.Role) {
		return Account{}, ErrForbidden
	}
	record, err := s.GetOwnedAccountRecord(actor, accountID)
	if err != nil {
		return Account{}, err
	}
	if !record.Enabled {
		return Account{}, ErrInvalidInput
	}
	if record.BrowserProfileID == "" {
		return record.Account, nil
	}
	record.BrowserProfileID = ""
	record.CheckOperationID = ""
	record.LastCheckStatus = "never"
	record.UpdatedAt = s.now()
	if err := s.store.SetProfile(record); err != nil {
		return Account{}, err
	}
	if err := s.audit(actor.ID, "media_account.profile.unbind", record.ID, map[string]string{}); err != nil {
		return Account{}, err
	}
	return s.GetAccount(actor, record.ID)
}

func (s *accountService) resolveAccountCheckProxyFact(profileID string) (accountCheckProxyFact, error) {
	if s.profileFacts == nil || strings.TrimSpace(profileID) == "" {
		return accountCheckProxyFact{}, nil
	}
	proxyID, enabled, lastCheckResult, expiresAt, bound, err := s.profileFacts.ResolveProxyForAccountCheck(profileID)
	if err != nil {
		return accountCheckProxyFact{}, err
	}
	return accountCheckProxyFact{
		proxyID:         strings.TrimSpace(proxyID),
		enabled:         enabled,
		lastCheckResult: strings.TrimSpace(lastCheckResult),
		expiresAt:       expiresAt,
		bound:           bound && strings.TrimSpace(proxyID) != "",
	}, nil
}

// mergeLastCheckItems combines account-check facts 1-4 with Agent items 5-8.
// Proxy checks intentionally never use na: an unbound Profile is a failed
// connectivity precondition and has no expiry to evaluate.
func mergeLastCheckItems(agentItems []AccountCheckItem, proxyFact accountCheckProxyFact, now time.Time) []AccountCheckItem {
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
	if !proxyFact.enabled {
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
	if !proxyFact.enabled {
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

func (s *accountService) resolveCreateCategoryIDs(categoryIDs []identityservice.CategoryID) ([]identityservice.CategoryID, error) {
	categoryIDs = normalizeCategoryIDs(categoryIDs)
	if err := s.validateCategoryIDs(categoryIDs); err != nil {
		return nil, err
	}
	return categoryIDs, nil
}

func (s *accountService) resolveUpdateCategoryIDs(categoryIDs *[]identityservice.CategoryID) (*[]identityservice.CategoryID, error) {
	if categoryIDs == nil {
		return nil, nil
	}
	resolved, err := s.resolveCreateCategoryIDs(*categoryIDs)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func (s *accountService) validateCategoryIDs(categoryIDs []identityservice.CategoryID) error {
	if s.categories == nil && len(categoryIDs) > 0 {
		return ErrInvalidInput
	}
	var selectedType identityservice.CategoryType
	for _, categoryID := range categoryIDs {
		category, found, err := s.categories.ResolveCategory(categoryID)
		if err != nil {
			return err
		}
		if !found || !category.Enabled {
			return ErrInvalidInput
		}
		if selectedType != "" && category.Type != selectedType {
			return ErrInvalidInput
		}
		selectedType = category.Type
	}
	return nil
}

func validateCategoryScope(user identityservice.PublicUser, categoryIDs []identityservice.CategoryID) error {
	if user.Role == identityservice.RoleAdmin {
		return nil
	}
	for _, categoryID := range categoryIDs {
		found := false
		for _, allowedID := range user.CategoryIDs {
			if allowedID == categoryID {
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

func normalizeCategoryIDs(categoryIDs []identityservice.CategoryID) []identityservice.CategoryID {
	seen := make(map[identityservice.CategoryID]struct{}, len(categoryIDs))
	result := make([]identityservice.CategoryID, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		if categoryID <= 0 {
			continue
		}
		if _, duplicate := seen[categoryID]; duplicate {
			continue
		}
		seen[categoryID] = struct{}{}
		result = append(result, categoryID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func formatCategoryIDs(categoryIDs []identityservice.CategoryID) string {
	parts := make([]string, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		parts = append(parts, strconv.FormatInt(int64(categoryID), 10))
	}
	return strings.Join(parts, ",")
}

// NormalizeCategoryIDs canonicalizes category identifiers at HTTP and persistence boundaries.
func NormalizeCategoryIDs(categoryIDs []identityservice.CategoryID) []identityservice.CategoryID {
	return normalizeCategoryIDs(categoryIDs)
}

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

func normalizedAccount(account Account) Account {
	if account.LastCheckStatus == "" {
		account.LastCheckStatus = "never"
	}
	if account.CategoryIDs == nil {
		account.CategoryIDs = []identityservice.CategoryID{}
	}
	if account.Tags == nil {
		account.Tags = []string{}
	}
	if account.LastCheckItems == nil {
		account.LastCheckItems = []AccountCheckItem{}
	}
	return account
}

func displayName(user identityservice.PublicUser) string {
	if strings.TrimSpace(user.Nickname) != "" {
		return strings.TrimSpace(user.Nickname)
	}
	return strings.TrimSpace(user.Username)
}

func validActor(actor identityservice.PublicUser) bool {
	if actor.ID <= 0 || actor.Status != identityservice.UserStatusEnabled {
		return false
	}
	return actor.Role == identityservice.RoleOperator || actor.Role == identityservice.RoleSeniorOperator || actor.Role == identityservice.RoleAdmin
}

func validPlatform(platform Platform) bool {
	return platform == PlatformDouyin || platform == PlatformBilibili || platform == PlatformBaidu
}

func validPlatformLoginStatus(status PlatformLoginStatus) bool {
	switch status {
	case PlatformLoginUnknown, PlatformLoginNormal, PlatformLoginNotLoggedIn, PlatformLoginVerificationNeeded, PlatformLoginExpired, PlatformLoginRestricted, PlatformLoginAccountMismatch, PlatformLoginEnvironmentError:
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

func contains[T comparable](values []T, target T) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *accountService) settledCheck(actor identityservice.PublicUser, record AccountRecord, cause error) (Account, error) {
	current, err := s.GetOwnedAccountRecord(actor, record.ID)
	if err != nil {
		return Account{}, err
	}
	if current.CheckOperationID == record.CheckOperationID && current.CheckProfileID == current.BrowserProfileID && (current.LastCheckStatus == "completed" || current.LastCheckStatus == "failed") {
		return s.GetAccount(actor, current.ID)
	}
	return Account{}, cause
}

func (s *accountService) ListAccountPage(actor identityservice.PublicUser, filter AccountFilter, page, size int) (dto.AccountPage, error) {
	if page < 1 || size < 1 || size > 100 {
		return dto.AccountPage{}, ErrInvalidInput
	}
	filter, err := s.scopedAccountFilter(actor, filter)
	if err != nil {
		return dto.AccountPage{}, err
	}
	filter.Limit = size
	filter.Offset = (page - 1) * size
	records, total, err := s.store.ListPage(filter)
	if err != nil {
		return dto.AccountPage{}, err
	}
	accounts, err := s.attachTags(records)
	return dto.AccountPage{Items: accounts, Total: total.Total, Stats: total, Page: page, PageSize: size}, err
}
