package repository_test

// These tests use the existing disposable MySQL schema. They do not open a
// browser or submit to a platform; receipts below are explicit test fixtures.
import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
)

var acceptanceCapacity = model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}

func acceptanceClaim(t *testing.T, s repo.ExecutionStore, key string, now time.Time) *model.Claim {
	t.Helper()
	c, err := s.ClaimWithCapacity(9, key, acceptanceCapacity, now)
	if err != nil || c == nil {
		t.Fatalf("claim %q: %+v %v", key, c, err)
	}
	return c
}
func acceptanceUnknown(t *testing.T, s repo.ExecutionStore, c *model.Claim, now time.Time) {
	t.Helper()
	raw := json.RawMessage(`{"status":"uncertain","error_code":"FIXTURE_UNKNOWN","evidence":{"scene_closed":false}}`)
	hash, err := model.CompletionHash(1, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Complete(c.Task, 9, model.Completion{AttemptID: c.Attempt.ID, Seq: 1, ResultHash: hash, Result: raw}, now); err != nil {
		t.Fatal(err)
	}
}
func acceptanceReviewing(t *testing.T, s repo.ExecutionStore, c *model.Claim, now time.Time) {
	t.Helper()
	in := c.Attempt.Input
	report := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStageReady, PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID, VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256, PortraitCoverSHA256: in.PortraitCoverSHA256}
	if _, err := s.Report(c.Task, 9, report, true, now); err != nil {
		t.Fatal(err)
	}
	report.Seq, report.Stage = 2, model.AttemptStageSubmitting
	ack, err := s.Report(c.Task, 9, report, true, now)
	if err != nil || !ack.AllowSubmit {
		t.Fatalf("fixture submit gate: %+v %v", ack, err)
	}
	raw, err := json.Marshal(model.ExecutionResult{Status: model.StatusReviewing, PlatformWorkID: "1834567890123456789", Evidence: model.ResultEvidence{
		PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID, VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256, PortraitCoverSHA256: in.PortraitCoverSHA256, SceneClosed: true, ReceiptID: "1834567890123456789"}})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := model.CompletionHash(3, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Complete(c.Task, 9, model.Completion{AttemptID: c.Attempt.ID, Seq: 3, ResultHash: hash, Result: raw}, now); err != nil {
		t.Fatal(err)
	}
}
func acceptancePause(t *testing.T, s repo.ExecutionStore, id uint64, now time.Time) model.Batch {
	t.Helper()
	b, err := s.GetBatch(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ControlBatch(id, 9, repo.BatchCommand{Action: "pause", ExpectedRevision: b.Revision, Reason: "fixture inspection"}, now); err != nil {
		t.Fatal(err)
	}
	b, err = s.GetBatch(id)
	if err != nil || b.Execution.Pause == nil {
		t.Fatalf("pause snapshot: %+v %v", b.Execution, err)
	}
	return b
}
func acceptanceResume(t *testing.T, s repo.ExecutionStore, b model.Batch, now time.Time) {
	t.Helper()
	tasks := make([]model.TaskRevision, 0, len(b.Execution.Pause.Exceptions))
	for _, e := range b.Execution.Pause.Exceptions {
		tasks = append(tasks, e.TaskRevision)
	}
	key := fmt.Sprintf("fixture-resume-%d", b.Execution.Pause.ID)
	if _, err := s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "resume", ExpectedRevision: b.Revision, ExpectedPauseID: b.Execution.Pause.ID, RequestKey: key, Digest: key, Tasks: tasks}, now); err != nil {
		t.Fatal(err)
	}
}

