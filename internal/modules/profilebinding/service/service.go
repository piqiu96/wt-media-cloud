// Package service 负责 BitBrowser 扫描暂存，以及确认后写入 Cloud 的 Profile 镜像。
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

// ProfileListFilter 只包含列表页筛选条件。窗口运行状态由 Desktop 实时提供，不属于
// Cloud 行字段。
type ProfileListFilter struct {
	ID             string
	Name           string
	Group          string
	BitProfileID   string
	Remark         string
	LocalStatus    string
	BusinessStatus string
}

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

	ProfileInput        = dto.ProfileInput
	SnapshotInput       = dto.SnapshotInput
	BitAccountBindInput = dto.BitAccountBindInput
)

var (
	ErrForbidden                  = model.ErrForbidden
	ErrInvalidInput               = model.ErrInvalidInput
	ErrIdentityUnverifiable       = model.ErrIdentityUnverifiable
	ErrBitNotBound                = model.ErrBitNotBound
	ErrBitBrowserOffline          = model.ErrBitBrowserOffline
	ErrBitMainUserMismatch        = model.ErrBitMainUserMismatch
	ErrScanNotFound               = model.ErrScanNotFound
	ErrScanNotReady               = model.ErrScanNotReady
	ErrScanExpired                = model.ErrScanExpired
	ErrProfileNotFound            = model.ErrProfileNotFound
	ErrProfileAssignedToOtherUser = model.ErrProfileAssignedToOtherUser
	ErrProfileInactive            = model.ErrProfileInactive
	ErrProfileReferenced          = model.ErrProfileReferenced
	ErrProfileNotDisabled         = model.ErrProfileNotDisabled
)

const (
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
	BindBitAccount(userID identityservice.UserID, mainUserID string, rebind bool, at time.Time) (BitAccountBinding, error)
	ListProfiles(userID identityservice.UserID, filter ProfileListFilter) ([]BrowserProfile, error)
	GetProfile(profileID string) (BrowserProfile, bool, error)
	// GetProfileInScope 只在 Profile 仍属于其所有者当前绑定的 BitBrowser 主账号时
	// 返回。旧主账号下的 Profile 保留在表中，但正常读取不可见。
	GetProfileInScope(profileID string) (BrowserProfile, bool, error)
	CreateScan(ProfileScan) error
	FindScan(scanID string) (ProfileScan, bool, error)
	ApplyScan(scan ProfileScan, at time.Time) error
	DeleteProfile(id string) error
	UpdateProfile(profileID string, cloudRemark *string, businessStatus *ProfileBusinessStatus, at time.Time) error
	ProfileHasAccountReferences(profileID string) (bool, error)
	ProfileHasDependencies(profileID string) (bool, error)
	AssignProfileOwner(profileID string, userID identityservice.UserID, teamID *identityservice.TeamID, actorID identityservice.UserID, at time.Time) error
}

