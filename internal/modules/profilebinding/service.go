// Package profilebinding owns staged BitBrowser identity scans and confirmed
// Cloud Browser Profile mirrors.
package profilebinding

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type BitAccountStatus string

const (
	BitAccountBound BitAccountStatus = "bound"
)

type ProfileLocalStatus string

const (
	ProfileActive       ProfileLocalStatus = "active"
	ProfileLocalMissing ProfileLocalStatus = "local_missing"
	ProfileArchived     ProfileLocalStatus = "archived"
)

type ScanStatus string

const (
	ScanReady     ScanStatus = "ready"
	ScanConfirmed ScanStatus = "confirmed"
)

type DiffKind string

const (
	DiffAdded   DiffKind = "added"
	DiffChanged DiffKind = "changed"
	DiffMissing DiffKind = "missing"
)

var (
	ErrForbidden            = errors.New("profile binding operation is forbidden")
	ErrIdentityUnverifiable = errors.New("BitBrowser identity is unverifiable")
	ErrIdentityMismatch     = errors.New("BitBrowser identity does not match the bound user")
	ErrScanNotFound         = errors.New("profile scan was not found")
	ErrScanNotReady         = errors.New("profile scan is not ready")
	ErrScanExpired          = errors.New("profile scan has expired")
	ErrProfileNotFound      = errors.New("browser profile was not found")
	ErrProfileInactive      = errors.New("browser profile is not active")
)

const (
	auditMainAccountBind   = "bitbrowser.main_account.bind"
	auditMainAccountRebind = "bitbrowser.main_account.rebind"
)

type BitAccountBinding struct {
	UserID         identity.UserID  `json:"user_id"`
	MainUserID     string           `json:"main_user_id"`
	Status         BitAccountStatus `json:"status"`
	BoundAt        *time.Time       `json:"bound_at,omitempty"`
	LastVerifiedAt *time.Time       `json:"last_verified_at,omitempty"`
}

