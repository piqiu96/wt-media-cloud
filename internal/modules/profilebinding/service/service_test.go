package service

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
)

var testNow = time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)

func TestSubmitScanStagesDiffWithoutFormalMutation(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatalf("BindBitAccount() error = %v", err)
	}

	scan, err := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		ObservedAt: testNow,
		Profiles:   []ProfileInput{{BitProfileID: "bit-profile-1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "窗口一", Seq: 1}},
	})
	if err != nil {
		t.Fatalf("SubmitScan() error = %v", err)
	}
	if scan.Status != ScanReady || len(scan.Diff) != 1 || scan.Diff[0].Kind != DiffAdded {
		t.Fatalf("scan = %#v", scan)
	}
	if len(store.profiles) != 0 {
		t.Fatalf("profiles changed before confirmation: %v", store.profiles)
	}
}

func TestSubmitScanRejectsUnverifiableIdentity(t *testing.T) {
	service := newTestService(newMemoryStore())
	actor := profileActor("user-1")
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatalf("BindBitAccount() error = %v", err)
	}
	cases := []SnapshotInput{
		{MainUserID: "main-user-1", ObservedAt: testNow},
		{MainUserID: "", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: ""}}},
		{MainUserID: "main-user-1", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-2"}}},
		{MainUserID: "main-user-1", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "", MainUserID: "main-user-1"}}},
		{MainUserID: "main-user-1", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "", ProfileUserID: "bit-user-1", MainUserID: "main-user-1"}}},
	}
	for _, input := range cases {
		if _, err := service.SubmitScan(actor, input); !errors.Is(err, ErrIdentityUnverifiable) {
			t.Fatalf("SubmitScan(%#v) error = %v", input, err)
		}
	}
}

func TestSubmitScanRequiresBindingFreshObservationAndMatchingMain(t *testing.T) {
	actor := profileActor("user-1")
	profiles := []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1"}}

	unbound := newMemoryStore()
	if _, err := newTestService(unbound).SubmitScan(actor, SnapshotInput{MainUserID: "main-user-1", ObservedAt: testNow, Profiles: profiles}); !errors.Is(err, ErrBitNotBound) {
		t.Fatalf("SubmitScan(unbound) error = %v", err)
	}

	stale := newMemoryStore()
	bound := newTestService(stale)
	if _, err := bound.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	if _, err := bound.SubmitScan(actor, SnapshotInput{MainUserID: "main-user-1", ObservedAt: testNow.Add(-3 * time.Minute), Profiles: profiles}); !errors.Is(err, ErrBitBrowserOffline) {
		t.Fatalf("SubmitScan(stale) error = %v", err)
	}
	if _, err := bound.SubmitScan(actor, SnapshotInput{MainUserID: "main-user-2", ObservedAt: testNow, Profiles: profiles}); !errors.Is(err, ErrBitMainUserMismatch) {
		t.Fatalf("SubmitScan(mismatch) error = %v", err)
	}
	if len(stale.scans) != 0 {
		t.Fatal("rejected scan was staged")
	}
}

func TestSensitiveObservationCannotBypassFreshness(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1"}); !errors.Is(err, ErrBitBrowserOffline) {
		t.Fatalf("BindBitAccount(no observed_at) error = %v", err)
	}
	if _, err := service.SubmitScan(actor, SnapshotInput{MainUserID: "main-user-1", Profiles: []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1"}}}); !errors.Is(err, ErrBitBrowserOffline) {
		t.Fatalf("SubmitScan(no observed_at) error = %v", err)
	}
}

func TestBindBitAccountTransitions(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")

	bound, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-a", ObservedAt: testNow})
	if err != nil {
		t.Fatalf("first bind error = %v", err)
	}
	if bound.MainUserID != "main-user-a" || bound.BoundAt == nil || bound.LastVerifiedAt == nil {
		t.Fatalf("bound = %#v", bound)
	}
	if store.bindings[actor.ID].MainUserID != "main-user-a" {
		t.Fatal("first bind was not stored")
	}

	later := testNow.Add(time.Minute)
	laterService := newTestServiceAt(store, later)
	verified, err := laterService.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-a", ObservedAt: later})
	if err != nil {
		t.Fatalf("verify error = %v", err)
	}
	if !verified.BoundAt.Equal(testNow) || !verified.LastVerifiedAt.Equal(later) {
		t.Fatalf("verify changed bound_at: %#v", verified)
	}

	_, err = laterService.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-b", ObservedAt: later})
	if !errors.Is(err, ErrBitMainUserMismatch) {
		t.Fatalf("mismatch error = %v, want ErrBitMainUserMismatch", err)
	}
	if got := store.bindings[actor.ID].MainUserID; got != "main-user-a" {
		t.Fatalf("mismatch silently changed binding to %q", got)
	}

	rebound, err := laterService.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-b", ObservedAt: later, Rebind: true})
	if err != nil {
		t.Fatalf("rebind error = %v", err)
	}
	if rebound.MainUserID != "main-user-b" || !rebound.BoundAt.Equal(later) {
		t.Fatalf("rebound = %#v", rebound)
	}
	if got := store.bindings[actor.ID].MainUserID; got != "main-user-b" {
		t.Fatalf("rebind stored %q, want main-user-b", got)
	}

	actions := make([]string, 0, len(store.audits))
	for _, event := range store.audits {
		actions = append(actions, event.Action)
	}
	want := []string{model.AuditMainAccountBind, model.AuditMainAccountVerify, model.AuditMainAccountRebind}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("binding audits = %v, want %v", actions, want)
	}
}

