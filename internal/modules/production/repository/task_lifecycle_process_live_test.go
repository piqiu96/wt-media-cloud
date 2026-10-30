package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

// These gated tests run against the local MySQL. The capacity test re-execs
// the test binary as two independent claim processes; its own tasks are
// deleted and the capacity row restored afterwards. The dev stack worker
// must be stopped: another claimer would compete for the seeded tasks.
const composeClaimGate = "WT_MEDIA_TEST_COMPOSE_CLAIM_PROCESS"

func composeClaimSetup(t *testing.T) (store ComposeTaskStore, requestKey string, strategyID int64) {
	t.Helper()
	if os.Getenv(composeClaimGate) != "1" {
		t.Skipf("set %s=1 against an isolated local Cloud database with the dev worker stopped", composeClaimGate)
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	err = database.DB().Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN material_usages mu ON mu.user_id = s.created_by AND mu.team_id = s.team_id AND mu.status = 'active' JOIN materials m ON m.id = mu.material_id AND m.status = 'available' JOIN source_contents sc ON sc.id = m.source_content_id AND sc.category_id = s.category_id WHERE s.status = 'enabled' AND t.status = 'active' ORDER BY s.id LIMIT 1`).Row().Scan(&strategyID)
	if err != nil {
		t.Skipf("no active strategy with eligible creator material: %v", err)
	}
	requestKey = fmt.Sprintf("chg094-claim-%d", time.Now().UnixNano())
	return
}

func seedComposeTask(t *testing.T, requestKey string, strategyID int64) {
	t.Helper()
	result := database.DB().Exec(`INSERT INTO compose_tasks (team_id, category_id, trigger_type, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, request_key, created_by, created_at, updated_at) SELECT s.team_id, s.category_id, 'manual', mu.material_id, mu.id, s.id, t.code, s.compose_template_version, '{}', 'pending', ?, s.created_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN material_usages mu ON mu.user_id = s.created_by AND mu.team_id = s.team_id AND mu.status = 'active' JOIN materials m ON m.id = mu.material_id AND m.status = 'available' JOIN source_contents sc ON sc.id = m.source_content_id AND sc.category_id = s.category_id WHERE s.id = ? ORDER BY mu.id LIMIT 1`, requestKey, strategyID)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("seed task %s: rows=%d err=%v", requestKey, result.RowsAffected, result.Error)
	}
}

func cleanupComposeTasks(t *testing.T, requestKey string) {
	t.Helper()
	t.Cleanup(func() {
		queries := []string{
			`DELETE ca FROM compose_task_attempts ca JOIN compose_tasks ct ON ca.task_id = ct.id WHERE ct.request_key LIKE ?`,
			`DELETE FROM compose_tasks WHERE request_key LIKE ?`,
		}
		for _, query := range queries {
			if err := database.DB().Exec(query, requestKey+"%").Error; err != nil {
				t.Errorf("claim cleanup: %v", err)
			}
		}
	})
}

func setComposeCapacity(t *testing.T, limit int) {
	t.Helper()
	var original int
	if err := database.DB().Raw(`SELECT max_running FROM compose_worker_capacity WHERE id = 1`).Row().Scan(&original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.DB().Exec(`UPDATE compose_worker_capacity SET max_running = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = 1`, original).Error; err != nil {
			t.Errorf("restore capacity: %v", err)
		}
	})
	if err := database.DB().Exec(`UPDATE compose_worker_capacity SET max_running = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = 1`, limit).Error; err != nil {
		t.Fatal(err)
	}
}

// The retry timestamp is a database fact. Two claimers with independent
// connections see the same eligibility, including after a process restart.
func TestLiveComposeRetryEligibilityPersists(t *testing.T) {
	_, requestKey, strategyID := composeClaimSetup(t)
	cleanupComposeTasks(t, requestKey)
	seedComposeTask(t, requestKey, strategyID)
	now := time.Now().UTC().Truncate(time.Second)
	future := now.Add(2 * time.Minute)
	if err := database.DB().Exec(`UPDATE compose_tasks SET next_attempt_at = ? WHERE request_key = ?`, future, requestKey).Error; err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE request_key = ? AND status = 'pending' AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= ?)`, requestKey, now).Row().Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("future retry was claimable: id=%d err=%v", id, err)
	}
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE request_key = ? AND status = 'pending' AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= ?)`, requestKey, future).Row().Scan(&id); err != nil || id <= 0 {
		t.Fatalf("due retry was not claimable: id=%d err=%v", id, err)
	}
}

func TestLiveComposeUncertainReviewIsAtomicAndSingleUse(t *testing.T) {
	_, requestKey, strategyID := composeClaimSetup(t)
	cleanupComposeTasks(t, requestKey)
	t.Cleanup(func() {
		if err := database.DB().Exec(`DELETE r FROM compose_uncertain_reviews r JOIN compose_tasks ct ON r.task_id = ct.id WHERE ct.request_key LIKE ?`, requestKey+"%").Error; err != nil {
			t.Errorf("review cleanup: %v", err)
		}
	})
	seedComposeTask(t, requestKey+"-eligible", strategyID)
	seedComposeTask(t, requestKey+"-output", strategyID)
	var eligibleID, outputID, actorID int64
	if err := database.DB().Raw(`SELECT id, created_by FROM compose_tasks WHERE request_key = ?`, requestKey+"-eligible").Row().Scan(&eligibleID, &actorID); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE request_key = ?`, requestKey+"-output").Row().Scan(&outputID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{eligibleID, outputID} {
		if err := database.DB().Exec(`UPDATE compose_tasks SET status = 'failed', error_code = 'execution_uncertain', attempt_count = 1 WHERE id = ?`, id).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 1, 'auto', 'failed', 'execution_uncertain', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.DB().Exec(`UPDATE compose_tasks SET output_object_key = 'compose-outputs/uncertain-already-committed.mp4' WHERE id = ?`, outputID).Error; err != nil {
		t.Fatal(err)
	}
	evidence := "核对旧执行节点的 FFmpeg 进程已退出，且数据库和存储中未见已提交成片"
	blocked, changed, err := resolveComposeUncertain(database.DB(), outputID, actorID, evidence, time.Now())
	if err != nil || changed || blocked.ID != 0 {
		t.Fatalf("output guard: review=%+v changed=%v err=%v", blocked, changed, err)
	}
	review, changed, err := resolveComposeUncertain(database.DB(), eligibleID, actorID, evidence, time.Now())
	if err != nil || !changed || review.TaskID != eligibleID || review.Attempt != 1 || review.Evidence != evidence {
		t.Fatalf("review=%+v changed=%v err=%v", review, changed, err)
	}
	if _, changed, err := resolveComposeUncertain(database.DB(), eligibleID, actorID, evidence, time.Now()); err != nil || changed {
		t.Fatalf("duplicate review changed=%v err=%v", changed, err)
	}
	var status string
	var count, maxAttempts int
	if err := database.DB().Raw(`SELECT status, max_attempts FROM compose_tasks WHERE id = ?`, eligibleID).Row().Scan(&status, &maxAttempts); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || maxAttempts != 4 {
		t.Fatalf("task after review: status=%s max_attempts=%d", status, maxAttempts)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_task_attempts WHERE task_id = ? AND attempt = 2 AND source = 'manual' AND outcome = 'pending'`, eligibleID).Row().Scan(&count); err != nil || count != 1 {
		t.Fatalf("manual attempt: count=%d err=%v", count, err)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_uncertain_reviews WHERE task_id = ?`, eligibleID).Row().Scan(&count); err != nil || count != 1 {
		t.Fatalf("review count=%d err=%v", count, err)
	}
	reviews, err := (ComposeTaskStore{}).ListUncertainReviews(eligibleID)
	if err != nil || len(reviews) != 1 || reviews[0].ID != review.ID || reviews[0].Evidence != evidence {
		t.Fatalf("review readback=%+v err=%v", reviews, err)
	}
	candidates, err := (ComposeTaskStore{}).FindStagingCleanupCandidates(100)
	if err != nil {
		t.Fatal(err)
	}
	foundReviewed := false
	for _, candidate := range candidates {
		if candidate.TaskID == outputID {
			t.Fatal("unreviewed uncertain attempt became cleanup eligible")
		}
		if candidate.TaskID == eligibleID && candidate.Attempt == 1 {
			foundReviewed = true
		}
	}
	if !foundReviewed {
		t.Fatal("reviewed uncertain attempt was not eligible for staging cleanup")
	}
	if ok, err := (ComposeTaskStore{}).MarkStagingCleaned(eligibleID, 1); err != nil || !ok {
		t.Fatalf("mark reviewed staging cleaned: ok=%v err=%v", ok, err)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM audit_logs WHERE target_type = 'compose_task' AND target_id = ? AND action = 'compose_task.resolve_uncertain'`, fmt.Sprint(eligibleID)).Row().Scan(&count); err != nil || count != 1 {
		t.Fatalf("review audit count=%d err=%v", count, err)
	}
	t.Cleanup(func() {
		_ = database.DB().Exec(`DELETE FROM audit_logs WHERE target_type = 'compose_task' AND target_id = ? AND action = 'compose_task.resolve_uncertain'`, fmt.Sprint(eligibleID)).Error
	})
}

func TestLiveComposeUncertainReviewConcurrentSingleWinner(t *testing.T) {
	_, requestKey, strategyID := composeClaimSetup(t)
	cleanupComposeTasks(t, requestKey)
	seedComposeTask(t, requestKey+"-race", strategyID)
	var taskID, actorID int64
	if err := database.DB().Raw(`SELECT id, created_by FROM compose_tasks WHERE request_key = ?`, requestKey+"-race").Row().Scan(&taskID, &actorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.DB().Exec(`DELETE FROM audit_logs WHERE target_type = 'compose_task' AND target_id = ? AND action = 'compose_task.resolve_uncertain'`, fmt.Sprint(taskID)).Error
		_ = database.DB().Exec(`DELETE FROM compose_uncertain_reviews WHERE task_id = ?`, taskID).Error
	})
	if err := database.DB().Exec(`UPDATE compose_tasks SET status = 'failed', error_code = 'execution_uncertain', attempt_count = 1 WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 1, 'auto', 'failed', 'execution_uncertain', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	type result struct {
		changed bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, changed, err := resolveComposeUncertain(database.DB(), taskID, actorID, "管理员核实旧 FFmpeg 已停止且没有已提交成片", time.Now())
			results <- result{changed: changed, err: err}
		}()
	}
	close(start)
	winners := 0
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.changed {
				winners++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent review did not finish")
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent winners=%d, want 1", winners)
	}
	var reviews, attempts int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_uncertain_reviews WHERE task_id = ?`, taskID).Row().Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_task_attempts WHERE task_id = ? AND attempt = 2`, taskID).Row().Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if reviews != 1 || attempts != 1 {
		t.Fatalf("concurrent readback: reviews=%d attempts=%d", reviews, attempts)
	}
}

