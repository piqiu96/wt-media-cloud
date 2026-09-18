package service

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

func TestSubmitScanStagesDiffWithoutFormalMutation(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")

	scan, err := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		Profiles:   []ProfileInput{{BitProfileID: "bit-profile-1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "窗口一", Seq: 1}},
	})
	if err != nil {
		t.Fatalf("SubmitScan() error = %v", err)
	}
	if scan.Status != ScanReady || len(scan.Diff) != 1 || scan.Diff[0].Kind != DiffAdded {
		t.Fatalf("scan = %#v", scan)
	}
	if len(store.profiles) != 0 || len(store.bindings) != 0 {
		t.Fatalf("formal state changed before confirmation: profiles=%v bindings=%v", store.profiles, store.bindings)
	}
}

func TestSubmitScanRejectsUnverifiableIdentity(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := profileActor("user-1")
	cases := []SnapshotInput{
		{MainUserID: "main-user-1"},
		{MainUserID: "", Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: ""}}},
		{MainUserID: "main-user-1", Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-2"}}},
		{MainUserID: "main-user-1", Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "", MainUserID: "main-user-1"}}},
		{MainUserID: "main-user-1", Profiles: []ProfileInput{{BitProfileID: "", ProfileUserID: "bit-user-1", MainUserID: "main-user-1"}}},
	}
	for _, input := range cases {
		if _, err := service.SubmitScan(actor, input); !errors.Is(err, ErrIdentityUnverifiable) {
			t.Fatalf("SubmitScan(%#v) error = %v", input, err)
		}
	}
}

func TestSubmitScanRejectsExistingBindingMismatch(t *testing.T) {
	store := newMemoryStore()
	store.bindings[1] = BitAccountBinding{UserID: 1, MainUserID: "main-user-1", Status: BitAccountBound}
	service := newTestService(store)

	_, err := service.SubmitScan(profileActor("user-1"), SnapshotInput{
		MainUserID: "main-user-2",
		Profiles:   []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-2", MainUserID: "main-user-2"}},
	})
	if !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("SubmitScan() error = %v", err)
	}
	if len(store.scans) != 0 {
		t.Fatal("mismatched scan was staged")
	}
}

func TestConfirmMainIdentityDirectRejectsSilentRebind(t *testing.T) {
	store := newMemoryStore()
	store.bindings[1] = BitAccountBinding{UserID: 1, MainUserID: "main-user-a", Status: BitAccountBound}
	service := newTestService(store)

	_, err := service.ConfirmMainIdentityDirect(profileActor("user-1"), MainIdentityInput{MainUserID: "main-user-b"})
	if !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("ConfirmMainIdentityDirect() error=%v, want ErrIdentityMismatch", err)
	}
	if got := store.bindings[1].MainUserID; got != "main-user-a" {
		t.Fatalf("binding was silently changed to %q", got)
	}
}

func TestAdminCanClearMainIdentityWithoutDeletingProfiles(t *testing.T) {
	store := newMemoryStore()
	store.bindings[2] = BitAccountBinding{UserID: 2, MainUserID: "main-user-a", Status: BitAccountBound}
	store.profiles["profile-1"] = BrowserProfile{ID: "profile-1", UserID: 2, BitProfileID: "bit-profile-1", MainUserID: "main-user-a"}
	service := newTestService(store)
	admin := identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}

	if err := service.ClearMainIdentity(admin, identityservice.UserID(2)); err != nil {
		t.Fatalf("ClearMainIdentity() error=%v", err)
	}
	if _, ok := store.bindings[2]; ok {
		t.Fatalf("binding was not cleared: %#v", store.bindings[2])
	}
	if _, ok := store.profiles["profile-1"]; !ok {
		t.Fatal("clearing main identity deleted browser profiles")
	}
}

func TestOperatorCannotClearMainIdentity(t *testing.T) {
	err := newTestService(newMemoryStore()).ClearMainIdentity(profileActor("user-1"), identityservice.UserID(1))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ClearMainIdentity(operator) error=%v, want ErrForbidden", err)
	}
}