func TestRebindKeepsOtherMainProfilesAndSwitchingBackRestoresThem(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-a", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	scan, err := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-a",
		ObservedAt: testNow,
		Profiles:   []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-a", Name: "旧主窗口"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, scan.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-b", ObservedAt: testNow, Rebind: true}); err != nil {
		t.Fatal(err)
	}

	visible, err := service.ListProfiles(actor, 0, ProfileListFilter{})
	if err != nil {
		t.Fatalf("ListProfiles(after rebind) error = %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("old-main profiles still visible after rebind: %#v", visible)
	}
	if got := len(store.profileList(actor.ID)); got != 1 {
		t.Fatalf("rebind deleted old-main profiles: %d rows", got)
	}

	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-a", ObservedAt: testNow, Rebind: true}); err != nil {
		t.Fatal(err)
	}
	visible, err = service.ListProfiles(actor, 0, ProfileListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].BitProfileID != "p1" {
		t.Fatalf("switching back did not restore the old profile: %#v", visible)
	}
}

func TestListsRequireBindingForSelfButNotForSeniorPeeking(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	operator := profileActor("user-1")

	if _, err := service.ListProfiles(operator, 0, ProfileListFilter{}); !errors.Is(err, ErrBitNotBound) {
		t.Fatalf("ListProfiles(unbound self) error = %v", err)
	}
	teamA := identityservice.TeamID(10)
	senior := identityservice.PublicUser{ID: 9, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamA}
	store.profiles["p"] = BrowserProfile{ID: "p", UserID: 2, TeamID: &teamA, MainUserID: "main-user-2", LocalStatus: ProfileActive}
	store.bindings[2] = BitAccountBinding{UserID: 2, MainUserID: "main-user-2", Status: BitAccountBound}
	profiles, err := service.ListProfiles(senior, 0, ProfileListFilter{})
	if err != nil || len(profiles) != 1 {
		t.Fatalf("ListProfiles(senior) profiles=%#v error=%v", profiles, err)
	}
}

func TestSeniorCanViewButCannotMutateSameTeamProfileCloudRecord(t *testing.T) {
	store := newMemoryStore()
	teamA := identityservice.TeamID(10)
	teamB := identityservice.TeamID(20)
	bindMain(store, 2, "main-user-2")
	bindMain(store, 3, "main-user-3")
	store.profiles["same-team"] = BrowserProfile{ID: "same-team", UserID: 2, TeamID: &teamA, MainUserID: "main-user-2", LocalStatus: ProfileActive}
	store.profiles["other-team"] = BrowserProfile{ID: "other-team", UserID: 3, TeamID: &teamB, MainUserID: "main-user-3", LocalStatus: ProfileActive}
	service := newTestService(store)
	actor := identityservice.PublicUser{ID: 1, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamA}

	profiles, err := service.ListProfiles(actor, 0, ProfileListFilter{})
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
			bindMain(store, 1, "main-user-1")
			bindMain(store, 2, "main-user-2")
			bindMain(store, 3, "main-user-3")
			store.profiles["own"] = BrowserProfile{ID: "own", UserID: 1, TeamID: &teamA, MainUserID: "main-user-1"}
			store.profiles["same-team"] = BrowserProfile{ID: "same-team", UserID: 2, TeamID: &teamA, MainUserID: "main-user-2"}
			store.profiles["other-team"] = BrowserProfile{ID: "other-team", UserID: 3, TeamID: &teamB, MainUserID: "main-user-3"}
			profiles, err := newTestService(store).ListProfiles(tc.actor, 0, ProfileListFilter{})
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
			bindMain(store, 2, "main-user-2")
			store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, MainUserID: "main-user-2", BusinessStatus: ProfileBusinessDisabled, LocalStatus: ProfileLocalMissing}
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
	bindMain(store, 2, "main-user-2")
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, MainUserID: "main-user-2", BusinessStatus: ProfileBusinessEnabled, LocalStatus: ProfileActive}
	err := newTestService(store).DeleteProfile(profileActor("user-2"), "profile")
	if !errors.Is(err, ErrProfileNotDisabled) {
		t.Fatalf("DeleteProfile() error=%v, want ErrProfileNotDisabled", err)
	}
	if _, ok := store.profiles["profile"]; !ok {
		t.Fatal("enabled profile was removed despite rejection")
	}
}

