package config

import "testing"

func TestPublicationPlatformDefaultsRejectMalformedConfiguration(t *testing.T) {
	for _, defaults := range []BilibiliPublicationDefaults{
		{PartitionID: -1}, {PartitionID: 17}, {PartitionName: "游戏"}, {Visibility: "unlisted"},
		{Tags: []string{"游戏", "游戏"}}, {Tags: []string{"游戏,生活"}}, {Tags: []string{" "}},
	} {
		if err := (PublicationPlatformsConfig{Bilibili: defaults}).Validate(); err == nil {
			t.Fatalf("accepted invalid deployment defaults: %+v", defaults)
		}
	}
	for _, defaults := range []BilibiliPublicationDefaults{{}, {PartitionID: 17, PartitionName: "单机游戏", Tags: []string{"游戏"}, Visibility: "public"}} {
		if err := (PublicationPlatformsConfig{Bilibili: defaults}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
