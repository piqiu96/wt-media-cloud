package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type fixedCrawler func(context.Context, CrawlerRequest) (CrawlerResult, error)

func (f fixedCrawler) Discover(ctx context.Context, req CrawlerRequest) (CrawlerResult, error) {
	return f(ctx, req)
}

type discoveryMemory struct {
	strategies map[int64]DiscoveryStrategy
	tasks      map[int64]CrawlTask
	next       int64
	claims     int
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
func (m *discoveryMemory) ListStrategies(team *identityservice.TeamID) ([]DiscoveryStrategy, error) {
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
func (m *discoveryMemory) ListCrawlTasks(team *identityservice.TeamID, strategyID *int64) ([]CrawlTask, error) {
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

func (m *discoveryMemory) ClaimPendingCrawlTask(now time.Time) (CrawlTask, bool, error) {
	for id, task := range m.tasks {
		if task.Status != CrawlPending {
			continue
		}
		task.Status = CrawlRunning
		task.TaskID = fmt.Sprintf("crawl-%d", task.ID)
		task.StartedAt = &now
		task.UpdatedAt = now
		m.tasks[id] = task
		m.claims++
		return task, true, nil
	}
	return CrawlTask{}, false, nil
}

func TestDiscoveryRunQueuesThenWorkerExecutesCloudCrawler(t *testing.T) {
	store := newDiscoveryMemory()
	content := NewService(newMemoryStore())
	service := NewDiscoveryService(store, content)
	team := identityservice.TeamID(7)
	strategy, err := service.CreateStrategy(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}, DiscoveryStrategy{TeamID: team, Name: "热点", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "王者荣耀"}, Status: StrategyEnabled})
	if err != nil {
		t.Fatal(err)
	}
	service.SetCrawler(fixedCrawler(func(_ context.Context, req CrawlerRequest) (CrawlerResult, error) {
		if req.Operation != "keyword" {
			t.Fatalf("unexpected operation %s", req.Operation)
		}
		return CrawlerResult{Items: []map[string]any{{"platform_content_id": "run-1", "title": "run"}}}, nil
	}))
	run, err := service.CreateRun(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}, strategy.ID)
	if err != nil || run.TaskID != "" || run.TeamID != team || run.Status != CrawlPending {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	processed, err := service.RunNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	completed, _, _ := store.FindCrawlTask(run.ID)
	if completed.TaskID == "" || completed.Status != CrawlSuccess {
		t.Fatalf("completed=%+v", completed)
	}
}

func TestManualRunPersistsOperationForResultProjection(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()))
	team := identityservice.TeamID(7)
	service.SetCrawler(fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) { return CrawlerResult{}, nil }))
	task, err := service.CreateManualRun(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}, "douyin", "url", map[string]any{"url": "https://v.douyin.com/demo"})
	if err != nil || task.Snapshot["operation"] != "url" || task.Status != CrawlPending {
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
	team := identityservice.TeamID(7)
	task, err := service.CreateManualRun(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}, "douyin", "url", map[string]any{"url": "https://v.douyin.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := service.RunNext(context.Background()); runErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, runErr)
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

func TestHistoricalManualTaskConfirmationStillWorks(t *testing.T) {
	contentStore := newMemoryStore()
	service := NewDiscoveryService(newDiscoveryMemory(), NewService(contentStore), fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{Items: []map[string]any{{"platform_content_id": "a1", "title": "one"}, {"platform_content_id": "a1", "title": "one duplicate"}, {"platform_content_id": "a2", "title": "two"}}}, nil
	}))
	store := service.store.(*discoveryMemory)
	team := identityservice.TeamID(7)
	task, err := service.CreateManualRun(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}, "douyin", "keyword", map[string]any{"keyword": "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := service.RunNext(context.Background()); runErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	updated, _, _ := store.FindCrawlTask(task.ID)
	if len(contentStore.items) != 0 || len(updated.Results) != 2 || updated.Stats.Found != 2 {
		t.Fatalf("expected unpromoted search results, content=%d task=%+v", len(contentStore.items), updated)
	}
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
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
	team := identityservice.TeamID(7)
	strategy, err := service.CreateStrategy(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}, DiscoveryStrategy{TeamID: team, Name: "定时热点", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "demo"}, Schedule: "daily 09:00", Timezone: "Asia/Shanghai", Status: StrategyEnabled})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	service.SetCrawler(fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) { calls++; return CrawlerResult{}, nil }))
	if got := service.RunDue(time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)); got != 1 {
		t.Fatalf("expected Asia/Shanghai 09:00 trigger, got %d", got)
	}
	tasks, _ := store.ListCrawlTasks(&team, &strategy.ID)
	if len(tasks) != 1 || tasks[0].Status != CrawlPending || tasks[0].ScheduleKey == "" || calls != 0 {
		t.Fatalf("expected one pending task without crawler execution, tasks=%+v calls=%d", tasks, calls)
	}
}