func TestSeniorCanViewButCannotMutateSameTeamProfileCloudRecord(t *testing.T) {
	store := newMemoryStore()
	teamA := identityservice.TeamID(10)
	teamB := identityservice.TeamID(20)
	store.profiles["same-team"] = BrowserProfile{ID: "same-team", UserID: 2, TeamID: &teamA, LocalStatus: ProfileActive}
	store.profiles["other-team"] = BrowserProfile{ID: "other-team", UserID: 3, TeamID: &teamB, LocalStatus: ProfileActive}
	service := newTestService(store)
	actor := identityservice.PublicUser{ID: 1, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamA}

	profiles, err := service.ListProfiles(actor, 0)
	if err != nil || len(profiles) != 1 || profiles[0].ID != "same-team" {
		t.Fatalf("ListProfiles(same team) profiles=%+v error=%v", profiles, err)
	}
	if err := service.DeleteProfile(actor, "same-team"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DeleteProfile(same team) error=%v, want ErrForbidden", err)
	}
	if _, err := service.GetActiveProfile(actor, "other-team"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetActiveProfile(other owner) error=%v, want ErrForbidden", err)
	}
}

func TestDefaultProfileListAppliesRoleVisibility(t *testing.T) {
	teamA := identityservice.TeamID(10)
	teamB := identityservice.TeamID(20)
	for name, tc := range map[string]struct {
		actor identityservice.PublicUser
		want  []string
	}{
		"admin":    {actor: identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}, want: []string{"other-team", "own", "same-team"}},
		"senior":   {actor: identityservice.PublicUser{ID: 1, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamA}, want: []string{"own", "same-team"}},
		"operator": {actor: identityservice.PublicUser{ID: 1, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamA}, want: []string{"own"}},
	} {
		t.Run(name, func(t *testing.T) {
			store := newMemoryStore()
			store.profiles["own"] = BrowserProfile{ID: "own", UserID: 1, TeamID: &teamA}
			store.profiles["same-team"] = BrowserProfile{ID: "same-team", UserID: 2, TeamID: &teamA}
			store.profiles["other-team"] = BrowserProfile{ID: "other-team", UserID: 3, TeamID: &teamB}
			profiles, err := newTestService(store).ListProfiles(tc.actor, 0)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, len(profiles))
			for i := range profiles {
				ids[i] = profiles[i].ID
			}
			sort.Strings(ids)
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("visible profiles=%v, want %v", ids, tc.want)
			}
		})
	}
}

func TestProfileOwnerAndAdminCanSyncDeleteDisabledProfile(t *testing.T) {
	for name, actor := range map[string]identityservice.PublicUser{
		"owner": profileActor("user-2"),
		"admin": {ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled},
	} {
		t.Run(name, func(t *testing.T) {
			store := newMemoryStore()
			teamID := identityservice.TeamID(2)
			store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BusinessStatus: ProfileBusinessDisabled, LocalStatus: ProfileLocalMissing}
			if err := newTestService(store).DeleteProfile(actor, "profile"); err != nil {
				t.Fatalf("DeleteProfile() error=%v", err)
			}
			if _, ok := store.profiles["profile"]; ok {
				t.Fatal("disabled profile not removed by sync-delete")
			}
		})
	}
}

func TestProfileSyncDeleteRejectsEnabledProfile(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(2)
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BusinessStatus: ProfileBusinessEnabled, LocalStatus: ProfileActive}
	err := newTestService(store).DeleteProfile(profileActor("user-2"), "profile")
	if err == nil {
		t.Fatal("DeleteProfile() on enabled profile succeeded, want ErrProfileNotDisabled")
	}
	if _, ok := store.profiles["profile"]; !ok {
		t.Fatal("enabled profile was removed despite rejection")
	}
}

func TestServiceUpdateProfileCloudRemarkAndBusinessStatus(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(2)
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BusinessStatus: ProfileBusinessEnabled, Remark: "比特备注", CloudRemark: "旧备注"}
	service := newTestService(store)

	cloudRemark := "新备注"
	disabled := ProfileBusinessDisabled
	updated, err := service.UpdateProfile(profileActor("user-2"), "profile", &cloudRemark, &disabled)
	if err != nil {
		t.Fatalf("UpdateProfile() error=%v", err)
	}
	if updated.CloudRemark != "新备注" || updated.Remark != "比特备注" || updated.BusinessStatus != ProfileBusinessDisabled {
		t.Fatalf("updated=%+v", updated)
	}
	if _, ok := store.profiles["profile"]; !ok {
		t.Fatal("profile removed by update")
	}
}

