package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type memoryStore struct {
	material model.Material
	found    bool
	created  repository.CreateUsageInput
	filter   repository.MaterialFilter
}

func (s *memoryStore) FindMaterial(int64) (model.Material, bool, error) {
	return s.material, s.found, nil
}
func (s *memoryStore) CreateOrRestoreUsage(input repository.CreateUsageInput, now time.Time) (model.MaterialUsage, error) {
	s.created = input
	return model.MaterialUsage{ID: 1, TeamID: input.TeamID, MaterialID: input.MaterialID, UserID: input.UserID, Status: model.MaterialUsageActive, CreatedAt: now, UpdatedAt: now}, nil
}
func (s *memoryStore) ListMaterials(filter repository.MaterialFilter) ([]model.Material, error) {
	s.filter = filter
	return []model.Material{s.material}, nil
}

func TestAddUsageRejectsMaterialOutsideActorGameScope(t *testing.T) {
	team := service.TeamID(7)
	game := "game-b"
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}}
	svc := NewService(store)
	_, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AddUsage() error = %v, want ErrForbidden", err)
	}
	if store.created.MaterialID != 0 {
		t.Fatalf("store must not be mutated for forbidden material: %+v", store.created)
	}
}

func TestAddUsageCreatesTheSoleMaterialUsageForScopedActor(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}}
	svc := NewService(store)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	usage, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
	if err != nil {
		t.Fatalf("AddUsage() error = %v", err)
	}
	if usage.ID != 1 || store.created.TeamID != team || store.created.MaterialID != 42 || store.created.UserID != 9 {
		t.Fatalf("usage=%+v created=%+v", usage, store.created)
	}
}

func TestListMaterialsProjectsTheActorTeamAndGameScopeIntoTheStoreQuery(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}}
	svc := NewService(store)
	items, err := svc.ListMaterials(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a", "game-b"}}, "demo")
	if err != nil {
		t.Fatalf("ListMaterials() error = %v", err)
	}
	if len(items) != 1 || store.filter.TeamID == nil || *store.filter.TeamID != team || len(store.filter.GameIDs) != 2 || store.filter.Search != "demo" {
		t.Fatalf("items=%+v filter=%+v", items, store.filter)
	}
}
