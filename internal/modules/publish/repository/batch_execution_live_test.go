package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"reflect"
	"testing"
	"time"
)

func TestBatchFirstStartRollsBackEveryTaskAndReplaysOriginalReceipt(t *testing.T) {
	s, _ := executionStore(t)
	rows := []model.BatchCandidate{
		{BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: 42}, RowKey: "first", Order: 0},
		{BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: 43}, RowKey: "second", Order: 1},
	}
	for i := range rows {
		rows[i].Content = task("ignored", 42, 1).Content
	}
	batch, err := s.CreateBatch(model.Batch{TeamID: 7, OperatorID: 9, CreatedBy: 9, Name: "batch", Platform: "baidu", Revision: 1,
		RequestKey: "saved", RequestHash: "saved", ConfigurationStatus: model.BatchReady, ExecutionStatus: model.BatchNotStarted,
		Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 2}}, Config: model.BatchConfig{Candidates: rows},
		ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 2, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Create(task("collision", 43, 1))
	if err != nil {
		t.Fatal(err)
	}
	prepared := []model.Task{task("", 42, 1), task("", 43, 1)}
	for i := range prepared {
		prepared[i].RowKey = &rows[i].RowKey
		prepared[i].ResourceSnapshot = []byte(`{"profile_id":37}`)
	}
	in := model.BatchStartInput{RequestKey: "start", ExpectedRevision: 1, IncludedRowKeys: []string{"first", "second"}, ExcludedRowKeys: []string{}, ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1, Shortfall: 0}}}
	now := time.Now().UTC()
	if _, err = s.StartBatch(batch.ID, 9, in, "digest", prepared, now); !errors.Is(err, repo.ErrOccupied) {
		t.Fatalf("expected collision rollback, got %v", err)
	}
	var count int64
	if err = s.DB.Model(&model.Task{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("partial create: %d %v", count, err)
	}
	if saved, err := s.GetBatch(batch.ID); err != nil || saved.Revision != 1 || saved.StartedAt != nil {
		t.Fatalf("changed batch after rollback: %+v %v", saved, err)
	}
	if err = s.DB.Model(&model.Task{}).Where("id=?", before.ID).Update("status", model.StatusAbandoned).Error; err != nil {
		t.Fatal(err)
	}
	started, err := s.StartBatch(batch.ID, 9, in, "digest", prepared, now)
	if err != nil || started.Revision != 2 || len(started.TaskIDs) != 2 {
		t.Fatalf("start: %+v %v", started, err)
	}
	replay, err := s.StartBatch(batch.ID, 9, in, "digest", nil, now.Add(time.Second))
	if err != nil || !reflect.DeepEqual(replay, started) {
		t.Fatalf("replay: %+v vs %+v %v", replay, started, err)
	}
	if _, err = s.StartBatch(batch.ID, 9, in, "different", nil, now); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("different request replay: %v", err)
	}
}

