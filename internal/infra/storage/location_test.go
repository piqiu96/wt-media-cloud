package storage

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStoredLocationsRemainAbsoluteWhileLegacyKeysKeepTheirOldHome(t *testing.T) {
	cfg := testStorageConfig("storage.test")
	cfg.Bucket = "wt-media"
	cfg.Prefix = ""
	cfg.LegacyBucket = "data"
	cfg.LegacyPrefix = "wt_media/dev/"
	s, err := newMinioStore(cfg, testCredential())
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Reference("media-ku/material/42.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if ref != "s3://wt-media/media-ku/material/42.mp4" {
		t.Fatal(ref)
	}
	for _, tc := range []struct{ ref, path string }{{ref, "/wt-media/media-ku/material/42.mp4"}, {"materials/42/old.mp4", "/data/wt_media/dev/materials/42/old.mp4"}} {
		grant, err := s.PresignGet(context.Background(), tc.ref, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(grant.URL)
		if u.Path != tc.path {
			t.Fatalf("%s: %s", tc.ref, u.Path)
		}
	}
	s.prefix = "changed/"
	grant, err := s.PresignGet(context.Background(), ref, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(grant.URL, "/wt-media/media-ku/material/42.mp4?") {
		t.Fatal(grant.URL)
	}
	for _, bad := range []string{"s3://other/a.mp4", "s3://wt-media/a/../b.mp4", "s3://wt-media/a.mp4?token=x", "s3://user@wt-media/a.mp4", "s3://wt-media/%2e%2e/a.mp4"} {
		if _, err := s.PresignGet(context.Background(), bad, time.Minute); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