func TestOutOfMainScopeProfileIsNotFound(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(2)
	bindMain(store, 2, "main-user-2")
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, MainUserID: "main-user-old", BusinessStatus: ProfileBusinessEnabled, LocalStatus: ProfileActive}
	service := newTestService(store)

	if _, err := service.GetActiveProfile(profileActor("user-2"), "profile"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("GetActiveProfile(out of scope) error=%v, want ErrProfileNotFound", err)
	}
	if _, err := service.UpdateProfile(profileActor("user-2"), "profile", nil, nil); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("UpdateProfile(out of scope) error=%v, want ErrProfileNotFound", err)
	}
	if err := service.DeleteProfile(profileActor("user-2"), "profile"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("DeleteProfile(out of scope) error=%v, want ErrProfileNotFound", err)
	}
}

func TestServiceUpdateProfileCloudRemarkAndBusinessStatus(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(2)
	bindMain(store, 2, "main-user-2")
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, MainUserID: "main-user-2", BusinessStatus: ProfileBusinessEnabled, Remark: "比特备注", CloudRemark: "旧备注"}
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
	bindMain(store, 2, "main-user-shared")
	bindMain(store, 3, "main-user-shared")
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, MainUserID: "main-user-shared", BitProfileID: "bit-profile-1", LocalStatus: ProfileActive}
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

func TestAssignProfileOwnerRejectsUnboundOrMismatchedTarget(t *testing.T) {
	teamID := identityservice.TeamID(10)
	admin := identityservice.PublicUser{ID: 99, Role: identityservice.RoleAdmin, Status: identityservice.UserStatusEnabled}
	operator := identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID}
	senior := identityservice.PublicUser{ID: 4, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID}

	store := newMemoryStore()
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, MainUserID: "main-user-a", LocalStatus: ProfileActive}
	service := newTestService(store)
	if _, err := service.AssignProfileOwner(operator, "profile", operator); !errors.Is(err, ErrForbidden) {
		t.Fatalf("AssignProfileOwner(non-admin) error=%v, want ErrForbidden", err)
	}
	if _, err := service.AssignProfileOwner(admin, "profile", senior); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AssignProfileOwner(senior target) error=%v, want ErrInvalidInput", err)
	}
	if _, err := service.AssignProfileOwner(admin, "profile", operator); !errors.Is(err, ErrBitNotBound) {
		t.Fatalf("AssignProfileOwner(unbound target) error=%v, want ErrBitNotBound", err)
	}

	bindMain(store, 3, "main-user-b")
	if _, err := service.AssignProfileOwner(admin, "profile", operator); !errors.Is(err, ErrBitMainUserMismatch) {
		t.Fatalf("AssignProfileOwner(cross-main) error=%v, want ErrBitMainUserMismatch", err)
	}

	store.profiles["profile-same-main"] = BrowserProfile{ID: "profile-same-main", UserID: 2, TeamID: &teamID, MainUserID: "main-user-b", LocalStatus: ProfileActive}
	store.profileAccountRefs["profile-same-main"] = true
	if _, err := service.AssignProfileOwner(admin, "profile-same-main", operator); !errors.Is(err, ErrProfileReferenced) {
		t.Fatalf("AssignProfileOwner(referenced) error=%v, want ErrProfileReferenced", err)
	}
}

func TestSeniorCanReviewSameTeamScanButCannotConfirmIt(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(10)
	store.scans["scan-same-team"] = ProfileScan{ID: "scan-same-team", UserID: 2, TeamID: &teamID, Status: ScanReady, ExpiresAt: testNow.Add(time.Hour)}
	service := newTestService(store)
	actor := identityservice.PublicUser{ID: 1, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID}

	if _, err := service.GetScan(actor, "scan-same-team"); err != nil {
		t.Fatalf("GetScan(same team) error=%v", err)
	}
	if _, err := service.ConfirmScan(actor, "scan-same-team"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ConfirmScan(other owner) error=%v, want ErrForbidden", err)
	}
}

