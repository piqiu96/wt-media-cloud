package repository

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"gorm.io/gorm"
)

// 这些用例跑在隔离的本地 MySQL 上，验证迁移 070 之后的真实读写；sqlmock
// 只能证明 SQL 形状，证明不了 CHECK 约束、唯一键和行锁行为。
const taskRuntimeGate = "WT_MEDIA_TEST_TASK_RUNTIME"

var runtimeUserSeq atomic.Int64

func taskRuntimeSetup(t *testing.T) {
	t.Helper()
	if os.Getenv(taskRuntimeGate) != "1" {
		t.Skipf("set %s=1 against an isolated local Cloud database after migration 070", taskRuntimeGate)
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
}

func seedRuntimeUser(t *testing.T, mainUserID string) int64 {
	t.Helper()
	username := fmt.Sprintf("chg101-runtime-%d-%d", time.Now().UnixNano(), runtimeUserSeq.Add(1))
	var userID int64
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO users (username, password_hash, role, status, bit_main_user_id, created_at, updated_at)
			VALUES (?, 'x', 'admin', 'enabled', NULLIF(?, ''), UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, username, mainUserID).Error; err != nil {
			return err
		}
		return tx.Raw(`SELECT LAST_INSERT_ID()`).Row().Scan(&userID)
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		for _, query := range []string{
			`DELETE FROM media_accounts WHERE user_id = ?`,
			`DELETE FROM browser_profiles WHERE user_id = ?`,
			`DELETE FROM tasks WHERE user_id = ?`,
			`DELETE FROM users WHERE id = ?`,
		} {
			if err := database.DB().Exec(query, userID).Error; err != nil {
				t.Errorf("cleanup user %d: %v", userID, err)
			}
		}
	})
	return userID
}

func seedRuntimeProfile(t *testing.T, userID int64, bitProfileID, mainUserID string) int64 {
	t.Helper()
	var profileID int64
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO browser_profiles
			(user_id, bit_profile_id, main_user_id, profile_user_id, name, seq, local_status, last_synced_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, '', 0, 'active', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
			userID, bitProfileID, mainUserID, mainUserID).Error; err != nil {
			return err
		}
		return tx.Raw(`SELECT LAST_INSERT_ID()`).Row().Scan(&profileID)
	})
	if err != nil {
		t.Fatalf("seed browser profile: %v", err)
	}
	return profileID
}

func seedRuntimeAccount(t *testing.T, userID int64) int64 {
	t.Helper()
	var accountID int64
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO media_accounts
			(user_id, platform, platform_account_id, name, created_at, updated_at)
			VALUES (?, 'douyin', ?, '', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
			userID, fmt.Sprintf("chg101-account-%d", time.Now().UnixNano())).Error; err != nil {
			return err
		}
		return tx.Raw(`SELECT LAST_INSERT_ID()`).Row().Scan(&accountID)
	})
	if err != nil {
		t.Fatalf("seed media account: %v", err)
	}
	return accountID
}

func mustCreateTask(t *testing.T, userID int64, taskType, idempotencyKey string, payload map[string]any) model.Task {
	t.Helper()
	task, err := CreateTask(CreateTaskInput{UserID: userID, TaskType: taskType, IdempotencyKey: idempotencyKey, Payload: payload})
	if err != nil {
		t.Fatalf("create %s: %v", taskType, err)
	}
	return task
}

func mustClaimTask(t *testing.T, userID int64) model.Task {
	t.Helper()
	task, err := ClaimTask(ClaimTaskInput{UserID: userID, AgentID: "chg101-test-agent"})
	if err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if task.AttemptID == "" {
		t.Fatalf("claimed task %s has no attempt_id", task.TaskID)
	}
	return task
}

func mustReportTask(t *testing.T, taskID string, input ReportTaskInput) model.Task {
	t.Helper()
	task, err := ReportTask(taskID, input)
	if err != nil {
		t.Fatalf("report %s: %v", taskID, err)
	}
	return task
}

