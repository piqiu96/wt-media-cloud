package repository

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
)

func TestLiveSourceMetricCaptureFreshness(t *testing.T) {
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
	tx := database.DB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	var v model.SourceContent
	if err = tx.Raw("SELECT team_id, category_id, created_by FROM source_contents WHERE category_id IS NOT NULL LIMIT 1").Row().Scan(&v.TeamID, &v.CategoryID, &v.CreatedBy); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	v.Platform, v.PlatformContentID, v.SourceType = "douyin", fmt.Sprintf("capture-test-%d", now.UnixNano()), "search"
	v.Title, v.CreatedAt, v.UpdatedAt, v.MetricsUpdatedAt, v.LikeCount = "Capture test", now, now, &now, 10
	first, err := createSource(tx, v, []byte(`{"statistics":{"digg_count":10}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec("UPDATE source_contents SET status='ignored', ignored_reason='keep me' WHERE id=?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	fresh := now.Add(time.Second)
	v.MetricsUpdatedAt, v.LikeCount, v.UpdatedAt = &fresh, 20, fresh
	if _, err = createSource(tx, v, []byte(`{"statistics":{"digg_count":20}}`)); !errors.Is(err, errDuplicate) {
		t.Fatal(err)
	}
	// Same values still advance the capture time.
	freshest := fresh.Add(time.Second)
	v.MetricsUpdatedAt, v.UpdatedAt = &freshest, freshest
	if _, err = createSource(tx, v, []byte(`{"statistics":{"digg_count":20}}`)); !errors.Is(err, errDuplicate) {
		t.Fatal(err)
	}
	// Older delayed import and unknown history cannot overwrite the latest snapshot.
	for _, captured := range []*time.Time{&now, nil} {
		v.MetricsUpdatedAt, v.LikeCount = captured, 99
		if _, err = createSource(tx, v, []byte(`{}`)); !errors.Is(err, errDuplicate) {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		got, found, err := findSource(tx, first.ID)
		if err != nil || !found {
			t.Fatalf("read: %v %v", found, err)
		}
		if got.LikeCount != 20 || got.MetricsUpdatedAt == nil || !got.MetricsUpdatedAt.Equal(freshest) || got.Status != model.StatusIgnored || got.IgnoredReason != "keep me" || !got.CreatedAt.Equal(now) || got.CreatedBy != v.CreatedBy {
			t.Fatalf("capture/identity changed: %+v", got)
		}
	}
}
