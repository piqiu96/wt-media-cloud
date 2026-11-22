package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
	image "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
)

func renderedCoverFixture() (filemodel.File, *model.CoverRender) {
	snapshot := image.ArtworkSnapshot{TemplateID: 8, PublishedRevision: 2, Title: "正式封面标题", BackgroundOutputID: 42, BackgroundFileID: 81, RendererVersion: render.Version, Document: render.DefaultDocument(1280, 720)}
	raw, _ := json.Marshal(snapshot)
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	metadata, _ := json.Marshal(filemodel.ImageMetadata{RenderVersion: render.Version, RenderInputHash: hash, RenderInput: raw, Width: 1280, Height: 720, Format: "png"})
	return filemodel.File{ID: 5, Type: "image", SizeBytes: 10, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MetadataJSON: metadata}, &model.CoverRender{TemplateID: 8, PublishedRevision: 2, Title: snapshot.Title, BackgroundOutputID: 42, RenderInputHash: hash}
}
func TestCreateAndEditRequireMatchingFrozenRenderedCover(t *testing.T) {
	for _, operation := range []string{"create", "edit"} {
		for _, change := range []string{"none", "missing", "title", "output", "hash"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				s, a, in := fixture()
				f, ref := renderedCoverFixture()
				originalFile := s.File
				s.File = func(ctx context.Context, a identity.PublicUser, id, cat int64) (filemodel.File, error) {
					if id == 6 {
						return originalFile(ctx, a, id, cat)
					}
					return f, nil
				}
				s.SavedFile = func(ctx context.Context, id int64) (filemodel.File, error) {
					if id == 6 {
						return originalFile(ctx, a, id, in.CategoryID)
					}
					return f, nil
				}
				in.Content.CoverRender = ref
				original := in
				var task model.Task
				var err error
				if operation == "edit" {
					task, err = s.Create(context.Background(), a, original)
					if err != nil {
						t.Fatal(err)
					}
				}
				switch change {
				case "missing":
					in.Content.CoverRender = nil
				case "title":
					ref.Title = "修改标题"
				case "output":
					ref.BackgroundOutputID = 43
				case "hash":
					ref.RenderInputHash = "bad"
				}
				if operation == "edit" {
					task, err = s.Edit(context.Background(), a, task.ID, EditInput{ExpectedRevision: task.Revision, Content: in.Content})
				} else {
					task, err = s.Create(context.Background(), a, in)
				}
				if change == "none" {
					if err != nil {
						t.Fatal(err)
					}
					var snapshot map[string]any
					if json.Unmarshal(task.ResourceSnapshot, &snapshot) != nil || snapshot["cover_render"] == nil {
						t.Fatal("missing frozen snapshot")
					}
				} else if err == nil {
					t.Fatal("invalid rendered input accepted")
				}
			})
		}
	}
}

func TestBaiduCoverSpecRejectsWrongRatioMissingMetadataAndOversize(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		size          int64
		valid         bool
	}{
		{"horizontal", 1280, 720, 100, true}, {"portrait", 1080, 1440, 100, false},
		{"unknown", 0, 0, 100, false}, {"too-large", 1280, 720, 5*1024*1024 + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(filemodel.ImageMetadata{Width: tc.width, Height: tc.height, Format: "png"})
			f := filemodel.File{Type: "image", SizeBytes: tc.size, MetadataJSON: raw}
			if (verifyCoverSpec(f, 16, 9) == nil) != tc.valid {
				t.Fatalf("spec validation mismatch: %s", tc.name)
			}
		})
	}
}

func TestCreateRequiresSeparatePortraitBeforeOccupyingOutput(t *testing.T) {
	for _, name := range []string{"missing", "wrong-ratio", "too-large"} {
		t.Run(name, func(t *testing.T) {
			s, a, in := fixture()
			if name == "missing" {
				in.Content.PortraitCoverFileID = 0
			} else {
				read := s.File
				s.File = func(ctx context.Context, a identity.PublicUser, id, category int64) (filemodel.File, error) {
					f, err := read(ctx, a, id, category)
					if id == in.Content.PortraitCoverFileID {
						if name == "too-large" {
							f.SizeBytes = 5*1024*1024 + 1
						} else {
							f.MetadataJSON = []byte(`{"width":1280,"height":720,"format":"png"}`)
						}
					}
					return f, err
				}
			}
			if _, err := s.Create(context.Background(), a, in); err == nil {
				t.Fatal("incomplete cover set accepted")
			}
			if len(s.Repo.(*memoryTasks).items) != 0 {
				t.Fatal("invalid cover set occupied output")
			}
		})
	}
}
