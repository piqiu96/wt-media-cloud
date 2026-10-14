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
	err = database.DB().Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN material_usages mu ON mu.user_id = s.created_by AND mu.team_id = s.team_id AND mu.status = 'active' JOIN materials m ON m.id = mu.material_id AND m.status = 'available' JOIN source_contents sc ON sc.id = m.source_content_id AND sc.game_id = s.game_id WHERE s.status = 'active' AND t.status = 'active' ORDER BY s.id LIMIT 1`).Row().Scan(&strategyID)
	if err != nil {
		t.Skipf("no active strategy with eligible creator material: %v", err)
	}
	requestKey = fmt.Sprintf("chg094-claim-%d", time.Now().UnixNano())
	return
}

func seedComposeTask(t *testing.T, requestKey string, strategyID int64) {
	t.Helper()
	result := database.DB().Exec(`INSERT INTO compose_tasks (team_id, game_id, trigger_type, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, request_key, created_by, created_at, updated_at) SELECT s.team_id, s.game_id, 'manual', mu.material_id, mu.id, s.id, t.code, s.compose_template_version, '{}', 'pending', ?, s.created_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN material_usages mu ON mu.user_id = s.created_by AND mu.team_id = s.team_id AND mu.status = 'active' JOIN materials m ON m.id = mu.material_id AND m.status = 'available' JOIN source_contents sc ON sc.id = m.source_content_id AND sc.game_id = s.game_id WHERE s.id = ? ORDER BY mu.id LIMIT 1`, requestKey, strategyID)
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
	alive, found, err := store.ClaimNextComposeTask(900101, 5*time.Minute)
	if err != nil || !found {
		t.Fatalf("second claim: found=%v err=%v", found, err)
	}
	if alive.ID != taskID || alive.AttemptCount != 2 {
		t.Fatalf("second claim must be the same task at attempt 2: task=%+v", alive)
	}

	out := model.CompositeOutput{TeamID: alive.TeamID, GameID: alive.GameID, MaterialID: alive.MaterialID, MaterialUsageID: alive.MaterialUsageID, TaskID: alive.ID, StrategyID: alive.StrategyID, TemplateCode: alive.TemplateCode, TemplateVersion: alive.TemplateVersion, ObjectKey: "outputs/" + requestKey + ".mp4", FileSizeBytes: 1, SHA256: "deadbeef", Media: []byte(`{}`), PublishStatus: "pending_publish", CreatedBy: alive.CreatedBy}
	if _, err := store.CommitComposeOutput(out, out.ObjectKey, 900101, lost.AttemptCount); !errors.Is(err, ErrComposeCommitLost) {
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
