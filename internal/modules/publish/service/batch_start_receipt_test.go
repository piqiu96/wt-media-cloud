package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

type noNewStartFixture struct {
	BatchExecutionRepository
	t *testing.T
}

func (f noNewStartFixture) StartBatch(uint64, int64, model.BatchStartInput, string, []model.Task, time.Time) (model.BatchStartResult, error) {
	f.t.Fatal("authoritative replay conflict must not start another transaction")
	return model.BatchStartResult{}, nil
}
func TestStartBatchDifferentPersistedKeyReturnsReadOnlyNotAppliedProof(t *testing.T) {
	svc, a, _ := fixture()
	now := time.Now().UTC()
	key, hash := "prior-private-key", "prior-private-digest"
	b := model.Batch{ID: 11, TeamID: int64(*a.TeamID), OperatorID: int64(a.ID), Revision: 9, StartedAt: &now, ExecutionStatus: model.BatchRunning,
		Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 3}}, StartRequestKey: &key, StartRequestHash: &hash,
		StartReceipt: &model.BatchStartResult{BatchID: 11, Revision: 2, ExecutionStatus: model.BatchRunning, FirstStartedAt: now, TaskIDs: []string{"100"}, CreatedCount: 1, TargetCount: 3, CreationShortfall: 2}}
	svc.BatchRepo = savedBatchFixture{batch: b}
	svc.BatchExecutionRepo = noNewStartFixture{t: t}
	_, err := svc.StartBatch(context.Background(), a, 11, model.BatchStartInput{RequestKey: "pending-key", ExpectedRevision: 1, IncludedRowKeys: []string{"row"}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1, Shortfall: 2}}})
	if err == nil {
		t.Fatal("another key must remain a conflict")
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
	if !present || flag != false || details["current_revision"] != "9" {
		t.Fatalf("service did not preserve authoritative conflict: %s", raw)
	}
	if strings.Contains(string(raw), key) || strings.Contains(string(raw), hash) {
		t.Fatalf("original key leaked: %s", raw)
	}
}
