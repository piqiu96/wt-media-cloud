package repository_test

import (
	"errors"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	filerepo "github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	imagemodel "github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	imagerepo "github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	imageservice "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"testing"
	"time"
)

func TestLiveArtworkSourceReferenceAndReadyFence(t *testing.T) {
	db := isolated(t)
	seedFile(t, db, 1, 640, 360)
	seedFile(t, db, 2, 640, 360)
	metadata := `{"render_version":"noto-cjk-20261121.1","render_input":{"background_file_id":"1"}}`
	if err := db.Exec("UPDATE operations_files SET metadata_json=? WHERE id=2", metadata).Error; err != nil {
		t.Fatal(err)
	}
	fileStore := filerepo.Store{DB: db, CheckReferences: imageservice.CheckArtworkFileReferences}
	if err := fileStore.BeginDelete(1); !errors.Is(err, files.ErrInUse) {
		t.Fatalf("source deletion not blocked: %v", err)
	}
	refs, err := imagerepo.ListArtworkFileReference(db, imagemodel.Scope{OwnerID: 1}, 1)
	if err != nil || len(refs) != 1 || refs[0].ID != "2" {
		t.Fatalf("reference missing: %v %v", refs, err)
	}
	refs, err = imagerepo.ListArtworkFileReference(db, imagemodel.Scope{OwnerID: 199}, 1)
	if err != nil || len(refs) != 0 {
		t.Fatalf("cross owner reference exposed: %v %v", refs, err)
	}
	if err = db.Exec("UPDATE operations_files SET uploaded_at=NULL WHERE id=2").Error; err != nil {
		t.Fatal(err)
	}
	if err = fileStore.BeginDelete(1); err != nil {
		t.Fatal(err)
	}
	var pending filemodel.File
	if err = db.First(&pending, 2).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	pending.UploadedAt = &now
	pending.ReferenceFileIDs = []int64{1}
	if err = fileStore.Ready(pending); !errors.Is(err, filerepo.ErrNotFound) {
		t.Fatalf("deleted source accepted: %v", err)
	}
	var saved filemodel.File
	if err = db.First(&saved, 2).Error; err != nil || saved.UploadedAt != nil {
		t.Fatalf("failed reference committed: %v", err)
	}
	if err = db.Exec("UPDATE operations_files SET deleted_at=NULL WHERE id=1").Error; err != nil {
		t.Fatal(err)
	}
	if err = fileStore.Ready(pending); err != nil {
		t.Fatal(err)
	}
	if err = fileStore.BeginDelete(1); !errors.Is(err, files.ErrInUse) {
		t.Fatalf("committed source unprotected: %v", err)
	}
}
