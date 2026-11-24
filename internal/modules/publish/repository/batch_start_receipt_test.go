package repository_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

func startReceiptFixture() model.Batch {
	now := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	key, hash := "original-private-key", "original-private-hash"
	return model.Batch{ID: 11, Revision: 9, StartedAt: &now, ExecutionStatus: model.BatchRunning, StartRequestKey: &key, StartRequestHash: &hash,
		Targets:      []model.BatchTarget{{CategoryID: 1, TargetCount: 3}},
		StartReceipt: &model.BatchStartResult{BatchID: 11, Revision: 2, ExecutionStatus: model.BatchRunning, FirstStartedAt: now, TaskIDs: []string{"100"}, CreatedCount: 1, TargetCount: 3, CreationShortfall: 2}}
}
func TestBatchStartOnlyDifferentDurableKeyWithValidReceiptProvesNotApplied(t *testing.T) {
	for _, tc := range []struct {
		name, key, digest string
		change            func(*model.Batch)
		notApplied        bool
	}{
		{name: "another key already won", key: "this-pending-key", digest: "this-digest", notApplied: true},
		{name: "same key conflicting digest", key: "original-private-key", digest: "changed-digest"},
		{name: "another key missing receipt", key: "this-pending-key", change: func(b *model.Batch) { b.StartReceipt = nil }},
		{name: "another key missing hash", key: "this-pending-key", change: func(b *model.Batch) { b.StartRequestHash = nil }},
		{name: "another key empty stored key", key: "this-pending-key", change: func(b *model.Batch) { *b.StartRequestKey = "" }},
		{name: "another key wrong batch receipt", key: "this-pending-key", change: func(b *model.Batch) { b.StartReceipt.BatchID++ }},
		{name: "another key inconsistent task count", key: "this-pending-key", change: func(b *model.Batch) { b.StartReceipt.CreatedCount = 2 }},
		{name: "another key wrong initial status", key: "this-pending-key", change: func(b *model.Batch) { b.StartReceipt.ExecutionStatus = model.BatchEnded }},
		{name: "another key wrong start time", key: "this-pending-key", change: func(b *model.Batch) { b.StartReceipt.FirstStartedAt = b.StartReceipt.FirstStartedAt.Add(time.Second) }},
		{name: "another key invalid task identity", key: "this-pending-key", change: func(b *model.Batch) { b.StartReceipt.TaskIDs = []string{"not-a-task"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := startReceiptFixture()
			if tc.change != nil {
				tc.change(&batch)
			}
			receipt, err := repo.ReplayBatchStart(batch, tc.key, tc.digest)
			if receipt != nil || !errors.Is(err, repo.ErrConflict) {
				t.Fatalf("expected read-only conflict: %+v %v", receipt, err)
			}
			raw, e := json.Marshal(err)
			if e != nil {
				t.Fatal(e)
			}
			var details map[string]any
			if e = json.Unmarshal(raw, &details); e != nil {
				t.Fatal(e)
			}
			flag, present := details["start_request_applied"]
			if present != tc.notApplied || present && flag != false {
				t.Fatalf("invalid authoritative not-applied proof: %s", raw)
			}
			if tc.notApplied && (details["batch_id"] != "11" || details["current_revision"] != "9" || details["execution_status"] != "running") {
				t.Fatalf("missing current conflict scope: %s", raw)
			}
			if strings.Contains(string(raw), "original-private-key") || strings.Contains(string(raw), "original-private-hash") {
				t.Fatalf("receipt exposed private original request identity: %s", raw)
			}
		})
	}
}
func TestBatchStartSameOriginalKeyStillReplaysImmutableReceipt(t *testing.T) {
	b := startReceiptFixture()
	receipt, err := repo.ReplayBatchStart(b, *b.StartRequestKey, *b.StartRequestHash)
	if err != nil || receipt == nil || receipt.Revision != 2 || b.Revision != 9 {
		t.Fatalf("original immutable receipt lost: %+v %v", receipt, err)
	}
}

func TestBatchStartTransactionProvesAnotherKeyWithoutChangingParent(t *testing.T) {
	s, b, now := testExecutableBatch(t, 1, 10, "auto_continue")
	paused, err := s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "pause", ExpectedRevision: b.Revision, Reason: "fixture current revision after start"}, now)
	if err != nil {
		t.Fatal(err)
	}
	input := model.BatchStartInput{RequestKey: "losing-pending-key", ExpectedRevision: 1, IncludedRowKeys: []string{"row-0"}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1}}}
	_, err = s.StartBatch(b.ID, 9, input, "losing-digest", nil, now)
	var conflict *repo.BatchExecutionConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("expected scoped conflict: %v", err)
	}
	raw, e := json.Marshal(conflict)
	if e != nil {
		t.Fatal(e)
	}
	var details map[string]any
	if e = json.Unmarshal(raw, &details); e != nil {
		t.Fatal(e)
	}
	if flag, present := details["start_request_applied"]; !present || flag != false || conflict.Revision != paused.Revision {
		t.Fatalf("transaction lacks authoritative current proof: %s", raw)
	}
	saved, e := s.GetBatch(b.ID)
	if e != nil || saved.Revision != paused.Revision || saved.ExecutionStatus != model.BatchPaused {
		t.Fatalf("proof mutated parent: %+v %v", saved, e)
	}
	var tasks, attempts int64
	if e = s.DB.Model(&model.Task{}).Where("batch_id=?", b.ID).Count(&tasks).Error; e != nil || tasks != 1 {
		t.Fatalf("proof created tasks: %d %v", tasks, e)
	}
	if e = s.DB.Model(&model.Attempt{}).Count(&attempts).Error; e != nil || attempts != 0 {
		t.Fatalf("proof created attempts: %d %v", attempts, e)
	}
	input.RequestKey = "start"
	replay, e := s.StartBatch(b.ID, 9, input, "body", nil, now)
	if e != nil || replay.Revision != 2 || replay.ExecutionStatus != model.BatchRunning {
		t.Fatalf("same-key replay lost immutable original after pause: %+v %v", replay, e)
	}
	_, err = s.StartBatch(b.ID, 9, input, "changed-same-key-digest", nil, now)
	if !errors.As(err, &conflict) {
		t.Fatalf("expected same-key conflict: %v", err)
	}
	raw, e = json.Marshal(conflict)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), "start_request_applied") {
		t.Fatalf("same key ambiguity was cleared: %s", raw)
	}
}
