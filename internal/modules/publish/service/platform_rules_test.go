package service

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func TestPlatformRulesAreClosedByDefaultAndKeepBaiduCoverRules(t *testing.T) {
	rules := platformRules(config.PublicationPlatformsConfig{})
	if len(rules.Platforms) != 2 || rules.Revision == "" {
		t.Fatalf("missing fixed rules: %+v", rules)
	}
	baidu, bili := rules.Platforms[0], rules.Platforms[1]
	if baidu.Platform != "baidu" || !baidu.Enabled || baidu.TitleMaxLength != 50 || baidu.Cover.WidthRatio != 16 || baidu.SecondaryCover.Field != "portrait_cover_file_id" || baidu.SecondaryCover.HeightRatio != 4 {
		t.Fatalf("Baidu changed: %+v", baidu)
	}
	if bili.Platform != "bilibili" || bili.Enabled || bili.Cover.WidthRatio != 4 || bili.SecondaryCover.Field != "space_cover_file_id" || bili.SecondaryCover.WidthRatio != 16 {
		t.Fatalf("Bilibili must be closed with separate covers: %+v", bili)
	}
	if _, exists := bili.Defaults["declaration"]; exists {
		t.Fatal("default must not invent a content statement")
	}
}

func TestPlatformRulesDefaultsAreCopiedAndNeverClaimOriginality(t *testing.T) {
	cfg := config.PublicationPlatformsConfig{BilibiliEnabled: true, Bilibili: config.BilibiliPublicationDefaults{PartitionID: 17, PartitionName: "单机游戏", Tags: []string{"游戏"}, Visibility: "public"}}
	rules := platformRules(cfg)
	bili := rules.Platforms[1]
	if !bili.Enabled || bili.Defaults["partition_id"] != int64(17) || bili.Defaults["visibility"] != "public" {
		t.Fatalf("configured defaults lost: %+v", bili)
	}
	bili.Defaults["tags"].([]string)[0] = "changed"
	if cfg.Bilibili.Tags[0] != "游戏" || platformRules(cfg).Platforms[1].Defaults["tags"].([]string)[0] != "游戏" {
		t.Fatal("returned rules mutated configured defaults")
	}
	if _, exists := bili.Defaults["rights_reserved"]; exists {
		t.Fatal("must require an explicit authorship choice")
	}
}
