package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

func TestBilibiliBatchCoversKeepBothIndependentResourcePaths(t *testing.T) {
	s, actor, _ := fixture()
	row := model.BatchCandidate{RowKey: "bili-row", Content: biliContentFixture()}
	s.File = func(_ context.Context, _ identity.PublicUser, id, _ int64) (filemodel.File, error) {
		size := []byte(`{"width":1200,"height":900,"format":"png"}`)
		if id == row.Content.SpaceCoverFileID {
			size = []byte(`{"width":1280,"height":720,"format":"png"}`)
		}
		return filemodel.File{ID: id, Type: "image", SizeBytes: 1, SHA256: strings.Repeat("a", 64), ResourceToken: "cover-token", MetadataJSON: size}, nil
	}
	issues, resources, err := s.batchCandidateCovers(context.Background(), actor, "bilibili", row, nil, false)
	if err != nil || len(issues) != 0 || resources.CoverResourcePath == "" || resources.SpaceCoverResourcePath == "" || resources.PortraitCoverResourcePath != "" {
		t.Fatalf("wrong cover association: %+v %+v %v", issues, resources, err)
	}
	old := s.File
	s.File = func(ctx context.Context, a identity.PublicUser, id, cat int64) (filemodel.File, error) {
		f, e := old(ctx, a, id, cat)
		if id == row.Content.SpaceCoverFileID {
			f.MetadataJSON = []byte(`{"width":900,"height":1200}`)
		}
		return f, e
	}
	issues, _, err = s.batchCandidateCovers(context.Background(), actor, "bilibili", row, nil, false)
	if err != nil || len(issues) != 1 || issues[0].Field != "space_cover_file_id" {
		t.Fatalf("Baidu portrait accepted as space cover: %+v %v", issues, err)
	}
}

func TestCopyBilibiliCandidateDropsFormalProductsAndOwnsOptions(t *testing.T) {
	c := biliContentFixture()
	copy := copyCandidateContent(c)
	if copy.Title != c.Title || copy.Description != c.Description || copy.Bilibili == nil || copy.Bilibili.Statement != c.Bilibili.Statement || copy.Confirmed || copy.CoverFileID != 0 || copy.SpaceCoverFileID != 0 || copy.CoverRender != nil || copy.SpaceCoverRender != nil {
		t.Fatalf("copied execution products or lost text: %+v", copy)
	}
	copy.Bilibili.Tags[0] = "changed"
	if c.Bilibili.Tags[0] == "changed" {
		t.Fatal("copy mutates saved source")
	}
}

func TestBilibiliPlanRequiresDeploymentAvailabilityAndConservesEachPlatform(t *testing.T) {
	s, a, _ := fixture()
	s.BatchCategory = func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}
	in := PlanInput{Name: "多平台计划", DateStart: "2026-11-26", DateEnd: "2026-11-27", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 1}, {Platform: "bilibili", CategoryID: 1, TargetCount: 2}}, Batches: []model.PlanStructureBatch{
		{RowKey: "a", Name: "百度批次", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}},
		{RowKey: "b", Name: "B站批次", Platform: "bilibili", GroupOrder: 2, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 2}}},
	}}
	if err := s.validatePlan(a, in); !errors.Is(err, ErrClosed) {
		t.Fatalf("default opened Bilibili plan: %v", err)
	}
	s.PlatformSettings = config.PublicationPlatformsConfig{BilibiliEnabled: true}
	if err := s.validatePlan(a, in); err != nil {
		t.Fatal(err)
	}
	in.Batches[1].Targets[0].TargetCount = 1
	if err := s.validatePlan(a, in); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cross-platform target leak: %v", err)
	}
}
