package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type memoryStore struct {
	material model.Material
	found    bool
	created  repository.CreateUsageInput
	filter   repository.MaterialFilter
	usages   []model.MaterialUsage

	// createdUsage is what the store reports back about the upsert. It defaults
	// to false — "the relation was already there" — so a test that expects a
	// creation has to say so, rather than getting a 201 by omission.
	createdUsage bool

	// removed records the usage the service asked to soft-remove, and removedFor
	// the user it scoped that to. Both are kept because the scoping is the part
	// that stops one user removing another user's row: a store call that received
	// the right ID and the wrong user would satisfy an ID-only assertion.
	removed    int64
	removedFor sharedidentity.UserID
}

// stubNodes answers the one question this module asks the runtime-binding
// domain, and refuses by default: a download with no node is a 409, so a test
// that expects a task to be created has to say which machine it goes to rather
// than getting an empty node id by omission.
type stubNodes struct {
	node runtimeservice.AgentNode
	err  error
	ask  []sharedidentity.UserID
}

func (s *stubNodes) ResolveFreshLocalNode(userID sharedidentity.UserID) (runtimeservice.AgentNode, error) {
	s.ask = append(s.ask, userID)
	if s.err != nil {
		return runtimeservice.AgentNode{}, s.err
	}
	if s.node.ID == "" {
		return runtimeservice.AgentNode{}, runtimeservice.ErrLocalTrustUnavailable
	}
	return s.node, nil
}

// stubTransfers records the request rather than only its answer, because what
// this module has to get right is the material facts it copies onto the task —
// the executor runs later, on another machine, and cannot read them back.
type stubTransfers struct {
	task  transferdto.Task
	err   error
	last  transferservice.CreateUserDownloadInput
	count int
}

func (s *stubTransfers) CreateUserDownload(input transferservice.CreateUserDownloadInput) (transferdto.Task, error) {
	s.count++
	s.last = input
	if s.err != nil {
		return transferdto.Task{}, s.err
	}
	if s.task.ID == "" {
		return transferdto.Task{ID: "transfer-1", Status: "pending"}, nil
	}
	return s.task, nil
}

func testService(store Store) *Service {
	return NewService(store, &stubNodes{}, &stubTransfers{})
}

func (s *memoryStore) FindMaterial(int64) (model.Material, bool, error) {
	return s.material, s.found, nil
}
func (s *memoryStore) CreateOrRestoreUsage(input repository.CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	s.created = input
	return model.MaterialUsage{ID: 1, TeamID: input.TeamID, MaterialID: input.MaterialID, UserID: input.UserID, Status: model.MaterialUsageActive, CreatedAt: now, UpdatedAt: now}, s.createdUsage, nil
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
	svc := testService(store)
	_, _, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
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
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, createdUsage: true}
	svc := testService(store)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	usage, created, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
	if err != nil {
		t.Fatalf("AddUsage() error = %v", err)
	}
	if usage.ID != 1 || store.created.TeamID != team || store.created.MaterialID != 42 || store.created.UserID != 9 {
		t.Fatalf("usage=%+v created=%+v", usage, store.created)
	}
	if !created {
		t.Fatal("the store reported a new relation and AddUsage answered with an existing one")
	}
}

// The flag has to survive the service, not just the repository: it is the whole
// reason the route can answer 200 sometimes and 201 other times, and a service
// that dropped it would answer 201 to every click while every store test stayed
// green.
func TestAddUsageReportsAnAlreadyActiveRelationAsNotCreated(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}}
	svc := testService(store)

	usage, created, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
	if err != nil {
		t.Fatalf("AddUsage() error = %v", err)
	}
	if created {
		t.Fatal("a relation that was already active must not report as created")
	}
	if usage.ID != 1 {
		t.Fatalf("the existing relation is still the answer, got %+v", usage)
	}
}

func TestListMaterialsProjectsTheActorTeamAndGameScopeIntoTheStoreQuery(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}}
	svc := testService(store)
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
	svc := testService(store)
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
	svc := testService(store)
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
	svc := testService(store)
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
	svc := testService(store)
	if err := svc.RemoveUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 5); err != nil {
		t.Fatalf("RemoveUsage() error = %v", err)
	}
	if store.removed != 5 || store.removedFor != 9 {
		t.Fatalf("removed=%d removedFor=%d, want 5 and 9", store.removed, store.removedFor)
	}
}

func readyMaterial(team service.TeamID, game string) model.Material {
	size := int64(2048)
	return model.Material{
		ID: 42, TeamID: team, GameID: &game, Title: "示例视频",
		VideoStatus:     model.VideoReady,
		SourceObjectKey: "materials/42/aaaaaaaa.mp4",
		VideoSizeBytes:  &size,
		VideoSHA256:     strings.Repeat("a", 64),
	}
}

func scopedActor(team service.TeamID, game string) service.PublicUser {
	return service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{game}}
}

