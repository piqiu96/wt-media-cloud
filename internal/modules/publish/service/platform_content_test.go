package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/config"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

func TestBilibiliCreateUsesItsOwnCoversAndFrozenContent(t *testing.T) {
	s, actor, input := fixture()
	input.Platform = "bilibili"
	input.Content = biliContentFixture()
	if _, err := s.Create(context.Background(), actor, input); !errors.Is(err, ErrClosed) {
		t.Fatalf("default deployment must be closed: %v", err)
	}
	s.PlatformSettings = config.PublicationPlatformsConfig{BilibiliEnabled: true}
	oldAccount, oldOutput, oldFile := s.Account, s.Output, s.File
	s.Account = func(a identity.PublicUser, id string) (accounts.Account, error) {
		v, e := oldAccount(a, id)
		v.Platform = "bilibili"
		return v, e
	}
	s.Output = func(a identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
		v, e := oldOutput(a, id)
		v.Output.Platforms = []string{"bilibili"}
		return v, e
	}
	s.File = func(ctx context.Context, a identity.PublicUser, id, category int64) (filemodel.File, error) {
		v, e := oldFile(ctx, a, id, category)
		if id == input.Content.CoverFileID {
			v.MetadataJSON = []byte(`{"width":1200,"height":900,"format":"png"}`)
		}
		return v, e
	}
	v, e := s.Create(context.Background(), actor, input)
	if e != nil {
		t.Fatal(e)
	}
	if v.Platform != "bilibili" || v.Content.Title != input.Content.Title || v.Content.PortraitCoverFileID != 0 || v.AuthorizedAt != nil {
		t.Fatalf("wrong task: %+v", v)
	}
	var snap map[string]any
	if e = json.Unmarshal(v.ResourceSnapshot, &snap); e != nil {
		t.Fatal(e)
	}
	if snap["space_cover_sha256"] == nil || snap["portrait_cover_sha256"] != nil {
		t.Fatalf("foreign cover snapshot: %+v", snap)
	}
	again, e := s.Create(context.Background(), actor, input)
	if e != nil || again.ID != v.ID {
		t.Fatalf("idempotent replay: %+v %v", again, e)
	}
	s.Repo = &memoryTasks{items: map[uint64]model.Task{}}
	s.File = oldFile
	if _, e = s.Create(context.Background(), actor, input); !errors.Is(e, ErrInvalid) {
		t.Fatalf("accepted Baidu ratio for Bilibili main cover: %v", e)
	}
}

func biliContentFixture() model.Content {
	return model.Content{Title: "游戏画中画演示", Description: "可换行\n的简介", CoverFileID: 5, SpaceCoverFileID: 7, Confirmed: true,
		Bilibili: &model.BilibiliContent{PartitionID: 4, PartitionName: "游戏", Tags: []string{"游戏"}, Statement: "内容含营销信息", Visibility: "public"}}
}

func TestBilibiliContentBoundariesAndSeparateCovers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*model.Content)
		valid  bool
	}{
		{"valid", func(c *model.Content) {}, true},
		{"title80", func(c *model.Content) { c.Title = strings.Repeat("字", 80) }, true},
		{"title81", func(c *model.Content) { c.Title = strings.Repeat("字", 81) }, false},
		{"title40-emoji", func(c *model.Content) { c.Title = strings.Repeat("😀", 40) }, true},
		{"title41-emoji", func(c *model.Content) { c.Title = strings.Repeat("😀", 41) }, false},
		{"title-empty", func(c *model.Content) { c.Title = " " }, false},
		{"title-newline", func(c *model.Content) { c.Title = "第一行\n第二行" }, false},
		{"description-optional", func(c *model.Content) { c.Description = "" }, true},
		{"description2001", func(c *model.Content) { c.Description = strings.Repeat("字", 2001) }, false},
		{"no-options", func(c *model.Content) { c.Bilibili = nil }, false},
		{"no-partition", func(c *model.Content) { c.Bilibili.PartitionID = 0 }, false},
		{"no-tags", func(c *model.Content) { c.Bilibili.Tags = nil }, false},
		{"duplicate-tag", func(c *model.Content) { c.Bilibili.Tags = []string{"游戏", "游戏"} }, false},
		{"eleven-tags", func(c *model.Content) {
			for i := 0; i < 10; i++ {
				c.Bilibili.Tags = append(c.Bilibili.Tags, string(rune('a'+i)))
			}
		}, false},
		{"source-with-other-statement", func(c *model.Content) { c.Bilibili.Source = "来源说明" }, false},
		{"repost-source200", func(c *model.Content) {
			c.Bilibili.Statement = "内容为转载"
			c.Bilibili.Source = strings.Repeat("字", 200)
		}, true},
		{"repost-source201", func(c *model.Content) {
			c.Bilibili.Statement = "内容为转载"
			c.Bilibili.Source = strings.Repeat("字", 201)
		}, false},
		{"repost-missing-source", func(c *model.Content) { c.Bilibili.Statement = "内容为转载" }, false},
		{"repost-rights-conflict", func(c *model.Content) {
			c.Bilibili.Statement = "内容为转载"
			c.Bilibili.Source = "转自本人授权素材"
			c.Bilibili.RightsReserved = true
		}, false},
		{"explicit-rights", func(c *model.Content) { c.Bilibili.RightsReserved = true }, true},
		{"no-statement", func(c *model.Content) { c.Bilibili.Statement = "" }, false},
		{"baidu-statement", func(c *model.Content) { c.Declaration = "无需声明" }, false},
		{"baidu-portrait", func(c *model.Content) { c.PortraitCoverFileID = 6 }, false},
		{"no-space-cover", func(c *model.Content) { c.SpaceCoverFileID = 0 }, false},
		{"unconfirmed", func(c *model.Content) { c.Confirmed = false }, false},
		{"private", func(c *model.Content) { c.Bilibili.Visibility = "private" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := biliContentFixture()
			tc.change(&c)
			if got := contentValidFor("bilibili", c); got != tc.valid {
				t.Fatalf("got %v want %v", got, tc.valid)
			}
		})
	}
}

func TestBaiduRejectsForeignContentWithoutRelaxingItsLimits(t *testing.T) {
	_, _, in := fixture()
	if !contentValidFor("baidu", in.Content) {
		t.Fatal("old content rejected")
	}
	in.Content.Title = "foreign"
	if contentValidFor("baidu", in.Content) || contentValidFor("xiaohongshu", in.Content) || contentValidFor("bilibili", in.Content) {
		t.Fatal("cross-platform content accepted")
	}
}
