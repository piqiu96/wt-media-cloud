package repository_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestClaimFullOwnerCapacityDoesNotTraverseQueuedBacklog(t *testing.T) {
	for _, parked := range []bool{false, true} {
		name := "automatic full"
		if parked {
			name = "windows full after reliable park"
		}
		t.Run(name, func(t *testing.T) {
			s, makeTask := executionStore(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			capacity := model.ExecutionCapacity{AutomaticSlots: 1, WindowSlots: 1}
			holder := makeTask("capacity-scan-holder", 42, "holder-account")
			if _, err := s.Start(holder, holder.Revision, 9, now); err != nil {
				t.Fatal(err)
			}
			held, err := s.ClaimWithCapacity(9, "capacity-scan-original", capacity, now)
			if err != nil || held == nil {
				t.Fatalf("holder claim: %+v %v", held, err)
			}
			if parked {
				ack, err := s.Report(held.Task, 9, model.ExecutionReport{AttemptID: held.Attempt.ID, Seq: 1, Stage: model.AttemptStageAwaitingHuman, SceneParked: true}, true, now)
				if err != nil || !ack.SceneParked {
					t.Fatalf("reliable parked holder: %+v %v", ack, err)
				}
			}
			for i := 0; i < 205; i++ {
				output := int64(3000 + i)
				if err := s.DB.Exec("INSERT INTO composite_outputs VALUES(?)", output).Error; err != nil {
					t.Fatal(err)
				}
				queued := makeTask(fmt.Sprintf("capacity-backlog-%d", i), output, fmt.Sprintf("waiting-account-%d", i))
				if _, err := s.Start(queued, queued.Revision, 9, now); err != nil {
					t.Fatal(err)
				}
			}
			inputReads, taskQueries := 0, 0
			read := s.ReadInput
			s.ReadInput = func(tx *gorm.DB, task model.Task) (model.ExecutionInput, error) {
				inputReads++
				return read(tx, task)
			}
			if err := s.DB.Callback().Query().Before("gorm:query").Register("test:bounded-capacity-scan", func(tx *gorm.DB) {
				if tx.Statement.Table == "publication_task" {
					taskQueries++
				}
			}); err != nil {
				t.Fatal(err)
			}
			if claim, err := s.ClaimWithCapacity(9, "capacity-scan-no-room", capacity, now); err != nil || claim != nil {
				t.Fatalf("full owner capacity admitted work: %+v %v", claim, err)
			}
			if inputReads > 1 || taskQueries > 1 {
				t.Fatalf("full owner capacity traversed backlog: input reads=%d task queries=%d", inputReads, taskQueries)
			}
			if err := s.DB.Callback().Query().Remove("test:bounded-capacity-scan"); err != nil {
				t.Fatal(err)
			}
			replay, err := s.ClaimWithCapacity(9, "capacity-scan-original", capacity, now)
			if err != nil || replay == nil || !replay.Replayed || replay.Attempt.ID != held.Attempt.ID {
				t.Fatalf("full capacity lost original claim: %+v %v", replay, err)
			}
			var attempts, uses, queued int64
			if err := s.DB.Model(&model.Attempt{}).Count(&attempts).Error; err != nil || attempts != 1 {
				t.Fatalf("empty claim created attempts: %d %v", attempts, err)
			}
			if err := s.DB.Model(&model.ProfileUse{}).Where("released_at IS NULL").Count(&uses).Error; err != nil || uses != 1 {
				t.Fatalf("holder Profile changed: %d %v", uses, err)
			}
			if err := s.DB.Model(&model.Task{}).Where("status=? AND current_attempt_id IS NULL", model.StatusQueued).Count(&queued).Error; err != nil || queued != 205 {
				t.Fatalf("full capacity changed queued backlog: %d %v", queued, err)
			}
		})
	}
}

func TestClaimFullCapacityReturnsConcurrentOriginalKeyWinner(t *testing.T) {
	for _, parked := range []bool{false, true} {
		t.Run(fmt.Sprintf("parked=%t", parked), func(t *testing.T) {
			s, makeTask := executionStore(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			capacity := model.ExecutionCapacity{AutomaticSlots: 1, WindowSlots: 1}
			candidate := makeTask("capacity-winner", 42, "winner-account")
			if _, err := s.Start(candidate, candidate.Revision, 9, now); err != nil {
				t.Fatal(err)
			}
			const key = "capacity-concurrent-original-key"
			var winner *model.Claim
			var winnerErr error
			armed := true
			if err := s.DB.Callback().Query().Before("gorm:query").Register("test:concurrent-capacity-winner", func(tx *gorm.DB) {
				selected, ok := tx.Statement.Clauses["SELECT"].Expression.(clause.Select)
				if !armed || tx.Statement.Table != "publication_attempt" || !ok || len(selected.Columns) != 1 || selected.Columns[0].Name != "active_automatic_slot" {
					return
				}
				// The outer claim has already found no prior key. Commit the real
				// competing claim before it reads owner-wide capacity in this tx.
				armed = false
				winner, winnerErr = s.ClaimWithCapacity(9, key, capacity, now)
				if winnerErr == nil && winner != nil && parked {
					_, winnerErr = s.Report(winner.Task, 9, model.ExecutionReport{AttemptID: winner.Attempt.ID, Seq: 1, Stage: model.AttemptStageAwaitingHuman, SceneParked: true}, true, now)
				}
			}); err != nil {
				t.Fatal(err)
			}
			got, err := s.ClaimWithCapacity(9, key, capacity, now)
			if e := s.DB.Callback().Query().Remove("test:concurrent-capacity-winner"); e != nil {
				t.Fatal(e)
			}
			if winnerErr != nil || winner == nil || armed {
				t.Fatalf("concurrent winner fixture: %+v %v armed=%t", winner, winnerErr, armed)
			}
			if err != nil || got == nil || !got.Replayed || got.Attempt.ID != winner.Attempt.ID {
				t.Fatalf("capacity early exit lost concurrent original key: %+v %v", got, err)
			}
			var attempts int64
			if err := s.DB.Model(&model.Attempt{}).Count(&attempts).Error; err != nil || attempts != 1 {
				t.Fatalf("concurrent replay created extra attempts: %d %v", attempts, err)
			}
		})
	}
}
