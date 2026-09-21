package service

import (
	"context"
	"testing"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type cloudCrawlerStub struct {
	requests []CrawlerRequest
	items    []map[string]any
}

func (c *cloudCrawlerStub) Discover(_ context.Context, request CrawlerRequest) (CrawlerResult, error) {
	c.requests = append(c.requests, request)
	if c.items != nil {
		return CrawlerResult{Items: c.items}, nil
	}
	return CrawlerResult{Items: []map[string]any{{
		"platform_content_id": "douyin-1",
		"title":               "Cloud crawler result",
		"source_url":          "https://www.douyin.com/video/douyin-1",
	}}}, nil
}

func TestCreateRunQueuesAndWorkerProjectsSource(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	crawler := &cloudCrawlerStub{}
	service := NewDiscoveryService(store, NewService(contentStore), crawler)
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{
		TeamID: team, Name: "Cloud 热点", StrategyType: "keyword", Platform: "douyin",
		Config: map[string]any{"keyword": "王者荣耀"}, Status: StrategyEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != CrawlPending || len(crawler.requests) != 0 || len(contentStore.items) != 0 {
		t.Fatalf("expected queued Cloud run without side effects, task=%+v requests=%d content=%d", task, len(crawler.requests), len(contentStore.items))
	}
	processed, err := service.RunNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	task, _, _ = store.FindCrawlTask(task.ID)
	if task.Status != CrawlSuccess || task.Stats.Added != 1 {
		t.Fatalf("expected completed worker run, got %+v", task)
	}
	if len(crawler.requests) != 1 || crawler.requests[0].Operation != "keyword" {
		t.Fatalf("unexpected crawler requests: %+v", crawler.requests)
	}
	if len(contentStore.items) != 1 || contentStore.items[1].SourceType != "strategy" {
		t.Fatalf("expected one strategy source, got %+v", contentStore.items)
	}
	if task.TaskID == "" {
		t.Fatal("expected internal crawl task identifier")
	}
}

func TestWorkerProjectsTaskResultDetails(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	crawler := &cloudCrawlerStub{}
	service := NewDiscoveryService(store, NewService(contentStore), crawler)
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{
		TeamID: team, Name: "指标策略", StrategyType: "keyword", Platform: "douyin",
		Config: map[string]any{"keyword": "三角洲", "auto_material": false}, Status: StrategyEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	crawler.items = []map[string]any{{
		"platform_content_id": "douyin-metrics",
		"title":               "metrics result",
		"like_count":          12000,
		"favorite_count":      700,
	}}

	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RunNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, _, _ = store.FindCrawlTask(task.ID)
	source := contentStore.items[1]
	if source.StrategyID == nil || *source.StrategyID != strategy.ID || source.CrawlTaskID == nil || *source.CrawlTaskID != task.ID {
		t.Fatalf("source strategy/task link missing: %+v", source)
	}
	if source.LikeCount != 12000 || source.FavoriteCount != 700 {
		t.Fatalf("source metrics missing: %+v", source)
	}
	if len(task.Results) != 1 || task.Results[0]["processing_status"] != "pending" || task.Results[0]["source_content_id"] != source.ID {
		t.Fatalf("task result details missing: %+v", task.Results)
	}
	if task.Stats.Added != 1 || task.Stats.Pending != 1 || task.Stats.Failed != 0 {
		t.Fatalf("unexpected task stats: %+v", task.Stats)
	}
}
