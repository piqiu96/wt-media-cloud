package service

import (
	"context"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShortTitleAndClipBudget(t *testing.T) {
	for _, title := range []string{"../../舞蹈/视频:测试", "", "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十"} {
		got := ShortTitle(title)
		if got == "" || len([]rune(got)) > 32 || len(got) > 96 {
			t.Fatalf("unsafe title %q", got)
		}
	}
	if got := ClipCount(17.9, 6, 8); got != 2 {
		t.Fatalf("remainder included: %d", got)
	}
	if got := ClipCount(60, 6, 3); got != 3 {
		t.Fatalf("source cap ignored: %d", got)
	}
}
func TestDecisionBindsDownloadedBytes(t *testing.T) {
	c := Candidate{SourceID: "123", SHA256: "abc"}
	if err := ValidateDecision(c, Decision{SourceID: "123", SHA256: "changed", Suitable: true, Title: "舞蹈", Reason: "ok"}); err == nil {
		t.Fatal("stale review accepted")
	}
	if err := ValidateDecision(c, Decision{SourceID: "123", SHA256: "abc", Suitable: true, Title: "舞蹈", Reason: "画面适合"}); err != nil {
		t.Fatal(err)
	}
}

func TestRejectedCandidatePublishesNothing(t *testing.T) {
	dir := t.TempDir()
	raw := `{"source_id":"123","sha256":"abc"}`
	if err := os.WriteFile(filepath.Join(dir, "candidate.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	s := Service{Config: config.SliceLibraryConfig{Prefix: "file-ku/test/", ClipSeconds: 6, MaxCandidates: 2, MaxClipsPerSource: 2, MaxSourceBytes: 1024, Timeout: config.Duration{Duration: time.Minute}, Preset: "medium", CRF: 20, SampleCount: 8}}
	if _, err := s.Publish(context.Background(), dir, Decision{SourceID: "123", SHA256: "abc", Suitable: false, Reason: "非舞蹈素材"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("rejected source retained: %v", err)
	}
}

func TestPublishCannotExtendReviewedRange(t *testing.T) {
	c := Candidate{Profile: model.Profile{ClipSeconds: 6, MaxClips: 1, CRF: 20, Preset: "medium"}, ReviewedSeconds: 6}
	p, err := ApprovedProfile(c, 8)
	if err != nil || p.MaxClips != 1 {
		t.Fatalf("publish extended review: %+v %v", p, err)
	}
	c.Profile.MaxClips = 8
	if _, err = ApprovedProfile(c, 8); err == nil {
		t.Fatal("profile extending sampled span accepted")
	}
}

func TestLongAudioCannotMakeShortVideoUsable(t *testing.T) {
	if got := UsableDuration(media.Info{Duration: 60, VideoDuration: 2, AudioDuration: 60}); got != 2 {
		t.Fatalf("audio expanded video: %g", got)
	}
}
