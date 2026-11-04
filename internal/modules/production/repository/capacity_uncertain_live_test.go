package repository

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
)

// All runtime statements execute on one dedicated MySQL session. Temporary
// copies of the real tables shadow the live queue, so a running dev worker is
// unaffected and every assertion still exercises actual InnoDB transactions.
func TestLiveUncertainCapacityRequiresCurrentReviewAndFencesOldAttempt(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = database.DB().WithContext(ctx).Connection(func(db *gorm.DB) error {
		db = db.Session(&gorm.Session{NewDB: true})
		names := []string{"compose_tasks", "compose_task_attempts", "compose_worker_capacity", "composite_outputs", "audit_logs"}
		for _, name := range names {
			if err := db.Exec("CREATE TEMPORARY TABLE shadow118_" + name + " LIKE " + name).Error; err != nil {
				return err
			}
			defer db.Exec("DROP TEMPORARY TABLE IF EXISTS shadow118_" + name)
			if err := db.Exec("ALTER TABLE shadow118_" + name + " RENAME TO " + name).Error; err != nil {
				return err
			}
			defer db.Exec("DROP TEMPORARY TABLE IF EXISTS " + name)
		}
		exec := func(q string, args ...any) {
			t.Helper()
			if err := db.Exec(q, args...).Error; err != nil {
				t.Fatal(err)
			}
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		exec(`INSERT INTO compose_worker_capacity(id,max_running,updated_at) VALUES(1,1,?)`, now)
		for _, id := range []int64{1, 2} {
			exec(`INSERT INTO compose_tasks(id,team_id,category_id,operator_id,trigger_type,material_id,compose_strategy_id,template_code,template_version,config_snapshot_json,status,request_key,created_by,created_at,updated_at) VALUES(?,7,1,9,'manual',42,3,'basic_cut',1,'{}','pending',?,9,?,?)`, id, "capacity118-"+strconv.FormatInt(id, 10), now, now)
		}
		exec(`UPDATE compose_tasks SET status='failed',error_code='execution_uncertain',attempt_count=2,claimed_node_id=19 WHERE id=1`)
		evidence := "核实旧节点执行已停止，数据库与对象存储均无提交产物，允许释放占用。"
		exec(`INSERT INTO compose_task_attempts(task_id,attempt,source,outcome,error_code,uncertain_review_actor_id,uncertain_review_evidence,uncertain_reviewed_at,created_at,updated_at) VALUES(1,1,'auto','failed','execution_uncertain',9,?,?,?,?)`, evidence, now, now, now)
		exec(`INSERT INTO compose_task_attempts(task_id,attempt,source,outcome,error_code,created_at,updated_at) VALUES(1,2,'auto','failed','execution_uncertain',?,?)`, now, now)
		blocked := func() {
			t.Helper()
			if task, found, err := claimNextComposeTask(db, 29, time.Minute, now); err != nil || found {
				t.Fatalf("uncertain slot did not block task=%+v found=%v err=%v", task, found, err)
			}
		}
		blocked() // A reviewed prior attempt must not release this attempt's slot.
		exec(`DELETE FROM compose_task_attempts WHERE task_id=1 AND attempt=2`)
		blocked() // Missing evidence is unknown, never proof that the process stopped.
		if _, changed, err := resolveComposeUncertain(db, 1, 9, evidence, now); err == nil || changed {
			t.Fatalf("missing-attempt review changed=%v err=%v", changed, err)
		}
		var status string
		if err := db.Raw(`SELECT status FROM compose_tasks WHERE id=1`).Row().Scan(&status); err != nil || status != "failed" {
			t.Fatalf("failed review did not roll back status=%s err=%v", status, err)
		}
		blocked()
		exec(`INSERT INTO compose_task_attempts(task_id,attempt,source,outcome,error_code,created_at,updated_at) VALUES(1,2,'auto','failed','execution_uncertain',?,?)`, now, now)
		exec(`UPDATE compose_tasks SET output_object_key='already-committed.mp4' WHERE id=1`)
		if _, changed, err := resolveComposeUncertain(db, 1, 9, evidence, now); err != nil || changed {
			t.Fatalf("output guard changed=%v err=%v", changed, err)
		}
		blocked()
		exec(`UPDATE compose_tasks SET output_object_key='' WHERE id=1`)
		review, changed, err := resolveComposeUncertain(db, 1, 9, evidence, now)
		if err != nil || !changed || review.Attempt != 2 || review.ActorUserID != 9 || review.Evidence != evidence {
			t.Fatalf("review=%+v changed=%v err=%v", review, changed, err)
		}
		if _, changed, err := resolveComposeUncertain(db, 1, 9, evidence, now); err != nil || changed {
			t.Fatalf("review reused changed=%v err=%v", changed, err)
		}
		task, found, err := claimNextComposeTask(db, 29, time.Minute, now)
		if err != nil || !found || task.ID != 1 || task.AttemptCount != 3 {
			t.Fatalf("released claim task=%+v found=%v err=%v", task, found, err)
		}
		var source, outcome string
		if err := db.Raw(`SELECT source,outcome FROM compose_task_attempts WHERE task_id=1 AND attempt=3`).Row().Scan(&source, &outcome); err != nil || source != "manual" || outcome != "running" {
			t.Fatalf("manual claim %s/%s err=%v", source, outcome, err)
		}
		blocked() // The newly running task uses the same global slot.
		for _, identity := range [][2]int64{{19, 2}, {29, 2}, {19, 3}} {
			if ok, err := renewComposeLease(db, 1, identity[0], int(identity[1]), time.Minute, now); err != nil || ok {
				t.Fatalf("old lease renewed %v ok=%v err=%v", identity, ok, err)
			}
			if ok, err := reportComposeProgress(db, 1, identity[0], int(identity[1]), "compose", 50, now); err != nil || ok {
				t.Fatalf("old worker progressed %v ok=%v err=%v", identity, ok, err)
			}
		}
		if ok, err := renewComposeLease(db, 1, 29, 3, time.Minute, now.Add(time.Second)); err != nil || !ok {
			t.Fatalf("current lease ok=%v err=%v", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
