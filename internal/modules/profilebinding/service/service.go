// Package profilebinding owns staged BitBrowser identity scans and confirmed
// Cloud Browser Profile mirrors.
package service

import (
	"sort"
	"strings"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
)

type (
	BitAccountStatus      = model.BitAccountStatus
	ProfileLocalStatus    = model.ProfileLocalStatus
	ProfileBusinessStatus = model.ProfileBusinessStatus
	ScanStatus            = model.ScanStatus
	DiffKind              = model.DiffKind
	BitAccountBinding     = model.BitAccountBinding
	BrowserProfile        = model.BrowserProfile
	ProfileDiff           = model.ProfileDiff
	ProfileScan           = model.ProfileScan

	ProfileInput      = dto.ProfileInput
	SnapshotInput     = dto.SnapshotInput
	MainIdentityInput = dto.MainIdentityInput
)

var (
	ErrForbidden            = model.ErrForbidden
	ErrInvalidInput         = model.ErrInvalidInput
	ErrIdentityUnverifiable = model.ErrIdentityUnverifiable
	ErrIdentityMismatch     = model.ErrIdentityMismatch
	ErrScanNotFound         = model.ErrScanNotFound
	ErrScanNotReady         = model.ErrScanNotReady
	ErrScanExpired          = model.ErrScanExpired
	ErrProfileNotFound      = model.ErrProfileNotFound
	ErrProfileInactive      = model.ErrProfileInactive
	ErrProfileReferenced    = model.ErrProfileReferenced
	ErrProfileNotDisabled   = model.ErrProfileNotDisabled
)

const (
	auditMainAccountBind   = "bitbrowser.main_account.bind"
	auditMainAccountRebind = "bitbrowser.main_account.rebind"

	BitAccountBound         = model.BitAccountBound
	ProfileActive           = model.ProfileActive
	ProfileLocalMissing     = model.ProfileLocalMissing
	ProfileArchived         = model.ProfileArchived
	ProfileBusinessEnabled  = model.ProfileBusinessEnabled
	ProfileBusinessDisabled = model.ProfileBusinessDisabled
	ScanReady               = model.ScanReady
	ScanConfirmed           = model.ScanConfirmed
	DiffAdded               = model.DiffAdded
	DiffChanged             = model.DiffChanged
	DiffMissing             = model.DiffMissing
)

