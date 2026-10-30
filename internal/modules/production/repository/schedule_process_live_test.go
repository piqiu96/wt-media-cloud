package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

// This gated test uses the local MySQL and actual compose-scheduler binary.
// Its own strategy and tasks are deleted after the two-process/restart check.
func TestLiveComposeSchedulerProcessesDeduplicate(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_SCHEDULER_PROCESS") != "1" {
		t.Skip("set WT_MEDIA_TEST_SCHEDULER_PROCESS=1 against an isolated local Cloud database")
	}
	root, err := filepath.Abs("../../../..")
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
	var sourceID int64
	err = database.DB().Raw(`SELECT s.id FROM compose_strategies s JOIN compose_templates t ON t.id = s.compose_template_id AND t.version = s.compose_template_version JOIN users u ON u.id = s.operator_id AND u.status = 'enabled' AND u.role IN ('operator', 'senior_operator') AND u.team_id = s.team_id JOIN user_category_scopes g ON g.user_id = u.id AND g.category_id = s.category_id WHERE s.status = 'enabled' AND t.status = 'active' AND EXISTS (SELECT 1 FROM materials m JOIN source_contents sc ON sc.id = m.source_content_id WHERE m.team_id = s.team_id AND sc.category_id = s.category_id AND m.status = 'available') ORDER BY s.id LIMIT 1`).Row().Scan(&sourceID)
	if err != nil {
		t.Skipf("no active strategy with an available operator and a team material: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "compose-scheduler")
	build := exec.Command("go", "build", "-o", binary, "./cmd/compose-scheduler")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build scheduler: %v\n%s", err, output)
	}
	schedule := model.ComposeSchedule{Mode: "interval", IntervalMinutes: 30, Timezone: "Asia/Shanghai"}
	scheduleJSON, err := json.Marshal(schedule)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture needs its own operator (copied) and a rule that matches the
	// team pool, so the two processes race on a due strategy that really
	// creates a task. The rule is explicit: a copied rule may match nothing in
	// the now team-wide pool.
	rules, err := json.Marshal([]model.ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []model.ComposeCondition{{Field: "like_count", Operator: "gte", Value: json.RawMessage(`0`)}}}})
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("CHG092-process-smoke-%d", time.Now().UnixNano())
	result := database.DB().Exec(`INSERT INTO compose_strategies (team_id, category_id, operator_id, compose_template_id, compose_template_version, name, config_json, execution_mode, schedule_json, priority_rules_json, single_limit, daily_limit, next_run_at, status, created_by, updated_by, created_at, updated_at) SELECT team_id, category_id, operator_id, compose_template_id, compose_template_version, ?, config_json, 'scheduled', ?, ?, 1, 2, ?, 'enabled', created_by, updated_by, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) FROM compose_strategies WHERE id = ?`, name, scheduleJSON, rules, time.Now().UTC().Add(-time.Minute), sourceID)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("create smoke strategy: rows=%d err=%v", result.RowsAffected, result.Error)
	}
	var strategyID int64
	if err := database.DB().Raw(`SELECT id FROM compose_strategies WHERE name = ? ORDER BY id DESC LIMIT 1`, name).Row().Scan(&strategyID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		queries := []string{
			`DELETE a FROM audit_logs a JOIN compose_tasks ct ON a.target_type = 'compose_task' AND a.target_id = CAST(ct.id AS CHAR) WHERE ct.compose_strategy_id = ?`,
			`DELETE ca FROM compose_task_attempts ca JOIN compose_tasks ct ON ca.task_id = ct.id WHERE ct.compose_strategy_id = ?`,
			`DELETE FROM compose_tasks WHERE compose_strategy_id = ?`,
			`DELETE FROM compose_schedule_runs WHERE compose_strategy_id = ?`,
			`DELETE FROM compose_strategies WHERE id = ?`,
		}
		for _, query := range queries {
			if err := database.DB().Exec(query, strategyID).Error; err != nil {
				t.Errorf("smoke cleanup: %v", err)
			}
		}
	})
	start := func() (*exec.Cmd, *bytes.Buffer) {
		command := exec.Command(binary)
		command.Dir = root
		command.Env = append(os.Environ(), "WT_MEDIA_CLOUD_HOME="+root)
		output := &bytes.Buffer{}
		command.Stdout, command.Stderr = output, output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		return command, output
	}
	stop := func(command *exec.Cmd) {
		_ = command.Process.Kill()
		_ = command.Wait()
	}
	first, firstLog := start()
	second, secondLog := start()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_schedule_runs WHERE compose_strategy_id = ?`, strategyID).Row().Scan(&count); err != nil {
			stop(first)
			stop(second)
			t.Fatal(err)
		}
		if count == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	stop(first)
	stop(second)
	var runs, tasks int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_schedule_runs WHERE compose_strategy_id = ?`, strategyID).Row().Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ? AND trigger_type = 'scheduled'`, strategyID).Row().Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || tasks != 1 {
		t.Fatalf("concurrent processes: runs=%d tasks=%d, process 1=%s process 2=%s", runs, tasks, firstLog.String(), secondLog.String())
	}
	restarted, restartLog := start()
	time.Sleep(500 * time.Millisecond)
	stop(restarted)
	var after int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ?`, strategyID).Row().Scan(&after); err != nil || after != tasks {
		t.Fatalf("restart task count=%d want=%d err=%v process=%s", after, tasks, err, restartLog.String())
	}
	t.Logf("independent processes: strategy=%d runs=%d tasks=%d after restart=%d", strategyID, runs, tasks, after)
}