func TestConfirmScanAppliesProfilesWithoutChangingBinding(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	bound, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1", ObservedAt: testNow})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		ObservedAt: testNow,
		Profiles: []ProfileInput{
			{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "窗口一", GroupID: "g1", GroupName: "运营组"},
			{BitProfileID: "p2", ProfileUserID: "bit-user-2", MainUserID: "main-user-1", Name: "窗口二"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitScan() error = %v", err)
	}

	confirmed, err := service.ConfirmScan(actor, scan.ID)
	if err != nil {
		t.Fatalf("ConfirmScan() error = %v", err)
	}
	if confirmed.Status != ScanConfirmed || confirmed.ConfirmedAt == nil {
		t.Fatalf("confirmed scan = %#v", confirmed)
	}
	binding := store.bindings[actor.ID]
	if binding.MainUserID != bound.MainUserID || !binding.LastVerifiedAt.Equal(*bound.LastVerifiedAt) {
		t.Fatalf("confirm rewrote the binding: %#v", binding)
	}
	if len(store.audits) != 2 || store.audits[0].Action != model.AuditMainAccountBind || store.audits[1].Action != "bitbrowser.profile_scan.confirm" {
		t.Fatalf("audits = %#v", store.audits)
	}
	profiles := store.profileList(actor.ID)
	if len(profiles) != 2 || profiles[0].BitProfileID != "p1" || profiles[0].ProfileUserID != "bit-user-1" || profiles[1].ProfileUserID != "bit-user-2" {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestConfirmScanRejectsScanFromPreviousMainAccount(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-a", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	scan, err := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-a",
		ObservedAt: testNow,
		Profiles:   []ProfileInput{{BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-b", ObservedAt: testNow, Rebind: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, scan.ID); !errors.Is(err, ErrBitMainUserMismatch) {
		t.Fatalf("ConfirmScan(previous main) error = %v, want ErrBitMainUserMismatch", err)
	}
	if len(store.profiles) != 0 {
		t.Fatalf("stale scan wrote profiles: %#v", store.profiles)
	}
}

func TestConfirmScanRejectsUnboundExpiredAndRepeated(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	if _, err := service.ConfirmScan(actor, "missing"); !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("ConfirmScan(missing) error = %v", err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{MainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	scan, _ := service.SubmitScan(actor, SnapshotInput{
		MainUserID: "main-user-1",
		ObservedAt: testNow,
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
	bindMain(store, 1, "main-user-1")
	store.profiles["profile-old-1"] = BrowserProfile{ID: "profile-old-1", UserID: 1, BitProfileID: "p1", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "旧名称", BitStatus: "0", ProxyType: "http", ProxyHost: "1.1.1.1", ProxyPort: 8080, Remark: "旧备注", LocalStatus: ProfileActive}
	store.profiles["profile-old-2"] = BrowserProfile{ID: "profile-old-2", UserID: 1, BitProfileID: "p2", ProfileUserID: "bit-user-1", MainUserID: "main-user-1", Name: "即将缺失", LocalStatus: ProfileActive}
	store.profiles["profile-old-main"] = BrowserProfile{ID: "profile-old-main", UserID: 1, BitProfileID: "p9", ProfileUserID: "bit-user-1", MainUserID: "main-user-old", Name: "旧主账号窗口", LocalStatus: ProfileActive}
	service := newTestService(store)

	scan, err := service.SubmitScan(profileActor("user-1"), SnapshotInput{
		MainUserID: "main-user-1",
		ObservedAt: testNow,
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
	// 缺失窗口只标记，不删除；其他主账号下的行不受本次扫描影响。
	if byBitID["p2"].ID == "" {
		t.Fatal("missing window was deleted instead of marked")
	}
	if byBitID["p9"].LocalStatus != ProfileActive {
		t.Fatalf("scan touched a profile from another main account: %#v", byBitID["p9"])
	}
}

func profileActor(userID string) identityservice.PublicUser {
	parsed, _ := strconv.ParseInt(strings.TrimPrefix(userID, "user-"), 10, 64)
	teamID := identityservice.TeamID(parsed)
	return identityservice.PublicUser{ID: identityservice.UserID(parsed), Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a"}}
}

func newTestService(store Store) *Service {
	return newTestServiceAt(store, testNow)
}

func newTestServiceAt(store Store, now time.Time) *Service {
	sequence := 0
	return NewService(store,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func(prefix string) string {
			sequence++
			return prefix + "-test-" + string(rune('0'+sequence))
		}),
	)
}

func bindMain(store *memoryStore, userID identityservice.UserID, mainUserID string) {
	boundAt := testNow
	store.bindings[userID] = BitAccountBinding{UserID: userID, MainUserID: mainUserID, Status: BitAccountBound, BoundAt: &boundAt, LastVerifiedAt: &boundAt}
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

func (s *memoryStore) BindBitAccount(userID identityservice.UserID, mainUserID string, rebind bool, at time.Time) (BitAccountBinding, error) {
	current, exists := s.bindings[userID]
	action, result := model.AuditMainAccountBind, "bound"
	firstBoundAt := at
	previous := ""
	switch {
	case !exists || current.MainUserID == "":
	case current.MainUserID == mainUserID:
		action, result = model.AuditMainAccountVerify, "verified"
		if current.BoundAt != nil {
			firstBoundAt = *current.BoundAt
		}
	case !rebind:
		return BitAccountBinding{}, ErrBitMainUserMismatch
	default:
		action, result, previous = model.AuditMainAccountRebind, "rebound", current.MainUserID
	}
	verifiedAt := at
	binding := BitAccountBinding{UserID: userID, MainUserID: mainUserID, Status: BitAccountBound, BoundAt: &firstBoundAt, LastVerifiedAt: &verifiedAt}
	s.bindings[userID] = binding
	summary := map[string]string{"main_user_id": mainUserID, "result": result}
	if previous != "" {
		summary["previous_main_user_id"] = previous
	}
	s.audits = append(s.audits, identityservice.AuditEvent{Action: action, TargetType: "user", TargetID: strconv.FormatInt(int64(userID), 10), Summary: summary, CreatedAt: at})
	return binding, nil
}

func (s *memoryStore) ListProfiles(userID identityservice.UserID, filter ProfileListFilter) ([]BrowserProfile, error) {
	profiles := s.visibleProfiles()
	if userID > 0 {
		filtered := profiles[:0]
		for _, profile := range profiles {
			if profile.UserID == userID {
				filtered = append(filtered, profile)
			}
		}
		profiles = filtered
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].BitProfileID < profiles[j].BitProfileID })
	return profiles, nil
}

func (s *memoryStore) GetProfile(profileID string) (BrowserProfile, bool, error) {
	profile, found := s.profiles[profileID]
	return profile, found, nil
}

func (s *memoryStore) GetProfileInScope(profileID string) (BrowserProfile, bool, error) {
	profile, found := s.profiles[profileID]
	if !found || !s.inScope(profile) {
		return BrowserProfile{}, false, nil
	}
	return profile, true, nil
}

func (s *memoryStore) CreateScan(scan ProfileScan) error {
	s.scans[scan.ID] = cloneScan(scan)
	return nil
}

func (s *memoryStore) FindScan(scanID string) (ProfileScan, bool, error) {
	scan, ok := s.scans[scanID]
	return cloneScan(scan), ok, nil
}

func (s *memoryStore) ApplyScan(scan ProfileScan, at time.Time) error {
	binding, bound := s.bindings[scan.UserID]
	if !bound {
		return ErrBitNotBound
	}
	if binding.MainUserID != scan.MainUserID {
		return ErrBitMainUserMismatch
	}
	seen := map[string]struct{}{}
	for _, candidate := range scan.Profiles {
		seen[candidate.BitProfileID] = struct{}{}
		if existing, ok := s.findByScope(candidate.UserID, candidate.MainUserID, candidate.BitProfileID); ok {
			candidate.ID = existing.ID
			candidate.CreatedAt = existing.CreatedAt
			candidate.Remark = existing.Remark
			candidate.CloudRemark = existing.CloudRemark
			candidate.BusinessStatus = existing.BusinessStatus
		}
		candidate.LocalStatus = ProfileActive
		candidate.LastSyncedAt = at
		s.profiles[candidate.ID] = candidate
	}
	for id, profile := range s.profiles {
		if profile.UserID != scan.UserID || profile.MainUserID != scan.MainUserID {
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

func (s *memoryStore) visibleProfiles() []BrowserProfile {
	result := make([]BrowserProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		if s.inScope(profile) {
			result = append(result, profile)
		}
	}
	return result
}

func (s *memoryStore) inScope(profile BrowserProfile) bool {
	binding, bound := s.bindings[profile.UserID]
	return bound && binding.MainUserID == profile.MainUserID
}

func (s *memoryStore) findByScope(userID identityservice.UserID, mainUserID, bitProfileID string) (BrowserProfile, bool) {
	for _, profile := range s.profiles {
		if profile.UserID == userID && profile.MainUserID == mainUserID && profile.BitProfileID == bitProfileID {
			return profile, true
		}
	}
	return BrowserProfile{}, false
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
