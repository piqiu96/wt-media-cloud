package service

import (
	"context"
	"encoding/json"
	"errors"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"strings"
	"testing"
)

func TestComposeDoesNotCommitWhenThumbnailCannotBeSaved(t *testing.T) {
	deps, lifecycle, _, _, task, runDir := attemptFixture(t)
	deps.CreateThumbnail = func(context.Context, model.ComposeTask, int64, string, string) (int64, error) {
		return 0, errors.New("storage unavailable")
	}
	outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir, nil)
	if outcome == ComposeAttemptSuccess || lifecycle.committed != nil {
		t.Fatalf("output committed without a thumbnail: %s %+v", outcome, lifecycle.committed)
	}
}

func TestThumbnailProjectionRequiresExactOutputAndFileEvidence(t *testing.T) {
	output := model.CompositeOutput{ID: 55, SHA256: "video-sha", Media: json.RawMessage(`{"thumbnail_file_id":"9007199254740993"}`)}
	meta := filemodel.ImageMetadata{Width: 640, Height: 360, Format: "png", RenderVersion: "output-thumbnail.v1", RenderInput: json.RawMessage(`{"composite_output_id":"55","video_sha256":"video-sha","frame_ms":0}`)}
	metadata, _ := json.Marshal(meta)
	file := filemodel.File{ID: 9007199254740993, Type: "image", ResourceToken: strings.Repeat("a", 64), MetadataJSON: metadata}
	s := ComposeOutputService{thumbnailFile: func(context.Context, int64) (filemodel.File, error) { return file, nil }}
	if err := s.decorateThumbnail(context.Background(), &output); err != nil || output.Thumbnail == nil || output.Thumbnail.FileID != file.ID || output.Thumbnail.Width != 640 {
		t.Fatalf("%+v %v", output.Thumbnail, err)
	}
	output.SHA256 = "another-output"
	if err := s.decorateThumbnail(context.Background(), &output); !errors.Is(err, ErrComposeOutputVerificationFailed) || output.Thumbnail != nil {
		t.Fatal("foreign thumbnail accepted", err)
	}
	output.SHA256 = "video-sha"
	s.thumbnailFile = func(context.Context, int64) (filemodel.File, error) {
		return filemodel.File{}, errors.New("network unavailable")
	}
	if err := s.decorateThumbnail(context.Background(), &output); !errors.Is(err, ErrComposeOutputVerificationFailed) || output.Thumbnail != nil {
		t.Fatal("read failure became usable", err)
	}
	output.Media = json.RawMessage(`{"duration_ms":1000}`)
	if err := s.decorateThumbnail(context.Background(), &output); err != nil || output.Thumbnail != nil {
		t.Fatal("thumbnail invented for older output", err)
	}
}

func TestThumbnailCleanupOnlyAfterKnownCommitRollback(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		clean bool
	}{{"rollback", repository.ErrComposeCommitLost, true}, {"lost response", errors.New("connection lost"), false}, {"committed", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			deps, lifecycle, _, _, task, runDir := attemptFixture(t)
			lifecycle.commitErr = tc.err
			removed := false
			deps.RemoveThumbnail = func(context.Context, model.ComposeTask, int64) error { removed = true; return nil }
			RunComposeAttempt(context.Background(), deps, task, 9, runDir, nil)
			if removed != tc.clean {
				t.Fatalf("cleanup=%v want=%v", removed, tc.clean)
			}
		})
	}
}
