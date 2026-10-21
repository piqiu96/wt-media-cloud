package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
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
func (m *discoveryMemory) ListStrategiesFiltered(team *identityservice.TeamID, query StrategyListQuery) ([]DiscoveryStrategy, error) {
	items, err := m.ListStrategies(team)
	if err != nil {
		return nil, err
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	out := make([]DiscoveryStrategy, 0, len(items))
	for _, item := range items {
		if query.Type != "" && item.StrategyType != query.Type {
			continue
		}
		if query.Status != "" && string(item.Status) != query.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(item.Name), search) && strconv.FormatInt(item.ID, 10) != search {
			continue
		}
		out = append(out, item)
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
func (m *discoveryMemory) DeleteStrategy(id int64) error {
	delete(m.strategies, id)
	return nil
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
func (m *discoveryMemory) ListCrawlTasksFiltered(team *identityservice.TeamID, query CrawlTaskListQuery) ([]CrawlTask, error) {
	items, err := m.ListCrawlTasks(team, query.StrategyID)
	if err != nil {
		return nil, err
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	out := make([]CrawlTask, 0, len(items))
	for _, item := range items {
		if query.Status != "" && string(item.Status) != query.Status {
			continue
		}
		if search != "" {
			display := crawlTaskDisplayName(item)
			if !strings.Contains(strings.ToLower(display+" "+item.TaskID), search) && strconv.FormatInt(item.ID, 10) != search {
				continue
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func crawlTaskDisplayName(task CrawlTask) string {
	switch task.TaskType {
	case "discovery_task":
		if name, ok := task.Snapshot["strategy_name"]; ok {
			return fmt.Sprint(name)
		}
		return "策略"
	case "retry_failed_task":
		if name, ok := task.Snapshot["strategy_name"]; ok {
			return fmt.Sprint(name)
		}
		return "重试"
	default:
		if fmt.Sprint(task.Snapshot["operation"]) == "url" {
			return "ID发现"
		}
		return "关键词发现"
	}
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

func TestDeleteStrategyScopesByTeamAndReportsMissing(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()))
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	strategy, err := service.CreateStrategy(actor, DiscoveryStrategy{TeamID: team, Name: "待删", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteStrategy(actor, strategy.ID); err != nil {
		t.Fatalf("DeleteStrategy() err=%v", err)
	}
	if _, ok, _ := store.FindStrategy(strategy.ID); ok {
		t.Fatalf("strategy still present after delete")
	}
	if err := service.DeleteStrategy(actor, strategy.ID); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("missing strategy err=%v, want ErrStrategyNotFound", err)
	}
	other := identityservice.TeamID(8)
	if _, err := service.CreateStrategy(identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, TeamID: &other}, DiscoveryStrategy{TeamID: other, Name: "越权", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "x"}}); err != nil {
		t.Fatal(err)
	}
	for _, v := range store.strategies {
		if err := service.DeleteStrategy(actor, v.ID); !errors.Is(err, ErrDiscoveryForbidden) {
			t.Fatalf("cross-team delete err=%v, want forbidden", err)
		}
		break
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

func TestCrawlStatusFromStatsClassifiesPartialSuccess(t *testing.T) {
	tests := []struct {
		name  string
		stats CrawlStats
		err   error
		want  CrawlStatus
	}{
		{name: "success", stats: CrawlStats{Added: 1}, want: CrawlSuccess},
		{name: "partial", stats: CrawlStats{Added: 1, Failed: 1}, want: CrawlPartialSuccess},
		{name: "failed", stats: CrawlStats{Failed: 1}, want: CrawlFailed},
		{name: "crawl error with processed items", stats: CrawlStats{Added: 1}, err: errors.New("provider failed"), want: CrawlPartialSuccess},
	}
	for _, test := range tests {
		if got := crawlStatusFromStats(test.stats, test.err); got != test.want {
			t.Fatalf("%s status = %s, want %s", test.name, got, test.want)
		}
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

func TestStrategyMaterialRulesValidationAndSnapshot(t *testing.T) {
	store := newDiscoveryMemory()
	service := NewDiscoveryService(store, NewService(newMemoryStore()), fixedCrawler(func(_ context.Context, _ CrawlerRequest) (CrawlerResult, error) {
		return CrawlerResult{}, nil
	}))
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	base := func() DiscoveryStrategy {
		return DiscoveryStrategy{TeamID: team, Name: "热点", StrategyType: "keyword", Platform: "douyin", Status: StrategyEnabled}
	}
	condition := func(field, operator string, value any) map[string]any {
		return map[string]any{"field": field, "operator": operator, "value": value}
	}

	invalid := base()
	invalid.Config = map[string]any{"keyword": "demo", "material_rules": []any{
		map[string]any{"priority": 2, "action": "match", "logic": "all", "conditions": []any{condition("like_count", "gte", 1000)}},
	}}
	if _, err := service.CreateStrategy(actor, invalid); !errors.Is(err, ErrDiscoveryInvalid) {
		t.Fatalf("non-contiguous priority should be invalid, got %v", err)
	}
	invalid.Config = map[string]any{"keyword": "demo", "material_rules": []any{
		map[string]any{"priority": 1, "action": "filter", "logic": "all", "conditions": []any{condition("published_at", "recent_hours", 24)}},
	}}
	if _, err := service.CreateStrategy(actor, invalid); !errors.Is(err, ErrDiscoveryInvalid) {
		t.Fatalf("filter-only rules should be invalid, got %v", err)
	}
	invalid.Config = map[string]any{"keyword": "demo", "material_rules": []any{
		map[string]any{"priority": 1, "action": "match", "logic": "all", "conditions": []any{}},
	}}
	if _, err := service.CreateStrategy(actor, invalid); !errors.Is(err, ErrDiscoveryInvalid) {
		t.Fatalf("empty condition list should be invalid, got %v", err)
	}

	// Historical auto_material/threshold configs are translated on read and
	// rewritten to the rule-group shape on save.
	input := base()
	input.Config = map[string]any{"keyword": "demo", "auto_material": true, "material_rule": "or", "like_threshold": 10000, "favorite_threshold": 500}
	strategy, err := service.CreateStrategy(actor, input)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := model.ResolveMaterialRules(strategy.Config)
	if err != nil || len(rules) != 1 || rules[0].Action != "match" || rules[0].Logic != "any" || len(rules[0].Conditions) != 2 {
		t.Fatalf("legacy material config was not normalized: rules=%+v err=%v config=%+v", rules, err, strategy.Config)
	}
	for _, legacy := range []string{"auto_material", "material_rule", "like_threshold", "favorite_threshold"} {
		if _, exists := strategy.Config[legacy]; exists {
			t.Fatalf("legacy key %q survived normalization: %+v", legacy, strategy.Config)
		}
	}

	task, err := service.CreateRun(actor, strategy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Snapshot["strategy_id"] != strategy.ID || task.Snapshot["strategy_name"] != "热点" || task.Snapshot["strategy_type"] != "keyword" || task.Snapshot["platform"] != "douyin" || task.Snapshot["schedule"] != "manual" || task.Snapshot["operation"] != "keyword" {
		t.Fatalf("task strategy snapshot is incomplete: %+v", task.Snapshot)
	}

	updated := base()
	updated.Name = "新热点"
	updated.Config = map[string]any{"keyword": "new", "material_rules": []any{
		map[string]any{"priority": 1, "action": "match", "logic": "all", "conditions": []any{condition("favorite_count", "gte", 1000)}},
	}}
	if _, err = service.UpdateStrategy(actor, strategy.ID, updated); err != nil {
		t.Fatal(err)
	}
	snapshotRules, err := model.ResolveMaterialRules(task.Snapshot)
	if err != nil || len(snapshotRules) != 1 || snapshotRules[0].Logic != "any" {
		t.Fatalf("historical task snapshot was changed: rules=%+v err=%v snapshot=%+v", snapshotRules, err, task.Snapshot)
	}
}

// explodingCrawler fails loudly if the scheduling path ever tries to crawl.
type explodingCrawler struct{}

func (explodingCrawler) Discover(context.Context, CrawlerRequest) (CrawlerResult, error) {
	panic("scheduling path must not crawl")
}

// The scheduling entry point must not build a crawler: cmd/discovery-scheduler's
// resource plan assembles no clients, and douyinclient.Get() panics before
// Initialize, so a crawler here takes the whole process down on the first tick.
func TestSchedulerDiscoveryServiceCarriesNoCrawler(t *testing.T) {
	if svc := defaultSchedulerDiscoveryService(); svc.crawler != nil {
		t.Fatal("scheduling service must not build a crawler")
	}
}

// runDue only reads the database and enqueues pending tasks, so it must not
// touch the crawler even when one is attached.
func TestRunDueNeverCrawls(t *testing.T) {
	store := newDiscoveryMemory()
	svc := NewDiscoveryService(store, NewService(newMemoryStore()), explodingCrawler{})
	team := identityservice.TeamID(7)
	strategy, err := svc.CreateStrategy(
		identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team},
		DiscoveryStrategy{TeamID: team, Name: "定时热点", StrategyType: "keyword", Platform: "douyin", Config: map[string]any{"keyword": "demo"}, Schedule: "daily 09:00", Timezone: "Asia/Shanghai", Status: StrategyEnabled},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.RunDue(time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)); got != 1 {
		t.Fatalf("expected one enqueued task, got %d", got)
	}
	tasks, _ := store.ListCrawlTasks(&team, &strategy.ID)
	if len(tasks) != 1 || tasks[0].Status != CrawlPending || tasks[0].ScheduleKey == "" {
		t.Fatalf("expected one pending task with a schedule key, tasks=%+v", tasks)
	}
}
