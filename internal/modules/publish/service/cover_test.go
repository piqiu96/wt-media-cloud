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
	snapshot := image.ArtworkSnapshot{TemplateID: 8, PublishedRevision: 2, Title: "正式封面标题", BackgroundOutputID: 42, BackgroundFileID: 81, RendererVersion: render.Version, Document: render.DefaultDocument(128, 128)}
	raw, _ := json.Marshal(snapshot)
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	metadata, _ := json.Marshal(filemodel.ImageMetadata{RenderVersion: render.Version, RenderInputHash: hash, RenderInput: raw, Width: 128, Height: 128, Format: "png"})
	return filemodel.File{ID: 5, Type: "image", SizeBytes: 10, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MetadataJSON: metadata}, &model.CoverRender{TemplateID: 8, PublishedRevision: 2, Title: snapshot.Title, BackgroundOutputID: 42, RenderInputHash: hash}
}
func TestCreateAndEditRequireMatchingFrozenRenderedCover(t *testing.T) {
	for _, operation := range []string{"create", "edit"} {
		for _, change := range []string{"none", "missing", "title", "output", "hash"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				s, a, in := fixture()
				f, ref := renderedCoverFixture()
				s.File = func(context.Context, identity.PublicUser, int64, int64) (filemodel.File, error) { return f, nil }
				s.SavedFile = func(context.Context, int64) (filemodel.File, error) { return f, nil }
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
