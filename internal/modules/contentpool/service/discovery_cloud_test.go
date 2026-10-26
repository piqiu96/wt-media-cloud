package service

import (
	"context"
	"testing"
	"time"

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
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
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
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
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
		"view_count":          340000,
		"comment_count":       560,
		"share_count":         88,
		"author_id":           "author-uid",
		"author_sec_uid":      "author-sec-uid",
		"author_uid":          "author-uid",
		"author_home_url":     "https://www.douyin.com/user/author-sec-uid?showSubTab=video&showTab=post",
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
	if source.LikeCount != 12000 || source.FavoriteCount != 700 || source.ViewCount != 340000 || source.CommentCount != 560 || source.ShareCount != 88 {
		t.Fatalf("source metrics missing: %+v", source)
	}
	if source.AuthorID != "author-uid" || source.AuthorUID != "author-uid" || source.AuthorSecUID != "author-sec-uid" || source.AuthorHomeURL != "https://www.douyin.com/user/author-sec-uid?showSubTab=video&showTab=post" {
		t.Fatalf("source author identity missing: %+v", source)
	}
	if len(task.Results) != 1 || task.Results[0]["processing_status"] != "pending" || task.Results[0]["source_content_id"] != source.ID {
		t.Fatalf("task result details missing: %+v", task.Results)
	}
	if task.Stats.Added != 1 || task.Stats.Pending != 1 || task.Stats.Failed != 0 {
		t.Fatalf("unexpected task stats: %+v", task.Stats)
	}
}

