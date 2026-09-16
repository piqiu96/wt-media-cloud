package contentpool

import (
	"context"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type fixedCrawler func(context.Context, CrawlerRequest) (CrawlerResult, error)

func (f fixedCrawler) Discover(ctx context.Context, req CrawlerRequest) (CrawlerResult, error) {
	return f(ctx, req)
}

type discoveryMemory struct {
	strategies map[int64]DiscoveryStrategy
	tasks      map[int64]CrawlTask
	next       int64
}

func newDiscoveryMemory() *discoveryMemory {
	return &discoveryMemory{strategies: map[int64]DiscoveryStrategy{}, tasks: map[int64]CrawlTask{}, next: 1}
}
func (m *discoveryMemory) CreateStrategy(v DiscoveryStrategy) (DiscoveryStrategy, error) {
	v.ID = m.next
	m.next++
	m.strategies[v.ID] = v
	return v, nil
}
func (m *discoveryMemory) ListStrategies(team *identity.TeamID) ([]DiscoveryStrategy, error) {
	out := []DiscoveryStrategy{}
	for _, v := range m.strategies {
		if team == nil || v.TeamID == *team {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *discoveryMemory) FindStrategy(id int64) (DiscoveryStrategy, bool, error) {
	v, ok := m.strategies[id]
	return v, ok, nil
}
func (m *discoveryMemory) UpdateStrategy(v DiscoveryStrategy) (DiscoveryStrategy, error) {
	m.strategies[v.ID] = v
	return v, nil
}
func (m *discoveryMemory) CreateCrawlTask(v CrawlTask) (CrawlTask, error) {
	v.ID = m.next
	m.next++
	m.tasks[v.ID] = v
	return v, nil
}
func (m *discoveryMemory) ListCrawlTasks(team *identity.TeamID, strategyID *int64) ([]CrawlTask, error) {
	out := []CrawlTask{}
	for _, v := range m.tasks {
		if team != nil && v.TeamID != *team {
			continue
		}
		if strategyID != nil && (v.StrategyID == nil || *v.StrategyID != *strategyID) {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}
func (m *discoveryMemory) FindCrawlTask(id int64) (CrawlTask, bool, error) {
	v, ok := m.tasks[id]
	return v, ok, nil
}
func (m *discoveryMemory) UpdateCrawlTask(v CrawlTask) (CrawlTask, error) {
	m.tasks[v.ID] = v
	return v, nil
}

func TestDiscoveryRunExecutesCloudCrawlerAndPreservesTeam(t *testing.T) {
	store := newDiscoveryMemory()
	content := NewService(newMemoryStore())
	service := NewDiscoveryService(store, content)
	team := identity.TeamID(7)
	strategy, err := service.CreateStrategy(identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}, DiscoveryStrategy{TeamID: team, Name: "热点", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "王者荣耀"}, Status: StrategyEnabled})
	if err != nil {
		t.Fatal(err)
	}
	service.SetCrawler(fixedCrawler(func(_ context.Context, req CrawlerRequest) (CrawlerResult, error) {
		if req.Operation != "keyword" {
			t.Fatalf("unexpected operation %s", req.Operation)
		}
		return CrawlerResult{Items: []map[string]any{{"platform_content_id": "run-1", "title": "run"}}}, nil
	}))
	run, err := service.CreateRun(identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}, strategy.ID)
	if err != nil || run.TaskID == "" || run.TeamID != team || run.Status != CrawlSuccess {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestManualRunPersistsOperationForResultProjection(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()))
	team := identity.TeamID(7)
	service.SetCrawler(fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) { return CrawlerResult{}, nil }))
	task, err := service.CreateManualRun(identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}, "douyin", "url", map[string]any{"url": "https://v.douyin.com/demo"})
	if err != nil || task.Snapshot["operation"] != "url" {
		t.Fatalf("task=%+v err=%v", task, err)
	}
}

func TestDiscoveryResultDeduplicatesIntoContentPool(t *testing.T) {
	contentStore := newMemoryStore()
	content := NewService(contentStore)
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, content, fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{Items: []map[string]any{{"platform_content_id": "a1", "title": "demo"}, {"platform_content_id": "a1", "title": "demo"}}}, nil
	}))
	team := identity.TeamID(7)
	task, err := service.CreateManualRun(identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}, "douyin", "url", map[string]any{"url": "https://v.douyin.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(contentStore.items) != 1 {
		t.Fatalf("expected one unique source, got %d", len(contentStore.items))
	}
	if contentStore.items[1].SourceType != "link" {
		t.Fatalf("expected link source type, got %s", contentStore.items[1].SourceType)
	}
	updated, _, _ := store.FindCrawlTask(task.ID)
	if updated.Status != CrawlSuccess || updated.Stats.Added != 1 || updated.Stats.Duplicate != 1 {
		t.Fatalf("unexpected stats %+v", updated.Stats)
	}
}

func TestManualSearchStoresResultsUntilSelection(t *testing.T) {
	contentStore := newMemoryStore()
	service := NewDiscoveryService(newDiscoveryMemory(), NewService(contentStore), fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{Items: []map[string]any{{"platform_content_id": "a1", "title": "one"}, {"platform_content_id": "a1", "title": "one duplicate"}, {"platform_content_id": "a2", "title": "two"}}}, nil
	}))
	store := service.store.(*discoveryMemory)
	team := identity.TeamID(7)
	task, err := service.CreateManualRun(identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}, "douyin", "keyword", map[string]any{"keyword": "demo"})
	if err != nil {
		t.Fatal(err)
	}
	updated, _, _ := store.FindCrawlTask(task.ID)
	if len(contentStore.items) != 0 || len(updated.Results) != 2 || updated.Stats.Found != 2 {
		t.Fatalf("expected unpromoted search results, content=%d task=%+v", len(contentStore.items), updated)
	}
	actor := identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}
	if _, err := service.ConfirmResults(actor, task.ID, []string{"a2"}); err != nil {
		t.Fatal(err)
	}
	if len(contentStore.items) != 1 || contentStore.items[1].PlatformContentID != "a2" {
		t.Fatalf("expected only selected result, got %+v", contentStore.items)
	}
}

func TestRunDueUsesStrategyTimezone(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()))
	team := identity.TeamID(7)
	strategy, err := service.CreateStrategy(identity.PublicUser{ID: 2, Role: identity.RoleOperator, TeamID: &team}, DiscoveryStrategy{TeamID: team, Name: "定时热点", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "demo"}, Schedule: "daily 09:00", Timezone: "Asia/Shanghai", Status: StrategyEnabled})
	if err != nil {
		t.Fatal(err)
	}
	service.SetCrawler(fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) { return CrawlerResult{}, nil }))
	if got := service.RunDue(time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)); got != 1 {
		t.Fatalf("expected Asia/Shanghai 09:00 trigger, got %d", got)
	}
	_ = strategy
}

func TestUpdateStrategyPreservesTeamAndAllowsEdit(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()), fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) { return CrawlerResult{}, nil }))
	team := identity.TeamID(9)
	actor := identity.PublicUser{ID: 3, Role: identity.RoleOperator, TeamID: &team}
	created, err := service.CreateStrategy(actor, DiscoveryStrategy{TeamID: team, Name: "old", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keywords": []any{"one"}}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateStrategy(actor, created.ID, DiscoveryStrategy{Name: "new", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keywords": []any{"two"}}, Status: StrategyEnabled})
	if err != nil || updated.Name != "new" || updated.TeamID != team || updated.Status != StrategyEnabled {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
}