func TestLiveTaskRuntimeLifecycleAndUserIsolation(t *testing.T) {
	taskRuntimeSetup(t)
	userA := seedRuntimeUser(t, "")
	userB := seedRuntimeUser(t, "")

	key := fmt.Sprintf("chg101-idem-%d", time.Now().UnixNano())
	taskA := mustCreateTask(t, userA, model.TaskTypeCookieRead.String(), key, map[string]any{"account_id": "1"})
	again := mustCreateTask(t, userA, model.TaskTypeCookieRead.String(), key, nil)
	if again.TaskID != taskA.TaskID {
		t.Fatalf("same user + same key created %s and %s", taskA.TaskID, again.TaskID)
	}
	taskB := mustCreateTask(t, userB, model.TaskTypeCookieRead.String(), key, nil)
	if taskB.TaskID == taskA.TaskID || taskB.UserID != userB {
		t.Fatalf("same key under another user returned %#v, want a new task owned by %d", taskB, userB)
	}

	claimedB := mustClaimTask(t, userB)
	if claimedB.TaskID != taskB.TaskID {
		t.Fatalf("user B claimed %s, want own task %s", claimedB.TaskID, taskB.TaskID)
	}
	claimed := mustClaimTask(t, userA)
	if claimed.TaskID != taskA.TaskID {
		t.Fatalf("user A claimed %s, want own task %s", claimed.TaskID, taskA.TaskID)
	}
	if claimed.Attempt != 1 || claimed.Status != model.TaskStatusLeased.String() || claimed.StartedAt == "" {
		t.Fatalf("claimed task = %#v, want attempt 1 leased with started_at", claimed)
	}
	if claimed.ExecutionTimeoutSeconds != 300 || claimed.ExternalSideEffect {
		t.Fatalf("cookie_read defaults = timeout %d side effect %v", claimed.ExecutionTimeoutSeconds, claimed.ExternalSideEffect)
	}

	if _, err := GetTask(taskB.TaskID, userA); !errors.Is(err, model.ErrTaskNotFound) {
		t.Fatalf("user A read user B task: err = %v, want ErrTaskNotFound", err)
	}
	if _, err := ReportTask(taskB.TaskID, ReportTaskInput{UserID: userA, AttemptID: claimedB.AttemptID, Status: "running"}); !errors.Is(err, model.ErrTaskNotFound) {
		t.Fatalf("user A reported user B task: err = %v, want ErrTaskNotFound", err)
	}
	if _, err := ReportTask(taskA.TaskID, ReportTaskInput{UserID: userA, AttemptID: "attempt-bogus", Status: "running"}); !errors.Is(err, model.ErrStaleAttempt) {
		t.Fatalf("wrong attempt: err = %v, want ErrStaleAttempt", err)
	}

	running := mustReportTask(t, taskA.TaskID, ReportTaskInput{UserID: userA, AttemptID: claimed.AttemptID, Status: "running", Progress: 10})
	if running.Status != model.TaskStatusRunning.String() {
		t.Fatalf("status after running report = %q", running.Status)
	}
	succeeded := mustReportTask(t, taskA.TaskID, ReportTaskInput{UserID: userA, AttemptID: claimed.AttemptID, Status: "succeeded", Progress: 100, Result: map[string]any{"ok": true}})
	if succeeded.Status != model.TaskStatusSucceeded.String() || succeeded.SettledBy != "agent" || succeeded.FinishedAt == "" {
		t.Fatalf("terminal report = %#v", succeeded)
	}
	repeated := mustReportTask(t, taskA.TaskID, ReportTaskInput{UserID: userA, AttemptID: claimed.AttemptID, Status: "succeeded", Progress: 100, Result: map[string]any{"ok": true}})
	if repeated.Status != model.TaskStatusSucceeded.String() {
		t.Fatalf("duplicate terminal report changed status to %q", repeated.Status)
	}
	if _, err := RetryTask(taskA.TaskID, RetryTaskInput{UserID: userA}); !errors.Is(err, model.ErrTaskNotRetryable) {
		t.Fatalf("retry succeeded task: err = %v, want ErrTaskNotRetryable", err)
	}
	if _, err := ClaimTask(ClaimTaskInput{UserID: userA, AgentID: "chg101-test-agent"}); !errors.Is(err, model.ErrNoPendingTask) {
		t.Fatalf("claim with no pending task: err = %v, want ErrNoPendingTask", err)
	}

	if _, err := CreateTask(CreateTaskInput{UserID: userA, TaskType: "unknown_task"}); !errors.Is(err, model.ErrUnsupportedTaskType) {
		t.Fatalf("unknown type: err = %v, want ErrUnsupportedTaskType", err)
	}
	if _, err := CreateTask(CreateTaskInput{UserID: userA, TaskType: ""}); !errors.Is(err, model.ErrUnsupportedTaskType) {
		t.Fatalf("empty type: err = %v, want ErrUnsupportedTaskType", err)
	}
	if _, err := CreateTask(CreateTaskInput{UserID: 0, TaskType: model.TaskTypeNoop.String()}); !errors.Is(err, model.ErrInvalidTask) {
		t.Fatalf("missing user: err = %v, want ErrInvalidTask", err)
	}
	profileOpen := mustCreateTask(t, userA, model.TaskTypeProfileOpen.String(), "", nil)
	if profileOpen.ExecutionTimeoutSeconds != 120 || profileOpen.ExternalSideEffect {
		t.Fatalf("profile_open defaults = timeout %d side effect %v", profileOpen.ExecutionTimeoutSeconds, profileOpen.ExternalSideEffect)
	}
	cookieWrite := mustCreateTask(t, userA, model.TaskTypeCookieWrite.String(), "", nil)
	if cookieWrite.ExecutionTimeoutSeconds != 300 || !cookieWrite.ExternalSideEffect {
		t.Fatalf("cookie_write defaults = timeout %d side effect %v", cookieWrite.ExecutionTimeoutSeconds, cookieWrite.ExternalSideEffect)
	}
}