func testExecutableBatch(t *testing.T, n int, exceptionLimit uint32, policy string) (repo.ExecutionStore, model.Batch, time.Time) {
	t.Helper()
	s, _ := executionStore(t)
	for i := 2; i < n; i++ {
		if err := s.DB.Exec("INSERT INTO browser_profiles VALUES(?)", 37+i).Error; err != nil {
			t.Fatal(err)
		}
		if i >= 3 {
			if err := s.DB.Exec("INSERT INTO composite_outputs VALUES(?)", 42+i).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	rows := make([]model.BatchCandidate, n)
	tasks := make([]model.Task, n)
	keys := make([]string, n)
	for i := range rows {
		key := fmt.Sprintf("row-%d", i)
		v := task("", int64(42+i), 1)
		v.PlatformAccountUID = fmt.Sprintf("uid-%d", i)
		v.ResourceSnapshot = []byte(fmt.Sprintf(`{"profile_id":%d}`, 37+i))
		v.RowKey = &key
		tasks[i] = v
		keys[i] = key
		rows[i] = model.BatchCandidate{BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: int64(42 + i)}, RowKey: key, Order: i, Content: v.Content}
	}
	saved, err := s.CreateBatch(model.Batch{TeamID: 7, OperatorID: 9, CreatedBy: 9, Name: "batch", Platform: "baidu", Revision: 1,
		RequestKey: "save", RequestHash: "save", ConfigurationStatus: model.BatchReady, ExecutionStatus: model.BatchNotStarted,
		Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: int64(n)}}, Config: model.BatchConfig{Candidates: rows},
		ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 2, GroupIntervalSeconds: 10, ExceptionTaskLimit: exceptionLimit, BetweenGroupPolicy: policy},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err = s.StartBatch(saved.ID, 9, model.BatchStartInput{RequestKey: "start", ExpectedRevision: 1, IncludedRowKeys: keys, ExcludedRowKeys: []string{},
		ExpectedShortfalls: []model.ExpectedShortfall{{CategoryID: 1}}}, "body", tasks, now)
	if err != nil {
		t.Fatal(err)
	}
	saved, err = s.GetBatch(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, saved, now
}

func failOriginalAttempt(t *testing.T, s repo.ExecutionStore, c *model.Claim, now time.Time) {
	t.Helper()
	in := c.Attempt.Input
	raw, _ := json.Marshal(model.ExecutionResult{Status: model.StatusFailed, ErrorCode: "fixture_confirmed_failed", Evidence: model.ResultEvidence{
		PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID, VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256, PortraitCoverSHA256: in.PortraitCoverSHA256,
		NotSubmitted: true, SceneClosed: true,
	}})
	hash, err := model.CompletionHash(1, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Complete(c.Task, 9, model.Completion{AttemptID: c.Attempt.ID, Seq: 1, ResultHash: hash, Result: raw}, now); err != nil {
		t.Fatal(err)
	}
}

func TestFixedGroupDoesNotRefillAndWaitsAcrossStoreRestart(t *testing.T) {
	s, b, now := testExecutableBatch(t, 3, 10, "auto_continue")
	capacity := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	first, err := s.ClaimWithCapacity(9, "first", capacity, now)
	if err != nil || first == nil || first.ExecutionGroupNo == nil || *first.ExecutionGroupNo != 1 {
		t.Fatalf("first claim %+v %v", first, err)
	}
	failOriginalAttempt(t, s, first, now)
	second, err := s.ClaimWithCapacity(9, "second", capacity, now)
	if err != nil || second == nil || second.ExecutionGroupNo == nil || *second.ExecutionGroupNo != 1 {
		t.Fatalf("second claim %+v %v", second, err)
	}
	if extra, err := s.ClaimWithCapacity(9, "no rolling refill", capacity, now); err != nil || extra != nil {
		t.Fatalf("rolling refill: %+v %v", extra, err)
	}
	completedAt := now.Add(time.Second)
	failOriginalAttempt(t, s, second, completedAt)
	saved, err := s.GetBatch(b.ID)
	if err != nil || saved.NextGroupAt == nil || !saved.NextGroupAt.Equal(completedAt.Add(10*time.Second)) {
		t.Fatalf("persisted interval: %+v %v", saved.NextGroupAt, err)
	}
	restarted := repo.ExecutionStore{Store: s.Store, ReadInput: s.ReadInput}
	if extra, err := restarted.ClaimWithCapacity(9, "too-early", capacity, completedAt.Add(9*time.Second)); err != nil || extra != nil {
		t.Fatalf("restarted group opened too early: %+v %v", extra, err)
	}
	third, err := restarted.ClaimWithCapacity(9, "third", capacity, completedAt.Add(10*time.Second))
	if err != nil || third == nil || third.ExecutionGroupNo == nil || *third.ExecutionGroupNo != 2 {
		t.Fatalf("next persisted group: %+v %v", third, err)
	}
}

func TestBatchStopAndSubmitPermissionSerialize(t *testing.T) {
	for _, submitFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(submitFirst), func(t *testing.T) {
			s, b, now := testExecutableBatch(t, 1, 10, "auto_continue")
			claim, err := s.ClaimWithCapacity(9, "claim", model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}, now)
			if err != nil || claim == nil {
				t.Fatalf("claim: %+v %v", claim, err)
			}
			input := claim.Attempt.Input
			report := model.ExecutionReport{AttemptID: claim.Attempt.ID, Seq: 1, Stage: model.AttemptStageReady, PlatformUID: input.PlatformUID, BitProfileID: input.BitProfileID,
				VideoSHA256: input.VideoSHA256, CoverSHA256: input.CoverSHA256, PortraitCoverSHA256: input.PortraitCoverSHA256}
			if _, err = s.Report(claim.Task, 9, report, true, now); err != nil {
				t.Fatal(err)
			}
			report.Stage, report.Seq = model.AttemptStageSubmitting, 2
			stop := func() {
				latest, e := s.GetBatch(b.ID)
				if e != nil {
					t.Fatal(e)
				}
				result, e := s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "stop", ExpectedRevision: latest.Revision, Reason: "exercise stop"}, now)
				if e != nil || result.ExecutionStatus != model.BatchStopping || result.InflightCount != 1 {
					t.Fatalf("stop: %+v %v", result, e)
				}
			}
			if !submitFirst {
				stop()
			}
			grant, err := s.Report(claim.Task, 9, report, true, now)
			if err != nil || grant.AllowSubmit != submitFirst || !submitFirst && !grant.StopRequested {
				t.Fatalf("grant=%+v error=%v", grant, err)
			}
			if submitFirst {
				stop()
			}
			attempts, err := s.Attempt(claim.Task.ID)
			if err != nil || (attempts[0].SubmitStartedAt != nil) != submitFirst {
				t.Fatalf("recorded original grant: %+v %v", attempts, err)
			}
		})
	}
}