func TestAdminCanAssignUnreferencedProfileToOperator(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	targetTeamID := identityservice.TeamID(20)
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BitProfileID: "bit-profile-1", LocalStatus: ProfileActive}
	service := newTestService(store)
	admin := identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}
	target := identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &targetTeamID}

	profile, err := service.AssignProfileOwner(admin, "profile", target)
	if err != nil {
		t.Fatalf("AssignProfileOwner() error=%v", err)
	}
	if profile.UserID != target.ID || profile.TeamID == nil || *profile.TeamID != targetTeamID {
		t.Fatalf("assigned profile=%#v", profile)
	}
	if stored := store.profiles["profile"]; stored.UserID != target.ID || stored.TeamID == nil || *stored.TeamID != targetTeamID {
		t.Fatalf("stored profile=%#v", stored)
	}
	if len(store.audits) != 1 || store.audits[0].Action != "bitbrowser.profile.assign_owner" {
		t.Fatalf("audits=%#v", store.audits)
	}
}

func TestAssignProfileOwnerRejectsNonAdminInvalidTargetAndReferencedProfile(t *testing.T) {
	teamID := identityservice.TeamID(10)
	admin := identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}
	operator := identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID}
	senior := identityservice.PublicUser{ID: 4, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID}

	store := newMemoryStore()
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, LocalStatus: ProfileActive}
	service := newTestService(store)
	if _, err := service.AssignProfileOwner(operator, "profile", operator); !errors.Is(err, ErrForbidden) {
		t.Fatalf("AssignProfileOwner(non-admin) error=%v, want ErrForbidden", err)
	}
	if _, err := service.AssignProfileOwner(admin, "profile", senior); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AssignProfileOwner(senior target) error=%v, want ErrInvalidInput", err)
	}
	store.profileAccountRefs["profile"] = true
	if _, err := service.AssignProfileOwner(admin, "profile", operator); !errors.Is(err, ErrProfileReferenced) {
		t.Fatalf("AssignProfileOwner(referenced) error=%v, want ErrProfileReferenced", err)
	}
}

func TestSeniorCanReviewSameTeamScanButCannotConfirmIt(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	store.scans["scan-same-team"] = ProfileScan{ID: "scan-same-team", UserID: 2, TeamID: &teamID, Status: ScanReady, ExpiresAt: time.Now().Add(time.Hour)}
	service := newTestService(store)
	actor := identityservice.PublicUser{ID: 1, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID}

	if _, err := service.GetScan(actor, "scan-same-team"); err != nil {
		t.Fatalf("GetScan(same team) error=%v", err)
	}
	if _, err := service.ConfirmScan(actor, "scan-same-team"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ConfirmScan(other owner) error=%v, want ErrForbidden", err)
	}
}

func TestConfirmScanBindsUserAndAppliesProfiles(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	scan, _ := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		Profiles: []ProfileInput{
			{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "窗口一", GroupID: "g1", GroupName: "运营组"},
			{BitProfileID: "p2", ProfileUserID: "bit-user-2", MainUserID: "main-user-1", Name: "窗口二"},
		},
	})

	confirmed, err := service.ConfirmScan(actor, scan.ID)
	if err != nil {
		t.Fatalf("ConfirmScan() error = %v", err)
	}
	if confirmed.Status != ScanConfirmed || confirmed.ConfirmedAt == nil {
		t.Fatalf("confirmed scan = %#v", confirmed)
	}
	binding := store.bindings[actor.ID]
	if binding.MainUserID != "main-user-1" || binding.Status != BitAccountBound || binding.BoundAt == nil {
		t.Fatalf("binding = %#v", binding)
	}
	if len(store.audits) != 2 || store.audits[1].Action != auditMainAccountBind {
		t.Fatalf("binding audits = %#v", store.audits)
	}
	profiles := store.profileList(actor.ID)
	if len(profiles) != 2 || profiles[0].BitProfileID != "p1" || profiles[0].ProfileUserID != "bit-user-1" || profiles[1].ProfileUserID != "bit-user-2" {
		t.Fatalf("profiles = %#v", profiles)
	}

	rescan, err := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		Profiles:   []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "窗口一"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, rescan.ID); err != nil {
		t.Fatal(err)
	}
	if store.audits[len(store.audits)-1].Action != auditMainAccountRebind {
		t.Fatalf("rescan binding audit = %#v", store.audits[len(store.audits)-1])
	}
}

