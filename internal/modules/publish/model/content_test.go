package model

import (
	"encoding/json"
	"testing"
)

func TestBaiduContentSerializationIsUnchanged(t *testing.T) {
	c := Content{Description: "旧标题", CoverFileID: 5, PortraitCoverFileID: 6, Topics: "旧话题", Declaration: "无需声明", Confirmed: true}
	b, err := json.Marshal(c)
	const old = `{"portrait_cover_file_id":"6","description":"旧标题","cover_file_id":"5","topics":"旧话题","declaration":"无需声明","confirmed":true}`
	if err != nil || string(b) != old {
		t.Fatalf("legacy request/frozen snapshot changed: %s %v", b, err)
	}
}

func TestPublicationTitleKeepsPlatformMeaning(t *testing.T) {
	c := Content{Title: "B站标题", Description: "简介"}
	if c.PublicationTitle("bilibili") != "B站标题" || c.PublicationTitle("baidu") != "简介" {
		t.Fatal("title meaning changed")
	}
}
