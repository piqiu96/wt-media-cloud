package contentpool

import (
	"context"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type cloudCrawlerStub struct {
	requests []CrawlerRequest
}

func (c *cloudCrawlerStub) Discover(_ context.Context, request CrawlerRequest) (CrawlerResult, error) {
	c.requests = append(c.requests, request)
	return CrawlerResult{Items: []map[string]any{{
		"platform_content_id": "douyin-1",
		"title":               "Cloud crawler result",
		"source_url":          "https://www.douyin.com/video/douyin-1",
	}}}, nil
}

func TestCreateRunExecutesCloudCrawlerAndProjectsSource(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	crawler := &cloudCrawlerStub{}
	service := NewDiscoveryService(store, NewService(contentStore), crawler)
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}
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
	if task.Status != CrawlSuccess || task.Stats.Added != 1 {
		t.Fatalf("expected completed Cloud run, got %+v", task)
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
