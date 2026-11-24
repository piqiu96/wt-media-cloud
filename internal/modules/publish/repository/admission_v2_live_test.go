package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

func TestParkedAttemptsKeepWindowButReturnAutomaticSlot(t *testing.T) {
	s, _ := executionStore(t)
	now := time.Now().UTC()
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	for i := 0; i < 11; i++ {
		profileID, outputID := 37+i, 42+i
		if i >= 2 {
			if err := s.DB.Exec("INSERT INTO browser_profiles VALUES(?)", profileID).Error; err != nil {
				t.Fatal(err)
			}
		}
		if i >= 3 {
			if err := s.DB.Exec("INSERT INTO composite_outputs VALUES(?)", outputID).Error; err != nil {
				t.Fatal(err)
			}
		}
		task := task(fmt.Sprintf("task-%d", i), int64(outputID), 1)
		task.PlatformAccountUID = fmt.Sprintf("owner-account-%d", i)
		task.ResourceSnapshot = []byte(fmt.Sprintf(`{"profile_id":%d}`, profileID))
		task, err := s.Create(task)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Start(task, task.Revision, 9, now); err != nil {
			t.Fatal(err)
		}
	}
	claims := []*model.Claim{}
	for i := 0; i < 5; i++ {
		claim, err := s.ClaimWithCapacity(9, fmt.Sprintf("claim-%d", i), cap, now)
		if err != nil || claim == nil {
			t.Fatalf("initial claim %d: %+v %v", i, claim, err)
		}
		claims = append(claims, claim)
	}
	if none, err := s.ClaimWithCapacity(9, "third-phase", cap, now); err != nil || none != nil {
		t.Fatalf("automated capacity exceeded: %+v %v", none, err)
	}
	for _, claim := range claims {
		report := model.ExecutionReport{AttemptID: claim.Attempt.ID, Seq: 1, Stage: model.AttemptStageAwaitingHuman, SceneParked: true}
		ack, err := s.Report(claim.Task, 9, report, true, now)
		if err != nil || !ack.SceneParked {
			t.Fatalf("park: %+v %v", ack, err)
		}
		ack, err = s.Report(claim.Task, 9, report, true, now)
		if err != nil || !ack.SceneParked {
			t.Fatalf("park replay: %+v %v", ack, err)
		}
	}
	for i := 5; i < 10; i++ {
		claim, err := s.ClaimWithCapacity(9, fmt.Sprintf("claim-%d", i), cap, now)
		if err != nil || claim == nil {
			t.Fatalf("claim with reliable parked windows %d: %+v %v", i, claim, err)
		}
	}
	if none, err := s.ClaimWithCapacity(9, "window-cap", cap, now); err != nil || none != nil {
		t.Fatalf("window capacity exceeded: %+v %v", none, err)
	}
	var n int64
	if err := s.DB.Model(&model.ProfileUse{}).Where("released_at IS NULL").Count(&n).Error; err != nil || n != 10 {
		t.Fatalf("active windows %d %v", n, err)
	}
	parked, _ := s.Get(claims[0].Task.ID)
	if _, err := s.ResumeWithCapacity(parked, 9, model.HumanContinuation{AttemptID: claims[0].Attempt.ID, ExpectedRevision: parked.Revision}, cap, now); !errors.Is(err, repo.ErrCapacityWait) {
		t.Fatalf("original resume should wait on automatic slots: %v", err)
	}
}

func TestLostCheckClaimCanSettleWithOriginalKeyAcrossSessions(t *testing.T) {
	s, _, reviewing := reviewingPublication(t)
	now := time.Now().UTC()
	if err := s.SchedulePublicCheck(now); err != nil {
		t.Fatal(err)
	}
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	initial := model.CheckActor{UserID: 9, SessionID: "old-session", Current: true}
	entry := model.CheckClaimInput{RequestKey: "lost-response", Capacity: cap}
	got, err := s.ClaimPublicCheckV2(initial, entry, now)
	if err != nil || got == nil {
		t.Fatalf("claim: %+v %v", got, err)
	}
	fresh := model.CheckActor{UserID: 9, SessionID: "new-session", Current: true}
	settled, err := s.ClaimPublicCheckV2(fresh, model.CheckClaimInput{RequestKey: entry.RequestKey, SettleUnreceived: true}, now)
	ack, ok := settled.(model.PublicCheckReleaseAck)
	if err != nil || !ok || !ack.Released || ack.PublicationID != reviewing.ID {
		t.Fatalf("lost claim settlement: %#v %v", settled, err)
	}
	previous := initial
	previous.Current = false
	if _, err = s.ClaimPublicCheckV2(previous, entry, now); !errors.Is(err, repo.ErrAgentUpgrade) {
		t.Fatalf("non-current strict claim accepted: %v", err)
	}
	initial.Current = true
	replay, err := s.ClaimPublicCheckV2(initial, entry, now)
	if replayAck, ok := replay.(model.PublicCheckReleaseAck); err != nil || !ok || replayAck != ack {
		t.Fatalf("released key replay: %#v %v", replay, err)
	}
	cancelled, err := s.ClaimPublicCheckV2(previous, model.CheckClaimInput{RequestKey: "before-commit", SettleUnreceived: true}, now)
	if result, ok := cancelled.(model.CancelledCheckClaim); err != nil || !ok || result.ReservationState != "cancelled" {
		t.Fatalf("pre-commit fence: %#v %v", cancelled, err)
	}
	late, err := s.ClaimPublicCheckV2(initial, model.CheckClaimInput{RequestKey: "before-commit", Capacity: cap}, now.Add(time.Hour))
	if result, ok := late.(model.CancelledCheckClaim); err != nil || !ok || result.ReservationState != "cancelled" {
		t.Fatalf("late original key reopened work: %#v %v", late, err)
	}
}