type Store interface {
	FindBinding(userID identityservice.UserID) (BitAccountBinding, bool, error)
	ListProfiles(userID identityservice.UserID) ([]BrowserProfile, error)
	ListAllProfiles() ([]BrowserProfile, error)
	GetProfile(profileID string) (BrowserProfile, bool, error)
	CreateScan(ProfileScan) error
	FindScan(scanID string) (ProfileScan, bool, error)
	ConfirmMainIdentity(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error
	ConfirmMainIdentityDirect(binding BitAccountBinding, at time.Time, bindingAuditAction string) error
	ClearMainIdentity(userID identityservice.UserID, actorID identityservice.UserID, at time.Time) error
	ApplyScan(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error
	DeleteProfile(id string) error
	UpdateProfile(profileID string, cloudRemark *string, businessStatus *ProfileBusinessStatus, at time.Time) error
	ProfileHasAccountReferences(profileID string) (bool, error)
	ProfileHasDependencies(profileID string) (bool, error)
	AssignProfileOwner(profileID string, userID identityservice.UserID, teamID *identityservice.TeamID, actorID identityservice.UserID, at time.Time) error
}

type Service struct {
	store   Store
	now     func() time.Time
	newID   func(string) string
	scanTTL time.Duration
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(service *Service) { service.now = now }
}

func WithIDGenerator(newID func(string) string) Option {
	return func(service *Service) { service.newID = newID }
}

func WithScanTTL(ttl time.Duration) Option {
	return func(service *Service) { service.scanTTL = ttl }
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{
		store:   store,
		now:     func() time.Time { return time.Now().UTC() },
		newID:   id.NewID,
		scanTTL: 15 * time.Minute,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) SubmitScan(actor identityservice.PublicUser, input SnapshotInput) (ProfileScan, error) {
	if !validActor(actor) {
		return ProfileScan{}, ErrForbidden
	}
	mainUserID := strings.TrimSpace(input.MainUserID)
	if mainUserID == "" || len(input.Profiles) == 0 {
		return ProfileScan{}, ErrIdentityUnverifiable
	}
	binding, hasBinding, err := s.store.FindBinding(actor.ID)
	if err != nil {
		return ProfileScan{}, err
	}
	if hasBinding && binding.MainUserID != mainUserID {
		return ProfileScan{}, ErrIdentityMismatch
	}
	existing, err := s.store.ListProfiles(actor.ID)
	if err != nil {
		return ProfileScan{}, err
	}
	existingByBitID := make(map[string]BrowserProfile, len(existing))
	for _, profile := range existing {
		existingByBitID[profile.BitProfileID] = profile
	}

	now := s.now()
	seen := make(map[string]struct{}, len(input.Profiles))
	candidates := make([]BrowserProfile, 0, len(input.Profiles))
	diff := make([]ProfileDiff, 0)
	for _, item := range input.Profiles {
		bitProfileID := strings.TrimSpace(item.BitProfileID)
		itemMainUserID := strings.TrimSpace(item.MainUserID)
		profileUserID := strings.TrimSpace(item.ProfileUserID)
		if bitProfileID == "" || itemMainUserID == "" || profileUserID == "" || itemMainUserID != mainUserID {
			return ProfileScan{}, ErrIdentityUnverifiable
		}
		if _, duplicate := seen[bitProfileID]; duplicate {
			return ProfileScan{}, ErrIdentityUnverifiable
		}
		seen[bitProfileID] = struct{}{}
		candidate := BrowserProfile{
			ID:            s.newID("browser_profile"),
			UserID:        actor.ID,
			TeamID:        actor.TeamID,
			BitProfileID:  bitProfileID,
			MainUserID:    mainUserID,
			ProfileUserID: profileUserID,
			Name:          strings.TrimSpace(item.Name),
			Seq:           item.Seq,
			GroupID:       strings.TrimSpace(item.GroupID),
			GroupName:     strings.TrimSpace(item.GroupName),
			BitStatus:     strings.TrimSpace(item.BitStatus),
			BitUpdatedAt:  strings.TrimSpace(item.BitUpdatedAt),
			ProxyType:     strings.TrimSpace(item.ProxyType),
			ProxyHost:     strings.TrimSpace(item.ProxyHost),
			ProxyPort:     item.ProxyPort,
			Remark:        strings.TrimSpace(item.Remark),
			LocalStatus:   ProfileActive,
			LastSyncedAt:  now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if current, ok := existingByBitID[bitProfileID]; ok {
			candidate.ID = current.ID
			candidate.CreatedAt = current.CreatedAt
			fields := changedFields(current, candidate)
			if len(fields) > 0 || current.LocalStatus != ProfileActive {
				diff = append(diff, ProfileDiff{Kind: DiffChanged, BitProfileID: bitProfileID, ProfileID: current.ID, Fields: fields})
			}
		} else {
			diff = append(diff, ProfileDiff{Kind: DiffAdded, BitProfileID: bitProfileID, ProfileID: candidate.ID, Fields: []string{}})
		}
		candidates = append(candidates, candidate)
	}
	for _, current := range existing {
		if _, ok := seen[current.BitProfileID]; !ok && current.LocalStatus != ProfileArchived {
			diff = append(diff, ProfileDiff{Kind: DiffMissing, BitProfileID: current.BitProfileID, ProfileID: current.ID, Fields: []string{"local_status"}})
		}
	}
	sort.Slice(diff, func(i, j int) bool {
		if diff[i].Kind == diff[j].Kind {
			return diff[i].BitProfileID < diff[j].BitProfileID
		}
		return diff[i].Kind < diff[j].Kind
	})
	scan := ProfileScan{
		ID: s.newID("profile_scan"), UserID: actor.ID, TeamID: actor.TeamID, MainUserID: mainUserID,
		Status: ScanReady, Profiles: candidates, Diff: diff, CreatedAt: now, ExpiresAt: now.Add(s.scanTTL),
	}
	if err := s.store.CreateScan(scan); err != nil {
		return ProfileScan{}, err
	}
	return scan, nil
}

func (s *Service) GetScan(actor identityservice.PublicUser, scanID string) (ProfileScan, error) {
	scan, found, err := s.store.FindScan(strings.TrimSpace(scanID))
	if err != nil {
		return ProfileScan{}, err
	}
	if !found {
		return ProfileScan{}, ErrScanNotFound
	}
	if !validActor(actor) || !actor.CanAccessOwnedResource(scan.UserID, scan.TeamID) {
		return ProfileScan{}, ErrForbidden
	}
	return scan, nil
}

func (s *Service) ConfirmScan(actor identityservice.PublicUser, scanID string) (ProfileScan, error) {
	return s.confirmScan(actor, scanID, true)
}

func (s *Service) ConfirmMainIdentity(actor identityservice.PublicUser, scanID string) (ProfileScan, error) {
	return s.confirmScan(actor, scanID, false)
}

func (s *Service) ConfirmMainIdentityDirect(actor identityservice.PublicUser, input MainIdentityInput) (BitAccountBinding, error) {
	if !validActor(actor) {
		return BitAccountBinding{}, ErrForbidden
	}
	mainUserID := strings.TrimSpace(input.MainUserID)
	if mainUserID == "" {
		return BitAccountBinding{}, ErrIdentityUnverifiable
	}
	now := s.now()
	binding, found, err := s.store.FindBinding(actor.ID)
	if err != nil {
		return BitAccountBinding{}, err
	}
	if found && binding.MainUserID != mainUserID {
		return BitAccountBinding{}, ErrIdentityMismatch
	}
	if !found {
		boundAt := now
		binding = BitAccountBinding{UserID: actor.ID, MainUserID: mainUserID, Status: BitAccountBound, BoundAt: &boundAt}
	}
	verifiedAt := now
	binding.LastVerifiedAt = &verifiedAt
	bindingAuditAction := auditMainAccountBind
	if found {
		bindingAuditAction = auditMainAccountRebind
	}
	if err := s.store.ConfirmMainIdentityDirect(binding, now, bindingAuditAction); err != nil {
		return BitAccountBinding{}, err
	}
	return binding, nil
}

func (s *Service) ClearMainIdentity(actor identityservice.PublicUser, userID identityservice.UserID) error {
	if !validActor(actor) || actor.Role != identityservice.RoleAdmin {
		return ErrForbidden
	}
	if userID <= 0 {
		return ErrInvalidInput
	}
	return s.store.ClearMainIdentity(userID, actor.ID, s.now())
}

func (s *Service) confirmScan(actor identityservice.PublicUser, scanID string, applyProfiles bool) (ProfileScan, error) {
	scan, err := s.GetScan(actor, scanID)
	if err != nil {
		return ProfileScan{}, err
	}
	if scan.UserID != actor.ID {
		return ProfileScan{}, ErrForbidden
	}
	if scan.Status != ScanReady {
		return ProfileScan{}, ErrScanNotReady
	}
	now := s.now()
	if now.After(scan.ExpiresAt) {
		return ProfileScan{}, ErrScanExpired
	}
	binding, found, err := s.store.FindBinding(actor.ID)
	if err != nil {
		return ProfileScan{}, err
	}
	if found && binding.MainUserID != scan.MainUserID {
		return ProfileScan{}, ErrIdentityMismatch
	}
	if !found {
		boundAt := now
		binding = BitAccountBinding{UserID: actor.ID, MainUserID: scan.MainUserID, Status: BitAccountBound, BoundAt: &boundAt}
	}
	verifiedAt := now
	binding.LastVerifiedAt = &verifiedAt
	scan.Status = ScanConfirmed
	scan.ConfirmedAt = &now
	bindingAuditAction := auditMainAccountBind
	if found {
		bindingAuditAction = auditMainAccountRebind
	}
	if applyProfiles {
		err = s.store.ApplyScan(scan, binding, now, bindingAuditAction)
	} else {
		err = s.store.ConfirmMainIdentity(scan, binding, now, bindingAuditAction)
	}
	if err != nil {
		return ProfileScan{}, err
	}
	return scan, nil
}

// RejectScan marks a scan as rejected without applying changes.
func (s *Service) RejectScan(actor identityservice.PublicUser, scanID string) error {
	if !validActor(actor) {
		return ErrForbidden
	}
	scan, found, err := s.store.FindScan(scanID)
	if err != nil {
		return err
	}
	if !found {
		return ErrScanNotFound
	}
	if scan.UserID != actor.ID {
		return ErrForbidden
	}
	if scan.Status != ScanReady {
		return ErrScanNotReady
	}
	return nil
}

func (s *Service) ListProfiles(actor identityservice.PublicUser, userID identityservice.UserID) ([]BrowserProfile, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	var profiles []BrowserProfile
	var err error
	if userID > 0 {
		profiles, err = s.store.ListProfiles(userID)
	} else if actor.Role == identityservice.RoleAdmin || actor.Role == identityservice.RoleSeniorOperator {
		profiles, err = s.store.ListAllProfiles()
	} else {
		profiles, err = s.store.ListProfiles(actor.ID)
	}
	if err != nil {
		return nil, err
	}
	visible := profiles[:0]
	for _, profile := range profiles {
		if actor.CanAccessOwnedResource(profile.UserID, profile.TeamID) {
			visible = append(visible, profile)
		}
	}
	return visible, nil
}

func (s *Service) DeleteProfile(actor identityservice.PublicUser, profileID string) error {
	if !validActor(actor) {
		return ErrForbidden
	}
	profile, found, err := s.store.GetProfile(strings.TrimSpace(profileID))
	if err != nil {
		return err
	}
	if !found {
		return ErrProfileNotFound
	}
	if actor.Role != identityservice.RoleAdmin && profile.UserID != actor.ID {
		return ErrForbidden
	}
	// Sync-delete is only allowed for disabled windows: the window is already
	// gone from BitBrowser (scan reports it missing) and is disabled, so the
	// Cloud mirror has no value left. Active windows must not be deleted.
	if profile.BusinessStatus != ProfileBusinessDisabled {
		return ErrProfileNotDisabled
	}
	referenced, err := s.store.ProfileHasDependencies(profile.ID)
	if err != nil {
		return err
	}
	if referenced {
		return ErrProfileReferenced
	}
	return s.store.DeleteProfile(profileID)
}

func (s *Service) UpdateProfile(actor identityservice.PublicUser, profileID string, cloudRemark *string, businessStatus *ProfileBusinessStatus) (BrowserProfile, error) {
	if !validActor(actor) {
		return BrowserProfile{}, ErrForbidden
	}
	profile, found, err := s.store.GetProfile(strings.TrimSpace(profileID))
	if err != nil {
		return BrowserProfile{}, err
	}
	if !found {
		return BrowserProfile{}, ErrProfileNotFound
	}
	if actor.Role != identityservice.RoleAdmin && profile.UserID != actor.ID {
		return BrowserProfile{}, ErrForbidden
	}
	if businessStatus != nil && *businessStatus != ProfileBusinessEnabled && *businessStatus != ProfileBusinessDisabled {
		return BrowserProfile{}, ErrInvalidInput
	}
	if err := s.store.UpdateProfile(profileID, cloudRemark, businessStatus, s.now()); err != nil {
		return BrowserProfile{}, err
	}
	updated, found, err := s.store.GetProfile(profileID)
	if err != nil {
		return BrowserProfile{}, err
	}
	if !found {
		return BrowserProfile{}, ErrProfileNotFound
	}
	return updated, nil
}

func (s *Service) AssignProfileOwner(actor identityservice.PublicUser, profileID string, target identityservice.PublicUser) (BrowserProfile, error) {
	if !validActor(actor) || actor.Role != identityservice.RoleAdmin {
		return BrowserProfile{}, ErrForbidden
	}
	profile, found, err := s.store.GetProfile(strings.TrimSpace(profileID))
	if err != nil {
		return BrowserProfile{}, err
	}
	if !found {
		return BrowserProfile{}, ErrProfileNotFound
	}
	if target.ID <= 0 || target.Status != identityservice.UserStatusEnabled || target.Role != identityservice.RoleOperator || target.TeamID == nil {
		return BrowserProfile{}, ErrInvalidInput
	}
	referenced, err := s.store.ProfileHasAccountReferences(profile.ID)
	if err != nil {
		return BrowserProfile{}, err
	}
	if referenced {
		return BrowserProfile{}, ErrProfileReferenced
	}
	now := s.now()
	if err := s.store.AssignProfileOwner(profile.ID, target.ID, target.TeamID, actor.ID, now); err != nil {
		return BrowserProfile{}, err
	}
	profile.UserID = target.ID
	profile.TeamID = cloneTeamID(target.TeamID)
	profile.UpdatedAt = now
	return profile, nil
}

func (s *Service) GetActiveProfile(actor identityservice.PublicUser, profileID string) (BrowserProfile, error) {
	if !validActor(actor) {
		return BrowserProfile{}, ErrForbidden
	}
	profile, found, err := s.store.GetProfile(strings.TrimSpace(profileID))
	if err != nil {
		return BrowserProfile{}, err
	}
	if !found {
		return BrowserProfile{}, ErrProfileNotFound
	}
	if profile.UserID != actor.ID {
		return BrowserProfile{}, ErrForbidden
	}
	if profile.LocalStatus != ProfileActive {
		return BrowserProfile{}, ErrProfileInactive
	}
	return profile, nil
}

func cloneTeamID(teamID *identityservice.TeamID) *identityservice.TeamID {
	if teamID == nil {
		return nil
	}
	value := *teamID
	return &value
}

func validActor(actor identityservice.PublicUser) bool {
	return actor.ID > 0 && actor.Status == identityservice.UserStatusEnabled &&
		(actor.Role == identityservice.RoleOperator || actor.Role == identityservice.RoleSeniorOperator || actor.Role == identityservice.RoleAdmin)
}

func changedFields(current, candidate BrowserProfile) []string {
	fields := make([]string, 0, 7)
	if current.MainUserID != candidate.MainUserID {
		fields = append(fields, "main_user_id")
	}
	if current.ProfileUserID != candidate.ProfileUserID {
		fields = append(fields, "profile_user_id")
	}
	if current.Name != candidate.Name {
		fields = append(fields, "name")
	}
	if current.Seq != candidate.Seq {
		fields = append(fields, "seq")
	}
	if current.GroupID != candidate.GroupID {
		fields = append(fields, "group_id")
	}
	if current.GroupName != candidate.GroupName {
		fields = append(fields, "group_name")
	}
	if current.Remark != candidate.Remark {
		fields = append(fields, "remark")
	}
	// bit_status (open/close running state) and bit_updated_at (BitBrowser
	// timestamp) are operational state that changes on open/close; they are NOT
	// window configuration and must not trigger a "changed" diff.
	if current.ProxyType != candidate.ProxyType {
		fields = append(fields, "proxy_type")
	}
	if current.ProxyHost != candidate.ProxyHost {
		fields = append(fields, "proxy_host")
	}
	if current.ProxyPort != candidate.ProxyPort {
		fields = append(fields, "proxy_port")
	}
	return fields
}
