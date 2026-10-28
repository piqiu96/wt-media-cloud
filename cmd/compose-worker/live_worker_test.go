package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/jobs"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

// These gated tests run the real compose-worker binary against the local MySQL,
// object storage and FFmpeg. The dev stack's other processes must be stopped:
// a second claimer competes for the seeded tasks. The tests seed their own
// tasks (cloned from a real successful run) and delete them afterwards.
const liveWorkerGate = "WT_MEDIA_TEST_COMPOSE_WORKER_PROCESS"

func liveWorkerSetup(t *testing.T) (binary, requestKey string) {
	t.Helper()
	if os.Getenv(liveWorkerGate) != "1" {
		t.Skipf("set %s=1 against the local Cloud database, object storage and FFmpeg", liveWorkerGate)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
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
	built := filepath.Join(t.TempDir(), "compose-worker")
	build := exec.Command("go", "build", "-o", built, "./cmd/compose-worker")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build compose-worker: %v\n%s", err, output)
	}
	requestKey = fmt.Sprintf("chg094-worker-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		queries := []string{
			`DELETE o FROM composite_outputs o JOIN compose_tasks ct ON o.compose_task_id = ct.id WHERE ct.request_key LIKE ?`,
			`DELETE ca FROM compose_task_attempts ca JOIN compose_tasks ct ON ca.task_id = ct.id WHERE ct.request_key LIKE ?`,
			`DELETE FROM compose_tasks WHERE request_key LIKE ?`,
		}
		for _, query := range queries {
			if err := database.DB().Exec(query, requestKey+"%").Error; err != nil {
				t.Errorf("worker live cleanup: %v", err)
			}
		}
	})
	return built, requestKey
}

func startWorkerProcess(t *testing.T, binary string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	command := exec.Command(binary)
	command.Env = os.Environ()
	output := &bytes.Buffer{}
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	return command, output
}

func stopWorkerProcess(t *testing.T, command *exec.Cmd, output *bytes.Buffer) {
	t.Helper()
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Fatal("compose worker did not stop on SIGTERM")
	}
	// A controlled stop must exit cleanly: a panic here means the process's
	// own logging (or teardown) is broken, and its evidence is not readable.
	if strings.Contains(output.String(), "panic") {
		t.Fatalf("worker panicked on SIGTERM: %s", output.String())
	}
}

func waitForTaskStage(t *testing.T, taskID int64, stage string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var current string
		if err := database.DB().Raw(`SELECT stage FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&current); err != nil {
			t.Fatal(err)
		}
		if current == stage {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("task %d did not reach stage %s in time", taskID, stage)
}

func waitForTaskStatus(t *testing.T, taskID int64, status string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var current string
		if err := database.DB().Raw(`SELECT status FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&current); err != nil {
			t.Fatal(err)
		}
		if current == status {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	var row struct {
		status, stage, errorCode, errorMessage string
		attempts                               int
		materialID                             int64
	}
	if err := database.DB().Raw(`SELECT status, stage, error_code, error_message, attempt_count, material_id FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&row.status, &row.stage, &row.errorCode, &row.errorMessage, &row.attempts, &row.materialID); err == nil {
		var videoStatus string
		_ = database.DB().Raw(`SELECT video_status FROM materials WHERE id = ?`, row.materialID).Row().Scan(&videoStatus)
		t.Fatalf("task %d did not reach %s in time: status=%s stage=%s attempts=%d error=%s/%s material=%d video_status=%s", taskID, status, row.status, row.stage, row.attempts, row.errorCode, row.errorMessage, row.materialID, videoStatus)
	}
	t.Fatalf("task %d did not reach %s in time", taskID, status)
}

// cloneSuccessfulTask re-runs the deployment's latest successful basic-cut
// compose as a fresh pending task: the snapshot it carries is one a real run
// already accepted, so the clone exercises the worker, not the template
// registry. Guide templates are skipped — their guide URL is a per-use
// presigned object that expires with the task that used it.
func cloneSuccessfulTask(t *testing.T, requestKey string) int64 {
	t.Helper()
	var source int64
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE status = 'success' AND template_code = 'basic_cut' ORDER BY id DESC LIMIT 1`).Row().Scan(&source); err != nil {
		t.Skipf("no successful basic-cut compose task to clone: %v", err)
	}
	result := database.DB().Exec(`INSERT INTO compose_tasks (team_id, category_id, trigger_type, material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, status, request_key, created_by, created_at, updated_at) SELECT team_id, category_id, 'manual', material_id, material_usage_id, compose_strategy_id, template_code, template_version, config_snapshot_json, 'pending', ?, created_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_tasks WHERE id = ?`, requestKey, source)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("clone task %d: rows=%d err=%v", source, result.RowsAffected, result.Error)
	}
	var taskID int64
	if err := database.DB().Raw(`SELECT id FROM compose_tasks WHERE request_key = ?`, requestKey).Row().Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	return taskID
}

