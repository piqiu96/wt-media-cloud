package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type memoryStore struct {
	items       map[int64]SourceContent
	created     int
	materialize int
}

func newMemoryStore() *memoryStore { return &memoryStore{items: map[int64]SourceContent{}} }

func (m *memoryStore) CreateSource(v SourceContent, _ json.RawMessage) (SourceContent, error) {
	for _, item := range m.items {
		if item.TeamID == v.TeamID && item.Platform == v.Platform && item.PlatformContentID == v.PlatformContentID {
			return SourceContent{}, ErrDuplicate
		}
	}
	m.created++
	v.ID = int64(m.created)
	m.items[v.ID] = v
	return v, nil
}

func (m *memoryStore) ListSources(f Filter) ([]SourceContent, error) {
	var out []SourceContent
	for _, item := range m.items {
		if f.TeamID != nil && item.TeamID != *f.TeamID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (m *memoryStore) FindSource(id int64) (SourceContent, bool, error) {
	v, ok := m.items[id]
	return v, ok, nil
}

func (m *memoryStore) UpdateStatus(id int64, status Status, reason string, auditNote string) (SourceContent, error) {
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
	first, err := service.CreateSource(actor, SourceInput{Platform: " douyin ", PlatformContentID: " aweme-1 ", SourceType: "search", RawJSON: json.RawMessage(`{"vid":"aweme-1"}`)})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if first.Platform != "douyin" || first.PlatformContentID != "aweme-1" || first.Status != StatusPending || first.TeamID != team {
		t.Fatalf("unexpected source: %+v", first)
	}
	if _, err := service.CreateSource(actor, SourceInput{Platform: "douyin", PlatformContentID: "aweme-1", SourceType: "search"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate should be typed, got %v", err)
	}
}

func TestMaterializeIsTeamScoped(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	teamA, teamB := identityservice.TeamID(10), identityservice.TeamID(20)
	created, err := service.CreateSource(identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &teamA}, SourceInput{Platform: "douyin", PlatformContentID: "aweme-2", SourceType: "url", Title: "demo"})
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
	first, err := service.CreateSource(actor, SourceInput{Platform: "douyin", PlatformContentID: "batch-1", SourceType: "search"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSource(actor, SourceInput{Platform: "douyin", PlatformContentID: "batch-2", SourceType: "search"})
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
	first, err := service.CreateSource(actor, SourceInput{Platform: "douyin", PlatformContentID: "material-1", SourceType: "search"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSource(actor, SourceInput{Platform: "douyin", PlatformContentID: "material-2", SourceType: "search"})
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
