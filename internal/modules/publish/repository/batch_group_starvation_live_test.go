package repository_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

// Exhausted auto-continue batches intentionally remain running for explicit
// failure processing. They must not permanently fill the dispatch scan window.
func TestBatchGroupAdmissionSkipsHundredExhaustedBatchesBeforeCandidateLimit(t *testing.T) {
	s, _ := executionStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	create := func(index int) model.Batch {
		t.Helper()
		outputID := int64(1000 + index)
		if err := s.DB.Exec("INSERT INTO composite_outputs VALUES(?)", outputID).Error; err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("starvation-row-%d", index)
		prepared := task("", outputID, 1)
		prepared.RowKey = &key
		prepared.PlatformAccountUID = "fixture-starvation-owner"
		prepared.ResourceSnapshot = []byte(`{"profile_id":37}`)
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
	oldRevisions := map[uint64]uint64{}
	for i := 0; i < 100; i++ {
		b := create(i)
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
	fresh := create(100)
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