// 两个 Agent 同时领取同一条任务，只能有一个拿到 attempt。
func TestLiveTaskRuntimeConcurrentClaimHasSingleWinner(t *testing.T) {
	taskRuntimeSetup(t)
	user := seedRuntimeUser(t, "")
	task := mustCreateTask(t, user, model.TaskTypeNoop.String(), "", nil)

	type claimResult struct {
		task model.Task
		err  error
	}
	results := make(chan claimResult, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			claimed, err := ClaimTask(ClaimTaskInput{UserID: user, AgentID: "chg101-test-agent"})
			results <- claimResult{task: claimed, err: err}
		}()
	}
	close(start)

	winners := 0
	for i := 0; i < 2; i++ {
		result := <-results
		switch {
		case result.err == nil && result.task.TaskID == task.TaskID:
			winners++
		case errors.Is(result.err, model.ErrNoPendingTask):
		default:
			t.Fatalf("unexpected claim result: task=%s err=%v", result.task.TaskID, result.err)
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent claims produced %d winners, want 1", winners)
	}
}

func TestLiveTaskRuntimeSilentTimeoutAndLateResult(t *testing.T) {
	taskRuntimeSetup(t)
	user := seedRuntimeUser(t, "")

	readTask := mustCreateTask(t, user, model.TaskTypeCookieRead.String(), "", nil)
	writeTask := mustCreateTask(t, user, model.TaskTypeCookieWrite.String(), "", nil)
	if writeTask.ExecutionTimeoutSeconds != 300 || !writeTask.ExternalSideEffect {
		t.Fatalf("cookie_write defaults = %#v", writeTask)
	}
	claimedRead := mustClaimTask(t, user)
	claimedWrite := mustClaimTask(t, user)
	if claimedRead.TaskID != readTask.TaskID || claimedWrite.TaskID != writeTask.TaskID {
		t.Fatalf("claim order = %s, %s; want %s, %s", claimedRead.TaskID, claimedWrite.TaskID, readTask.TaskID, writeTask.TaskID)
	}
	for _, claimed := range []model.Task{claimedRead, claimedWrite} {
		mustReportTask(t, claimed.TaskID, ReportTaskInput{UserID: user, AttemptID: claimed.AttemptID, Status: "running", Progress: 5})
	}
	if err := database.DB().Exec(`UPDATE tasks SET last_report_at = DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 400 SECOND)
		WHERE task_id IN (?, ?)`, readTask.TaskID, writeTask.TaskID).Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := RecoverTimedOutTasks(time.Now().UTC(), 1000)
	if err != nil {
		t.Fatalf("recover timed out tasks: %v", err)
	}
	codes := map[string]string{}
	for _, task := range recovered {
		codes[task.TaskID] = task.ErrorCode
	}
	if codes[readTask.TaskID] != "EXECUTION_INTERRUPTED" || codes[writeTask.TaskID] != "RESULT_UNKNOWN" {
		t.Fatalf("recovered codes = %v, want read=EXECUTION_INTERRUPTED write=RESULT_UNKNOWN", codes)
	}

	interrupted, err := GetTask(readTask.TaskID, user)
	if err != nil {
		t.Fatal(err)
	}
	if interrupted.Status != model.TaskStatusFailed.String() || interrupted.ErrorCode != "EXECUTION_INTERRUPTED" ||
		!interrupted.Retryable || interrupted.RecoveryAction != "retry" || interrupted.SettledBy != "timeout" {
		t.Fatalf("interrupted task = %#v", interrupted)
	}
	unknown, err := GetTask(writeTask.TaskID, user)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Status != model.TaskStatusFailed.String() || unknown.ErrorCode != "RESULT_UNKNOWN" ||
		unknown.Retryable || unknown.RecoveryAction != "check_platform_state" || unknown.SettledBy != "timeout" {
		t.Fatalf("unknown-result task = %#v", unknown)
	}

	// 同一 attempt 且没有新 attempt 时，迟到的真实结果允许收敛。
	converged := mustReportTask(t, readTask.TaskID, ReportTaskInput{UserID: user, AttemptID: claimedRead.AttemptID, Status: "succeeded", Progress: 100})
	if converged.Status != model.TaskStatusSucceeded.String() || converged.SettledBy != "agent" || converged.ErrorCode != "" {
		t.Fatalf("late result did not converge: %#v", converged)
	}
	if _, err := RetryTask(writeTask.TaskID, RetryTaskInput{UserID: user}); !errors.Is(err, model.ErrResultUnknownRetry) {
		t.Fatalf("retry RESULT_UNKNOWN without confirmation: err = %v", err)
	}
	retried, err := RetryTask(writeTask.TaskID, RetryTaskInput{UserID: user, ConfirmResultUnknown: true})
	if err != nil {
		t.Fatalf("confirmed retry: %v", err)
	}
	if retried.Status != model.TaskStatusPending.String() || retried.AttemptID != "" || retried.Attempt != 1 {
		t.Fatalf("retried task = %#v, want pending with attempt 1 and no attempt_id", retried)
	}

	reclaimed := mustClaimTask(t, user)
	if reclaimed.TaskID != writeTask.TaskID || reclaimed.Attempt != 2 || reclaimed.AttemptID == claimedWrite.AttemptID {
		t.Fatalf("reclaimed task = %#v", reclaimed)
	}
	if _, err := ReportTask(writeTask.TaskID, ReportTaskInput{UserID: user, AttemptID: claimedWrite.AttemptID, Status: "succeeded"}); !errors.Is(err, model.ErrStaleAttempt) {
		t.Fatalf("old attempt result: err = %v, want ErrStaleAttempt", err)
	}
	if err := ActivityTask(writeTask.TaskID, ActivityTaskInput{UserID: user, AttemptID: claimedWrite.AttemptID}); !errors.Is(err, model.ErrStaleAttempt) {
		t.Fatalf("old attempt activity: err = %v, want ErrStaleAttempt", err)
	}
	if err := database.DB().Exec(`UPDATE tasks SET last_report_at = DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 HOUR) WHERE task_id = ?`, writeTask.TaskID).Error; err != nil {
		t.Fatal(err)
	}
	if err := ActivityTask(writeTask.TaskID, ActivityTaskInput{UserID: user, AttemptID: reclaimed.AttemptID}); err != nil {
		t.Fatalf("activity on current attempt: %v", err)
	}
	var fresh int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM tasks WHERE task_id = ? AND last_report_at > DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 60 SECOND)`, writeTask.TaskID).Row().Scan(&fresh); err != nil {
		t.Fatal(err)
	}
	if fresh != 1 {
		t.Fatal("activity did not refresh last_report_at")
	}

	cancelled, err := CancelTask(writeTask.TaskID, CancelTaskInput{UserID: user, Message: "stop"})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != model.TaskStatusCancelled.String() || cancelled.SettledBy != "user" {
		t.Fatalf("cancelled task = %#v", cancelled)
	}
	afterCancel := mustReportTask(t, writeTask.TaskID, ReportTaskInput{UserID: user, AttemptID: reclaimed.AttemptID, Status: "succeeded"})
	if afterCancel.Status != model.TaskStatusCancelled.String() {
		t.Fatalf("report after cancel changed status to %q", afterCancel.Status)
	}
}

func TestLiveTaskRuntimeProjectionStaysInUserAndMainScope(t *testing.T) {
	taskRuntimeSetup(t)
	userA := seedRuntimeUser(t, "main-A")
	userB := seedRuntimeUser(t, "main-B")
	suffix := time.Now().UnixNano()
	profileA := seedRuntimeProfile(t, userA, fmt.Sprintf("bit-%d-a", suffix), "main-A")
	profileOldMain := seedRuntimeProfile(t, userA, fmt.Sprintf("bit-%d-old", suffix), "main-old")
	profileB := seedRuntimeProfile(t, userB, fmt.Sprintf("bit-%d-b", suffix), "main-B")

	run := func(userID int64, taskType string, result map[string]any) {
		t.Helper()
		task := mustCreateTask(t, userID, taskType, "", nil)
		claimed := mustClaimTask(t, userID)
		if claimed.TaskID != task.TaskID {
			t.Fatalf("claimed %s, want %s", claimed.TaskID, task.TaskID)
		}
		mustReportTask(t, task.TaskID, ReportTaskInput{UserID: userID, AttemptID: claimed.AttemptID, Status: "succeeded", Progress: 100, Result: result})
	}
	bitStatus := func(profileID int64) string {
		t.Helper()
		var status string
		if err := database.DB().Raw(`SELECT COALESCE(bit_status, '') FROM browser_profiles WHERE id = ?`, profileID).Row().Scan(&status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	loginStatus := func(accountID int64) string {
		t.Helper()
		var status string
		if err := database.DB().Raw(`SELECT login_status FROM media_accounts WHERE id = ?`, accountID).Row().Scan(&status); err != nil {
			t.Fatal(err)
		}
		return status
	}

	run(userA, model.TaskTypeProfileUpdate.String(), map[string]any{"cloud_profile_id": strconv.FormatInt(profileB, 10)})
	if got := bitStatus(profileB); got != "" {
		t.Fatalf("user A projection wrote user B profile: bit_status = %q", got)
	}
	run(userA, model.TaskTypeProfileUpdate.String(), map[string]any{"cloud_profile_id": strconv.FormatInt(profileOldMain, 10)})
	if got := bitStatus(profileOldMain); got != "" {
		t.Fatalf("projection wrote a profile outside the bound main: bit_status = %q", got)
	}
	run(userA, model.TaskTypeProfileUpdate.String(), map[string]any{"cloud_profile_id": strconv.FormatInt(profileA, 10)})
	if got := bitStatus(profileA); got != "updated" {
		t.Fatalf("projection did not reach the in-scope profile: bit_status = %q", got)
	}

	accountA := seedRuntimeAccount(t, userA)
	accountB := seedRuntimeAccount(t, userB)
	run(userA, model.TaskTypeAccountCheck.String(), map[string]any{"account_id": strconv.FormatInt(accountB, 10), "login_status": "logged_in"})
	if got := loginStatus(accountB); got != "unknown" {
		t.Fatalf("user A projection wrote user B account: login_status = %q", got)
	}
	run(userA, model.TaskTypeAccountCheck.String(), map[string]any{"account_id": strconv.FormatInt(accountA, 10), "login_status": "logged_in"})
	if got := loginStatus(accountA); got != "normal" {
		t.Fatalf("account_check projection: login_status = %q, want normal", got)
	}
}