func TestConfirmMainIdentityBindsUserWithoutApplyingProfiles(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	scan, _ := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		Profiles: []ProfileInput{
			{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "窗口一"},
			{BitProfileID: "p2", ProfileUserID: "bit-user-2", MainUserID: "main-user-1", Name: "窗口二"},
		},
	})

	confirmed, err := service.ConfirmMainIdentity(actor, scan.ID)
	if err != nil {
		t.Fatalf("ConfirmMainIdentity() error = %v", err)
	}
	if confirmed.Status != ScanConfirmed || confirmed.ConfirmedAt == nil {
		t.Fatalf("confirmed scan = %#v", confirmed)
	}
	binding := store.bindings[actor.ID]
	if binding.MainUserID != "main-user-1" || binding.Status != BitAccountBound || binding.BoundAt == nil || binding.LastVerifiedAt == nil {
		t.Fatalf("binding = %#v", binding)
	}
	if len(store.profiles) != 0 {
		t.Fatalf("identity-only confirmation applied profiles: %#v", store.profiles)
	}
	if len(store.audits) != 1 || store.audits[0].Action != auditMainAccountBind {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestConfirmScanRejectsOtherUserExpiredAndRepeated(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	scan, _ := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		Profiles:   []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1"}},
	})

	if _, err := service.ConfirmScan(profileActor("user-2"), scan.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other-user confirm error = %v", err)
	}
	expired := store.scans[scan.ID]
	expired.ExpiresAt = expired.CreatedAt.Add(-time.Second)
	store.scans[scan.ID] = expired
	if _, err := service.ConfirmScan(actor, scan.ID); !errors.Is(err, ErrScanExpired) {
		t.Fatalf("expired confirm error = %v", err)
	}

	expired.ExpiresAt = expired.CreatedAt.Add(time.Hour)
	store.scans[scan.ID] = expired
	if _, err := service.ConfirmScan(actor, scan.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, scan.ID); !errors.Is(err, ErrScanNotReady) {
		t.Fatalf("repeated confirm error = %v", err)
	}
}

