package repository_test

import (
	"context"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"errors"
	driver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
)

// Bypass Claim deliberately: admission must be a database invariant, not a
// count whose correctness depends on a resource owner row lock.
func TestLiveAdmissionConstraintsAndRelease(t *testing.T) {
	s, makeTask := executionStore(t)
	v := makeTask("admission", 42, "one")
	now := time.Now().UTC()
	attemptNo := 0
	insert := func(profile uint64, slot int, key string) error {
		attemptNo++
		return s.DB.Exec(`INSERT INTO publication_attempt(publication_id,attempt_no,executor_user_id,browser_profile_id,claim_request_key,input_snapshot_json,status,stage,started_at,heartbeat_at,steps_json,evidence_json,admission_slot) VALUES(?, ?,9,?,?,'{}','running','claimed',?,?,'[]','{}',?)`, v.ID, attemptNo, profile, key, now, now, slot).Error
	}
	duplicate := func(err error, index string) {
		t.Helper()
		var d *driver.MySQLError
		if !errors.As(err, &d) || d.Number != 1062 || !strings.Contains(d.Message, index) {
			t.Fatalf("expected atomic duplicate rejection, got %v", err)
		}
	}
	if e := insert(37, 1, "first"); e != nil {
		t.Fatal(e)
	}
	duplicate(insert(37, 2, "same-profile"), "uq_attempt_active_profile")
	duplicate(insert(38, 1, "same-user-slot"), "uq_attempt_user_slot")
	if e := s.DB.Model(&model.Attempt{}).Where("claim_request_key=?", "first").Update("status", model.AttemptStatusInterrupted).Error; e != nil {
		t.Fatal(e)
	}
	duplicate(insert(38, 1, "unresolved-holds-slot"), "uq_attempt_user_slot")
	if e := s.DB.Model(&model.Attempt{}).Where("claim_request_key=?", "first").Update("resolved_at", now).Error; e != nil {
		t.Fatal(e)
	}
	if e := insert(38, 1, "resolved-releases"); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.Model(&model.Attempt{}).Where("claim_request_key=?", "resolved-releases").Update("status", model.AttemptStatusCompleted).Error; e != nil {
		t.Fatal(e)
	}
	if e := insert(38, 1, "completion-releases"); e != nil {
		t.Fatal(e)
	}
}

func TestLiveResourceReadsDoNotBlockMetadataUpdates(t *testing.T) {
	s, makeTask := ownedStore(t)
	v := makeTask(0)
	tx := s.DB.Begin()
	defer tx.Rollback()
	if _, e := s.ReadInput(tx, v); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		done <- s.DB.Transaction(func(other *gorm.DB) error {
			for _, q := range []string{
				"UPDATE media_accounts SET enabled=0 WHERE id=12",
				"UPDATE users SET status='disabled' WHERE id=9",
				"UPDATE categories SET enabled=0 WHERE id=1",
				"DELETE FROM user_category_scopes WHERE user_id=9",
				"DELETE FROM media_account_categories WHERE media_account_id=12",
				"UPDATE browser_profiles SET enabled=0 WHERE id=37",
				"UPDATE composite_outputs SET publish_status='archived' WHERE id=42",
				"UPDATE compose_tasks SET platforms_json='[]' WHERE id=42",
				"UPDATE operations_files SET name='renamed' WHERE id=1",
			} {
				if e := other.Exec(q).Error; e != nil {
					return e
				}
			}
			return nil
		})
	}()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		tx.Rollback()
		<-done
		t.Fatal("ordinary resource read blocked metadata update")
	}
}

func legacyAdmission(t *testing.T, n int, sameProfile bool) *gorm.DB {
	t.Helper()
	db := isolatedDB(t, true)
	for _, name := range []string{"20261118_103_operations_file_delete_state.sql", "20261119_106_operations_file_deleted_at.sql", "20261119_107_publication_attempt_revision.sql", "20261120_108_publication_task_profile_reference.sql"} {
		applyFileMigration(t, db, name)
	}
	if e := db.Exec("INSERT INTO browser_profiles VALUES(37),(38),(39),(40)").Error; e != nil {
		t.Fatal(e)
	}
	v := task("legacy-admission", 42, 1)
	e := db.Omit("LatestManualVerification", "BatchID", "TaskOrder", "RowKey", "ExecutionGroupNo", "PlatformSubmissionID", "TargetURL", "WorkID", "CheckBrowserUsedAt", "CheckNotFoundCount", "CheckStoppedAt", "NextCheckAt", "CheckGeneration", "LastCheckedAt", "LastCheckMessage").Create(&v).Error
	if e != nil {
		t.Fatal(e)
	}
	for i := 1; i <= n; i++ {
		profile := 36 + i
		if sameProfile {
			profile = 37
		}
		e := db.Exec(`INSERT INTO publication_attempt(publication_id,attempt_no,executor_user_id,browser_profile_id,claim_request_key,input_snapshot_json,status,stage,started_at,heartbeat_at,steps_json,evidence_json) VALUES(?,?,9,?,?,'{}','running','claimed',NOW(6),NOW(6),'[]','{}')`, v.ID, i, profile, fmt.Sprint("old-", i)).Error
		if e != nil {
			t.Fatal(e)
		}
	}
	return db
}
func TestLiveAdmissionMigrationBackfillsAndRejectsInvalidHistory(t *testing.T) {
	t.Run("backfill", func(t *testing.T) {
		db := legacyAdmission(t, 4, false)
		if e := db.Exec("UPDATE publication_attempt SET status='completed' WHERE attempt_no=4").Error; e != nil {
			t.Fatal(e)
		}
		applyFileMigration(t, db, "20261120_109_publication_attempt_admission.sql")
		var slots []int
		if e := db.Model(&model.Attempt{}).Where("status=?", model.AttemptStatusRunning).Order("id").Pluck("admission_slot", &slots).Error; e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(slots, []int{1, 2, 3}) {
			t.Fatalf("backfill=%v", slots)
		}
	})
	for _, test := range []struct {
		name string
		n    int
		same bool
	}{{"over-cap", 4, false}, {"duplicate-profile", 2, true}} {
		t.Run(test.name, func(t *testing.T) {
			db := legacyAdmission(t, test.n, test.same)
			raw, e := os.ReadFile(config.GetRuntimePaths().Migrations + "/20261120_109_publication_attempt_admission.sql")
			if e != nil {
				t.Fatal(e)
			}
			_, e = migration.Apply(context.Background(), db, []migration.Migration{{Version: "admission-invalid", Name: "admission-invalid", SQL: string(raw)}})
			var dbErr *driver.MySQLError
			expected := uint16(1062)
			if test.name == "over-cap" {
				expected = 3819
			}
			if !errors.As(e, &dbErr) || dbErr.Number != expected {
				t.Fatalf("invalid history: expected MySQL %d, got %v", expected, e)
			}
			var count int64
			if e := db.Model(&model.Attempt{}).Where("status=? AND resolved_at IS NULL", model.AttemptStatusRunning).Count(&count).Error; e != nil || count != int64(test.n) {
				t.Fatalf("history changed: %d %v", count, e)
			}
		})
	}
}
