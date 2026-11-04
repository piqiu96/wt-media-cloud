package repository

import (
	"encoding/json"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"os"
	"testing"
	"time"
)

// Own transaction rolls back all fixture facts. Tests use the fixed developer schema.
func TestLiveMultiSettingPreviewAndConfirmation(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires migrated local MySQL")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	var source int64
	err = tx.Raw(`SELECT s.id FROM compose_strategies s JOIN users u ON u.id=s.operator_id AND u.status='enabled' JOIN user_category_scopes cs ON cs.user_id=u.id AND cs.category_id=s.category_id JOIN materials m ON m.team_id=s.team_id JOIN source_contents sc ON sc.id=m.source_content_id AND sc.category_id=s.category_id WHERE s.status='enabled' AND m.status='available' LIMIT 1`).Row().Scan(&source)
	if err != nil {
		t.Fatalf("needs eligible local strategy fixture: %v", err)
	}
	rules := `[{"priority":1,"action":"match","logic":"all","conditions":[{"field":"like_count","operator":"gte","value":0}]}]`
	now := time.Now().UTC().Truncate(time.Microsecond)
	name := fmt.Sprintf("CHG115-%d", now.UnixNano())
	err = tx.Exec(`INSERT INTO compose_strategies(team_id,category_id,operator_id,compose_template_id,compose_template_version,name,config_json,execution_mode,priority_rules_json,single_limit,daily_limit,status,created_by,updated_by,created_at,updated_at) SELECT team_id,category_id,operator_id,compose_template_id,compose_template_version,?,config_json,'manual',?,2,0,'enabled',created_by,updated_by,?,? FROM compose_strategies WHERE id=?`, name, rules, now, now, source).Error
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := tx.Raw(`SELECT id FROM compose_strategies WHERE name=?`, name).Row().Scan(&id); err != nil {
		t.Fatal(err)
	}
	strategy, err := scanComposeStrategy(tx.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id=?`, id).Row())
	if err != nil {
		t.Fatal(err)
	}
	var params map[string]any
	if err := json.Unmarshal(strategy.Config, &params); err != nil {
		t.Fatal(err)
	}
	groups := []model.ComposeOutputSetting{{Key: "one", Name: "通用", Copies: 2, Platforms: []string{"bilibili", "baidu"}, Params: params}, {Key: "two", Name: "小红书", Copies: 3, Platforms: []string{"xiaohongshu"}, Params: params}}
	raw, _ := json.Marshal(groups)
	strategy.OperatorName = "实际负责人显示名"
	strategy.OutputSettings = groups
	strategy.OutputSettingsJSON = raw
	if err := tx.Exec(`UPDATE compose_strategies SET output_settings_json=? WHERE id=?`, raw, id).Error; err != nil {
		t.Fatal(err)
	}
	counts := func() (tasks, batches, runs int) {
		t.Helper()
		for _, q := range []struct {
			table string
			dest  *int
		}{{"compose_tasks", &tasks}, {"compose_task_batches", &batches}, {"compose_schedule_runs", &runs}} {
			if err := tx.Raw(`SELECT COUNT(*) FROM ` + q.table).Row().Scan(q.dest); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	ta, ba, ra := counts()
	p, err := composeExecutionPreview(tx, strategy, now, true)
	if err != nil {
		t.Fatal(err)
	}
	tb, bb, rb := counts()
	if ta != tb || ba != bb || ra != rb {
		t.Fatal("preview mutated persisted work")
	}
	if p.SelectedCount < 1 || p.EstimatedOutputCount != p.SelectedCount*5 || p.DailyRemaining != nil {
		t.Fatalf("preview %+v", p)
	}
	opts := ScheduleRunOptions{StrategyID: id, Now: now, ManualKey: "preview-confirm-1", PreviewToken: "stale", NextRun: func(model.ComposeStrategy, time.Time) (*time.Time, error) { return nil, nil }}
	if _, _, err := runComposeSchedule(tx, opts); err != ErrComposePreviewChanged {
		t.Fatalf("stale accepted: %v", err)
	}
	opts.PreviewToken = p.PreviewToken
	run, done, err := runComposeSchedule(tx, opts)
	if err != nil || !done || run.BatchID == nil || run.CreatedCount != p.EstimatedOutputCount {
		t.Fatalf("run %+v done %v err %v", run, done, err)
	}
	opts.PreviewToken = "different-content"
	if _, _, err := runComposeSchedule(tx, opts); err != ErrComposePreviewChanged {
		t.Fatalf("samekey changedbody accepted %v", err)
	}
	opts.PreviewToken = p.PreviewToken
	again, _, err := runComposeSchedule(tx, opts)
	if err != nil || again.BatchID == nil || *again.BatchID != *run.BatchID {
		t.Fatalf("idempotency %+v %v", again, err)
	}
	active, err := composeExecutionPreview(tx, strategy, now, true)
	if err != nil || active.ActiveBatch == nil || active.ActiveBatch.Counts.Pending != run.CreatedCount {
		t.Fatalf("active %+v %v", active, err)
	}
	opts.ManualKey = "preview-confirm-2"
	if _, _, err := runComposeSchedule(tx, opts); err != ErrComposeAlreadyRunning {
		t.Fatalf("overlap allowed %v", err)
	}
	var actual int
	if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE batch_id=?`, *run.BatchID).Row().Scan(&actual); err != nil || actual != run.CreatedCount {
		t.Fatalf("tasks %d err %v", actual, err)
	}
	t.Logf("readonly preview selected=%d expected=%d; confirmation batch=%d tasks=%d; replay/stale/overlap checked", p.SelectedCount, p.EstimatedOutputCount, *run.BatchID, actual)
}
