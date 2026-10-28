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
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatalf("BindBitAccount() error = %v", err)
	}

	scan, err := service.SubmitScan(actor, SnapshotInput{
		BitMainUserID: "main-user-1",
		ObservedAt:    testNow,
		Profiles:      []ProfileInput{{BitProfileID: "bit-profile-1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1", BitName: "窗口一", BitSeq: 1}},
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
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatalf("BindBitAccount() error = %v", err)
	}
	cases := []SnapshotInput{
		{BitMainUserID: "main-user-1", ObservedAt: testNow},
		{BitMainUserID: "", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: ""}}},
		{BitMainUserID: "main-user-1", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-2"}}},
		{BitMainUserID: "main-user-1", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "p1", BitUserID: "", BitMainUserID: "main-user-1"}}},
		{BitMainUserID: "main-user-1", ObservedAt: testNow, Profiles: []ProfileInput{{BitProfileID: "", BitUserID: "bit-user-1", BitMainUserID: "main-user-1"}}},
	}
	for _, input := range cases {
		if _, err := service.SubmitScan(actor, input); !errors.Is(err, ErrIdentityUnverifiable) {
			t.Fatalf("SubmitScan(%#v) error = %v", input, err)
		}
	}
}

func TestSubmitScanRequiresBindingFreshObservationAndMatchingMain(t *testing.T) {
	actor := profileActor("user-1")
	profiles := []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1"}}

	unbound := newMemoryStore()
	if _, err := newTestService(unbound).SubmitScan(actor, SnapshotInput{BitMainUserID: "main-user-1", ObservedAt: testNow, Profiles: profiles}); !errors.Is(err, ErrBitNotBound) {
		t.Fatalf("SubmitScan(unbound) error = %v", err)
	}

	stale := newMemoryStore()
	bound := newTestService(stale)
	if _, err := bound.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	if _, err := bound.SubmitScan(actor, SnapshotInput{BitMainUserID: "main-user-1", ObservedAt: testNow.Add(-3 * time.Minute), Profiles: profiles}); !errors.Is(err, ErrBitBrowserOffline) {
		t.Fatalf("SubmitScan(stale) error = %v", err)
	}
	if _, err := bound.SubmitScan(actor, SnapshotInput{BitMainUserID: "main-user-2", ObservedAt: testNow, Profiles: profiles}); !errors.Is(err, ErrBitMainUserMismatch) {
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
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1"}); !errors.Is(err, ErrBitBrowserOffline) {
		t.Fatalf("BindBitAccount(no observed_at) error = %v", err)
	}
	if _, err := service.SubmitScan(actor, SnapshotInput{BitMainUserID: "main-user-1", Profiles: []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1"}}}); !errors.Is(err, ErrBitBrowserOffline) {
		t.Fatalf("SubmitScan(no observed_at) error = %v", err)
	}
}

func TestGetBitAccountReturnsBindingOrAbsence(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)

	if _, found, err := service.GetBitAccount(1); err != nil || found {
		t.Fatalf("unbound GetBitAccount = found %v, err %v", found, err)
	}

	bindMain(store, 1, "main-user-a")
	binding, found, err := service.GetBitAccount(1)
	if err != nil || !found {
		t.Fatalf("bound GetBitAccount = found %v, err %v", found, err)
	}
	if binding.BitMainUserID != "main-user-a" {
		t.Fatalf("main_user_id = %q", binding.BitMainUserID)
	}
	if _, _, err := service.GetBitAccount(0); !errors.Is(err, ErrForbidden) {
		t.Fatalf("zero user error = %v", err)
	}
}