func TestPublishAndOriginalCheckDoNotShareAnActualProfile(t *testing.T) {
	for _, checkFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(checkFirst), func(t *testing.T) {
			s, makeTask := ownedStore(t)
			c, _ := unresolvedPublicationFrom(t, s, makeTask(0))
			reviewing := resolveReviewingPublication(t, s, c)
			now := time.Now().UTC()
			if err := s.SchedulePublicCheck(now); err != nil {
				t.Fatal(err)
			}
			v := makeTask(1)
			if err := s.DB.Exec("UPDATE media_accounts SET browser_profile_id=37 WHERE id=?", v.MediaAccountID).Error; err != nil {
				t.Fatal(err)
			}
			var snapshot map[string]any
			if err := json.Unmarshal(v.ResourceSnapshot, &snapshot); err != nil {
				t.Fatal(err)
			}
			snapshot["browser_profile_id"] = "37"
			v.ResourceSnapshot, _ = json.Marshal(snapshot)
			if err := s.DB.Model(&v).Update("resource_snapshot_json", v.ResourceSnapshot).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.Start(v, v.Revision, 9, now); err != nil {
				t.Fatal(err)
			}
			cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
			actor := model.CheckActor{UserID: 9, SessionID: "current", Current: true}
			check := func() (any, error) {
				return s.ClaimPublicCheckV2(actor, model.CheckClaimInput{RequestKey: "check", Capacity: cap}, now)
			}
			publish := func() (*model.Claim, error) { return s.ClaimWithCapacity(9, "pub", cap, now) }
			if checkFirst {
				got, err := check()
				claim, ok := got.(model.PublicCheckClaim)
				if err != nil || !ok {
					t.Fatalf("check did not reserve: %#v %v", got, err)
				}
				if other, err := publish(); err != nil || other != nil {
					t.Fatalf("publish intruded: %+v %v", other, err)
				}
				// A contradictory ordinary observation leaves the exact check holder intact.
				r := model.PublicCheckReport{ReservationID: claim.ReservationID, ProfileDisposition: "closed", Generation: claim.Generation,
					Observation: model.ResultResolution{AttemptID: c.Attempt.ID, ExpectedRevision: claim.ExpectedRevision, Conclusion: model.StatusUncertain,
						Evidence: model.VerificationEvidence{PlatformUID: c.Attempt.Input.PlatformUID, Reason: "fixture", Basis: "actual scene close cannot be inferred", SceneClosed: false}}}
				if _, err = s.ObservePublication(reviewing, 9, r, now); err == nil {
					t.Fatal("contradictory closure evidence accepted")
				}
				fresh := model.CheckActor{UserID: 9, SessionID: "replacement", Current: true}
				release := model.PublicCheckRelease{ReleaseOnly: true, ReservationID: claim.ReservationID, AttemptID: c.Attempt.ID, Generation: claim.Generation, ProfileDisposition: "closed"}
				if ack, err := s.ReleasePublicCheck(fresh, reviewing.ID, release, now); err != nil || !ack.Released {
					t.Fatalf("fresh owner release: %+v %v", ack, err)
				}
				if next, err := publish(); err != nil || next == nil {
					t.Fatalf("publish after exact release: %+v %v", next, err)
				}
				if _, err = s.ReleasePublicCheck(fresh, reviewing.ID, release, now); err != nil {
					t.Fatal(err)
				}
				var held int64
				if err = s.DB.Model(&model.ProfileUse{}).Where("released_at IS NULL AND kind='publish'").Count(&held).Error; err != nil || held != 1 {
					t.Fatalf("old release affected successor: %d %v", held, err)
				}
			} else {
				if got, err := publish(); err != nil || got == nil {
					t.Fatalf("publish did not reserve: %+v %v", got, err)
				}
				if other, err := check(); err != nil || other != nil {
					t.Fatalf("check intruded: %#v %v", other, err)
				}
				var n int64
				if err := s.DB.Model(&model.ProfileUse{}).Where("kind='check'").Count(&n).Error; err != nil || n != 0 {
					t.Fatalf("busy check created a use row: %d %v", n, err)
				}
			}
		})
	}
}

