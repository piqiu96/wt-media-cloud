package repository_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"gorm.io/gorm"
)

func checkingQueueReviewing(t *testing.T, s repo.ExecutionStore, task model.Task, index int) model.Task {
	t.Helper()
	now := time.Now().UTC()
	if _, err := s.Start(task, task.Revision, 9, now); err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimWithCapacity(9, fmt.Sprintf("check-backlog-original-%d", index), acceptanceCapacity, now)
	if err != nil || claim == nil || claim.Task.ID != task.ID {
		t.Fatalf("original attempt: %+v %v", claim, err)
	}
	raw := json.RawMessage(`{"status":"uncertain","error_code":"LOST_RESPONSE","evidence":{"scene_closed":false}}`)
	hash, err := model.CompletionHash(1, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Complete(claim.Task, 9, model.Completion{AttemptID: claim.Attempt.ID, Seq: 1, ResultHash: hash, Result: raw}, now); err != nil {
		t.Fatal(err)
	}
	return resolveReviewingPublication(t, s, *claim)
}

func TestPublicCheckPagesPastTwentyInvalidInputsAndStopsWhenWindowsFull(t *testing.T) {
	for _, mutation := range []string{"disabled account", "rebound Profile"} {
		t.Run(mutation, func(t *testing.T) {
			s, makeTask := ownedStore(t)
			base := makeTask(0)
			var output map[string]any
			if err := s.DB.Table("composite_outputs").Where("id=42").Take(&output).Error; err != nil {
				t.Fatal(err)
			}
			old := []model.Task{}
			for i := 0; i < 20; i++ {
				v := base
				if i > 0 {
					copyOutput := map[string]any{}
					for key, value := range output {
						copyOutput[key] = value
					}
					copyOutput["id"] = 4000 + i
					if err := s.DB.Table("composite_outputs").Create(copyOutput).Error; err != nil {
						t.Fatal(err)
					}
					v.ID, v.CompositeOutputID, v.RequestKey = 0, int64(4000+i), fmt.Sprintf("check-backlog-old-%d", i)
					var err error
					v, err = s.Create(v)
					if err != nil {
						t.Fatal(err)
					}
				}
				old = append(old, checkingQueueReviewing(t, s, v, i))
			}
			valid := checkingQueueReviewing(t, s, makeTask(1), 20)
			now := time.Now().UTC().Truncate(time.Microsecond)
			if err := s.SchedulePublicCheck(now); err != nil {
				t.Fatal(err)
			}
			for i, v := range old {
				due := now.Add(-time.Minute)
				if i < 10 {
					due = due.Add(-time.Minute)
				}
				if err := s.DB.Model(&model.Task{}).Where("id=?", v.ID).UpdateColumn("next_check_at", due).Error; err != nil {
					t.Fatal(err)
				}
				old[i], _ = s.Get(v.ID)
			}
			// Tie the second page's valid candidate to older IDs to exercise both
			// next_check_at and id in the cursor, without changing invalid schedules.
			if err := s.DB.Model(&model.Task{}).Where("id=?", valid.ID).UpdateColumn("next_check_at", now.Add(-time.Minute)).Error; err != nil {
				t.Fatal(err)
			}
			change := "UPDATE media_accounts SET enabled=0 WHERE id=12"
			if mutation == "rebound Profile" {
				change = "UPDATE media_accounts SET browser_profile_id=39 WHERE id=12"
			}
			if err := s.DB.Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			accepted := now.Add(-24*time.Hour + time.Minute)
			if err := s.DB.Model(&model.Task{}).Where("id=?", valid.ID).UpdateColumn("accepted_at", accepted).Error; err != nil {
				t.Fatal(err)
			}
			actor := model.CheckActor{UserID: 9, SessionID: "current-check-session", Current: true}
			input := model.CheckClaimInput{RequestKey: "check-after-invalid-page", Capacity: model.ExecutionCapacity{AutomaticSlots: 1, WindowSlots: 1}}
			got, err := s.ClaimPublicCheckV2(actor, input, now)
			claim, ok := got.(model.PublicCheckClaim)
			if err != nil || !ok || claim.Task.ID != valid.ID {
				t.Fatalf("twenty invalid old checks starved the valid candidate: %#v %v", got, err)
			}
			if claim.Attempt.ID != *valid.CurrentAttemptID || claim.Generation != 1 || claim.ReservationID == 0 {
				t.Fatalf("check did not retain original publication attempt: %+v", claim)
			}
			saved, err := s.Get(valid.ID)
			if err != nil || saved.NextCheckAt == nil || !saved.NextCheckAt.Equal(accepted.Add(24*time.Hour)) {
				t.Fatalf("check retry exceeded original acceptance deadline: %+v %v", saved.NextCheckAt, err)
			}
			for _, before := range old {
				after, err := s.Get(before.ID)
				if err != nil || after.Revision != before.Revision || after.CheckGeneration != before.CheckGeneration || !reflect.DeepEqual(after.NextCheckAt, before.NextCheckAt) {
					t.Fatalf("invalid candidate mutated: %+v %v", after, err)
				}
			}
			reads, taskQueries := 0, 0
			read := s.ReadInput
			s.ReadInput = func(tx *gorm.DB, v model.Task) (model.ExecutionInput, error) {
				reads++
				return read(tx, v)
			}
			if err := s.DB.Callback().Query().Before("gorm:query").Register("test:bounded-check-capacity", func(tx *gorm.DB) {
				if tx.Statement.Table == "publication_task" {
					taskQueries++
				}
			}); err != nil {
				t.Fatal(err)
			}
			if got, err := s.ClaimPublicCheckV2(actor, model.CheckClaimInput{RequestKey: "full-check-window", Capacity: input.Capacity}, now); err != nil || got != nil {
				t.Fatalf("full window admitted another check: %#v %v", got, err)
			}
			if reads != 0 || taskQueries != 0 {
				t.Fatalf("full owner windows scanned invalid backlog: reads=%d task queries=%d", reads, taskQueries)
			}
			if err := s.DB.Callback().Query().Remove("test:bounded-check-capacity"); err != nil {
				t.Fatal(err)
			}
			replayed, err := s.ClaimPublicCheckV2(actor, input, now)
			replay, ok := replayed.(model.PublicCheckClaim)
			if err != nil || !ok || !replay.Replayed || replay.ReservationID != claim.ReservationID || replay.Generation != claim.Generation || replay.Attempt.ID != claim.Attempt.ID {
				t.Fatalf("full capacity lost original key winner: %#v %v", replayed, err)
			}
			var attempts, uses int64
			if err := s.DB.Model(&model.Attempt{}).Count(&attempts).Error; err != nil || attempts != 21 {
				t.Fatalf("read-only check created publication attempts: %d %v", attempts, err)
			}
			if err := s.DB.Model(&model.ProfileUse{}).Where("released_at IS NULL").Count(&uses).Error; err != nil || uses != 1 {
				t.Fatalf("unexpected check Profile reservations: %d %v", uses, err)
			}
		})
	}
}
