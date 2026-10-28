package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type memoryStore struct {
	items       map[int64]SourceContent
	raws        map[int64]json.RawMessage
	created     int
	materialize int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: map[int64]SourceContent{}, raws: map[int64]json.RawMessage{}}
}

func (m *memoryStore) CreateSource(v SourceContent, raw json.RawMessage) (SourceContent, error) {
	for _, item := range m.items {
		if item.TeamID == v.TeamID && item.Platform == v.Platform && item.PlatformContentID == v.PlatformContentID {
			return SourceContent{}, ErrDuplicate
		}
	}
	m.created++
	v.ID = int64(m.created)
	m.items[v.ID] = v
	m.raws[v.ID] = raw
	return v, nil
}

func (m *memoryStore) ListSources(f Filter) ([]SourceContentView, error) {
	var out []SourceContentView
	for _, item := range m.items {
		if f.TeamID != nil && item.TeamID != *f.TeamID {
			continue
		}
		if f.Platform != "" && item.Platform != f.Platform {
			continue
		}
		if f.Status != "" && item.Status != f.Status {
			continue
		}
		if f.SourceType != "" && item.SourceType != f.SourceType {
			continue
		}
		if len(f.CategoryIDs) > 0 {
			matched := false
			for _, categoryID := range f.CategoryIDs {
				if item.CategoryID != nil && *item.CategoryID == categoryID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if f.StrategyID != nil && (item.StrategyID == nil || *item.StrategyID != *f.StrategyID) {
			continue
		}
		if f.CrawlTaskID != nil && (item.CrawlTaskID == nil || *item.CrawlTaskID != *f.CrawlTaskID) {
			continue
		}
		if f.MaterialID != nil && (item.MaterialID == nil || *item.MaterialID != *f.MaterialID) {
			continue
		}
		view := SourceContentView{SourceContent: item}
		if item.StrategyID != nil {
			view.StrategyName = "Strategy " + fmt.Sprint(*item.StrategyID)
		}
		if item.CrawlTaskID != nil {
			view.CrawlTaskName = "Task " + fmt.Sprint(*item.CrawlTaskID)
		}
		out = append(out, view)
	}
	return out, nil
}

func (m *memoryStore) FindSource(id int64) (SourceContentView, bool, error) {
	v, ok := m.items[id]
	if !ok {
		return SourceContentView{}, false, nil
	}
	view := SourceContentView{SourceContent: v}
	if v.StrategyID != nil {
		view.StrategyName = "Strategy " + fmt.Sprint(*v.StrategyID)
	}
	if v.CrawlTaskID != nil {
		view.CrawlTaskName = "Task " + fmt.Sprint(*v.CrawlTaskID)
	}
	return view, true, nil
}

func (m *memoryStore) UpdateStatus(id int64, status Status, reason string, auditNote string, actorID identityservice.UserID, now time.Time) (SourceContent, error) {
	v, ok := m.items[id]
	if !ok {
		return SourceContent{}, ErrNotFound
	}
	v.Status, v.IgnoredReason, v.AuditNote = status, reason, auditNote
	m.items[id] = v
	return v, nil
}

func (m *memoryStore) RecordMaterialFailure(id int64, reason string) (SourceContent, error) {
	v, ok := m.items[id]
	if !ok {
		return SourceContent{}, ErrNotFound
	}
	v.FailureReason = reason
	m.items[id] = v
	return v, nil
}

func (m *memoryStore) Materialize(id int64, creator identityservice.UserID, now time.Time) (Material, error) {
	v, ok := m.items[id]
	if !ok {
		return Material{}, ErrNotFound
	}
	m.materialize++
	v.Status = StatusMaterialCreated
	v.FailureReason = ""
	materialID := int64(m.materialize)
	v.MaterialID = &materialID
	m.items[id] = v
	return Material{ID: materialID, TeamID: v.TeamID, SourceContentID: id, Title: v.Title, CreatedBy: creator, CreatedAt: now, UpdatedAt: now}, nil
}

func TestScopeIsTeamWideAndAdminCanSeeAll(t *testing.T) {
	service := NewService(newMemoryStore())
	teamA, teamB := identityservice.TeamID(10), identityservice.TeamID(20)
	operator := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &teamA}
	if _, err := service.Scope(operator, nil); err != nil {
		t.Fatalf("operator should access own team: %v", err)
	}
	if _, err := service.Scope(operator, &teamB); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator should be forbidden from another team, got %v", err)
	}
	admin := identityservice.PublicUser{ID: 1, Role: identityservice.RoleAdmin}
	if scope, err := service.Scope(admin, nil); err != nil || scope != nil {
		t.Fatalf("admin should access all teams, scope=%v err=%v", scope, err)
	}
}

