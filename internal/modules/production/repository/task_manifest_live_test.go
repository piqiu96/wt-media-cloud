package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLiveComposeManifestCreationAndLeaseFreeze(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("explicit local MySQL verification")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.HomeEnvVar, root)
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
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	donor, err := scanComposeTask(tx.Raw(`SELECT ` + composeTaskColumns + ` FROM compose_tasks WHERE status IN ('success','failed','cancelled') ORDER BY id LIMIT 1 FOR UPDATE`).Row())
	if err != nil {
		t.Fatal(err)
	}
	donor.ID = 0
	donor.RequestKey = fmt.Sprintf("manifest124-%d", time.Now().UnixNano())
	donor.RandomSeed = 0
	donor.ExecutionManifest = nil
	donor.Status = model.ComposeTaskPending
	donor.TechnicalConfig = json.RawMessage(`{"width":1280,"preset":"medium"}`)
	if err = composeTaskInsertWithAudit(tx, &donor, "compose_task.create", donor.CreatedBy, time.Now()); err != nil {
		t.Fatal(err)
	}
	task, found, err := findComposeTask(tx, donor.ID)
	if err != nil || !found || task.RandomSeed == 0 || task.RandomSeed != donor.RandomSeed {
		t.Fatalf("snapshot readback failed: found=%v seed=%d err=%v", found, task.RandomSeed, err)
	}
	var technical map[string]any
	if json.Unmarshal(task.TechnicalConfig, &technical) != nil || technical["width"] != float64(1280) || technical["preset"] != "medium" {
		t.Fatal("technical configuration changed")
	}

	if err = tx.Exec(`UPDATE compose_tasks SET status='running',claimed_node_id=99124,attempt_count=1,cancel_requested=0,lease_expires_at=DATE_ADD(NOW(),INTERVAL 5 MINUTE) WHERE id=?`, task.ID).Error; err != nil {
		t.Fatal(err)
	}
	first := json.RawMessage(`{"version":1,"timeline":{"duration":49.2}}`)
	got, err := freezeComposeManifest(tx, task.ID, 99124, 1, first)
	if err != nil || !json.Valid(got) {
		t.Fatal(err)
	}
	got, err = freezeComposeManifest(tx, task.ID, 99124, 1, json.RawMessage(`{"different":1}`))
	if err != nil || string(got) == `{"different":1}` {
		t.Fatalf("first facts replaced: %s %v", got, err)
	}
	if _, err = freezeComposeManifest(tx, task.ID, 99125, 1, first); !errors.Is(err, ErrComposeCommitLost) {
		t.Fatal("stale node accepted", err)
	}
	tx.Exec(`UPDATE compose_tasks SET cancel_requested=1 WHERE id=?`, task.ID)
	if _, err = freezeComposeManifest(tx, task.ID, 99124, 1, first); !errors.Is(err, ErrComposeCommitLost) {
		t.Fatal("cancelled task accepted", err)
	}
}