func TestBatchMixedExceptionsCountTasksNotReviewingOrAttemptHistory(t *testing.T) {
	s, b, now := testExecutableBatch(t, 4, 10, "auto_continue")
	failed := acceptanceClaim(t, s, "mixed-failed", now)
	failOriginalAttempt(t, s, failed, now)
	reviewing := acceptanceClaim(t, s, "mixed-reviewing", now)
	acceptanceReviewing(t, s, reviewing, now)
	next := now.Add(11 * time.Second)
	human := acceptanceClaim(t, s, "mixed-human", next)
	if ack, err := s.Report(human.Task, 9, model.ExecutionReport{AttemptID: human.Attempt.ID, Seq: 1, Stage: model.AttemptStageAwaitingHuman, SceneParked: true}, true, next); err != nil || !ack.SceneParked {
		t.Fatalf("park: %+v %v", ack, err)
	}
	unknown := acceptanceClaim(t, s, "mixed-unknown", next)
	acceptanceUnknown(t, s, unknown, next)
	paused := acceptancePause(t, s, b.ID, next)
	got := map[uint64]model.Status{}
	for _, e := range paused.Execution.Pause.Exceptions {
		if _, duplicate := got[e.TaskID]; duplicate {
			t.Fatalf("duplicate task exception %+v", e)
		}
		got[e.TaskID] = e.Status
	}
	want := map[uint64]model.Status{failed.Task.ID: model.StatusFailed, human.Task.ID: model.StatusAwaitingHuman, unknown.Task.ID: model.StatusUncertain}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed exception set: %+v want %+v (reviewing task %d excluded)", got, want, reviewing.Task.ID)
	}
	if paused.Execution.CreatedCount != 4 || paused.Execution.TaskCounts[model.StatusReviewing] != 1 {
		t.Fatalf("task/receipt counts changed: %+v", paused.Execution)
	}
	acceptanceResume(t, s, paused, next)
	uncertain, err := s.Get(unknown.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResolveResult(uncertain, 9, resolution(*unknown, uncertain), next); err != nil {
		t.Fatal(err)
	}
	prior, err := s.Get(failed.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Retry(prior, prior.Revision, 9, next); err != nil {
		t.Fatal(err)
	}
	retried := acceptanceClaim(t, s, "mixed-retry", next.Add(11*time.Second))
	if retried.Task.ID != failed.Task.ID || retried.Attempt.ID == failed.Attempt.ID {
		t.Fatalf("retry must retain task, create one original new attempt: %+v", retried)
	}
	failOriginalAttempt(t, s, retried, next.Add(11*time.Second))
	after := acceptancePause(t, s, b.ID, next.Add(11*time.Second))
	counts := map[uint64]int{}
	for _, e := range after.Execution.Pause.Exceptions {
		counts[e.TaskID]++
	}
	if len(counts) != 3 || counts[failed.Task.ID] != 1 || counts[human.Task.ID] != 1 || counts[unknown.Task.ID] != 1 {
		t.Fatalf("attempt history inflated exception count: %+v", after.Execution.Pause.Exceptions)
	}
	history, err := s.Attempt(failed.Task.ID)
	if err != nil || len(history) != 2 || after.Execution.CreatedCount != 4 {
		t.Fatalf("expected 2 attempts but 4 original tasks: %+v %+v %v", history, after.Execution, err)
	}
}

func TestBatchParentFencesSingleTaskStartAndRetry(t *testing.T) {
	for _, state := range []string{"paused", "stopping", "cancelled", "ended"} {
		t.Run(state, func(t *testing.T) {
			n := 1
			if state == "stopping" {
				n = 2
			}
			s, b, now := testExecutableBatch(t, n, 10, "auto_continue")
			failed := acceptanceClaim(t, s, "parent-failed", now)
			failOriginalAttempt(t, s, failed, now)
			if state == "stopping" {
				acceptanceClaim(t, s, "parent-running", now)
			}
			task, err := s.Get(failed.Task.ID)
			if err != nil {
				t.Fatal(err)
			}
			b, err = s.GetBatch(b.ID)
			if err != nil {
				t.Fatal(err)
			}
			cmd := repo.BatchCommand{ExpectedRevision: b.Revision, Reason: "fixture parent fence"}
			switch state {
			case "paused":
				cmd.Action = "pause"
			case "stopping", "cancelled":
				cmd.Action = "stop"
			case "ended":
				cmd.Action = "finish"
				cmd.Tasks = []model.TaskRevision{{TaskID: task.ID, Revision: task.Revision}}
			}
			out, err := s.ControlBatch(b.ID, 9, cmd, now)
			if err != nil || out.ExecutionStatus != state {
				t.Fatalf("parent %s: %+v %v", state, out, err)
			}
			if _, err = s.Start(task, task.Revision, 9, now); !errors.Is(err, repo.ErrParentBlocked) {
				t.Fatalf("single-task start bypassed %s: %v", state, err)
			}
			if _, err = s.Retry(task, task.Revision, 9, now); !errors.Is(err, repo.ErrParentBlocked) {
				t.Fatalf("single-task retry bypassed %s: %v", state, err)
			}
			saved, err := s.Get(task.ID)
			if err != nil || saved.Status != task.Status || saved.Revision != task.Revision || saved.CurrentAttemptID == nil || *saved.CurrentAttemptID != failed.Attempt.ID {
				t.Fatalf("rejected command mutated original task: %+v %v", saved, err)
			}
			history, err := s.Attempt(task.ID)
			if err != nil || len(history) != 1 {
				t.Fatalf("rejected command created another attempt: %+v %v", history, err)
			}
		})
	}
}

func TestBatchFinishRejectsHumanAndUnknownWithoutChangingTheirHold(t *testing.T) {
	for _, status := range []model.Status{model.StatusAwaitingHuman, model.StatusUncertain} {
		t.Run(string(status), func(t *testing.T) {
			s, b, now := testExecutableBatch(t, 1, 10, "auto_continue")
			c := acceptanceClaim(t, s, "finish-unresolved", now)
			if status == model.StatusAwaitingHuman {
				if _, err := s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStageAwaitingHuman, SceneParked: true}, true, now); err != nil {
					t.Fatal(err)
				}
			} else {
				acceptanceUnknown(t, s, c, now)
			}
			b, err := s.GetBatch(b.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "finish", ExpectedRevision: b.Revision, Reason: "must not erase unresolved", Tasks: []model.TaskRevision{}}, now); !errors.Is(err, repo.ErrUnresolved) {
				t.Fatalf("finish accepted %s: %v", status, err)
			}
			after, err := s.GetBatch(b.ID)
			if err != nil || after.Revision != b.Revision || after.EndedAt != nil {
				t.Fatalf("failed finish changed batch: %+v %v", after, err)
			}
			saved, err := s.Get(c.Task.ID)
			if err != nil || saved.Status != status {
				t.Fatalf("failed finish changed task: %+v %v", saved, err)
			}
			var held int64
			if err = s.DB.Model(&model.ProfileUse{}).Where("attempt_id=? AND released_at IS NULL", c.Attempt.ID).Count(&held).Error; err != nil || held != 1 {
				t.Fatalf("failed finish released unresolved Profile: %d %v", held, err)
			}
		})
	}
}