func TestAutomaticMaterializationUsesTaskThresholdSnapshot(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	crawler := &cloudCrawlerStub{}
	service := NewDiscoveryService(store, NewService(contentStore), crawler)
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{
		TeamID: team, Name: "自动素材", StrategyType: "keyword", Platform: "douyin",
		Config: map[string]any{
			"keyword": "三角洲", "auto_material": true, "material_rule": "AND",
			"like_threshold": 10000, "favorite_threshold": 500,
		}, Status: StrategyEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	crawler.items = []map[string]any{
		{"platform_content_id": "and-pass", "title": "both pass", "like_count": 12000, "favorite_count": 700},
		{"platform_content_id": "and-fail", "title": "favorite only", "like_count": 8000, "favorite_count": 900},
	}
	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RunNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, _, _ = store.FindCrawlTask(task.ID)
	if contentStore.items[1].Status != StatusMaterialCreated || contentStore.items[1].MaterialID == nil {
		t.Fatalf("expected AND-pass source to be materialized: %+v", contentStore.items[1])
	}
	if contentStore.items[2].Status != StatusPending || contentStore.items[2].MaterialID != nil {
		t.Fatalf("expected AND-fail source to remain pending: %+v", contentStore.items[2])
	}
	if task.Stats.AutoMaterialized != 1 || task.Stats.Pending != 1 || task.Stats.Failed != 0 {
		t.Fatalf("unexpected automatic material stats: %+v", task.Stats)
	}
	if task.Results[0]["processing_status"] != "auto_materialized" || task.Results[0]["material_id"] != *contentStore.items[1].MaterialID {
		t.Fatalf("unexpected materialized result: %+v", task.Results[0])
	}
	if task.Results[1]["processing_status"] != "pending" {
		t.Fatalf("unexpected pending result: %+v", task.Results[1])
	}
}

func TestAutomaticMaterializationSupportsORRule(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	crawler := &cloudCrawlerStub{}
	service := NewDiscoveryService(store, NewService(contentStore), crawler)
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{
		TeamID: team, Name: "OR 素材", StrategyType: "keyword", Platform: "douyin",
		Config: map[string]any{
			"keyword": "热点", "auto_material": true, "material_rule": "OR",
			"like_threshold": 10000, "favorite_threshold": 500,
		}, Status: StrategyEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	crawler.items = []map[string]any{{"platform_content_id": "or-pass", "like_count": 8000, "favorite_count": 900}}
	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RunNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, _, _ = store.FindCrawlTask(task.ID)
	if contentStore.items[1].Status != StatusMaterialCreated || task.Stats.AutoMaterialized != 1 || task.Stats.Pending != 0 {
		t.Fatalf("expected OR-pass materialization, source=%+v stats=%+v", contentStore.items[1], task.Stats)
	}
}

func TestAutomaticMaterializationUsesRuleGroups(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	crawler := &cloudCrawlerStub{}
	service := NewDiscoveryService(store, NewService(contentStore), crawler)
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{
		TeamID: team, Name: "规则组素材", StrategyType: "keyword", Platform: "douyin",
		Config: map[string]any{
			"keyword": "热点",
			"material_rules": []any{
				map[string]any{"priority": 1, "action": "filter", "logic": "all", "conditions": []any{
					map[string]any{"field": "like_count", "operator": "lte", "value": 100},
				}},
				map[string]any{"priority": 2, "action": "match", "logic": "all", "conditions": []any{
					map[string]any{"field": "published_at", "operator": "recent_days", "value": 2},
					map[string]any{"field": "favorite_count", "operator": "gte", "value": 500},
				}},
			},
		}, Status: StrategyEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	fresh := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	stale := time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339)
	crawler.items = []map[string]any{
		{"platform_content_id": "fresh", "like_count": 5000, "favorite_count": 700, "published_at": fresh},
		{"platform_content_id": "stale", "like_count": 5000, "favorite_count": 700, "published_at": stale},
		{"platform_content_id": "low", "like_count": 50, "favorite_count": 700, "published_at": fresh},
	}
	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RunNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, _, _ = store.FindCrawlTask(task.ID)
	if contentStore.items[1].Status != StatusMaterialCreated {
		t.Fatalf("fresh source should match the date rule: %+v", contentStore.items[1])
	}
	if contentStore.items[2].Status != StatusPending {
		t.Fatalf("stale source should miss the date rule: %+v", contentStore.items[2])
	}
	if contentStore.items[3].Status != StatusPending {
		t.Fatalf("low-like source should be filtered out: %+v", contentStore.items[3])
	}
	if task.Stats.AutoMaterialized != 1 || task.Stats.Pending != 2 {
		t.Fatalf("unexpected stats: %+v", task.Stats)
	}
}

func TestRetryFailedUsesPublishedAtForDateRules(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	content := NewService(contentStore)
	service := NewDiscoveryService(store, content, fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{}, nil
	}))
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
	oldTime := time.Now().UTC().Add(-72 * time.Hour)
	freshTime := time.Now().UTC().Add(-time.Hour)
	oldSource, err := content.CreateSource(actor, SourceInput{TeamID: &team, Platform: "douyin", PlatformContentID: "retry-old", SourceType: "strategy", LikeCount: 12000, PublishedAt: &oldTime})
	if err != nil {
		t.Fatal(err)
	}
	freshSource, err := content.CreateSource(actor, SourceInput{TeamID: &team, Platform: "douyin", PlatformContentID: "retry-fresh", SourceType: "strategy", LikeCount: 12000, PublishedAt: &freshTime})
	if err != nil {
		t.Fatal(err)
	}
	task := CrawlTask{
		TeamID: team, TaskType: "discovery_task", Platform: "douyin", Status: CrawlFailed,
		Snapshot: map[string]any{"operation": "keyword", "material_rules": []any{
			map[string]any{"priority": 1, "action": "match", "logic": "all", "conditions": []any{
				map[string]any{"field": "published_at", "operator": "recent_days", "value": 2},
			}},
		}},
		Stats: CrawlStats{Scanned: 2, Found: 2, Failed: 2},
		Results: []map[string]any{
			{"platform_content_id": oldSource.PlatformContentID, "source_content_id": oldSource.ID, "processing_status": "material_failed", "failure_reason": "provider failed"},
			{"platform_content_id": freshSource.PlatformContentID, "source_content_id": freshSource.ID, "processing_status": "material_failed", "failure_reason": "provider failed"},
		},
	}
	task, err = store.CreateCrawlTask(task)
	if err != nil {
		t.Fatal(err)
	}
	retryTask, err := service.retryFailed(actor, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RunNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	completed, _, _ := store.FindCrawlTask(retryTask.ID)
	if completed.Stats.AutoMaterialized != 1 || completed.Stats.Pending != 1 {
		t.Fatalf("unexpected retry stats: %+v results=%+v", completed.Stats, completed.Results)
	}
	if oldSource, _, _ := contentStore.FindSource(oldSource.ID); oldSource.Status != StatusPending || oldSource.MaterialID != nil {
		t.Fatalf("stale source should stay pending: %+v", oldSource)
	}
	if freshSource, _, _ := contentStore.FindSource(freshSource.ID); freshSource.Status != StatusMaterialCreated || freshSource.MaterialID == nil {
		t.Fatalf("fresh source should materialize: %+v", freshSource)
	}
}

func TestRetryFailedOnlyProcessesFailedItems(t *testing.T) {
	store := newDiscoveryMemory()
	contentStore := newMemoryStore()
	content := NewService(contentStore)
	service := NewDiscoveryService(store, content, fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{}, nil
	}))
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleSeniorOperator, TeamID: &team}
	source, err := content.CreateSource(actor, SourceInput{TeamID: &team, Platform: "douyin", PlatformContentID: "retry-failed", SourceType: "strategy", LikeCount: 12000, FavoriteCount: 700})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := content.CreateSource(actor, SourceInput{TeamID: &team, Platform: "douyin", PlatformContentID: "retry-pending", SourceType: "strategy"})
	if err != nil {
		t.Fatal(err)
	}
	task := CrawlTask{
		TeamID: team, TaskType: "discovery_task", Platform: "douyin", Status: CrawlFailed,
		Snapshot: map[string]any{"operation": "keyword", "auto_material": true, "material_rule": "AND", "like_threshold": 10000, "favorite_threshold": 500},
		Stats:    CrawlStats{Scanned: 2, Found: 2, Failed: 1, Pending: 1},
		Results: []map[string]any{
			{"platform_content_id": source.PlatformContentID, "source_content_id": source.ID, "processing_status": "material_failed", "failure_reason": "provider failed"},
			{"platform_content_id": pending.PlatformContentID, "source_content_id": pending.ID, "processing_status": "pending"},
		},
	}
	task, err = store.CreateCrawlTask(task)
	if err != nil {
		t.Fatal(err)
	}
	retryTask, err := service.retryFailed(actor, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retryTask.Status != CrawlPending || retryTask.TaskType != "retry_failed_task" || retryTask.ParentTaskID == nil || *retryTask.ParentTaskID != task.ID {
		t.Fatalf("unexpected retry task: %+v", retryTask)
	}
	if len(anyMapSlice(retryTask.Snapshot["retry_items"])) != 1 {
		t.Fatalf("only failed item should be queued: %+v", retryTask.Snapshot)
	}
	original, _, _ := store.FindCrawlTask(task.ID)
	if original.Status != CrawlFailed || original.Stats.Failed != 1 || len(original.Results) != 2 {
		t.Fatalf("original task should remain immutable: %+v", original)
	}
	if _, err = service.RunNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	completed, _, _ := store.FindCrawlTask(retryTask.ID)
	if completed.Status != CrawlSuccess || completed.Stats.Failed != 0 || completed.Stats.AutoMaterialized != 1 {
		t.Fatalf("unexpected retry result: %+v", completed)
	}
	if len(completed.Results) != 1 || completed.Results[0]["processing_status"] != "auto_materialized" {
		t.Fatalf("only failed item should be retried: %+v", completed.Results)
	}
	original, _, _ = store.FindCrawlTask(task.ID)
	if original.Status != CrawlFailed || original.Stats.Failed != 1 || len(original.Results) != 2 {
		t.Fatalf("original task should remain immutable after worker execution: %+v", original)
	}
	if source, _, _ := contentStore.FindSource(source.ID); source.Status != StatusMaterialCreated || source.MaterialID == nil {
		t.Fatalf("failed source should be materialized: %+v", source)
	}
	if pending, _, _ := contentStore.FindSource(pending.ID); pending.Status != StatusPending || pending.MaterialID != nil {
		t.Fatalf("pending source should not be materialized: %+v", pending)
	}
}
