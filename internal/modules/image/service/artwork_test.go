package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	imagemodel "github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
	productionmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"io"
	"strings"
	"testing"
)

func TestArtworkFileKeepsIdentifierPrecision(t *testing.T) {
	const id = int64(9007199254740993)
	data, err := json.Marshal(ArtworkFile{FileView: filemodel.FileView{ID: id, Name: "实际输出"}, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != "9007199254740993" || result["name"] != "实际输出" {
		t.Fatalf("wire file: %s", data)
	}
}

func TestArtworkBackgroundUsesAuthorizedOutputThumbnail(t *testing.T) {
	calls := 0
	svc := &ArtworkService{Output: func(a identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		calls++
		if id != 71 || a.ID != 9 {
			t.Fatal("wrong scope")
		}
		return production.ComposeOutputDetail{Output: productionmodel.CompositeOutput{ID: 71, Thumbnail: &productionmodel.ComposeOutputThumbnail{FileID: 81}}}, nil
	}}
	id, asset, output, err := svc.backgroundFile(identity.PublicUser{ID: 9}, render.Document{BackgroundMode: "business"}, ArtworkInput{BackgroundOutputID: 71})
	if err != nil || id != 81 || asset != 0 || output != 71 || calls != 1 {
		t.Fatalf("%d %d %d %v", id, asset, output, err)
	}
	svc.Output = func(identity.PublicUser, int64) (production.ComposeOutputDetail, error) {
		return production.ComposeOutputDetail{}, errors.New("not authorized")
	}
	if _, _, _, err = svc.backgroundFile(identity.PublicUser{}, render.Document{BackgroundMode: "business"}, ArtworkInput{BackgroundOutputID: 71}); err == nil {
		t.Fatal("unauthorized output accepted")
	}
	svc.Output = func(identity.PublicUser, int64) (production.ComposeOutputDetail, error) {
		return production.ComposeOutputDetail{Output: productionmodel.CompositeOutput{ID: 71}}, nil
	}
	if _, _, _, err = svc.backgroundFile(identity.PublicUser{}, render.Document{BackgroundMode: "business"}, ArtworkInput{BackgroundOutputID: 71}); err == nil {
		t.Fatal("missing thumbnail accepted")
	}
	for _, d := range []render.Document{{BackgroundMode: "business"}, {BackgroundMode: "solid"}, {BackgroundMode: "fixed", ImageAssetID: 1}} {
		if _, _, _, err = svc.backgroundFile(identity.PublicUser{}, d, ArtworkInput{BackgroundOutputID: 71, BackgroundImageAssetID: 1}); err == nil {
			t.Fatal("ambiguous or ignored background accepted")
		}
	}
}
func TestArtworkReferenceRejectsChangedInputAndPreview(t *testing.T) {
	snapshot := ArtworkSnapshot{TemplateID: 8, PublishedRevision: 2, Title: "实际标题", BackgroundOutputID: 71, BackgroundFileID: 81, RendererVersion: render.Version, Document: render.DefaultDocument(128, 128)}
	raw, _ := json.Marshal(snapshot)
	hash := templateHash(snapshot)
	metadata := filemodel.ImageMetadata{RenderVersion: render.Version, RenderInputHash: hash, RenderInput: raw, Width: 128, Height: 128, Format: "png"}
	f := filemodel.File{ID: 91, Type: "image", SizeBytes: 10, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	f.MetadataJSON, _ = json.Marshal(metadata)
	ref := ArtworkReference{TemplateID: 8, PublishedRevision: 2, Title: "实际标题", BackgroundOutputID: 71, RenderInputHash: hash}
	if _, err := VerifyArtwork(f, ref); err != nil {
		t.Fatal(err)
	}
	changed := ref
	changed.Title = "改过标题"
	if _, err := VerifyArtwork(f, changed); err == nil {
		t.Fatal("stale title accepted")
	}
	changed = ref
	changed.BackgroundOutputID = 72
	if _, err := VerifyArtwork(f, changed); err == nil {
		t.Fatal("stale output accepted")
	}
	metadata.Preview = true
	f.MetadataJSON, _ = json.Marshal(metadata)
	if _, err := VerifyArtwork(f, ref); err == nil {
		t.Fatal("preview accepted")
	}
}

type assignedImageFile struct {
	files.Repository
	file filemodel.File
}

func (r assignedImageFile) Find(id int64) (filemodel.File, error) {
	if id != r.file.ID {
		return filemodel.File{}, files.ErrNotFound
	}
	return r.file, nil
}

type assignedImageObject struct {
	storage.ImmutableStore
	reached *bool
	failure error
}

func (o assignedImageObject) Open(context.Context, string) (io.ReadCloser, error) {
	*o.reached = true
	return nil, o.failure
}
func TestRenderUsesAuthorizedImageReferenceWithoutExpandingRawFileAccess(t *testing.T) {
	team := identity.TeamID(10)
	actor := identity.PublicUser{ID: 2, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{2}}
	// The administrator assigned this asset to the team; their raw upload has no team.
	asset := imagemodel.ImageAsset{ID: 1, FileID: 7, TeamID: 10, CreatedBy: 1, Enabled: true, AllowAsBackground: true, CategoryIDs: []string{"2"}, MetadataJSON: json.RawMessage(`{"width":640,"height":360,"format":"png"}`)}
	reached := false
	sentinel := errors.New("authorized object reached")
	fs := &files.Service{Repo: assignedImageFile{file: filemodel.File{ID: 7, OwnerID: 1, Type: "image", SizeBytes: 10, SHA256: strings.Repeat("a", 64)}}, Objects: assignedImageObject{reached: &reached, failure: sentinel}, MaxBytes: 1024}
	if _, err := fs.Read(actor, 7); !errors.Is(err, files.ErrNotFound) {
		t.Fatal("raw file scope unexpectedly expanded")
	}
	svc := &ArtworkService{Assets: &Service{Repo: &memoryAsset{rows: map[uint64]imagemodel.ImageAsset{1: asset}}}, FileService: func() (*files.Service, error) { return fs, nil }}
	doc := render.DefaultDocument(640, 360)
	doc.BackgroundMode = "business"
	_, _, err := svc.RenderDocument(context.Background(), actor, doc, "实际标题", 1, false, ArtworkInput{})
	if !errors.Is(err, sentinel) || !reached {
		t.Fatalf("authorized asset rejected by raw upload ownership: %v", err)
	}
	reached = false
	other := identity.TeamID(11)
	actor.TeamID = &other
	if _, _, err = svc.RenderDocument(context.Background(), actor, doc, "实际标题", 1, false, ArtworkInput{}); err == nil || reached {
		t.Fatal("foreign asset reached object storage")
	}
}
