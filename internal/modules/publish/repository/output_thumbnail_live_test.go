package repository_test

import (
	"errors"
	filerepo "github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	productionrepo "github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"testing"
)

func TestLiveOutputThumbnailReferencePreventsFileDeletion(t *testing.T) {
	store, _ := ownedStore(t)
	if err := store.DB.Exec(`UPDATE composite_outputs SET media_json='{"duration_ms":1000,"thumbnail_file_id":"1"}' WHERE id=42`).Error; err != nil {
		t.Fatal(err)
	}
	used, err := productionrepo.OutputFileReferenced(store.DB, 1)
	if err != nil || !used {
		t.Fatalf("reference absent: %v %v", used, err)
	}
	fileStore := filerepo.Store{DB: store.DB, CheckReferences: production.CheckOutputFileReferences}
	if err = fileStore.BeginDelete(1); !errors.Is(err, files.ErrInUse) {
		t.Fatalf("thumbnail deleted: %v", err)
	}
	var marked int
	if err = store.DB.Raw("SELECT COUNT(*) FROM operations_files WHERE id=1 AND deleted_at IS NOT NULL").Scan(&marked).Error; err != nil || marked != 0 {
		t.Fatalf("referenced file was marked deleting: %d %v", marked, err)
	}
	if err = fileStore.BeginDelete(2); err != nil {
		t.Fatalf("unreferenced file blocked: %v", err)
	}
}

func TestLiveThumbnailReferenceKeepsAdjacentLargeIDsDistinct(t *testing.T) {
	store, _ := ownedStore(t)
	if err := store.DB.Exec(`UPDATE composite_outputs SET media_json='{"thumbnail_file_id":"9007199254740993"}' WHERE id=42`).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   int64
		want bool
	}{{9007199254740992, false}, {9007199254740993, true}} {
		got, err := productionrepo.OutputFileReferenced(store.DB, tc.id)
		if err != nil || got != tc.want {
			t.Fatalf("id=%d used=%v want=%v error=%v", tc.id, got, tc.want, err)
		}
	}
}
