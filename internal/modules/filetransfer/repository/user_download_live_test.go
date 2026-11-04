package repository

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// The actual MySQL table, transactions and unique key arbitrate concurrent
// requests. Only uniquely named fixture rows are written/deleted. A completed
// fixture dependency without object facts keeps every new local task unclaimable
// and unreconcilable, so neither the dev Agent nor Cloud worker can consume it.
func TestLiveUserDownloadAfterSuccessCreatesNewGenerationAndConcurrentRequestsDedupe(t *testing.T) {
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
	db := database.DB().WithContext(ctx)
	var schema string
	if err = db.Raw(`SELECT DATABASE()`).Row().Scan(&schema); err != nil || schema != "wt_media_cloud" {
		t.Fatalf("fixed database=%s err=%v", schema, err)
	}
	var userID identity.UserID
	var teamID identity.TeamID
	if err = db.Raw(`SELECT id,team_id FROM users WHERE status='enabled' AND role='operator' AND team_id IS NOT NULL ORDER BY id LIMIT 1`).Row().Scan(&userID, &teamID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	prefix := fmt.Sprintf("qa118rdl-%d", now.UnixNano())
	parentID := prefix + "-p"
	assetID := now.UnixMicro()
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		clean := database.DB().WithContext(cleanupCtx)
		if err := clean.Exec(`DELETE FROM file_transfer_tasks WHERE id LIKE ? AND dependency_task_id=? AND requested_by=?`, prefix+"%", parentID, userID).Error; err != nil {
			t.Errorf("delete own download fixtures: %v", err)
		}
		if err := clean.Exec(`DELETE FROM file_transfer_tasks WHERE id=? AND requested_by=? AND asset_id=?`, parentID, userID, assetID).Error; err != nil {
			t.Errorf("delete own dependency fixture: %v", err)
		}
	}()
	// No actual object or unfinished cloud operation is represented by this guard.
	if err = db.Exec(`INSERT INTO file_transfer_tasks(id,team_id,asset_type,asset_id,asset_title,purpose,execution_scope,status,requested_by,dedupe_key,created_at,updated_at) VALUES(?,?,'material',?,'QA118 download isolation','compose_input_prepare','cloud','success',?,SHA2(?,256),?,?)`, parentID, teamID, assetID, userID, parentID, now, now).Error; err != nil {
		t.Fatal(err)
	}
	for caseIndex, assetType := range []model.AssetType{model.AssetMaterial, model.AssetCompositeOutput} {
		t.Run(string(assetType), func(t *testing.T) {
			input := CreateUserDownloadInput{ID: fmt.Sprintf("%s-%d-old", prefix, caseIndex), TeamID: teamID, AssetType: assetType, AssetID: assetID + int64(caseIndex), AssetTitle: "QA118 original instruction", CategoryName: "QA category", SourceObjectKey: "qa118/not-fetched.mp4", RequestedBy: userID, TotalBytes: 10, ExpectedSHA256: strings.Repeat("a", 64), DependencyTaskID: parentID, MaxAttempts: 3}
			first, err := createUserDownloadTask(db, input, now)
			if err != nil {
				t.Fatal(err)
			}
			if err = db.Exec(`UPDATE file_transfer_tasks SET status='success',transferred_bytes=10,integrity_bytes=10,integrity_sha256=?,file_name='original-file.mp4',finished_at=?,updated_at=? WHERE id=?`, strings.Repeat("a", 64), now, now, first.ID).Error; err != nil {
				t.Fatal(err)
			}
			original, err := getTask(db, first.ID)
			if err != nil {
				t.Fatal(err)
			}
			input.ID = fmt.Sprintf("%s-%d-new", prefix, caseIndex)
			next, err := createUserDownloadTask(db, input, now.Add(time.Second))
			if err != nil || next.ID == first.ID || next.Status != model.StatusPending {
				t.Fatalf("explicit request reused terminal result: first=%s next=%+v err=%v", first.ID, next, err)
			}
			input.ID = fmt.Sprintf("%s-%d-dup", prefix, caseIndex)
			duplicate, err := createUserDownloadTask(db, input, now.Add(time.Second))
			if err != nil || duplicate.ID != next.ID {
				t.Fatalf("pending duplicate=%+v expected=%s err=%v", duplicate, next.ID, err)
			}
			if err = db.Exec(`UPDATE file_transfer_tasks SET status='running',attempt_count=1,attempt_id='qa118-isolated',claimed_by='qa118-isolated',lease_expires_at=? WHERE id=?`, now.Add(time.Hour), next.ID).Error; err != nil {
				t.Fatal(err)
			}
			input.ID = fmt.Sprintf("%s-%d-run", prefix, caseIndex)
			running, err := createUserDownloadTask(db, input, now.Add(2*time.Second))
			if err != nil || running.ID != next.ID || running.Status != model.StatusRunning {
				t.Fatalf("running duplicate=%+v expected=%s err=%v", running, next.ID, err)
			}
			// Finish generation 2 without any executor. Every concurrent request must
			// now obtain one shared generation 3, never the old success or cancelled row.
			if err = db.Exec(`UPDATE file_transfer_tasks SET status='cancelled',finished_at=?,updated_at=? WHERE id=?`, now.Add(2*time.Second), now.Add(2*time.Second), next.ID).Error; err != nil {
				t.Fatal(err)
			}
			type result struct {
				task model.Task
				err  error
			}
			start := make(chan struct{})
			results := make(chan result, 8)
			for i := 0; i < 8; i++ {
				request := input
				request.ID = fmt.Sprintf("%s-%d-r%d", prefix, caseIndex, i)
				go func() {
					<-start
					task, err := createUserDownloadTask(db, request, now.Add(3*time.Second))
					results <- result{task, err}
				}()
			}
			close(start)
			responses := make([]result, 0, 8)
			for i := 0; i < 8; i++ {
				responses = append(responses, <-results)
			}
			survivor := ""
			for _, got := range responses {
				if got.err != nil || got.task.Status != model.StatusPending || got.task.ID == first.ID || got.task.ID == next.ID {
					t.Fatalf("concurrent new generation task=%+v err=%v", got.task, got.err)
				}
				if survivor == "" {
					survivor = got.task.ID
				} else if got.task.ID != survivor {
					t.Fatalf("duplicate concurrent requests produced %s and %s", survivor, got.task.ID)
				}
			}
			var total, terminal, pending int64
			if err = db.Raw(`SELECT COUNT(*),SUM(status IN ('success','failed','cancelled')),SUM(status='pending') FROM file_transfer_tasks WHERE asset_type=? AND asset_id=? AND purpose='user_download' AND requested_by=?`, assetType, input.AssetID, userID).Row().Scan(&total, &terminal, &pending); err != nil || total != 3 || terminal != 2 || pending != 1 {
				t.Fatalf("generation counts total=%d terminal=%d pending=%d err=%v", total, terminal, pending, err)
			}
			after, err := getTask(db, first.ID)
			if err != nil || !reflect.DeepEqual(original, after) {
				t.Fatalf("successful historical transfer changed before=%+v after=%+v err=%v", original, after, err)
			}
			t.Logf("old=%s generation2=%s concurrent_generation3=%s rows=%d", first.ID, next.ID, survivor, total)
		})
	}
}
