package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
)

// This runs the checked-in migration against disposable tables, including its
// DDL failure behavior, because MySQL DDL cannot be isolated by a rollback.
func TestLiveAttemptReviewMigrationPreservesFactsAndRejectsOrphans(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires local MySQL; creates only qa_chg116_ tables in wt_media_cloud")
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
	db, err := database.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var schema string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil || schema != "wt_media_cloud" {
		t.Fatalf("requires fixed wt_media_cloud schema, got %q: %v", schema, err)
	}
	var actorID int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&actorID); err != nil {
		t.Fatalf("requires an existing actor: %v", err)
	}
	source, err := os.ReadFile("../../../../migrations/20261105_091_compose_attempt_reviews.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, orphan := range []bool{false, true} {
		name := "preserves_review"
		if orphan {
			name = "rejects_orphan_before_mutation"
		}
		t.Run(name, func(t *testing.T) {
			prefix := fmt.Sprintf("qa_chg116_091_%d", time.Now().UnixNano())
			attempts, reviews := prefix+"_attempts", prefix+"_reviews"
			t.Cleanup(func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				for _, table := range []string{reviews, attempts} {
					if _, err := db.ExecContext(cleanupCtx, "DROP TABLE IF EXISTS "+table); err != nil {
						t.Errorf("clean disposable table %s: %v", table, err)
					}
				}
			})
			mustExec := func(query string, args ...any) {
				t.Helper()
				if _, err := db.ExecContext(ctx, query, args...); err != nil {
					t.Fatal(err)
				}
			}
			mustExec(`CREATE TABLE ` + attempts + ` (
				id BIGINT UNSIGNED PRIMARY KEY, task_id BIGINT UNSIGNED NOT NULL,
				attempt INT UNSIGNED NOT NULL, source VARCHAR(16) NOT NULL,
				outcome VARCHAR(16) NOT NULL, error_code VARCHAR(64) NOT NULL,
				created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
				UNIQUE KEY (task_id,attempt)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`)
			mustExec(`CREATE TABLE ` + reviews + ` (
				id BIGINT UNSIGNED PRIMARY KEY, task_id BIGINT UNSIGNED NOT NULL,
				attempt INT UNSIGNED NOT NULL, actor_user_id BIGINT UNSIGNED NOT NULL,
				evidence VARCHAR(2000) NOT NULL, created_at DATETIME(6) NOT NULL,
				UNIQUE KEY (task_id,attempt)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`)
			const timestamp = "2026-11-05 04:03:02.123456"
			const evidence = "人工核实旧执行已停止，成片数据库与对象存储均无产物；保留原始核对依据。"
			if !orphan {
				mustExec("INSERT INTO "+attempts+" VALUES (7,42,3,'manual','failed','execution_uncertain',?,?)", timestamp, timestamp)
			}
			mustExec("INSERT INTO "+reviews+" VALUES (9,42,3,?,?,?)", actorID, evidence, timestamp)
			rewritten := strings.NewReplacer(
				"compose_uncertain_reviews", reviews,
				"compose_task_attempts", attempts,
				"fk_compose_review_attempt_migration", prefix+"_attempt_fk",
				"fk_compose_attempt_review_actor", prefix+"_actor_fk",
				"chk_compose_attempt_review", prefix+"_review_check",
			).Replace(string(source))
			failedAt := -1
			var migrationErr error
			for i, statement := range strings.Split(rewritten, ";") {
				if strings.TrimSpace(statement) == "" {
					continue
				}
				if _, migrationErr = db.ExecContext(ctx, statement); migrationErr != nil {
					failedAt = i
					break
				}
			}
			count := func(query string, args ...any) int {
				t.Helper()
				var n int
				if err := db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
					t.Fatal(err)
				}
				return n
			}
			if orphan {
				if migrationErr == nil || failedAt != 0 {
					t.Fatalf("orphan must fail first DDL before any merge: statement=%d err=%v", failedAt, migrationErr)
				}
				if n := count("SELECT COUNT(*) FROM " + attempts); n != 0 {
					t.Fatalf("invented %d historical execution records", n)
				}
				if n := count("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=? AND table_name=? AND column_name='uncertain_reviewed_at'", schema, attempts); n != 0 {
					t.Fatal("attempt schema changed before orphan validation")
				}
				var savedActor int64
				var savedEvidence, savedTime string
				if err := db.QueryRowContext(ctx, "SELECT actor_user_id,evidence,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s.%f') FROM "+reviews).Scan(&savedActor, &savedEvidence, &savedTime); err != nil || savedActor != actorID || savedEvidence != evidence || savedTime != timestamp {
					t.Fatalf("orphan review lost: actor=%d evidence=%q time=%q err=%v", savedActor, savedEvidence, savedTime, err)
				}
				return
			}
			if migrationErr != nil {
				t.Fatalf("migration statement %d: %v", failedAt, migrationErr)
			}
			var savedActor int64
			var savedEvidence, savedTime, savedSource, outcome, code string
			if err := db.QueryRowContext(ctx, "SELECT uncertain_review_actor_id,uncertain_review_evidence,DATE_FORMAT(uncertain_reviewed_at,'%Y-%m-%d %H:%i:%s.%f'),source,outcome,error_code FROM "+attempts+" WHERE task_id=42 AND attempt=3").Scan(&savedActor, &savedEvidence, &savedTime, &savedSource, &outcome, &code); err != nil {
				t.Fatal(err)
			}
			if savedActor != actorID || savedEvidence != evidence || savedTime != timestamp || savedSource != "manual" || outcome != "failed" || code != "execution_uncertain" {
				t.Fatalf("facts changed: actor=%d evidence=%q time=%q source=%q outcome=%q error=%q", savedActor, savedEvidence, savedTime, savedSource, outcome, code)
			}
			if n := count("SELECT COUNT(*) FROM " + attempts); n != 1 {
				t.Fatalf("migration fabricated execution records: %d", n)
			}
			if n := count("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, reviews); n != 0 {
				t.Fatal("old review table survived successful merge")
			}
		})
	}
}