func TestScanDiffAndConfirmationUpdateAndMarkMissing(t *testing.T) {
	store := newMemoryStore()
	store.bindings[1] = BitAccountBinding{UserID: 1, MainUserID: "main-user-1", Status: BitAccountBound}
	store.profiles["profile-old-1"] = BrowserProfile{ID: "profile-old-1", UserID: 1, BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "旧名称", BitStatus: "0", ProxyType: "http", ProxyHost: "1.1.1.1", ProxyPort: 8080, Remark: "旧备注", LocalStatus: ProfileActive}
	store.profiles["profile-old-2"] = BrowserProfile{ID: "profile-old-2", UserID: 1, BitProfileID: "p2", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "即将缺失", LocalStatus: ProfileActive}
	service := newTestService(store)

	scan, err := service.SubmitScan(profileActor("user-1"), SnapshotInput{
		MainUserID: "main-user-1",
		Profiles: []ProfileInput{
			{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "新名称", BitStatus: "1", ProxyType: "socks5", ProxyHost: "2.2.2.2", ProxyPort: 1080, Remark: "新备注"},
			{BitProfileID: "p3", ProfileUserID: "bit-user-2", MainUserID: "main-user-1", Name: "新增"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	kinds := make([]DiffKind, 0, len(scan.Diff))
	for _, item := range scan.Diff {
		kinds = append(kinds, item.Kind)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	if len(kinds) != 3 || kinds[0] != DiffAdded || kinds[1] != DiffChanged || kinds[2] != DiffMissing {
		t.Fatalf("diff kinds = %v", kinds)
	}
	var changed ProfileDiff
	for _, item := range scan.Diff {
		if item.Kind == DiffChanged && item.BitProfileID == "p1" {
			changed = item
			break
		}
	}
	if strings.Join(changed.Fields, ",") != "name,remark,proxy_type,proxy_host,proxy_port" {
		t.Fatalf("changed fields = %v", changed.Fields)
	}
	if _, err := service.ConfirmScan(profileActor("user-1"), scan.ID); err != nil {
		t.Fatal(err)
	}
	profiles := store.profileList(1)
	byBitID := map[string]BrowserProfile{}
	for _, item := range profiles {
		byBitID[item.BitProfileID] = item
	}
	if byBitID["p1"].Name != "新名称" || byBitID["p1"].ProxyHost != "2.2.2.2" || byBitID["p1"].Remark != "旧备注" || byBitID["p2"].LocalStatus != ProfileLocalMissing || byBitID["p3"].LocalStatus != ProfileActive {
		t.Fatalf("profiles after apply = %#v", byBitID)
	}
}

func profileActor(userID string) identityservice.PublicUser {
	parsed, _ := strconv.ParseInt(strings.TrimPrefix(userID, "user-"), 10, 64)
	teamID := identityservice.TeamID(parsed)
	return identityservice.PublicUser{ID: identityservice.UserID(parsed), Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a"}}
}

func newTestService(store Store) *Service {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	sequence := 0
	return NewService(store,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func(prefix string) string {
			sequence++
			return prefix + "-test-" + string(rune('0'+sequence))
		}),
	)
}

type memoryStore struct {
	bindings           map[identityservice.UserID]BitAccountBinding
	profiles           map[string]BrowserProfile
	scans              map[string]ProfileScan
	profileAccountRefs map[string]bool
	audits             []identityservice.AuditEvent
}

func newMemoryStore() *memoryStore {
	return &memoryStore{bindings: map[identityservice.UserID]BitAccountBinding{}, profiles: map[string]BrowserProfile{}, scans: map[string]ProfileScan{}, profileAccountRefs: map[string]bool{}}
}

func (s *memoryStore) FindBinding(userID identityservice.UserID) (BitAccountBinding, bool, error) {
	binding, ok := s.bindings[userID]
	return binding, ok, nil
}

func (s *memoryStore) ListProfiles(userID identityservice.UserID) ([]BrowserProfile, error) {
	return s.profileList(userID), nil
}

func (s *memoryStore) ListAllProfiles() ([]BrowserProfile, error) {
	profiles := make([]BrowserProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (s *memoryStore) GetProfile(profileID string) (BrowserProfile, bool, error) {
	profile, found := s.profiles[profileID]
	return profile, found, nil
}

func (s *memoryStore) CreateScan(scan ProfileScan) error {
	s.scans[scan.ID] = cloneScan(scan)
	return nil
}

func (s *memoryStore) FindScan(scanID string) (ProfileScan, bool, error) {
	scan, ok := s.scans[scanID]
	return cloneScan(scan), ok, nil
}

func (s *memoryStore) ApplyScan(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error {
	s.bindings[binding.UserID] = binding
	seen := map[string]struct{}{}
	for _, candidate := range scan.Profiles {
		seen[candidate.BitProfileID] = struct{}{}
		if existing, ok := s.profiles[candidate.ID]; ok {
			candidate.Remark = existing.Remark
			candidate.CreatedAt = existing.CreatedAt
		}
		candidate.LocalStatus = ProfileActive
		candidate.LastSyncedAt = at
		s.profiles[candidate.ID] = candidate
	}
	for id, profile := range s.profiles {
		if profile.UserID != scan.UserID {
			continue
		}
		if _, ok := seen[profile.BitProfileID]; !ok && profile.LocalStatus != ProfileArchived {
			profile.LocalStatus = ProfileLocalMissing
			profile.LastSyncedAt = at
			s.profiles[id] = profile
		}
	}
	scan.Status = ScanConfirmed
	scan.ConfirmedAt = &at
	s.scans[scan.ID] = cloneScan(scan)
	s.audits = append(s.audits,
		identityservice.AuditEvent{Action: "bitbrowser.profile_scan.confirm", TargetType: "profile_sync_scan", TargetID: scan.ID, Summary: map[string]string{"main_user_id": scan.MainUserID}},
		identityservice.AuditEvent{Action: bindingAuditAction, TargetType: "user", TargetID: strconv.FormatInt(int64(scan.UserID), 10), Summary: map[string]string{"main_user_id": scan.MainUserID, "result": "verified"}},
	)
	return nil
}

func (s *memoryStore) ConfirmMainIdentity(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error {
	s.bindings[binding.UserID] = binding
	scan.Status = ScanConfirmed
	scan.ConfirmedAt = &at
	s.scans[scan.ID] = cloneScan(scan)
	s.audits = append(s.audits,
		identityservice.AuditEvent{Action: bindingAuditAction, TargetType: "user", TargetID: strconv.FormatInt(int64(scan.UserID), 10), Summary: map[string]string{"main_user_id": scan.MainUserID, "result": "identity_verified", "identity_only": "true"}},
	)
	return nil
}

func (s *memoryStore) ConfirmMainIdentityDirect(binding BitAccountBinding, _ time.Time, bindingAuditAction string) error {
	s.bindings[binding.UserID] = binding
	s.audits = append(s.audits,
		identityservice.AuditEvent{Action: bindingAuditAction, TargetType: "user", TargetID: strconv.FormatInt(int64(binding.UserID), 10), Summary: map[string]string{"main_user_id": binding.MainUserID, "result": "identity_verified", "identity_only": "true"}},
	)
	return nil
}

func (s *memoryStore) ClearMainIdentity(userID identityservice.UserID, actorID identityservice.UserID, at time.Time) error {
	delete(s.bindings, userID)
	s.audits = append(s.audits,
		identityservice.AuditEvent{ActorUserID: actorID, Action: "bitbrowser.main_account.clear", TargetType: "user", TargetID: strconv.FormatInt(int64(userID), 10), Summary: map[string]string{"result": "cleared"}, CreatedAt: at},
	)
	return nil
}

func (s *memoryStore) DeleteProfile(id string) error {
	if _, ok := s.profiles[id]; !ok {
		return ErrProfileNotFound
	}
	delete(s.profiles, id)
	return nil
}

func (s *memoryStore) UpdateProfile(profileID string, cloudRemark *string, businessStatus *ProfileBusinessStatus, at time.Time) error {
	profile, ok := s.profiles[profileID]
	if !ok {
		return ErrProfileNotFound
	}
	if cloudRemark != nil {
		profile.CloudRemark = *cloudRemark
	}
	if businessStatus != nil {
		profile.BusinessStatus = *businessStatus
	}
	profile.UpdatedAt = at
	s.profiles[profileID] = profile
	return nil
}

func (s *memoryStore) ProfileHasAccountReferences(profileID string) (bool, error) {
	return s.profileAccountRefs[profileID], nil
}

func (s *memoryStore) ProfileHasDependencies(profileID string) (bool, error) {
	return s.profileAccountRefs[profileID], nil
}

func (s *memoryStore) AssignProfileOwner(profileID string, userID identityservice.UserID, teamID *identityservice.TeamID, actorID identityservice.UserID, at time.Time) error {
	profile, ok := s.profiles[profileID]
	if !ok {
		return ErrProfileNotFound
	}
	profile.UserID = userID
	profile.TeamID = cloneTeamID(teamID)
	profile.UpdatedAt = at
	s.profiles[profileID] = profile
	s.audits = append(s.audits,
		identityservice.AuditEvent{ActorUserID: actorID, Action: "bitbrowser.profile.assign_owner", TargetType: "browser_profile", TargetID: profileID, Summary: map[string]string{"user_id": strconv.FormatInt(int64(userID), 10)}, CreatedAt: at},
	)
	return nil
}

func (s *memoryStore) profileList(userID identityservice.UserID) []BrowserProfile {
	var result []BrowserProfile
	for _, profile := range s.profiles {
		if profile.UserID == userID {
			result = append(result, profile)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].BitProfileID < result[j].BitProfileID })
	return result
}

func cloneScan(scan ProfileScan) ProfileScan {
	scan.Profiles = append([]BrowserProfile(nil), scan.Profiles...)
	scan.Diff = append([]ProfileDiff(nil), scan.Diff...)
	return scan
}
