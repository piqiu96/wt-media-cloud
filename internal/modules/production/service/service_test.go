package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type memoryStore struct {
	material model.Material
	found    bool
	created  repository.CreateUsageInput
	filter   repository.MaterialFilter
	usages   []model.MaterialUsage

	// removed records the usage the service asked to soft-remove, and removedFor
	// the user it scoped that to. Both are kept because the scoping is the part
	// that stops one user removing another user's row: a store call that received
	// the right ID and the wrong user would satisfy an ID-only assertion.
	removed    int64
	removedFor sharedidentity.UserID
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
func (s *memoryStore) ListActiveUsages(sharedidentity.UserID) ([]model.MaterialUsage, error) {
	return s.usages, nil
}
func (s *memoryStore) FindUsageForUser(usageID int64, _ sharedidentity.UserID) (model.MaterialUsage, bool, error) {
	for _, usage := range s.usages {
		if usage.ID == usageID {
			return usage, true, nil
		}
	}
	return model.MaterialUsage{}, false, nil
}
func (s *memoryStore) RemoveUsageByID(usageID int64, userID sharedidentity.UserID, _ time.Time) (bool, error) {
	s.removed = usageID
	s.removedFor = userID
	return true, nil
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

func TestListMyMaterialsReturnsOnlyUsagesWithCurrentlyVisibleMaterials(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, usages: []model.MaterialUsage{usage}}
	svc := NewService(store)
	items, err := svc.ListMyMaterials(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}})
	if err != nil {
		t.Fatalf("ListMyMaterials() error = %v", err)
	}
	if len(items) != 1 || items[0].Material == nil || items[0].Material.ID != 42 {
		t.Fatalf("items = %+v", items)
	}
}

func TestRemoveUsageChecksTheMaterialScopeBeforeSoftRemoval(t *testing.T) {
	team := service.TeamID(7)
	game := "game-b"
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, usages: []model.MaterialUsage{usage}}
	svc := NewService(store)
	err := svc.RemoveUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 5)
	if !errors.Is(err, ErrForbidden) || store.removed != 0 {
		t.Fatalf("err=%v removed=%d", err, store.removed)
	}
}

// "This is no longer in your materials" and "this material does not exist" are
// different answers to the user, and the frozen contract publishes a distinct
// errcode for each. They are therefore distinct sentinels, and the second arm
// below is what makes the first meaningful: an ErrUsageNotFound defined to wrap
// ErrNotFound would satisfy `errors.Is(err, ErrUsageNotFound)` and still map to
// the material errcode, telling the user the wrong thing.
func TestRemoveUsageReportsAMissingUsageSeparatelyFromAMissingMaterial(t *testing.T) {
	team := service.TeamID(7)
	store := &memoryStore{found: false}
	svc := NewService(store)
	err := svc.RemoveUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team}, 5)
	if !errors.Is(err, ErrUsageNotFound) {
		t.Fatalf("RemoveUsage() error = %v, want ErrUsageNotFound", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing usage must not be reported as a missing material: %v", err)
	}
	if store.removed != 0 {
		t.Fatalf("nothing may be removed when the usage is not the actor's: %d", store.removed)
	}
}

// The removal is scoped to the acting user, which is what stops one operator from
// removing a row they can see but do not own. Asserting the ID alone would pass
// even if the user never reached the store, so both are checked.
func TestRemoveUsageScopesTheSoftRemovalToTheActingUser(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, usages: []model.MaterialUsage{usage}}
	svc := NewService(store)
	if err := svc.RemoveUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 5); err != nil {
		t.Fatalf("RemoveUsage() error = %v", err)
	}
	if store.removed != 5 || store.removedFor != 9 {
		t.Fatalf("removed=%d removedFor=%d, want 5 and 9", store.removed, store.removedFor)
	}
}