func TestWorkerClaimsPendingTaskOnlyOnce(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()), fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{}, nil
	}))
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{TeamID: team, Name: "one", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "demo"}, Status: StrategyEnabled})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := service.RunNext(context.Background()); runErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	if processed, runErr := service.RunNext(context.Background()); runErr != nil || processed {
		t.Fatalf("second processed=%v err=%v", processed, runErr)
	}
	completed, _, _ := store.FindCrawlTask(task.ID)
	if store.claims != 1 || completed.Status != CrawlSuccess {
		t.Fatalf("claims=%d task=%+v", store.claims, completed)
	}
}

func TestWorkerRecoversLegacyStrategyTaskWithoutOperationSnapshot(t *testing.T) {
	store := newDiscoveryMemory()
	calls := 0
	service := NewDiscoveryService(store, NewService(newMemoryStore()), fixedCrawler(func(_ context.Context, req CrawlerRequest) (CrawlerResult, error) {
		calls++
		if req.Operation != "keyword" {
			t.Fatalf("operation=%q", req.Operation)
		}
		return CrawlerResult{}, nil
	}))
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{TeamID: team, Name: "legacy", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "demo"}, Status: StrategyEnabled})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	task, err := store.CreateCrawlTask(CrawlTask{TeamID: team, StrategyID: &strategy.ID, TaskType: "discovery_task", Platform: "douyin", Status: CrawlPending, Snapshot: map[string]any{"keyword": "demo"}, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := service.RunNext(context.Background()); runErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, runErr)
	}
	completed, _, _ := store.FindCrawlTask(task.ID)
	if calls != 1 || completed.Status != CrawlSuccess {
		t.Fatalf("calls=%d task=%+v", calls, completed)
	}
}

func TestUpdateStrategyPreservesTeamAndAllowsEdit(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()), fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) { return CrawlerResult{}, nil }))
	team := identityservice.TeamID(9)
	actor := identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, TeamID: &team}
	created, err := service.CreateStrategy(actor, DiscoveryStrategy{TeamID: team, Name: "old", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keywords": []any{"one"}}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateStrategy(actor, created.ID, DiscoveryStrategy{Name: "new", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keywords": []any{"two"}}, Status: StrategyEnabled})
	if err != nil || updated.Name != "new" || updated.TeamID != team || updated.Status != StrategyEnabled {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
}

func TestAdminManualRunUsesExplicitTeamScope(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()))
	if _, err := service.CreateManualRunWithTeam(identityservice.PublicUser{ID: 1, Role: identityservice.RoleAdmin}, nil, "douyin", "url", map[string]any{"url": "https://v.douyin.com/demo"}); !errors.Is(err, ErrDiscoveryInvalid) {
		t.Fatalf("admin without team err=%v", err)
	}
	team := identityservice.TeamID(7)
	task, err := service.CreateManualRunWithTeam(identityservice.PublicUser{ID: 1, Role: identityservice.RoleAdmin}, &team, "douyin", "url", map[string]any{"url": "https://v.douyin.com/demo"})
	if err != nil || task.TeamID != team {
		t.Fatalf("task=%+v err=%v", task, err)
	}
}