func TestBindBitAccountTransitions(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")

	bound, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-a", ObservedAt: testNow})
	if err != nil {
		t.Fatalf("first bind error = %v", err)
	}
	if bound.BitMainUserID != "main-user-a" || bound.BoundAt == nil || bound.LastVerifiedAt == nil {
		t.Fatalf("bound = %#v", bound)
	}
	if store.bindings[actor.ID].BitMainUserID != "main-user-a" {
		t.Fatal("first bind was not stored")
	}

	later := testNow.Add(time.Minute)
	laterService := newTestServiceAt(store, later)
	verified, err := laterService.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-a", ObservedAt: later})
	if err != nil {
		t.Fatalf("verify error = %v", err)
	}
	if !verified.BoundAt.Equal(testNow) || !verified.LastVerifiedAt.Equal(later) {
		t.Fatalf("verify changed bound_at: %#v", verified)
	}

	_, err = laterService.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-b", ObservedAt: later})
	if !errors.Is(err, ErrBitMainUserMismatch) {
		t.Fatalf("mismatch error = %v, want ErrBitMainUserMismatch", err)
	}
	if got := store.bindings[actor.ID].BitMainUserID; got != "main-user-a" {
		t.Fatalf("mismatch silently changed binding to %q", got)
	}

	rebound, err := laterService.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-b", ObservedAt: later, Rebind: true})
	if err != nil {
		t.Fatalf("rebind error = %v", err)
	}
	if rebound.BitMainUserID != "main-user-b" || !rebound.BoundAt.Equal(later) {
		t.Fatalf("rebound = %#v", rebound)
	}
	if got := store.bindings[actor.ID].BitMainUserID; got != "main-user-b" {
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
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-a", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	scan, err := service.SubmitScan(actor, SnapshotInput{
		BitMainUserID: "main-user-a",
		ObservedAt:    testNow,
		Profiles:      []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-a", BitName: "旧主窗口"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, scan.ID, []string{"p1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-b", ObservedAt: testNow, Rebind: true}); err != nil {
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

	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-a", ObservedAt: testNow, Rebind: true}); err != nil {
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
	store.profiles["p"] = BrowserProfile{ID: "p", UserID: 2, TeamID: &teamA, BitMainUserID: "main-user-2", LocalState: ProfileActive}
	store.bindings[2] = BitAccountBinding{UserID: 2, BitMainUserID: "main-user-2", Status: BitAccountBound}
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
	store.profiles["same-team"] = BrowserProfile{ID: "same-team", UserID: 2, TeamID: &teamA, BitMainUserID: "main-user-2", LocalState: ProfileActive}
	store.profiles["other-team"] = BrowserProfile{ID: "other-team", UserID: 3, TeamID: &teamB, BitMainUserID: "main-user-3", LocalState: ProfileActive}
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
			store.profiles["own"] = BrowserProfile{ID: "own", UserID: 1, TeamID: &teamA, BitMainUserID: "main-user-1"}
			store.profiles["same-team"] = BrowserProfile{ID: "same-team", UserID: 2, TeamID: &teamA, BitMainUserID: "main-user-2"}
			store.profiles["other-team"] = BrowserProfile{ID: "other-team", UserID: 3, TeamID: &teamB, BitMainUserID: "main-user-3"}
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
			store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BitMainUserID: "main-user-2", Enabled: false, LocalState: ProfileLocalMissing}
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
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BitMainUserID: "main-user-2", Enabled: true, LocalState: ProfileActive}
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
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BitMainUserID: "main-user-old", Enabled: true, LocalState: ProfileActive}
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

func TestServiceUpdateProfileRemarkAndBusinessStatus(t *testing.T) {
	store := newMemoryStore()
	teamID := identityservice.TeamID(2)
	bindMain(store, 2, "main-user-2")
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, TeamID: &teamID, BitMainUserID: "main-user-2", Enabled: true, BitRemark: "比特备注", Remark: "旧备注"}
	service := newTestService(store)

	cloudBitRemark := "新备注"
	disabled := false
	updated, err := service.UpdateProfile(profileActor("user-2"), "profile", &cloudBitRemark, &disabled)
	if err != nil {
		t.Fatalf("UpdateProfile() error=%v", err)
	}
	if updated.Remark != "新备注" || updated.BitRemark != "比特备注" || updated.Enabled {
		t.Fatalf("updated=%+v", updated)
	}
	if _, ok := store.profiles["profile"]; !ok {
		t.Fatal("profile removed by update")
	}
}