func TestBatchExceptionLimitAndPauseAcknowledgementWithSixtyTasks(t *testing.T) {
	s, b, now := testExecutableBatch(t, 60, 1, "auto_continue")
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	first, err := s.ClaimWithCapacity(9, "batch-60-first", cap, now)
	if err != nil || first == nil {
		t.Fatalf("first: %+v %v", first, err)
	}
	failOriginalAttempt(t, s, first, now)
	paused, err := s.GetBatch(b.ID)
	if err != nil || paused.ExecutionStatus != model.BatchPaused || paused.Execution.CreatedCount != 60 ||
		paused.Execution.Pause == nil || paused.Execution.Pause.Kind != "exception_limit" || len(paused.Execution.Pause.Exceptions) != 1 {
		t.Fatalf("threshold: %+v %v", paused.Execution, err)
	}
	if next, err := s.ClaimWithCapacity(9, "blocked-second", cap, now); err != nil || next != nil {
		t.Fatalf("dispatched after threshold: %+v %v", next, err)
	}
	cmd := repo.BatchCommand{Action: "resume", ExpectedRevision: paused.Revision, ExpectedPauseID: paused.Execution.Pause.ID,
		RequestKey: "resume-original", Digest: "stale", Tasks: []model.TaskRevision{{TaskID: first.Task.ID, Revision: first.Task.Revision}}}
	if _, err = s.ControlBatch(b.ID, 9, cmd, now); !errors.Is(err, repo.ErrPauseChanged) {
		t.Fatalf("accepted stale exception revision: %v", err)
	}
	cmd.Tasks[0] = paused.Execution.Pause.Exceptions[0].TaskRevision
	cmd.Digest = "correct"
	resumed, err := s.ControlBatch(b.ID, 9, cmd, now)
	if err != nil || resumed.ExecutionStatus != model.BatchRunning {
		t.Fatalf("acknowledged continue: %+v %v", resumed, err)
	}
	replay, err := s.ControlBatch(b.ID, 9, cmd, now.Add(time.Second))
	if err != nil || !reflect.DeepEqual(replay, resumed) {
		t.Fatalf("resume replay: %+v %v", replay, err)
	}
	failOriginalAttempt(t, s, first, now)
	afterReplay, err := s.GetBatch(b.ID)
	if err != nil || afterReplay.Revision != resumed.Revision || afterReplay.ExecutionStatus != model.BatchRunning {
		t.Fatalf("repeat observation changed confirmed exception: %+v %v", afterReplay.Execution, err)
	}
	second, err := s.ClaimWithCapacity(9, "batch-60-second", cap, now)
	if err != nil || second == nil || *second.ExecutionGroupNo != *first.ExecutionGroupNo {
		t.Fatalf("remaining original group member: %+v %v", second, err)
	}
	failOriginalAttempt(t, s, second, now)
	again, err := s.GetBatch(b.ID)
	if err != nil || again.Execution.Pause == nil || again.Execution.Pause.ID == cmd.ExpectedPauseID || len(again.Execution.Pause.Exceptions) != 2 {
		t.Fatalf("new exception did not advance pause fence: %+v %v", again.Execution, err)
	}
	cmd.RequestKey, cmd.Digest, cmd.ExpectedRevision = "new-key", "old-pause", again.Revision
	if _, err = s.ControlBatch(b.ID, 9, cmd, now); !errors.Is(err, repo.ErrPauseChanged) {
		t.Fatalf("stale pause identity accepted after new exception: %v", err)
	}
}

