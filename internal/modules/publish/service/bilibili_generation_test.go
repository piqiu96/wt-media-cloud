package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func TestBilibiliGenerationSeparatesTitleDescriptionAndTagsWithoutAdopting(t *testing.T) {
	s, a, _ := fixture()
	s.PlatformSettings = config.PublicationPlatformsConfig{BilibiliEnabled: true}
	in := GenerationInput{RowKey: "bili", InputRevision: "1", Platform: "bilibili", CategoryID: 1, CompositeOutputID: 42, Fields: []string{"title", "description", "cover_title", "tags"}, FieldRevisions: map[string]string{"title": "1", "description": "2", "cover_title": "0", "tags": "3"}, Content: map[string]string{"title": "人工标题", "description": "人工简介", "tags": "游戏"}}
	s.GenerateJSON = func(_ context.Context, system, user string) (string, error) {
		if !strings.Contains(system, "B站") || !strings.Contains(system, "不得生成声明") || !strings.Contains(user, "人工标题") {
			t.Fatal("wrong generation basis")
		}
		return `{"title":"独立标题","description":"独立简介","cover_title":"封面短题","tags":"游戏\n实战"}`, nil
	}
	run, err := s.PreparePublicationGeneration(a, in)
	if err != nil {
		t.Fatal(err)
	}
	r, err := run(context.Background())
	if err != nil || r.Content["title"] != "独立标题" || r.Content["tags"] != "游戏\n实战" || r.FieldRevisions["description"] != "2" {
		t.Fatalf("bad suggestions: %+v %v", r, err)
	}
	if in.Content["title"] != "人工标题" || len(s.Repo.(*memoryTasks).items) != 0 {
		t.Fatal("generation adopted or created a task")
	}
	for _, raw := range []map[string]string{
		{"title": strings.Repeat("😀", 41), "description": "简介", "cover_title": "封面", "tags": "游戏"},
		{"title": "标题", "description": "简介", "cover_title": "封面", "tags": "游戏\n游戏"},
		{"title": "标题", "description": "简介", "cover_title": "封面", "tags": "游戏,实战"},
		{"title": "标题", "description": "简介", "cover_title": "封面", "tags": strings.Repeat("😀", 11)},
		{"title": "标题", "description": "简介", "cover_title": "封面", "tags": "游戏", "statement": "内容无需标注"},
	} {
		s.GenerateJSON = func(context.Context, string, string) (string, error) {
			b, _ := json.Marshal(raw)
			return string(b), nil
		}
		if _, err := run(context.Background()); err == nil {
			t.Fatalf("invalid suggestion accepted: %+v", raw)
		}
	}
	for _, field := range []string{"statement", "rights_reserved", "source", "topics"} {
		in.Fields = []string{field}
		in.FieldRevisions = map[string]string{field: "0"}
		if _, err := s.PreparePublicationGeneration(a, in); err == nil {
			t.Fatalf("foreign/factual field generated: %s", field)
		}
	}
}
