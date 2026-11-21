package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/render"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"testing"
	"time"
)

func TestLiveTemplatePublicationPreservesPublishedContentAndUsage(t *testing.T) {
	db := isolated(t)
	assets, a := fixture(db)
	seedFile(t, db, 41, 1080, 1440)
	image, e := assets.RegisterImageAsset(context.Background(), a, service.RegisterInput{FileID: 41, Name: "底图", AllowAsBackground: true, CategoryIDs: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	doc := render.DefaultDocument(1080, 1440)
	doc.BackgroundMode = "fixed"
	doc.ImageAssetID = image.ID
	data, _ := json.Marshal(doc)
	now := time.Now().UTC()
	store := repository.TemplateStore{DB: db}
	v, e := store.Create(model.CoverTemplate{TeamID: 10, Name: "测试模板", CanvasWidth: 1080, CanvasHeight: 1440, Scope: "general", PlatformCodes: []string{}, CategoryIDs: []string{}, DraftConfig: data, RequestKey: "template-test", RequestHash: "hash", Revision: 1, CreatedBy: 1, UpdatedBy: 1, CreatedAt: now, UpdatedAt: now}, []repository.AssetBinding{{ID: image.ID, FileID: 41}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.SetUsage(v.ID, v.Revision, 1, true); !errors.Is(e, repository.ErrConflict) {
		t.Fatal("unpublished draft enabled")
	}
	if used, e := repository.TemplateFileReferenced(db, 41); e != nil || !used {
		t.Fatalf("fixed background not protected: %v", e)
	}
	v.PublishedConfig = v.DraftConfig
	v.PublishedThumbnailFileID = new(int64)
	*v.PublishedThumbnailFileID = 41
	v, e = store.Publish(v, v.Revision, []repository.AssetBinding{{ID: image.ID, FileID: 41}})
	if e != nil || v.Enabled || v.PublishedRevision != 1 || v.DraftConfig != nil {
		t.Fatalf("first publish: %+v %v", v, e)
	}
	v, e = store.SetUsage(v.ID, v.Revision, 1, true)
	if e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), v.PublishedConfig...)
	doc.Layers[0].Text = "新的草稿"
	v.DraftConfig, _ = json.Marshal(doc)
	v, e = store.SaveDraft(v, v.Revision, []repository.AssetBinding{{ID: image.ID, FileID: 41}})
	if e != nil || !v.Enabled || !repository.SameJSON(original, v.PublishedConfig) {
		t.Fatalf("draft changed published: %v", e)
	}
	if e = store.Delete(v.ID, v.Revision); !errors.Is(e, repository.ErrConflict) {
		t.Fatal("published template deleted")
	}
	stale := v
	v.PublishedConfig = v.DraftConfig
	v, e = store.Publish(v, v.Revision, []repository.AssetBinding{{ID: image.ID, FileID: 41}})
	if e != nil || !v.Enabled || v.PublishedRevision != 2 {
		t.Fatalf("republish: %v", e)
	}
	if _, e = store.SaveDraft(stale, stale.Revision, nil); !errors.Is(e, repository.ErrConflict) {
		t.Fatal("stale draft overwrote new publication")
	}
}

func TestLiveTemplateLostReplyReplaysSavedResultAfterDependenciesChange(t *testing.T) {
	db := isolated(t)
	assets, a := fixture(db)
	s := &service.TemplateService{Repo: repository.TemplateStore{DB: db}, Assets: assets}
	input := service.CreateTemplateInput{RequestKey: "create-lost-reply", Name: "可重放模板", CanvasWidth: 1080, CanvasHeight: 1440, CategoryIDs: []string{"2"}, PlatformCodes: []string{}}
	first, err := s.CreateCoverTemplate(context.Background(), a, input)
	if err != nil {
		t.Fatal(err)
	}
	assets.CategoryAllowed = func(identity.PublicUser, int64) (bool, error) { return false, nil }
	replay, err := s.CreateCoverTemplate(context.Background(), a, input)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("lost response cannot recover saved template after category disabled: %v", err)
	}
	assets.CategoryAllowed = func(identity.PublicUser, int64) (bool, error) { return true, nil }
	input.RequestKey = "copy-lost-reply"
	input.Name = "复制结果"
	copied, err := s.CopyCoverTemplate(context.Background(), a, first.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteCoverTemplate(a, first.ID, first.Revision); err != nil {
		t.Fatal(err)
	}
	replay, err = s.CopyCoverTemplate(context.Background(), a, first.ID, input)
	if err != nil || replay.ID != copied.ID {
		t.Fatalf("lost copy response cannot recover saved copy after source deletion: %v", err)
	}
}