func TestBatchBetweenGroupPolicyWaitsForEveryOriginalMember(t *testing.T) {
	s, b, now := testExecutableBatch(t, 3, 10, "pause_on_exception")
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	first, err := s.ClaimWithCapacity(9, "first", cap, now)
	if err != nil || first == nil {
		t.Fatalf("first: %+v %v", first, err)
	}
	failOriginalAttempt(t, s, first, now)
	before, err := s.GetBatch(b.ID)
	if err != nil || before.ExecutionStatus != model.BatchRunning || before.GroupEndedAt != nil {
		t.Fatalf("paused mid-group: %+v %v", before.Execution, err)
	}
	second, err := s.ClaimWithCapacity(9, "second", cap, now)
	if err != nil || second == nil {
		t.Fatalf("second: %+v %v", second, err)
	}
	failOriginalAttempt(t, s, second, now.Add(time.Second))
	after, err := s.GetBatch(b.ID)
	if err != nil || after.ExecutionStatus != model.BatchPaused || after.GroupEndedAt == nil ||
		after.Execution.Pause == nil || after.Execution.Pause.Kind != "group_exception" {
		t.Fatalf("did not pause between groups: %+v %v", after.Execution, err)
	}
	if third, err := s.ClaimWithCapacity(9, "third", cap, now.Add(time.Hour)); err != nil || third != nil {
		t.Fatalf("opened another group while paused: %+v %v", third, err)
	}
}

func TestBatchStopAndSubmitCompeteWithoutReplayedPermission(t *testing.T) {
	s, b, now := testExecutableBatch(t, 1, 10, "auto_continue")
	cap := model.ExecutionCapacity{AutomaticSlots: 5, WindowSlots: 10}
	c, err := s.ClaimWithCapacity(9, "race-claim", cap, now)
	if err != nil || c == nil {
		t.Fatalf("claim: %+v %v", c, err)
	}
	in := c.Attempt.Input
	r := model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStageReady, PlatformUID: in.PlatformUID, BitProfileID: in.BitProfileID,
		VideoSHA256: in.VideoSHA256, CoverSHA256: in.CoverSHA256, PortraitCoverSHA256: in.PortraitCoverSHA256}
	if _, err = s.Report(c.Task, 9, r, true, now); err != nil {
		t.Fatal(err)
	}
	b, err = s.GetBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	stopped := make(chan error, 1)
	type result struct {
		ack model.ExecutionFeedback
		err error
	}
	submitted := make(chan result, 1)
	r.Seq, r.Stage = 2, model.AttemptStageSubmitting
	go func() {
		<-start
		_, err := s.ControlBatch(b.ID, 9, repo.BatchCommand{Action: "stop", ExpectedRevision: b.Revision, Reason: "controlled stop race"}, now)
		stopped <- err
	}()
	go func() {
		<-start
		ack, err := s.Report(c.Task, 9, r, true, now)
		submitted <- result{ack, err}
	}()
	close(start)
	out := <-submitted
	if err = <-stopped; err != nil || out.err != nil {
		t.Fatalf("stop: %v submit: %+v", err, out)
	}
	history, err := s.Attempt(c.Task.ID)
	if err != nil || len(history) != 1 || (history[0].SubmitStartedAt != nil) != out.ack.AllowSubmit {
		t.Fatalf("permission and persisted fence diverged: %+v %v", history, err)
	}
	replay, err := s.Report(c.Task, 9, r, true, now)
	if err != nil || replay.AllowSubmit || !replay.StopRequested {
		t.Fatalf("post-stop replay: %+v %v", replay, err)
	}
}