func TestLiveComposeWorkerRefusesCapacityMismatch(t *testing.T) {
	binary, _ := liveWorkerSetup(t)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	var configured int
	if err := database.DB().Raw(`SELECT max_running FROM compose_worker_capacity WHERE id = 1`).Row().Scan(&configured); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.DB().Exec(`UPDATE compose_worker_capacity SET max_running = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = 1`, configured).Error; err != nil {
			t.Errorf("restore capacity: %v", err)
		}
	})
	if err := database.DB().Exec(`UPDATE compose_worker_capacity SET max_running = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = 1`, cfg.Compose.Worker.GlobalConcurrency+1).Error; err != nil {
		t.Fatal(err)
	}

	command, output := startWorkerProcess(t, binary)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("worker started despite the capacity mismatch: %s", output.String())
		}
		if !strings.Contains(output.String(), "differs from database capacity") {
			t.Fatalf("refusal must name the capacity mismatch: %s", output.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("worker did not refuse to start: %s", output.String())
	}
}

func TestLiveComposeWorkerRunsARealTaskToSuccess(t *testing.T) {
	binary, requestKey := liveWorkerSetup(t)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		t.Skipf("object storage not reachable: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	taskID := cloneSuccessfulTask(t, requestKey+"-success")
	command, output := startWorkerProcess(t, binary)
	waitForTaskStatus(t, taskID, "success", 12*time.Minute)
	stopWorkerProcess(t, command, output)

	var outputID int64
	var objectKey string
	if err := database.DB().Raw(`SELECT composite_output_id, output_object_key FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&outputID, &objectKey); err != nil {
		t.Fatal(err)
	}
	if outputID == 0 || objectKey == "" {
		t.Fatalf("a success task must carry its output: id=%d key=%q", outputID, objectKey)
	}
	if _, err := storage.Get().Stat(t.Context(), objectKey); err != nil {
		t.Fatalf("formal object %s must exist: %v", objectKey, err)
	}
	t.Cleanup(func() { _ = storage.Get().Remove(t.Context(), objectKey) })

	entries, err := os.ReadDir(cfg.Compose.Worker.WorkspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	// Only this task's own run directory is this test's business: the root may
	// hold debris from worker processes killed by operators, which nothing can
	// reclaim.
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "task-"+strconv.FormatInt(taskID, 10)+"-") {
			t.Fatalf("run directory left behind: %s", entry.Name())
		}
	}
	t.Logf("real worker ran task %d to success: output %d at %s; worker log: %s", taskID, outputID, objectKey, output.String())
}

func TestLiveComposeWorkerHonorsRetryDeadlineAcrossRestart(t *testing.T) {
	binary, requestKey := liveWorkerSetup(t)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		t.Skipf("object storage not reachable: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	taskID := cloneSuccessfulTask(t, requestKey+"-backoff")
	future := time.Now().UTC().Add(30 * time.Second)
	if err := database.DB().Exec(`UPDATE compose_tasks SET attempt_count = 1, next_attempt_at = ?, error_code = 'storage_error' WHERE id = ?`, future, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 1, 'auto', 'failed', 'storage_error', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	assertWaiting := func() {
		t.Helper()
		var status string
		var attempts int
		if err := database.DB().Raw(`SELECT status, attempt_count FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&status, &attempts); err != nil {
			t.Fatal(err)
		}
		if status != "pending" || attempts != 1 {
			t.Fatalf("retry escaped deadline: status=%s attempts=%d", status, attempts)
		}
	}
	first, firstOutput := startWorkerProcess(t, binary)
	time.Sleep(2 * time.Second)
	assertWaiting()
	stopWorkerProcess(t, first, firstOutput)
	second, secondOutput := startWorkerProcess(t, binary)
	time.Sleep(time.Second)
	assertWaiting()
	if err := database.DB().Exec(`UPDATE compose_tasks SET next_attempt_at = UTC_TIMESTAMP(6) WHERE id = ? AND status = 'pending'`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	waitForTaskStatus(t, taskID, "success", 3*time.Minute)
	stopWorkerProcess(t, second, secondOutput)
	var attempts int
	var outputKey string
	if err := database.DB().Raw(`SELECT attempt_count, output_object_key FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&attempts, &outputKey); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || outputKey != fmt.Sprintf("compose-outputs/%d/2/output.mp4", taskID) {
		t.Fatalf("retry result: attempts=%d key=%q", attempts, outputKey)
	}
	if _, err := storage.Get().Stat(t.Context(), outputKey); err != nil {
		t.Fatalf("retry formal output missing: %v", err)
	}
	t.Cleanup(func() { _ = storage.Get().Remove(t.Context(), outputKey) })
	stagingKey := fmt.Sprintf("compose-staging/%d/2/output.mp4", taskID)
	if _, err := storage.Get().Stat(t.Context(), stagingKey); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("retry staging object survived: err=%v", err)
	}
	t.Logf("task %d held attempt 1 across worker restart, then completed attempt 2 at %s", taskID, outputKey)
}

func TestLiveComposeStagingCleanupRecoversStorageFailure(t *testing.T) {
	_, requestKey := liveWorkerSetup(t)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		t.Skipf("object storage not reachable: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	taskID := cloneSuccessfulTask(t, requestKey+"-cleanup")
	if err := database.DB().Exec(`UPDATE compose_tasks SET status = 'failed', attempt_count = 1, error_code = 'storage_error' WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 1, 'auto', 'failed', 'storage_error', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("compose-staging/%d/1/output.mp4", taskID)
	body := []byte("orphaned staged output")
	if err := storage.Get().Put(t.Context(), key, bytes.NewReader(body), int64(len(body)), "video/mp4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Get().Remove(t.Context(), key) })
	store := repository.ComposeTaskStore{}
	if err := jobs.CleanComposeStaging(t.Context(), store, func(context.Context, string) error { return errors.New("temporary storage outage") }); err == nil {
		t.Fatal("fault injection did not report cleanup failure")
	}
	var count int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_task_attempts WHERE task_id = ? AND attempt = 1 AND staging_cleaned_at IS NULL`, taskID).Row().Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed cleanup lost retry marker: count=%d err=%v", count, err)
	}
	if _, err := storage.Get().Stat(t.Context(), key); err != nil {
		t.Fatalf("fault injection removed object unexpectedly: %v", err)
	}
	if err := jobs.CleanComposeStaging(t.Context(), store, storage.Get().Remove); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get().Stat(t.Context(), key); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("staged object survived recovery: %v", err)
	}
	var cleaned sql.NullTime
	if err := database.DB().Raw(`SELECT staging_cleaned_at FROM compose_task_attempts WHERE task_id = ? AND attempt = 1`, taskID).Row().Scan(&cleaned); err != nil || !cleaned.Valid {
		t.Fatalf("recovered deletion was not recorded: cleaned=%v err=%v", cleaned, err)
	}
	t.Logf("staging cleanup for task %d recovered after injected storage failure", taskID)
}

func TestLiveComposeUncertainReviewEnablesStagingCleanup(t *testing.T) {
	_, requestKey := liveWorkerSetup(t)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
		t.Skipf("object storage not reachable: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	taskID := cloneSuccessfulTask(t, requestKey+"-uncertain-cleanup")
	var actorID int64
	if err := database.DB().Raw(`SELECT created_by FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&actorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.DB().Exec(`DELETE FROM audit_logs WHERE target_type = 'compose_task' AND target_id = ? AND action = 'compose_task.resolve_uncertain'`, strconv.FormatInt(taskID, 10)).Error
		_ = database.DB().Exec(`DELETE FROM compose_uncertain_reviews WHERE task_id = ?`, taskID).Error
	})
	if err := database.DB().Exec(`UPDATE compose_tasks SET status = 'failed', attempt_count = 1, error_code = 'execution_uncertain' WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, error_code, created_at, updated_at) VALUES (?, 1, 'auto', 'failed', 'execution_uncertain', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("compose-staging/%d/1/output.mp4", taskID)
	body := []byte("staging from uncertain execution")
	if err := storage.Get().Put(t.Context(), key, bytes.NewReader(body), int64(len(body)), "video/mp4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Get().Remove(t.Context(), key) })
	store := repository.ComposeTaskStore{}
	if err := jobs.CleanComposeStaging(t.Context(), store, storage.Get().Remove); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get().Stat(t.Context(), key); err != nil {
		t.Fatalf("unreviewed object was removed: %v", err)
	}
	if _, changed, err := store.ResolveUncertain(taskID, actorID, "已在旧 Worker 节点核对 FFmpeg 进程停止且数据库无成片"); err != nil || !changed {
		t.Fatalf("review changed=%v err=%v", changed, err)
	}
	if err := jobs.CleanComposeStaging(t.Context(), store, storage.Get().Remove); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get().Stat(t.Context(), key); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("reviewed staging object survived: %v", err)
	}
	var cleaned sql.NullTime
	if err := database.DB().Raw(`SELECT staging_cleaned_at FROM compose_task_attempts WHERE task_id = ? AND attempt = 1`, taskID).Row().Scan(&cleaned); err != nil || !cleaned.Valid {
		t.Fatalf("reviewed attempt cleanup marker: %v err=%v", cleaned, err)
	}
	t.Logf("reviewed uncertain task %d allowed only its old staging key to be removed", taskID)
}

func TestLiveComposeWorkerConfirmsACancelWhileWaitingForInput(t *testing.T) {
	binary, requestKey := liveWorkerSetup(t)
	// A material whose source is not ready keeps the attempt in its wait loop
	// (the dev tree has no transfer worker to prepare it), so the cancel has a
	// stable window to land in.
	var materialID, usageID int64
	err := database.DB().Raw(`SELECT m.id, mu.id FROM materials m JOIN material_usages mu ON mu.material_id = m.id AND mu.status = 'active' WHERE m.video_status IN ('not_downloaded', 'downloading') AND m.status = 'available' ORDER BY m.id DESC LIMIT 1`).Row().Scan(&materialID, &usageID)
	if err != nil {
		t.Skipf("no unprepared material to seed the wait with: %v", err)
	}
	taskID := cloneSuccessfulTask(t, requestKey+"-cancel")
	if err := database.DB().Exec(`UPDATE compose_tasks SET material_id = ?, material_usage_id = ? WHERE id = ?`, materialID, usageID, taskID).Error; err != nil {
		t.Fatal(err)
	}

	command, output := startWorkerProcess(t, binary)
	waitForTaskStatus(t, taskID, "running", time.Minute)
	if err := database.DB().Exec(`UPDATE compose_tasks SET cancel_requested = 1 WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	waitForTaskStatus(t, taskID, "cancelled", time.Minute)
	stopWorkerProcess(t, command, output)

	var outputs int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM composite_outputs WHERE compose_task_id = ?`, taskID).Row().Scan(&outputs); err != nil {
		t.Fatal(err)
	}
	if outputs != 0 {
		t.Fatalf("a cancelled task must not carry an output: %d", outputs)
	}
	t.Logf("real worker confirmed the cancel of task %d while it waited for input", taskID)
}

// The cancel lands while FFmpeg is cutting: the cancel watcher must stop the
// run, confirm the row, and leave neither output nor run directory behind.
func TestLiveComposeWorkerCancelsMidCut(t *testing.T) {
	binary, requestKey := liveWorkerSetup(t)
	taskID := cloneSuccessfulTask(t, requestKey+"-midcut")
	command, output := startWorkerProcess(t, binary)
	waitForTaskStage(t, taskID, "cutting", 12*time.Minute)
	if err := database.DB().Exec(`UPDATE compose_tasks SET cancel_requested = 1 WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	waitForTaskStatus(t, taskID, "cancelled", time.Minute)
	stopWorkerProcess(t, command, output)

	var outputs int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM composite_outputs WHERE compose_task_id = ?`, taskID).Row().Scan(&outputs); err != nil {
		t.Fatal(err)
	}
	if outputs != 0 {
		t.Fatalf("a cancelled mid-cut task must not carry an output: %d", outputs)
	}
	entries, err := os.ReadDir(cfgWorkspaceRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "task-"+strconv.FormatInt(taskID, 10)+"-") {
			t.Fatalf("run directory left behind: %s", entry.Name())
		}
	}
	t.Logf("real worker stopped FFmpeg mid-cut and confirmed the cancel of task %d", taskID)
}

func cfgWorkspaceRoot(t *testing.T) string {
	t.Helper()
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Compose.Worker.WorkspaceRoot
}

// The floor is raised above the volume's free space: the real worker must
// pause claiming (the pending task stays pending) and say so in job.log.
func TestLiveComposeWorkerPausesClaimsWhenTheDiskFloorExceedsFreeSpace(t *testing.T) {
	binary, requestKey := liveWorkerSetup(t)
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join("../..", "config", "scheduler", "compose.toml")
	original, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	raised := bytes.Replace(original, []byte("min_free_bytes = 10737418240"), []byte("min_free_bytes = 1099511627776"), 1)
	if bytes.Equal(raised, original) {
		t.Fatal("compose.toml no longer carries the expected min_free_bytes line")
	}
	t.Cleanup(func() {
		if err := os.WriteFile(tomlPath, original, 0o644); err != nil {
			t.Errorf("restore compose.toml: %v", err)
		}
	})
	if err := os.WriteFile(tomlPath, raised, 0o644); err != nil {
		t.Fatal(err)
	}

	taskID := cloneSuccessfulTask(t, requestKey+"-paused")
	command, output := startWorkerProcess(t, binary)
	time.Sleep(15 * time.Second)
	stopWorkerProcess(t, command, output)

	var status string
	if err := database.DB().Raw(`SELECT status FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("a paused worker must not claim: task %d reads %s", taskID, status)
	}
	logs, err := os.ReadFile(cfg.Loggers.Job.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logs), "pausing claims") {
		t.Fatal("job.log must record the pause")
	}

	// Restore the floor and restart: the pending task must be claimable again.
	if err := os.WriteFile(tomlPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	command, output = startWorkerProcess(t, binary)
	waitForTaskStatus(t, taskID, "running", time.Minute)
	stopWorkerProcess(t, command, output)
	t.Logf("real worker paused below the raised floor (task %d pending), then claimed it after the floor was restored", taskID)
}

func TestLiveComposeWorkerQuarantinesAnExpiredLeaseAtStartup(t *testing.T) {
	binary, requestKey := liveWorkerSetup(t)
	taskID := cloneSuccessfulTask(t, requestKey+"-expired")
	if err := database.DB().Exec(`UPDATE compose_tasks SET status = 'running', claimed_node_id = 888, lease_expires_at = DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 HOUR), heartbeat_at = DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 HOUR), attempt_count = 1 WHERE id = ?`, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, started_at, created_at, updated_at) VALUES (?, 1, 'auto', 'running', DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 2 HOUR), UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, taskID).Error; err != nil {
		t.Fatal(err)
	}

	command, output := startWorkerProcess(t, binary)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var status, errorCode string
		if err := database.DB().Raw(`SELECT status, error_code FROM compose_tasks WHERE id = ?`, taskID).Row().Scan(&status, &errorCode); err != nil {
			t.Fatal(err)
		}
		if status == "failed" {
			if errorCode != "execution_uncertain" {
				t.Fatalf("an expired lease must be quarantined, got error %q", errorCode)
			}
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	stopWorkerProcess(t, command, output)
	var attempts int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_task_attempts WHERE task_id = ? AND outcome = 'failed'`, taskID).Row().Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("the expired attempt must read failed: %d", attempts)
	}
	t.Logf("startup recovery quarantined task %d as execution_uncertain without requeueing", taskID)
}