// The rule this route exists to keep is "a download is addressed to one machine".
// A task created without an assignee would be claimed by whichever device polled
// first, so the node has to reach the task as the one the resolver chose — not as
// an empty string the transfer module would then reject, and not as the actor.
func TestCreateDownloadAddressesTheTaskToTheResolvedNode(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := readyMaterial(team, game)
	store := &memoryStore{found: true, material: material}
	nodes := &stubNodes{node: runtimeservice.AgentNode{ID: "node-7", UserID: 9}}
	transfers := &stubTransfers{}
	svc := NewService(store, nodes, transfers)
	now := time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	task, err := svc.CreateDownload(scopedActor(team, game), 42)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if len(nodes.ask) != 1 || nodes.ask[0] != 9 {
		t.Fatalf("node lookup users = %v, want the actor alone", nodes.ask)
	}
	if transfers.count != 1 {
		t.Fatalf("transfer calls = %d, want 1", transfers.count)
	}
	got := transfers.last
	if got.AssignedNodeID != "node-7" || got.RequestedBy != 9 || got.TeamID != team || got.AssetID != 42 {
		t.Fatalf("transfer input = %+v", got)
	}
	// The executor runs later, on another machine, and may not read `materials`:
	// everything it needs to name, size and verify the file has to be on the task.
	if got.AssetTitle != "示例视频" || got.SourceObjectKey != "materials/42/aaaaaaaa.mp4" || got.TotalBytes != 2048 || got.ExpectedSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("the material facts must travel with the task: %+v", got)
	}
	if task.ID != "transfer-1" {
		t.Fatalf("task = %+v, want the transfer module's answer", task)
	}
}

// The three refusals, each asserted separately because they are three different
// things to tell the user: not yours to see, not ready yet, and no machine to
// download to. Collapsing any two of them would answer 404 or 500 where the
// frozen contract publishes a 409.
//
// The last arm is the one that is easy to get wrong: a resolver failure must not
// fall through to creating a task with an empty assignee.
func TestCreateDownloadRefusesBeforeItQueuesAnything(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	actor := scopedActor(team, game)

	for _, testCase := range []struct {
		name        string
		material    model.Material
		found       bool
		nodes       *stubNodes
		transferErr error
		want        error
	}{
		{
			name:        "the transfer module rejects the input",
			material:    readyMaterial(team, game),
			found:       true,
			nodes:       &stubNodes{node: runtimeservice.AgentNode{ID: "node-7"}},
			transferErr: transferservice.ErrInvalidInput,
			want:        ErrInvalidInput,
		},
		{
			name:     "outside the actor's scope",
			material: func() model.Material { m := readyMaterial(team, "game-b"); return m }(),
			found:    true,
			nodes:    &stubNodes{node: runtimeservice.AgentNode{ID: "node-7"}},
			want:     ErrForbidden,
		},
		{
			name: "no prepared video",
			material: func() model.Material {
				m := readyMaterial(team, game)
				m.VideoStatus = model.VideoNotDownloaded
				m.SourceObjectKey = ""
				m.VideoSizeBytes = nil
				m.VideoSHA256 = ""
				return m
			}(),
			found: true,
			nodes: &stubNodes{node: runtimeservice.AgentNode{ID: "node-7"}},
			want:  ErrMaterialUnavailable,
		},
		{
			name:     "no fresh node",
			material: readyMaterial(team, game),
			found:    true,
			nodes:    &stubNodes{err: runtimeservice.ErrLocalTrustUnavailable},
			want:     ErrLocalNodeUnavailable,
		},
		{
			name:     "material does not exist",
			material: model.Material{},
			found:    false,
			nodes:    &stubNodes{node: runtimeservice.AgentNode{ID: "node-7"}},
			want:     ErrNotFound,
		},
	} {
		store := &memoryStore{found: testCase.found, material: testCase.material}
		transfers := &stubTransfers{err: testCase.transferErr}
		svc := NewService(store, testCase.nodes, transfers)

		if _, err := svc.CreateDownload(actor, 42); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: error = %v, want %v", testCase.name, err, testCase.want)
		}
		// The one arm that legitimately reaches the transfer module is the one
		// where that module itself refused; every other arm must stop before it.
		if testCase.transferErr == nil && transfers.count != 0 {
			t.Fatalf("%s: refused download still queued %d task(s)", testCase.name, transfers.count)
		}
	}
}

// `ready` is the projection's promise that the object key, size and hash were all
// written in the same step as the status. A row that breaks that promise is this
// module's own inconsistency, and answering the user's 409 ("try again later")
// would hide it behind a status that says the system is working.
func TestCreateDownloadDoesNotPresentAnIncompleteReadyRowAsUnavailable(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := readyMaterial(team, game)
	material.VideoSHA256 = ""
	store := &memoryStore{found: true, material: material}
	transfers := &stubTransfers{}
	svc := NewService(store, &stubNodes{node: runtimeservice.AgentNode{ID: "node-7"}}, transfers)

	_, err := svc.CreateDownload(scopedActor(team, game), 42)
	if err == nil {
		t.Fatal("a ready material with no hash must not be queued for download")
	}
	if errors.Is(err, ErrMaterialUnavailable) || errors.Is(err, ErrLocalNodeUnavailable) {
		t.Fatalf("error = %v, want an internal fault rather than a conflict the user is told to retry", err)
	}
	if transfers.count != 0 {
		t.Fatal("an inconsistent row must not reach the transfer module")
	}
}