func TestLiveComposeStagingCleanupCandidates(t *testing.T) {
	store, requestKey, strategyID := composeClaimSetup(t)
	cleanupComposeTasks(t, requestKey)
	seedComposeTask(t, requestKey, strategyID)
	var taskID int64
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE request_key = ?`, requestKey).Row().Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`UPDATE compose_tasks SET status = 'failed', attempt_count = 2, error_code = 'execution_uncertain' WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 1, 'auto', 'failed', 'storage_error', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 2, 'auto', 'failed', 'execution_uncertain', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	items, err := store.FindStagingCleanupCandidates(100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.TaskID == taskID && item.Attempt == 1 {
			found = true
		}
		if item.TaskID == taskID && item.Attempt == 2 {
			t.Fatal("uncertain execution must not be cleaned automatically")
		}
	}
	if !found {
		t.Fatalf("known failed attempt %d missing from cleanup candidates", taskID)
	}
	if ok, err := store.MarkStagingCleaned(taskID, 1); err != nil || !ok {
		t.Fatalf("mark cleaned: ok=%v err=%v", ok, err)
	}
	items, err = store.FindStagingCleanupCandidates(100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.TaskID == taskID {
			t.Fatalf("cleaned attempt %d still selected", taskID)
		}
	}
}

// TestHelperComposeClaimProcess is the claim loop executed as an independent
// process by TestLiveComposeClaimCapacityAcrossProcesses; it claims until the
// capacity gate or the queue runs dry and reports the claimed ids to a file.
func TestHelperComposeClaimProcess(t *testing.T) {
	if os.Getenv("WT_MEDIA_COMPOSE_CLAIM_HELPER") != "1" {
		t.Skip("claim helper for TestLiveComposeClaimCapacityAcrossProcesses")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	node, err := strconv.ParseInt(os.Getenv("WT_MEDIA_CLAIM_NODE"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	var claimed []string
	for {
		task, found, err := (ComposeTaskStore{}).ClaimNextComposeTask(node, 5*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
		claimed = append(claimed, strconv.FormatInt(task.ID, 10))
	}
	if err := os.WriteFile(os.Getenv("WT_MEDIA_CLAIM_OUT"), []byte(strings.Join(claimed, ",")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLiveComposeClaimCapacityAcrossProcesses(t *testing.T) {
	_, requestKey, strategyID := composeClaimSetup(t)
	cleanupComposeTasks(t, requestKey)
	for i := 1; i <= 4; i++ {
		seedComposeTask(t, fmt.Sprintf("%s-%d", requestKey, i), strategyID)
	}
	setComposeCapacity(t, 2)

	claim := func(node int64) (string, error) {
		out := filepath.Join(t.TempDir(), "claimed.txt")
		root, err := filepath.Abs("../../../..")
		if err != nil {
			return "", err
		}
		command := exec.Command("go", "test", "./internal/modules/production/repository", "-run", "TestHelperComposeClaimProcess", "-count=1")
		command.Dir = root
		command.Env = append(os.Environ(),
			"WT_MEDIA_COMPOSE_CLAIM_HELPER=1",
			"WT_MEDIA_CLAIM_NODE="+strconv.FormatInt(node, 10),
			"WT_MEDIA_CLAIM_OUT="+out,
		)
		if output, err := command.CombinedOutput(); err != nil {
			return "", fmt.Errorf("claim process %d: %w\n%s", node, err, output)
		}
		ids, err := os.ReadFile(out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(ids)), nil
	}
	type claimResult struct {
		node int64
		ids  string
		err  error
	}
	results := make(chan claimResult, 2)
	go func() {
		ids, err := claim(900001)
		results <- claimResult{900001, ids, err}
	}()
	go func() {
		ids, err := claim(900002)
		results <- claimResult{900002, ids, err}
	}()
	first, second := claimResult{}, claimResult{}
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.node == 900001 {
			first = result
		} else {
			second = result
		}
	}

	var running, pending, total int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_tasks WHERE request_key LIKE ? AND status = 'running'`, requestKey+"%").Row().Scan(&running); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_tasks WHERE request_key LIKE ? AND status = 'pending'`, requestKey+"%").Row().Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_tasks WHERE request_key LIKE ?`, requestKey+"%").Row().Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		statuses, _ := database.DB().Raw(`SELECT status, COUNT(*) FROM compose_tasks WHERE request_key LIKE ? GROUP BY status`, requestKey+"%").Rows()
		defer statuses.Close()
		for statuses.Next() {
			var status string
			var count int
			_ = statuses.Scan(&status, &count)
			t.Logf("seed status %s=%d", status, count)
		}
	}
	claimedByNode := map[int64]map[int64]bool{}
	for _, result := range []claimResult{first, second} {
		node := result.node
		claimedByNode[node] = map[int64]bool{}
		if result.ids == "" {
			continue
		}
		for _, id := range strings.Split(result.ids, ",") {
			taskID, err := strconv.ParseInt(id, 10, 64)
			if err != nil {
				t.Fatalf("process %d reported %q: %v", node, id, err)
			}
			claimedByNode[node][taskID] = true
		}
	}
	if len(claimedByNode[900001])+len(claimedByNode[900002]) != 2 {
		t.Fatalf("two processes must claim exactly the capacity limit: node1=%v node2=%v", first, second)
	}
	if total != 4 || running != 2 || pending != 2 {
		statuses, _ := database.DB().Raw(`SELECT id, status, claimed_node_id, attempt_count FROM compose_tasks WHERE request_key LIKE ? ORDER BY id`, requestKey+"%").Rows()
		for statuses.Next() {
			var id, attempts int64
			var status string
			var node any
			_ = statuses.Scan(&id, &status, &node, &attempts)
			t.Logf("seed task id=%d status=%s node=%v attempts=%d", id, status, node, attempts)
		}
		statuses.Close()
		t.Fatalf("total=%d running=%d pending=%d node1=%q node2=%q", total, running, pending, first.ids, second.ids)
	}
	for node, ids := range claimedByNode {
		for taskID := range ids {
			var owner int64
			if err := database.DB().Raw(`SELECT claimed_node_id FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&owner); err != nil {
				t.Fatal(err)
			}
			if owner != node {
				t.Fatalf("task %d owned by node %d, claimed by %d", taskID, owner, node)
			}
		}
	}
	t.Logf("independent claim processes: node1=%q node2=%q running=%d pending=%d", first.ids, second.ids, running, pending)
}

func TestLiveComposeOldGenerationWritesAreRejected(t *testing.T) {
	store, requestKey, strategyID := composeClaimSetup(t)
	cleanupComposeTasks(t, requestKey)
	seedComposeTask(t, requestKey+"-gen", strategyID)
	var taskID int64
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE request_key = ?`, requestKey+"-gen").Row().Scan(&taskID); err != nil {
		t.Fatal(err)
	}

	lost, found, err := store.ClaimNextComposeTask(900101, 5*time.Minute)
	if err != nil || !found || lost.ID != taskID || lost.AttemptCount != 1 {
		t.Fatalf("first claim: task=%+v found=%v err=%v", lost, found, err)
	}
	if ok, err := store.ReleaseComposeForRetry(lost.ID, 900101, lost.AttemptCount, "chg094_test", "requeued for generation test"); err != nil || !ok {
		t.Fatalf("release: ok=%v err=%v", ok, err)
	}
	// The next generation is intentionally ineligible until its persisted
	// backoff expires. Advance only this test row's deadline before claiming.
	if err := database.DB().Exec(`UPDATE compose_tasks SET next_attempt_at = UTC_TIMESTAMP(6) WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	alive, found, err := store.ClaimNextComposeTask(900101, 5*time.Minute)
	if err != nil || !found {
		t.Fatalf("second claim: found=%v err=%v", found, err)
	}
	if alive.ID != taskID || alive.AttemptCount != 2 {
		t.Fatalf("second claim must be the same task at attempt 2: task=%+v", alive)
	}

	out := model.CompositeOutput{TeamID: alive.TeamID, CategoryID: alive.CategoryID, MaterialID: alive.MaterialID, MaterialUsageID: alive.MaterialUsageID, TaskID: alive.ID, StrategyID: alive.StrategyID, TemplateCode: alive.TemplateCode, TemplateVersion: alive.TemplateVersion, ObjectKey: "outputs/" + requestKey + ".mp4", FileSizeBytes: 1, SHA256: "deadbeef", Media: []byte(`{}`), PublishStatus: "pending_publish", CreatedBy: alive.CreatedBy}
	closing := model.ComposeTaskAttemptStep{Step: model.ComposeStepCommit, StartedAt: time.Now(), FinishedAt: time.Now()}
	if _, err := store.CommitComposeOutput(out, out.ObjectKey, 900101, lost.AttemptCount, closing); !errors.Is(err, ErrComposeCommitLost) {
		t.Fatalf("old-generation commit must lose: %v", err)
	}
	if ok, err := store.RenewComposeLease(alive.ID, 900101, lost.AttemptCount, 5*time.Minute); err != nil || ok {
		t.Fatalf("old-generation renew must be refused: ok=%v err=%v", ok, err)
	}
	if ok, err := store.ReportComposeProgress(alive.ID, 900101, lost.AttemptCount, "cutting", 40); err != nil || ok {
		t.Fatalf("old-generation progress must be refused: ok=%v err=%v", ok, err)
	}
	if ok, err := store.FailComposeTask(alive.ID, 900101, lost.AttemptCount, "chg094_test", "old generation"); err != nil || ok {
		t.Fatalf("old-generation failure must be refused: ok=%v err=%v", ok, err)
	}
	if ok, err := store.ConfirmComposeCancel(alive.ID, 900101, lost.AttemptCount); err != nil || ok {
		t.Fatalf("old-generation cancel must be refused: ok=%v err=%v", ok, err)
	}
	if ok, err := store.RenewComposeLease(alive.ID, 900101, alive.AttemptCount, 5*time.Minute); err != nil || !ok {
		t.Fatalf("current generation renew: ok=%v err=%v", ok, err)
	}
	if ok, err := store.ReportComposeProgress(alive.ID, 900101, alive.AttemptCount, "cutting", 40); err != nil || !ok {
		t.Fatalf("current generation progress: ok=%v err=%v", ok, err)
	}
	if ok, err := store.FailComposeTask(alive.ID, 900101, alive.AttemptCount, "chg094_test", "generation test done"); err != nil || !ok {
		t.Fatalf("current generation failure: ok=%v err=%v", ok, err)
	}
	var attempts int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_task_attempts WHERE task_id = ?`, taskID).Row().Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempt rows=%d, want 2", attempts)
	}
	t.Logf("old generation (attempt 1) refused on the same node 900101 that now owns attempt 2 on task %d", taskID)
}
