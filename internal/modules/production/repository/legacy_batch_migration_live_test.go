package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
	"gorm.io/gorm"
)

// Exact checked-in SQL runs through the production migration runner. Disposable
// named tables retain foreign keys, so wrong deletion order cannot pass. Only
// identifiers are substituted; the real tasks/batches/outputs never change.
func TestLiveLegacyBatchMigrationRemovesOnlyProven087Artifacts(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires local MySQL; creates disposable qa118_094_ tables")
	}
	source, err := os.ReadFile("../../../../migrations/20261105_094_remove_synthetic_legacy_batches.sql")
	if err != nil {
		t.Fatal(err)
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
		var schema string
		if err := db.Raw(`SELECT DATABASE()`).Row().Scan(&schema); err != nil {
			return err
		}
		if schema != "wt_media_cloud" {
			return fmt.Errorf("requires fixed wt_media_cloud, got %s", schema)
		}
		prefix := fmt.Sprintf("qa118_094_%d_", time.Now().UnixNano())
		names := []string{"compose_task_batches", "compose_tasks", "compose_task_attempts", "composite_outputs", "compose_schedule_runs"}
		pairs := []string{}
		for _, name := range names {
			pairs = append(pairs, name, prefix+name)
		}
		rewrite := strings.NewReplacer(pairs...).Replace
		exec := func(q string, args ...any) {
			t.Helper()
			if err := db.Exec(rewrite(q), args...).Error; err != nil {
				t.Fatal(err)
			}
		}
		// Drops are deferred in reverse FK order, including partially seeded failures.
		for _, name := range names {
			defer func(table string) {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				if err := db.WithContext(cleanupCtx).Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
					t.Errorf("drop disposable table %s: %v", table, err)
				}
			}(prefix + name)
		}

		// Clone every current column, index and CHECK from the actual migrated
		// tables. CREATE LIKE omits foreign keys, restored below within QA.
		for _, name := range names {
			if err := db.Exec("CREATE TABLE " + prefix + name + " LIKE " + name).Error; err != nil {
				return err
			}
		}
		exec(`ALTER TABLE compose_tasks ADD FOREIGN KEY(batch_id) REFERENCES compose_task_batches(id)`)
		exec(`ALTER TABLE compose_schedule_runs ADD FOREIGN KEY(batch_id) REFERENCES compose_task_batches(id)`)
		exec(`ALTER TABLE compose_task_attempts ADD FOREIGN KEY(task_id) REFERENCES compose_tasks(id)`)
		exec(`ALTER TABLE composite_outputs ADD FOREIGN KEY(compose_task_id) REFERENCES compose_tasks(id)`)
		const stamp = "2026-11-05 04:03:02.123789"
		for id := 1; id <= 12; id++ {
			exec(`INSERT INTO compose_tasks(id,team_id,category_id,operator_id,trigger_type,material_id,compose_strategy_id,template_code,template_version,config_snapshot_json,status,request_key,created_by,created_at,updated_at) VALUES(?,7,1,9,'manual',42,3,'basic_cut',1,'{"start_seconds":1,"end_seconds":4}','success',?,9,?,?)`, id, fmt.Sprintf("migration118-%d", id), stamp, stamp)
		}
		exec(`UPDATE compose_tasks SET operator_id=NULL,status='failed' WHERE id=2`)
		// Reproduce 087's actual generated values including DATETIME(6)->(3).
		exec(`INSERT INTO compose_task_batches(id,name,team_id,category_id,operator_id,compose_strategy_id,trigger_type,created_by,request_key,request_hash,config_snapshot_json,created_at) SELECT id+100,CONCAT('历史任务 ',id),team_id,category_id,operator_id,compose_strategy_id,trigger_type,created_by,CONCAT('legacy-task-',id),SHA2(CONCAT('legacy-task-',id),256),JSON_OBJECT('legacy_task_id',id),created_at FROM compose_tasks`)
		exec(`UPDATE compose_tasks SET batch_id=id+100`)
		exec(`UPDATE compose_task_batches SET name='真实批次',request_key='real-command',config_snapshot_json='{"settings":[{"key":"default"}]}' WHERE id=103`)
		exec(`UPDATE compose_task_batches SET request_key='Legacy-task-4' WHERE id=104`)
		exec(`UPDATE compose_task_batches SET request_hash=REPEAT('0',64) WHERE id=105`)
		exec(`UPDATE compose_task_batches SET config_snapshot_json='{"legacy_task_id":6,"settings":[]}' WHERE id=106`)
		exec(`UPDATE compose_task_batches SET name='已人工改名' WHERE id=107`)
		exec(`UPDATE compose_task_batches SET operator_id=11 WHERE id=108`)
		exec(`UPDATE compose_tasks SET batch_id=109,copy_index=2 WHERE id=12`)
		exec(`INSERT INTO compose_schedule_runs(run_key,compose_strategy_id,trigger_type,schedule_slot,candidate_count,created_count,skipped_json,created_at,batch_id,request_hash) VALUES('original-run',3,'manual',?,4,2,'[{"material_id":42,"reason":"keep original scheduling evidence"}]',?,110,SHA2('original-run',256))`, stamp, stamp)
		exec(`UPDATE compose_task_batches SET config_snapshot_json='{"legacy_task_id":"11"}' WHERE id=111`)
		// 112 is now unreferenced, but its original association cannot be confirmed.
		exec(`INSERT INTO compose_task_attempts(id,task_id,attempt,source,outcome,error_code,created_at,updated_at) VALUES(1,1,1,'auto','success','',?,?),(2,2,3,'manual','failed','execution_uncertain',?,?)`, stamp, stamp, stamp, stamp)
		exec(`UPDATE compose_task_attempts SET uncertain_review_actor_id=9,uncertain_review_evidence='旧进程、对象存储和数据库已核对，原始证据需要完整保留。',uncertain_reviewed_at=? WHERE id=2`, stamp)
		exec(`INSERT INTO composite_outputs(id,team_id,category_id,operator_id,material_id,compose_task_id,compose_strategy_id,template_code,template_version,object_key,file_size_bytes,sha256,media_json,created_by,created_at,updated_at,name,file_name) VALUES(1,7,1,9,42,1,3,'basic_cut',1,'original/output.mp4',1024,REPEAT('a',64),'{"duration_ms":12345}',9,?,?,'真实历史成片','真实历史成片.mp4')`, stamp, stamp)
		snapshots := func(query string, omit string) [][]string {
			t.Helper()
			rows, err := db.Raw(rewrite(query)).Rows()
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				t.Fatal(err)
			}
			result := [][]string{}
			for rows.Next() {
				values := make([]sql.RawBytes, len(cols))
				args := make([]any, len(cols))
				for i := range values {
					args[i] = &values[i]
				}
				if err := rows.Scan(args...); err != nil {
					t.Fatal(err)
				}
				row := []string{}
				for i, v := range values {
					if cols[i] == omit {
						continue
					}
					if v == nil {
						row = append(row, "<NULL>")
					} else {
						row = append(row, string(v))
					}
				}
				result = append(result, row)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			return result
		}
		before := map[string][][]string{}
		for _, name := range []string{"compose_tasks", "compose_task_attempts", "composite_outputs", "compose_schedule_runs"} {
			order := "id"
			if name == "compose_schedule_runs" {
				order = "run_key"
			}
			before[name] = snapshots("SELECT * FROM "+name+" ORDER BY "+order, "")
		}
		// Every task field other than its batch pointer must survive byte-for-byte.
		before["compose_tasks"] = snapshots("SELECT * FROM compose_tasks ORDER BY id", "batch_id")
		retained := snapshots(`SELECT * FROM compose_task_batches WHERE id NOT IN(101,102) ORDER BY id`, "")
		exec(`CREATE TEMPORARY TABLE schema_migrations(version VARCHAR(128) PRIMARY KEY,name VARCHAR(255) NOT NULL,applied_at DATETIME(6) NOT NULL)`)
		defer db.Exec(`DROP TEMPORARY TABLE IF EXISTS schema_migrations`)
		defer db.Exec(`DROP TEMPORARY TABLE IF EXISTS chg118_legacy_batch_candidates`)
		item := migration.Migration{Version: "20261105_094_remove_synthetic_legacy_batches", Name: "remove_synthetic_legacy_batches", SQL: rewrite(string(source))}
		applied, err := migration.Apply(ctx, db, []migration.Migration{item})
		if err != nil {
			return err
		}
		if len(applied) != 1 {
			t.Fatalf("migration not applied: %+v", applied)
		}
		for _, name := range []string{"compose_tasks", "compose_task_attempts", "composite_outputs", "compose_schedule_runs"} {
			omit := ""
			if name == "compose_tasks" {
				omit = "batch_id"
			}
			order := "id"
			if name == "compose_schedule_runs" {
				order = "run_key"
			}
			after := snapshots("SELECT * FROM "+name+" ORDER BY "+order, omit)
			if !reflect.DeepEqual(before[name], after) {
				t.Fatalf("%s facts changed\nbefore=%v\nafter=%v", name, before[name], after)
			}
		}
		if after := snapshots(`SELECT * FROM compose_task_batches ORDER BY id`, ""); !reflect.DeepEqual(retained, after) {
			t.Fatalf("real or ambiguous batches changed\nexpected=%v\nactual=%v", retained, after)
		}
		for id := 1; id <= 12; id++ {
			var batch sql.NullInt64
			if err := db.Raw(rewrite(`SELECT batch_id FROM compose_tasks WHERE id=?`), id).Row().Scan(&batch); err != nil {
				return err
			}
			expected := int64(id + 100)
			if id == 12 {
				expected = 109
			}
			if id <= 2 {
				if batch.Valid {
					t.Fatalf("proven synthetic task%d still linked to %d", id, batch.Int64)
				}
			} else if !batch.Valid || batch.Int64 != expected {
				t.Fatalf("unconfirmed task%d changed: %+v", id, batch)
			}
		}
		// A second run sees the recorded version and makes no further mutations.
		if applied, err := migration.Apply(ctx, db, []migration.Migration{item}); err != nil || len(applied) != 0 {
			t.Fatalf("migration replay=%+v err=%v", applied, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