func TestScanReportsWindowsOwnedByAnotherOperatorAndNeverAdoptsThem(t *testing.T) {
	store := newMemoryStore()
	bindMain(store, 1, "main-user-shared")
	bindMain(store, 2, "main-user-shared")
	teamID := identityservice.TeamID(2)
	store.profiles["profile-owned-elsewhere"] = BrowserProfile{ID: "profile-owned-elsewhere", UserID: 2, TeamID: &teamID, BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-shared", BitName: "同事的窗口", LocalState: ProfileActive}
	service := newTestService(store)

	scan, err := service.SubmitScan(profileActor("user-1"), SnapshotInput{
		BitMainUserID: "main-user-shared",
		ObservedAt:    testNow,
		Profiles: []ProfileInput{
			{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-shared", BitName: "同事的窗口"},
			{BitProfileID: "p2", BitUserID: "bit-user-2", BitMainUserID: "main-user-shared", BitName: "自己的窗口"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitScan() error = %v", err)
	}
	conflicts := map[string]string{}
	for _, profile := range scan.Profiles {
		conflicts[profile.BitProfileID] = profile.OwnershipConflict
	}
	if conflicts["p1"] == "" {
		t.Fatalf("p1 conflict not reported: %#v", scan.Profiles)
	}
	if conflicts["p2"] != "" {
		t.Fatalf("p2 reported a conflict: %#v", scan.Profiles)
	}

	if _, err := service.ConfirmScan(profileActor("user-1"), scan.ID, []string{"p1", "p2"}); err != nil {
		t.Fatalf("ConfirmScan() error = %v", err)
	}
	mine := store.profileList(1)
	if len(mine) != 1 || mine[0].BitProfileID != "p2" {
		t.Fatalf("scan adopted a window it does not own: %#v", mine)
	}
	if stored := store.profiles["profile-owned-elsewhere"]; stored.UserID != 2 || stored.BitName != "同事的窗口" {
		t.Fatalf("existing owner row was changed: %#v", stored)
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
	if _, err := service.ConfirmScan(actor, "scan-same-team", nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ConfirmScan(other owner) error=%v, want ErrForbidden", err)
	}
}

func TestConfirmScanAppliesProfilesWithoutChangingBinding(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	bound, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1", ObservedAt: testNow})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := service.SubmitScan(actor, SnapshotInput{
		BitMainUserID: "main-user-1",
		ObservedAt:    testNow,
		Profiles: []ProfileInput{
			{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1", BitName: "窗口一", BitGroupID: "g1", BitGroupName: "运营组"},
			{BitProfileID: "p2", BitUserID: "bit-user-2", BitMainUserID: "main-user-1", BitName: "窗口二"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitScan() error = %v", err)
	}

	confirmed, err := service.ConfirmScan(actor, scan.ID, []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("ConfirmScan() error = %v", err)
	}
	if confirmed.Status != ScanConfirmed || confirmed.ConfirmedAt == nil {
		t.Fatalf("confirmed scan = %#v", confirmed)
	}
	binding := store.bindings[actor.ID]
	if binding.BitMainUserID != bound.BitMainUserID || !binding.LastVerifiedAt.Equal(*bound.LastVerifiedAt) {
		t.Fatalf("confirm rewrote the binding: %#v", binding)
	}
	if len(store.audits) != 2 || store.audits[0].Action != model.AuditMainAccountBind || store.audits[1].Action != "bitbrowser.profile_scan.confirm" {
		t.Fatalf("audits = %#v", store.audits)
	}
	profiles := store.profileList(actor.ID)
	if len(profiles) != 2 || profiles[0].BitProfileID != "p1" || profiles[0].BitUserID != "bit-user-1" || profiles[1].BitUserID != "bit-user-2" {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestConfirmScanRejectsScanFromPreviousMainAccount(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(store)
	actor := profileActor("user-1")
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-a", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	scan, err := service.SubmitScan(actor, SnapshotInput{
		BitMainUserID: "main-user-a",
		ObservedAt:    testNow,
		Profiles:      []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-b", ObservedAt: testNow, Rebind: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, scan.ID, nil); !errors.Is(err, ErrBitMainUserMismatch) {
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
	if _, err := service.ConfirmScan(actor, "missing", nil); !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("ConfirmScan(missing) error = %v", err)
	}
	if _, err := service.BindBitAccount(actor.ID, BitAccountBindInput{BitMainUserID: "main-user-1", ObservedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	scan, _ := service.SubmitScan(actor, SnapshotInput{
		BitMainUserID: "main-user-1",
		ObservedAt:    testNow,
		Profiles:      []ProfileInput{{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1"}},
	})

	if _, err := service.ConfirmScan(profileActor("user-2"), scan.ID, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other-user confirm error = %v", err)
	}
	expired := store.scans[scan.ID]
	expired.ExpiresAt = expired.CreatedAt.Add(-time.Second)
	store.scans[scan.ID] = expired
	if _, err := service.ConfirmScan(actor, scan.ID, nil); !errors.Is(err, ErrScanExpired) {
		t.Fatalf("expired confirm error = %v", err)
	}

	expired.ExpiresAt = expired.CreatedAt.Add(time.Hour)
	store.scans[scan.ID] = expired
	if _, err := service.ConfirmScan(actor, scan.ID, []string{"p1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmScan(actor, scan.ID, nil); !errors.Is(err, ErrScanNotReady) {
		t.Fatalf("repeated confirm error = %v", err)
	}
}

func TestScanDiffAndConfirmationUpdateAndMarkMissing(t *testing.T) {
	store := newMemoryStore()
	bindMain(store, 1, "main-user-1")
	store.profiles["profile-old-1"] = BrowserProfile{ID: "profile-old-1", UserID: 1, BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1", BitName: "旧名称", BitProfileStatus: "0", BitProxyType: "http", BitProxyHost: "1.1.1.1", BitProxyPort: 8080, Remark: "旧备注", LocalState: ProfileActive}
	store.profiles["profile-old-2"] = BrowserProfile{ID: "profile-old-2", UserID: 1, BitProfileID: "p2", BitUserID: "bit-user-1", BitMainUserID: "main-user-1", BitName: "即将缺失", LocalState: ProfileActive}
	store.profiles["profile-old-main"] = BrowserProfile{ID: "profile-old-main", UserID: 1, BitProfileID: "p9", BitUserID: "bit-user-1", BitMainUserID: "main-user-old", BitName: "旧主账号窗口", LocalState: ProfileActive}
	service := newTestService(store)

	scan, err := service.SubmitScan(profileActor("user-1"), SnapshotInput{
		BitMainUserID: "main-user-1",
		ObservedAt:    testNow,
		Profiles: []ProfileInput{
			{BitProfileID: "p1", BitUserID: "bit-user-1", BitMainUserID: "main-user-1", BitName: "新名称", BitProfileStatus: "1", BitProxyType: "socks5", BitProxyHost: "2.2.2.2", BitProxyPort: 1080, BitRemark: "新备注"},
			{BitProfileID: "p3", BitUserID: "bit-user-2", BitMainUserID: "main-user-1", BitName: "新增"},
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
	if strings.Join(changed.Fields, ",") != "bit_name,bit_remark,bit_proxy_type,bit_proxy_host,bit_proxy_port" {
		t.Fatalf("changed fields = %v", changed.Fields)
	}
	if _, err := service.ConfirmScan(profileActor("user-1"), scan.ID, []string{"p3"}); err != nil {
		t.Fatal(err)
	}
	profiles := store.profileList(1)
	byBitID := map[string]BrowserProfile{}
	for _, item := range profiles {
		byBitID[item.BitProfileID] = item
	}
	if byBitID["p1"].BitName != "新名称" || byBitID["p1"].BitProxyHost != "2.2.2.2" || byBitID["p1"].BitRemark != "新备注" || byBitID["p2"].LocalState != ProfileLocalMissing || byBitID["p3"].LocalState != ProfileActive {
		t.Fatalf("profiles after apply = %#v", byBitID)
	}
	// 缺失窗口只标记，不删除；其他主账号下的行不受本次扫描影响。
	if byBitID["p2"].ID == "" {
		t.Fatal("missing window was deleted instead of marked")
	}
	if byBitID["p9"].LocalState != ProfileActive {
		t.Fatalf("scan touched a profile from another main account: %#v", byBitID["p9"])
	}
}

func profileActor(userID string) identityservice.PublicUser {
	parsed, _ := strconv.ParseInt(strings.TrimPrefix(userID, "user-"), 10, 64)
	teamID := identityservice.TeamID(parsed)
	return identityservice.PublicUser{ID: identityservice.UserID(parsed), Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: &teamID, CategoryIDs: []identityservice.CategoryID{1}}
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
	store.bindings[userID] = BitAccountBinding{UserID: userID, BitMainUserID: mainUserID, Status: BitAccountBound, BoundAt: &boundAt, LastVerifiedAt: &boundAt}
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
	case !exists || current.BitMainUserID == "":
	case current.BitMainUserID == mainUserID:
		action, result = model.AuditMainAccountVerify, "verified"
		if current.BoundAt != nil {
			firstBoundAt = *current.BoundAt
		}
	case !rebind:
		return BitAccountBinding{}, ErrBitMainUserMismatch
	default:
		action, result, previous = model.AuditMainAccountRebind, "rebound", current.BitMainUserID
	}
	verifiedAt := at
	binding := BitAccountBinding{UserID: userID, BitMainUserID: mainUserID, Status: BitAccountBound, BoundAt: &firstBoundAt, LastVerifiedAt: &verifiedAt}
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

func (s *memoryStore) ListProfileOwners(mainUserID string) (map[string]identityservice.UserID, error) {
	owners := map[string]identityservice.UserID{}
	for _, profile := range s.profiles {
		if profile.BitMainUserID != mainUserID {
			continue
		}
		if _, exists := owners[profile.BitProfileID]; !exists {
			owners[profile.BitProfileID] = profile.UserID
		}
	}
	return owners, nil
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

func (s *memoryStore) FindScanByUser(userID identityservice.UserID) (ProfileScan, bool, error) {
	for _, scan := range s.scans {
		if scan.UserID == userID {
			return cloneScan(scan), true, nil
		}
	}
	return ProfileScan{}, false, nil
}

func (s *memoryStore) DeleteScanByUser(userID identityservice.UserID) error {
	for id, scan := range s.scans {
		if scan.UserID == userID {
			delete(s.scans, id)
		}
	}
	return nil
}

func (s *memoryStore) FindScan(scanID string) (ProfileScan, bool, error) {
	scan, ok := s.scans[scanID]
	return cloneScan(scan), ok, nil
}

func (s *memoryStore) ApplyScan(scan ProfileScan, observedBitProfileIDs []string, at time.Time) error {
	binding, bound := s.bindings[scan.UserID]
	if !bound {
		return ErrBitNotBound
	}
	if binding.BitMainUserID != scan.BitMainUserID {
		return ErrBitMainUserMismatch
	}
	seen := map[string]struct{}{}
	for _, bitID := range observedBitProfileIDs {
		seen[bitID] = struct{}{}
	}
	for _, candidate := range scan.Profiles {
		if existing, ok := s.findByScope(candidate.UserID, candidate.BitMainUserID, candidate.BitProfileID); ok {
			candidate.ID = existing.ID
			candidate.CreatedAt = existing.CreatedAt
			candidate.Remark = existing.Remark
			candidate.Enabled = existing.Enabled
		}
		candidate.LocalState = ProfileActive
		candidate.LastSyncedAt = at
		s.profiles[candidate.ID] = candidate
	}
	for id, profile := range s.profiles {
		if profile.UserID != scan.UserID || profile.BitMainUserID != scan.BitMainUserID {
			continue
		}
		if _, ok := seen[profile.BitProfileID]; !ok && profile.LocalState != ProfileArchived {
			profile.LocalState = ProfileLocalMissing
			profile.LastSyncedAt = at
			s.profiles[id] = profile
		}
	}
	scan.Status = ScanConfirmed
	scan.ConfirmedAt = &at
	s.scans[scan.ID] = cloneScan(scan)
	s.audits = append(s.audits,
		identityservice.AuditEvent{Action: "bitbrowser.profile_scan.confirm", TargetType: "profile_sync_scan", TargetID: scan.ID, Summary: map[string]string{"main_user_id": scan.BitMainUserID}},
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

func (s *memoryStore) UpdateProfile(profileID string, remark *string, enabled *bool, at time.Time) error {
	profile, ok := s.profiles[profileID]
	if !ok {
		return ErrProfileNotFound
	}
	if remark != nil {
		profile.Remark = *remark
	}
	if enabled != nil {
		profile.Enabled = *enabled
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
	return bound && binding.BitMainUserID == profile.BitMainUserID
}

func (s *memoryStore) findByScope(userID identityservice.UserID, mainUserID, bitProfileID string) (BrowserProfile, bool) {
	for _, profile := range s.profiles {
		if profile.UserID == userID && profile.BitMainUserID == mainUserID && profile.BitProfileID == bitProfileID {
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
