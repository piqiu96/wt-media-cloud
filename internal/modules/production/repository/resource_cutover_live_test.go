package repository

import (
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"os"
	"path/filepath"
	"testing"
)

func TestLiveLegacyCutoverRollbackAndCurrentAssetProtection(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_CUTOVER") != "1" {
		t.Skip("explicit local database maintenance test")
	}
	root, e := filepath.Abs("../../../../")
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv(config.HomeEnvVar, root)
	config.MustInitializeRuntimePaths()
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	if e = database.Initialize(cfg.Databases); e != nil {
		t.Fatal(e)
	}
	defer database.Close()
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	var oldID int64
	tx.Raw("SELECT id FROM materials WHERE COALESCE(source_object_key,'')<>'' AND source_object_key NOT LIKE 's3://wt-media/%' LIMIT 1").Scan(&oldID)
	if oldID == 0 {
		t.Skip("old data has already been cleaned")
	}
	var currentBefore int64
	tx.Table("materials").Where("source_object_key LIKE 's3://wt-media/%'").Count(&currentBefore)
	if currentBefore == 0 {
		t.Fatal("test requires a current asset to preserve")
	}
	errBackup := errors.New("backup disk unavailable")
	if _, e := retireLegacyResources(tx, true, func(CutoverBackup) error { return errBackup }); !errors.Is(e, errBackup) {
		t.Fatalf("backup failure=%v", e)
	}
	var remains int64
	tx.Table("materials").Where("id=?", oldID).Count(&remains)
	if remains != 1 {
		t.Fatal("backup failure deleted data")
	}
	// A current object attached to a retiring task must block the entire cleanup.
	var taskID int64
	tx.Raw("SELECT id FROM compose_tasks WHERE template_code='basic_cut' LIMIT 1").Scan(&taskID)
	if taskID == 0 {
		t.Fatal("need retired task")
	}
	tx.SavePoint("protected")
	if e = tx.Exec("UPDATE compose_tasks SET output_object_key='s3://wt-media/media-ku/finalVideo/999999.mp4' WHERE id=?", taskID).Error; e != nil {
		t.Fatal(e)
	}
	saved := false
	if _, e = retireLegacyResources(tx, true, func(CutoverBackup) error { saved = true; return nil }); e == nil || saved {
		t.Fatalf("current object protection err=%v saved=%v", e, saved)
	}
	tx.RollbackTo("protected")
	tx.SavePoint("unfinished")
	if e = tx.Exec("UPDATE compose_tasks SET status='pending' WHERE id=?", taskID).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = retireLegacyResources(tx, true, func(CutoverBackup) error { return nil }); e == nil {
		t.Fatal("unfinished task was removed")
	}
	tx.RollbackTo("unfinished")
	counts, e := retireLegacyResources(tx, true, func(b CutoverBackup) error {
		if len(b.Rows["source_contents"]) == 0 {
			return errors.New("missing pointer backup")
		}
		saved = true
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	var currentAfter, oldAfter, templates int64
	tx.Table("materials").Where("source_object_key LIKE 's3://wt-media/%'").Count(&currentAfter)
	tx.Table("materials").Where("id=?", oldID).Count(&oldAfter)
	tx.Table("compose_templates").Where("code IN ?", []string{"basic_cut", "guide_insert", "guide_pip"}).Count(&templates)
	if !saved || counts["materials"] == 0 || oldAfter != 0 || currentAfter != currentBefore || templates != 0 {
		t.Fatalf("bad scoped result counts=%v old=%d current=%d/%d templates=%d", counts, oldAfter, currentAfter, currentBefore, templates)
	}
	// Repeat within the outer rollback transaction is an empty, valid cleanup.
	again, e := retireLegacyResources(tx, true, func(CutoverBackup) error { return nil })
	if e != nil || again["materials"] != 0 || again["compose_tasks"] != 0 || again["file_transfer_tasks"] != 0 {
		t.Fatalf("repeat=%v err=%v", again, e)
	}
	t.Logf("rollback-tested rows=%v; current assets preserved=%d", counts, currentAfter)
}
