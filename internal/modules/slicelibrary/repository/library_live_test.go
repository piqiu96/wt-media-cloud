package repository_test

import (
	"context"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/repository"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLiveNamingAndLock(t *testing.T) {
	if os.Getenv("WT_MEDIA_SLICE_LIVE") != "1" {
		t.Skip("explicit local DB verification only")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WT_MEDIA_CLOUD_HOME", root)
	close, err := bootstrap.InitializeMigration()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	repo := repository.Repository{DB: database.DB()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := fmt.Sprint(time.Now().UnixNano())
	ids := []string{id, id + "1"}
	defer repo.DB.Where("source_id IN ?", ids).Delete(&model.Source{})
	err = repo.WithLock(ctx, "slice-test-"+id, func(locked repository.Repository) error {
		if e := repo.WithLock(ctx, "slice-test-"+id, func(repository.Repository) error { return nil }); e == nil {
			return fmt.Errorf("second connection acquired active lock")
		}
		first, e := locked.Reserve(ctx, model.Source{SourceID: ids[0], SHA256: "first", OriginalTitle: "原题", Basename: "测试" + id, ProfileJSON: `{}`, ReviewReason: "test"})
		if e != nil {
			return e
		}
		second, e := locked.Reserve(ctx, model.Source{SourceID: ids[1], SHA256: "second", OriginalTitle: "同名", Basename: first.Basename, ProfileJSON: `{}`, ReviewReason: "test"})
		if e != nil {
			return e
		}
		if second.Basename == first.Basename {
			return fmt.Errorf("different sources share filename")
		}
		retry, e := locked.Reserve(ctx, model.Source{SourceID: ids[0], SHA256: "first", Basename: "changed", ProfileJSON: `{}`})
		if e != nil {
			return e
		}
		if retry.Basename != first.Basename {
			return fmt.Errorf("retry renamed source")
		}
		if _, e = locked.Reserve(ctx, model.Source{SourceID: ids[0], SHA256: "different"}); e == nil {
			return fmt.Errorf("changed bytes accepted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