func TestLegacyGateBlocksNewClaimsAndAllowsOriginalSettlement(t *testing.T) {
	s, makeTask := ownedStore(t)
	c, _ := unresolvedPublicationFrom(t, s, makeTask(0))
	reviewing := resolveReviewingPublication(t, s, c)
	now := time.Now().UTC()
	if err := s.SchedulePublicCheck(now); err != nil {
		t.Fatal(err)
	}
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	actor := model.CheckActor{UserID: 9, SessionID: "old", Current: true}
	got, err := s.ClaimPublicCheckV2(actor, model.CheckClaimInput{RequestKey: "original-check", Capacity: cap}, now)
	claim, ok := got.(model.PublicCheckClaim)
	if err != nil || !ok {
		t.Fatalf("original check: %#v %v", got, err)
	}
	second := makeTask(1)
	if _, err = s.Start(second, 1, 9, now); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.Exec("UPDATE publication_admission_cutover SET verified_at=NULL, verification_reference=NULL WHERE id=1").Error; err != nil {
		t.Fatal(err)
	}
	s.RequireAdmission = true
	if _, err = s.ClaimPublicCheck(9, now); !errors.Is(err, repo.ErrAgentUpgrade) {
		t.Fatalf("internal legacy checkout bypassed the V2 gate: %v", err)
	}
	if _, err = s.ClaimPublicCheckV2(actor, model.CheckClaimInput{RequestKey: "new-check", Capacity: cap}, now); !errors.Is(err, repo.ErrAdmissionClosed) {
		t.Fatalf("new check passed closed gate: %v", err)
	}
	if _, err = s.ClaimWithCapacity(9, "publish", cap, now); !errors.Is(err, repo.ErrAdmissionClosed) {
		t.Fatalf("new publish passed closed gate: %v", err)
	}
	replacement := model.CheckActor{UserID: 9, SessionID: "replacement", Current: true}
	release := model.PublicCheckRelease{ReleaseOnly: true, ReservationID: claim.ReservationID, AttemptID: c.Attempt.ID, Generation: claim.Generation, ProfileDisposition: "not_opened"}
	if ack, err := s.ReleasePublicCheck(replacement, reviewing.ID, release, now); err != nil || !ack.Released {
		t.Fatalf("original settlement was blocked by cutover: %+v %v", ack, err)
	}
	if err = s.DB.Exec("UPDATE publication_admission_cutover SET verified_at=?, verification_reference='fixture drain' WHERE id=1", now).Error; err != nil {
		t.Fatal(err)
	}
	if started, err := s.ClaimWithCapacity(9, "publish", cap, now); err != nil || started == nil || started.Task.ID != second.ID {
		t.Fatalf("verified cutover did not admit queued work: %+v %v", started, err)
	}
}

func TestCheckClaimAndUnreceivedSettlementRaceOnTheOriginalKey(t *testing.T) {
	s, _, reviewing := reviewingPublication(t)
	now := time.Now().UTC()
	if err := s.SchedulePublicCheck(now); err != nil {
		t.Fatal(err)
	}
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	current := model.CheckActor{UserID: 9, SessionID: "original", Current: true}
	noncurrent := current
	noncurrent.Current = false
	claim := model.CheckClaimInput{RequestKey: "same-key", Capacity: cap}
	settle := model.CheckClaimInput{RequestKey: claim.RequestKey, SettleUnreceived: true}
	type outcome struct {
		ack any
		err error
	}
	start := make(chan struct{})
	claimed, settled := make(chan outcome, 1), make(chan outcome, 1)
	go func() {
		<-start
		ack, err := s.ClaimPublicCheckV2(current, claim, now)
		claimed <- outcome{ack, err}
	}()
	go func() {
		<-start
		ack, err := s.ClaimPublicCheckV2(noncurrent, settle, now)
		settled <- outcome{ack, err}
	}()
	close(start)
	admission, settlement := <-claimed, <-settled
	if admission.err != nil || settlement.err != nil {
		t.Fatalf("claim=%+v settlement=%+v", admission, settlement)
	}
	replay, err := s.ClaimPublicCheckV2(current, claim, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var held int64
	if err = s.DB.Model(&model.ProfileUse{}).Where("released_at IS NULL").Count(&held).Error; err != nil || held != 0 {
		t.Fatalf("orphaned profile: %d %v", held, err)
	}
	switch admitted := admission.ack.(type) {
	case model.PublicCheckClaim:
		ack, ok := settlement.ack.(model.PublicCheckReleaseAck)
		if !ok || !ack.Released || ack.PublicationID != reviewing.ID || ack.ReservationID != admitted.ReservationID || replay != ack {
			t.Fatalf("admitted key did not settle exact holder: %#v %#v %#v", admitted, settlement.ack, replay)
		}
	case model.CancelledCheckClaim:
		if admitted.ReservationState != "cancelled" || admitted != settlement.ack || admitted != replay {
			t.Fatalf("earlier settlement did not fence delayed claim: %#v %#v %#v", admitted, settlement.ack, replay)
		}
	default:
		t.Fatalf("unexpected race result: %#v", admission.ack)
	}
}