type Service struct {
	store            Store
	now              func() time.Time
	newID            func(string) string
	scanTTL          time.Duration
	observedFreshFor time.Duration
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

// WithObservedFreshFor 设置 Agent 的 BitBrowser 观察可用于判断的最大年龄。
func WithObservedFreshFor(freshFor time.Duration) Option {
	return func(service *Service) { service.observedFreshFor = freshFor }
}

const defaultObservedFreshFor = 120 * time.Second

func newService(store Store, options ...Option) *Service {
	service := &Service{
		store:            store,
		now:              func() time.Time { return time.Now().UTC() },
		newID:            id.NewID,
		scanTTL:          15 * time.Minute,
		observedFreshFor: defaultObservedFreshFor,
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
	if !s.observationFresh(input.ObservedAt) {
		return ProfileScan{}, ErrBitBrowserOffline
	}
	binding, hasBinding, err := s.store.FindBinding(actor.ID)
	if err != nil {
		return ProfileScan{}, err
	}
	if !hasBinding {
		return ProfileScan{}, ErrBitNotBound
	}
	if binding.MainUserID != mainUserID {
		return ProfileScan{}, ErrBitMainUserMismatch
	}
	existing, err := s.store.ListProfiles(actor.ID, ProfileListFilter{})
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

// BindBitAccount 记录 Local Agent 从正在运行的 BitBrowser 读到的主账号，是唯一
// 写入路径：首次观察建立绑定，相同账号只刷新验证时间，不同账号必须由用户明确确认
// 重绑。重绑不修改 browser_profiles，旧主账号的 Profile 原样保留，切回后自然恢复
// 可见。
func (s *Service) BindBitAccount(userID identityservice.UserID, input BitAccountBindInput) (BitAccountBinding, error) {
	if userID <= 0 {
		return BitAccountBinding{}, ErrForbidden
	}
	mainUserID := strings.TrimSpace(input.MainUserID)
	if mainUserID == "" {
		return BitAccountBinding{}, ErrIdentityUnverifiable
	}
	if !s.observationFresh(input.ObservedAt) {
		return BitAccountBinding{}, ErrBitBrowserOffline
	}
	return s.store.BindBitAccount(userID, mainUserID, input.Rebind, s.now())
}

func (s *Service) ConfirmScan(actor identityservice.PublicUser, scanID string) (ProfileScan, error) {
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
	if !found {
		return ProfileScan{}, ErrBitNotBound
	}
	if binding.MainUserID != scan.MainUserID {
		return ProfileScan{}, ErrBitMainUserMismatch
	}
	scan.Status = ScanConfirmed
	scan.ConfirmedAt = &now
	// 确认扫描只应用 Profile。绑定由 Local Agent 通过 /local-agent/bit-account/bind
	// 观察写入，扫描不能顺带建立或更换主账号。
	if err := s.store.ApplyScan(scan, now); err != nil {
		return ProfileScan{}, err
	}
	return scan, nil
}

// observationFresh 判断观察是否仍可用于决策。零值表示 Agent 没有说明观察时间，
// 按过期处理而不是默认可信。
func (s *Service) observationFresh(at time.Time) bool {
	if at.IsZero() {
		return false
	}
	now := s.now()
	if at.After(now) {
		return true
	}
	return now.Sub(at) <= s.observedFreshFor
}

// processObservedFreshFor 是 HTTP 入口使用的观察窗口。进程启动时由 bootstrap
// 从配置安装一次。
var processObservedFreshFor = defaultObservedFreshFor

// ConfigureObservedFreshFor 为 HTTP 处理函数使用的包级操作安装观察新鲜窗口。
func ConfigureObservedFreshFor(freshFor time.Duration) {
	if freshFor > 0 {
		processObservedFreshFor = freshFor
	}
}

// RejectScan 只校验扫描可以被拒绝，不写入任何状态。
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

func (s *Service) ListProfiles(actor identityservice.PublicUser, userID identityservice.UserID, filter ProfileListFilter) ([]BrowserProfile, error) {
	if !validActor(actor) {
		return nil, ErrForbidden
	}
	if userID <= 0 && actor.Role != identityservice.RoleAdmin && actor.Role != identityservice.RoleSeniorOperator {
		userID = actor.ID
	}
	// 查看自己、以及管理员指定某个用户时，没有绑定就没有作用域。页面需要区分
	// “未绑定”（引导绑定）与“没有 Profile”（空结果）。Senior 查看同团队用户时
	// 交给行可见性过滤，避免借此探测他人是否已绑定。
	if userID > 0 && (actor.Role == identityservice.RoleAdmin || userID == actor.ID) {
		if _, bound, err := s.store.FindBinding(userID); err != nil {
			return nil, err
		} else if !bound {
			return nil, ErrBitNotBound
		}
	}
	profiles, err := s.store.ListProfiles(userID, filter)
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
	profile, found, err := s.store.GetProfileInScope(strings.TrimSpace(profileID))
	if err != nil {
		return err
	}
	if !found {
		return ErrProfileNotFound
	}
	if actor.Role != identityservice.RoleAdmin && profile.UserID != actor.ID {
		return ErrForbidden
	}
	// 同步删除只允许已停用的窗口：BitBrowser 中已不存在（扫描报告缺失）且业务已
	// 停用，Cloud 镜像不再有价值。启用的窗口不允许删除。
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
	profile, found, err := s.store.GetProfileInScope(strings.TrimSpace(profileID))
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
	updated, found, err := s.store.GetProfileInScope(profileID)
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
	// 归属只在同一主账号内成立：目标用户必须已绑定，且 Profile 属于该主账号。
	// 跨主账号分配会把目标用户根本看不到的行悄悄转过去。
	binding, bound, err := s.store.FindBinding(target.ID)
	if err != nil {
		return BrowserProfile{}, err
	}
	if !bound {
		return BrowserProfile{}, ErrBitNotBound
	}
	if profile.MainUserID != binding.MainUserID {
		return BrowserProfile{}, ErrBitMainUserMismatch
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
	profile, found, err := s.store.GetProfileInScope(strings.TrimSpace(profileID))
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
	// bit_status 是开关窗口的运行状态，bit_updated_at 是 BitBrowser 时间戳，二者
	// 都会随开关变化，不属于窗口配置，不触发 changed 差异。
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