func TestCreateSourceNormalizesAndRejectsDuplicate(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	team := identityservice.TeamID(10)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	first, err := service.CreateSource(actor, SourceInput{CategoryID: testCategoryID(), Platform: " douyin ", PlatformContentID: " aweme-1 ", SourceType: "search", RawJSON: json.RawMessage(`{"vid":"aweme-1"}`)})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if first.Platform != "douyin" || first.PlatformContentID != "aweme-1" || first.Status != StatusPending || first.TeamID != team {
		t.Fatalf("unexpected source: %+v", first)
	}
	if _, err := service.CreateSource(actor, SourceInput{CategoryID: testCategoryID(), Platform: "douyin", PlatformContentID: "aweme-1", SourceType: "search"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate should be typed, got %v", err)
	}
}

func TestCreateSourceRequiresCategory(t *testing.T) {
	service := NewService(newMemoryStore())
	team := identityservice.TeamID(10)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	if _, err := service.CreateSource(actor, SourceInput{Platform: "douyin", PlatformContentID: "no-category", SourceType: "search"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("create source without category error = %v, want ErrInvalidInput", err)
	}
}

func TestListSourcesFiltersMultipleCategoriesWithoutCrossingTeams(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	teamA, teamB := identityservice.TeamID(10), identityservice.TeamID(20)
	operatorA := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &teamA}
	operatorB := identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, TeamID: &teamB}
	hy, sjz, other := identityservice.CategoryID(1), identityservice.CategoryID(2), identityservice.CategoryID(3)
	for index, input := range []SourceInput{
		{Platform: "douyin", PlatformContentID: "a-hy", SourceType: "search", CategoryID: &hy},
		{Platform: "douyin", PlatformContentID: "a-sjz", SourceType: "search", CategoryID: &sjz},
		{Platform: "douyin", PlatformContentID: "a-other", SourceType: "search", CategoryID: &other},
	} {
		if _, err := service.CreateSource(operatorA, input); err != nil {
			t.Fatalf("create source %d: %v", index, err)
		}
	}
	if _, err := service.CreateSource(operatorB, SourceInput{Platform: "douyin", PlatformContentID: "b-hy", SourceType: "search", CategoryID: &hy}); err != nil {
		t.Fatalf("create other-team source: %v", err)
	}

	items, err := service.List(operatorA, Filter{CategoryIDs: []identityservice.CategoryID{hy, sjz}})
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("multi-category filter returned %d items, want 2: %+v", len(items), items)
	}
	for _, item := range items {
		if item.TeamID != teamA || (item.CategoryID == nil || (*item.CategoryID != identityservice.CategoryID(1) && *item.CategoryID != identityservice.CategoryID(2))) {
			t.Fatalf("filter leaked an item: %+v", item)
		}
	}
	forgedTeam := teamB
	if _, err := service.List(operatorA, Filter{TeamID: &forgedTeam, CategoryIDs: []identityservice.CategoryID{hy}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("category filter crossed team boundary: %v", err)
	}
}

func TestMaterializeIsTeamScoped(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	teamA, teamB := identityservice.TeamID(10), identityservice.TeamID(20)
	created, err := service.CreateSource(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &teamA}, SourceInput{CategoryID: testCategoryID(), Platform: "douyin", PlatformContentID: "aweme-2", SourceType: "url", Title: "demo"})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := service.Materialize(identityservice.PublicUser{ID: 3, Role: identityservice.RoleOperator, TeamID: &teamB}, created.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-team materialization should be forbidden, got %v", err)
	}
	material, err := service.Materialize(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &teamA}, created.ID)
	if err != nil || material.SourceContentID != created.ID {
		t.Fatalf("materialize own team source: %+v err=%v", material, err)
	}
}

func TestBatchSetStatusProcessesItemsIndependently(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	team := identityservice.TeamID(10)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	first, err := service.CreateSource(actor, SourceInput{CategoryID: testCategoryID(), Platform: "douyin", PlatformContentID: "batch-1", SourceType: "search"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSource(actor, SourceInput{CategoryID: testCategoryID(), Platform: "douyin", PlatformContentID: "batch-2", SourceType: "search"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Materialize(actor, first.ID); err != nil {
		t.Fatal(err)
	}
	result, err := service.batchSetStatus(actor, []int64{first.ID, second.ID}, StatusPending, "restore", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 || result.Failed != 1 || len(result.Items) != 2 {
		t.Fatalf("expected independent batch result: %+v", result)
	}
	if result.Items[0].Success || result.Items[1].Message != "" {
		t.Fatalf("unexpected batch items: %+v", result.Items)
	}
	if store.items[second.ID].Status != StatusPending {
		t.Fatalf("second item should be restored: %+v", store.items[second.ID])
	}
}

type failingMaterializeStore struct {
	*memoryStore
	failIDs map[int64]struct{}
}

func (s *failingMaterializeStore) Materialize(id int64, creator identityservice.UserID, now time.Time) (Material, error) {
	if _, exists := s.failIDs[id]; exists {
		return Material{}, errors.New("material provider failed")
	}
	return s.memoryStore.Materialize(id, creator, now)
}

func TestBatchMaterializeProcessesItemsIndependently(t *testing.T) {
	memory := newMemoryStore()
	store := &failingMaterializeStore{memoryStore: memory, failIDs: map[int64]struct{}{}}
	service := NewService(store)
	team := identityservice.TeamID(10)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	first, err := service.CreateSource(actor, SourceInput{CategoryID: testCategoryID(), Platform: "douyin", PlatformContentID: "material-1", SourceType: "search"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSource(actor, SourceInput{CategoryID: testCategoryID(), Platform: "douyin", PlatformContentID: "material-2", SourceType: "search"})
	if err != nil {
		t.Fatal(err)
	}
	store.failIDs[first.ID] = struct{}{}
	result := service.batchMaterialize(actor, []int64{first.ID, second.ID})
	if result.Succeeded != 1 || result.Failed != 1 || len(result.Items) != 2 {
		t.Fatalf("expected independent materialize result: %+v", result)
	}
	if result.Items[0].Success || !result.Items[1].Success || store.items[second.ID].Status != StatusMaterialCreated {
		t.Fatalf("unexpected materialize result: %+v", result)
	}
}