type BrowserProfile struct {
	ID            string             `json:"id"`
	UserID        identity.UserID    `json:"user_id"`
	TeamID        *identity.TeamID   `json:"team_id"`
	BitProfileID  string             `json:"bit_profile_id"`
	MainUserID    string             `json:"main_user_id"`
	ProfileUserID string             `json:"profile_user_id"`
	Name          string             `json:"name"`
	Seq           int                `json:"seq,omitempty"`
	GroupID       string             `json:"group_id,omitempty"`
	GroupName     string             `json:"group_name,omitempty"`
	BitStatus     string             `json:"bit_status,omitempty"`
	BitUpdatedAt  string             `json:"bit_updated_at,omitempty"`
	ProxyType     string             `json:"proxy_type,omitempty"`
	ProxyHost     string             `json:"proxy_host,omitempty"`
	ProxyPort     int                `json:"proxy_port,omitempty"`
	Remark        string             `json:"remark,omitempty"`
	LocalStatus   ProfileLocalStatus `json:"local_status"`
	LastSyncedAt  time.Time          `json:"last_synced_at"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

type ProfileInput struct {
	BitProfileID  string `json:"bit_profile_id"`
	MainUserID    string `json:"main_user_id"`
	ProfileUserID string `json:"profile_user_id"`
	Name          string `json:"name"`
	Seq           int    `json:"seq"`
	GroupID       string `json:"group_id"`
	GroupName     string `json:"group_name"`
	BitStatus     string `json:"bit_status"`
	BitUpdatedAt  string `json:"bit_updated_at"`
}

type SnapshotInput struct {
	MainUserID string         `json:"main_user_id"`
	Profiles   []ProfileInput `json:"profiles"`
}

type ProfileDiff struct {
	Kind         DiffKind `json:"kind"`
	BitProfileID string   `json:"bit_profile_id"`
	ProfileID    string   `json:"profile_id,omitempty"`
	Fields       []string `json:"fields"`
}

type ProfileScan struct {
	ID          string           `json:"id"`
	UserID      identity.UserID  `json:"user_id"`
	TeamID      *identity.TeamID `json:"team_id"`
	MainUserID  string           `json:"main_user_id"`
	Status      ScanStatus       `json:"status"`
	Profiles    []BrowserProfile `json:"profiles"`
	Diff        []ProfileDiff    `json:"diff"`
	CreatedAt   time.Time        `json:"created_at"`
	ExpiresAt   time.Time        `json:"expires_at"`
	ConfirmedAt *time.Time       `json:"confirmed_at,omitempty"`
}

type Store interface {
	FindBinding(userID identity.UserID) (BitAccountBinding, bool, error)
	ListProfiles(userID identity.UserID) ([]BrowserProfile, error)
	CreateScan(ProfileScan) error
	FindScan(scanID string) (ProfileScan, bool, error)
	ApplyScan(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error
	DeleteProfile(id string) error
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
		newID:   common.NewID,
		scanTTL: 15 * time.Minute,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) SubmitScan(actor identity.PublicUser, input SnapshotInput) (ProfileScan, error) {
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

func (s *Service) GetScan(actor identity.PublicUser, scanID string) (ProfileScan, error) {
	scan, found, err := s.store.FindScan(strings.TrimSpace(scanID))
	if err != nil {
		return ProfileScan{}, err
	}
	if !found {
		return ProfileScan{}, ErrScanNotFound
	}
	if !validActor(actor) || scan.UserID != actor.ID {
		return ProfileScan{}, ErrForbidden
	}
	return scan, nil
}

func (s *Service) ConfirmScan(actor identity.PublicUser, scanID string) (ProfileScan, error) {
	scan, err := s.GetScan(actor, scanID)
	if err != nil {
		return ProfileScan{}, err
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
	if err := s.store.ApplyScan(scan, binding, now, bindingAuditAction); err != nil {
		return ProfileScan{}, err
	}
	return scan, nil
}

// RejectScan marks a scan as rejected without applying changes.
func (s *Service) RejectScan(actor identity.PublicUser, scanID string) error {
	if !validActor(actor) {
		return ErrForbidden
	}
	scan, found, err := s.store.FindScan(scanID)
	if err != nil {
		return err
	}
	if !found || scan.UserID != actor.ID {
		return ErrScanNotFound
	}
	if scan.Status != ScanReady {
		return ErrScanNotReady
	}
	return nil
}

func (s *Service) ListProfiles(actor identity.PublicUser, userID identity.UserID) ([]BrowserProfile, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	if userID <= 0 {
		userID = actor.ID
	}
	profiles, err := s.store.ListProfiles(userID)
	if err != nil {
		return nil, err
	}
	visible := profiles[:0]
	for _, profile := range profiles {
		if actor.Role == identity.RoleAdmin || actor.ID == profile.UserID || (actor.Role == identity.RoleSeniorOperator && actor.TeamID != nil && profile.TeamID != nil && *actor.TeamID == *profile.TeamID) {
			visible = append(visible, profile)
		}
	}
	if userID != actor.ID && actor.Role == identity.RoleOperator {
		return nil, ErrForbidden
	}
	return visible, nil
}

func (s *Service) DeleteProfile(actor identity.PublicUser, profileID string) error {
	if !validActor(actor) {
		return ErrForbidden
	}
	// Only an admin or the owning user can delete a profile.
	profiles, err := s.store.ListProfiles(actor.ID)
	if err != nil {
		return err
	}
	if actor.Role != identity.RoleAdmin {
		found := false
		for _, p := range profiles {
			if p.ID == profileID {
				found = true
				break
			}
		}
		if !found {
			return ErrForbidden
		}
	}
	return s.store.DeleteProfile(profileID)
}

func (s *Service) GetActiveProfile(actor identity.PublicUser, profileID string) (BrowserProfile, error) {
	profiles, err := s.ListProfiles(actor, 0)
	if err != nil {
		return BrowserProfile{}, err
	}
	for _, profile := range profiles {
		if profile.ID == profileID {
			if profile.LocalStatus != ProfileActive {
				return BrowserProfile{}, ErrProfileInactive
			}
			return profile, nil
		}
	}
	return BrowserProfile{}, ErrProfileNotFound
}

func validActor(actor identity.PublicUser) bool {
	return actor.ID > 0 && actor.Status == identity.UserStatusEnabled &&
		(actor.Role == identity.RoleOperator || actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin)
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
	if current.BitStatus != candidate.BitStatus {
		fields = append(fields, "bit_status")
	}
	if current.BitUpdatedAt != candidate.BitUpdatedAt {
		fields = append(fields, "bit_updated_at")
	}
	return fields
}
