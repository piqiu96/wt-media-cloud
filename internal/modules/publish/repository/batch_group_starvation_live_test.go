package repository_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

func starvationBatch(t *testing.T, s repo.ExecutionStore, index, profileID int, now time.Time) model.Batch {
	t.Helper()
	outputID := int64(1000 + index)
	if err := s.DB.Exec("INSERT INTO composite_outputs VALUES(?)", outputID).Error; err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("starvation-row-%d", index)
	prepared := task("", outputID, 1)
	prepared.RowKey = &key
	prepared.PlatformAccountUID = fmt.Sprintf("fixture-starvation-owner-%d", profileID)
	prepared.ResourceSnapshot = []byte(fmt.Sprintf(`{"profile_id":%d}`, profileID))
	b, err := s.CreateBatch(model.Batch{TeamID: 7, OperatorID: 9, CreatedBy: 9, Name: key, Platform: "baidu", Revision: 1,
		RequestKey: key, RequestHash: key, ConfigurationStatus: model.BatchReady, ExecutionStatus: model.BatchNotStarted,
		Targets:          []model.BatchTarget{{CategoryID: 1, TargetCount: 1}},
		Config:           model.BatchConfig{Candidates: []model.BatchCandidate{{BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: outputID}, RowKey: key, Content: prepared.Content}}},
		ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 1, GroupIntervalSeconds: 10, BetweenGroupPolicy: "auto_continue", ExceptionTaskLimit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.StartBatch(b.ID, 9, model.BatchStartInput{RequestKey: key, ExpectedRevision: 1, IncludedRowKeys: []string{key}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1}}}, key, []model.Task{prepared}, now)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Exhausted auto-continue batches intentionally remain running for explicit
// failure processing. They must not permanently fill the dispatch scan window.
func TestBatchGroupAdmissionSkipsHundredExhaustedBatchesBeforeCandidateLimit(t *testing.T) {
	s, _ := executionStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	oldRevisions := map[uint64]uint64{}
	for i := 0; i < 100; i++ {
		b := starvationBatch(t, s, i, 37, now)
		claim, err := s.ClaimWithCapacity(9, fmt.Sprintf("starvation-old-claim-%d", i), acceptanceCapacity, now)
		if err != nil || claim == nil || claim.Task.BatchID == nil || *claim.Task.BatchID != b.ID {
			t.Fatalf("old batch %d claim: %+v %v", i, claim, err)
		}
		failOriginalAttempt(t, s, claim, now)
		saved, err := s.GetBatch(b.ID)
		if err != nil || saved.ExecutionStatus != model.BatchRunning || saved.GroupEndedAt == nil || saved.NextGroupAt == nil || saved.Execution.TaskCounts[model.StatusFailed] != 1 {
			t.Fatalf("exhausted batch fixture: %+v %v", saved.Execution, err)
		}
		oldRevisions[b.ID] = saved.Revision
	}
	fresh := starvationBatch(t, s, 100, 37, now)
	// All one hundred earlier groups are now past their interval, with no
	// ungrouped dispatchable task; only batch 101 can produce new work.
	claim, err := s.ClaimWithCapacity(9, "starvation-fresh-claim", acceptanceCapacity, now.Add(11*time.Second))
	if err != nil || claim == nil || claim.Task.BatchID == nil || *claim.Task.BatchID != fresh.ID || claim.ExecutionGroupNo == nil || *claim.ExecutionGroupNo != 1 {
		t.Fatalf("100 exhausted batches starved the fresh first group: %+v %v", claim, err)
	}
	for id, revision := range oldRevisions {
		saved, err := s.GetBatch(id)
		if err != nil || saved.Revision != revision || saved.ExecutionStatus != model.BatchRunning || saved.Execution.TaskCounts[model.StatusFailed] != 1 {
			t.Fatalf("dispatch rewrote exhausted batch %d: %+v %v", id, saved.Execution, err)
		}
	}
	var total int64
	if err = s.DB.Model(&model.Attempt{}).Count(&total).Error; err != nil || total != 101 {
		t.Fatalf("expected original attempts only, got %d %v", total, err)
	}
	// A frozen current group remains assigned; another scan must not refill it.
	if extra, err := s.ClaimWithCapacity(9, "starvation-no-refill", acceptanceCapacity, now.Add(12*time.Second)); err != nil || extra != nil {
		t.Fatalf("unexpected refill: %+v %v", extra, err)
	}
}

func TestBatchGroupAdmissionPagesPastHundredBusyProfiles(t *testing.T) {
	s, _ := executionStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	holder := starvationBatch(t, s, 0, 37, now)
	original := acceptanceClaim(t, s, "busy-profile-holder", now)
	if original.Task.BatchID == nil || *original.Task.BatchID != holder.ID {
		t.Fatal("fixture holder differs")
	}
	waiting := map[uint64]uint64{}
	for i := 1; i <= 100; i++ {
		b := starvationBatch(t, s, i, 37, now)
		saved, err := s.GetBatch(b.ID)
		if err != nil {
			t.Fatal(err)
		}
		waiting[b.ID] = saved.Revision
	}
	fresh := starvationBatch(t, s, 101, 38, now)
	claim, err := s.ClaimWithCapacity(9, "independent-profile-after-busy-page", acceptanceCapacity, now)
	if err != nil || claim == nil || claim.Task.BatchID == nil || *claim.Task.BatchID != fresh.ID || claim.Attempt.Input.ProfileID != 38 {
		t.Fatalf("busy candidate page starved independent Profile: %+v %v", claim, err)
	}
	for id, revision := range waiting {
		saved, err := s.GetBatch(id)
		if err != nil || saved.Revision != revision || saved.ExecutionGroupNo != nil || saved.Execution.TaskCounts[model.StatusPending] != 1 {
			t.Fatalf("busy Profile batch was grouped or changed: %+v %v", saved.Execution, err)
		}
	}
	var attempts, uses int64
	if err = s.DB.Model(&model.Attempt{}).Count(&attempts).Error; err != nil || attempts != 2 {
		t.Fatalf("busy wait created attempts: %d %v", attempts, err)
	}
	if err = s.DB.Model(&model.ProfileUse{}).Where("released_at IS NULL").Count(&uses).Error; err != nil || uses != 2 {
		t.Fatalf("Profile holders: %d %v", uses, err)
	}
	originalHistory, err := s.Attempt(original.Task.ID)
	if err != nil || len(originalHistory) != 1 || originalHistory[0].Status != model.AttemptStatusRunning {
		t.Fatalf("original busy holder changed: %+v %v", originalHistory, err)
	}
}
