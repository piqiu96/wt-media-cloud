package repository

import (
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"os"
	"testing"
	"time"
)

func TestLiveMergedAttemptReviewIsAtomicSingleUseAndCleanupGuarded(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires migrated local MySQL")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tx := database.DB().Begin()
	defer tx.Rollback()
	now := time.Now().UTC()
	key := fmt.Sprintf("review116-%d", now.UnixNano())
	res := tx.Exec(`INSERT INTO compose_tasks(team_id,category_id,operator_id,trigger_type,material_id,material_usage_id,compose_strategy_id,template_code,template_version,config_snapshot_json,status,request_key,created_by,attempt_count,error_code,created_at,updated_at) SELECT team_id,category_id,operator_id,'manual',material_id,material_usage_id,compose_strategy_id,template_code,template_version,config_snapshot_json,'failed',?,created_by,1,'execution_uncertain',?,? FROM compose_tasks ORDER BY id LIMIT 1`, key, now, now)
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("seed %v rows%d", res.Error, res.RowsAffected)
	}
	var id, actor int64
	if err = tx.Raw(`SELECT id,created_by FROM compose_tasks WHERE request_key=?`, key).Row().Scan(&id, &actor); err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(`INSERT INTO compose_task_attempts(task_id,attempt,source,outcome,error_code,created_at,updated_at) VALUES(?,1,'auto','failed','execution_uncertain',?,?)`, id, now, now).Error; err != nil {
		t.Fatal(err)
	}
	evidence := "已核对旧执行进程退出，数据库与存储均未提交任何成片产物"
	review, changed, err := resolveComposeUncertain(tx, id, actor, evidence, now)
	if err != nil || !changed || review.Evidence != evidence {
		t.Fatalf("review %+v %v %v", review, changed, err)
	}
	attempts, err := listComposeTaskAttempts(tx, id)
	if err != nil || len(attempts) != 2 || attempts[0].UncertainReview == nil || attempts[0].UncertainReview.Evidence != evidence || attempts[1].StartedAt != nil {
		t.Fatalf("attempts %+v %v", attempts, err)
	}
	if _, changed, err = resolveComposeUncertain(tx, id, actor, evidence, now); err != nil || changed {
		t.Fatalf("duplicate changed%v %v", changed, err)
	}
	var status string
	var audits, reviewed int
	if err = tx.Raw(`SELECT status FROM compose_tasks WHERE id=?`, id).Row().Scan(&status); err != nil || status != "pending" {
		t.Fatalf("state %s %v", status, err)
	}
	if err = tx.Raw(`SELECT COUNT(*) FROM audit_logs WHERE target_type='compose_task' AND target_id=? AND action='compose_task.resolve_uncertain'`, fmt.Sprint(id)).Row().Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit %d %v", audits, err)
	}
	if err = tx.Raw(`SELECT COUNT(*) FROM compose_task_attempts a WHERE a.task_id=? AND a.attempt=1 AND a.uncertain_reviewed_at IS NOT NULL`, id).Row().Scan(&reviewed); err != nil || reviewed != 1 {
		t.Fatalf("cleanupguard %d %v", reviewed, err)
	}
	t.Log("review+requeue+manual pending attempt+audit committed within rollback-isolated transaction; duplicate rejected; privileged evidence readback preserved")
}