func TestBatchFinishRequiresExactFailuresAndKeepsOccupancyAfterEnd(t *testing.T) {
	s, b, now := testExecutableBatch(t, 1, 10, "auto_continue")
	c := acceptanceClaim(t, s, "finish-failure", now)
	failOriginalAttempt(t, s, c, now)
	failed, err := s.Get(c.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err = s.GetBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	cmd := repo.BatchCommand{Action: "finish", ExpectedRevision: b.Revision, Reason: "accept original failed result"}
	for _, wrong := range [][]model.TaskRevision{{}, {{TaskID: failed.ID, Revision: failed.Revision - 1}}, {{TaskID: failed.ID + 100, Revision: failed.Revision}}} {
		cmd.Tasks = wrong
		if _, err = s.ControlBatch(b.ID, 9, cmd, now); !errors.Is(err, repo.ErrUnresolved) {
			t.Fatalf("accepted missing/stale/foreign failure set %+v: %v", wrong, err)
		}
	}
	cmd.Tasks = []model.TaskRevision{{TaskID: failed.ID, Revision: failed.Revision}}
	ended, err := s.ControlBatch(b.ID, 9, cmd, now)
	if err != nil || ended.ExecutionStatus != model.BatchEnded || ended.Execution.TaskCounts[model.StatusFailed] != 1 {
		t.Fatalf("explicit finish: %+v %v", ended, err)
	}
	if _, err = s.Retry(failed, failed.Revision, 9, now); !errors.Is(err, repo.ErrParentBlocked) {
		t.Fatalf("retry after finish: %v", err)
	}
	var occupied int64
	if err = s.DB.Model(&model.Task{}).Where("id=? AND occupied_output_id IS NOT NULL AND status=?", failed.ID, model.StatusFailed).Count(&occupied).Error; err != nil || occupied != 1 {
		t.Fatalf("finish released original output: %d %v", occupied, err)
	}
	history, err := s.Attempt(failed.ID)
	if err != nil || len(history) != 1 || history[0].ResultHash == nil {
		t.Fatalf("finish changed original failed evidence: %+v %v", history, err)
	}
}
