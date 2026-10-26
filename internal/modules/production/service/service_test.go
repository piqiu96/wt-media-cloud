package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
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

	// restored is the same record for the inverse write, and listedFor is the user
	// the list was asked about — the list is scoped by the query rather than by
	// filtering, so a service that asked about the wrong user would return another
	// operator's materials with nothing in the rows themselves to show it.
	restored    int64
	restoredFor sharedidentity.UserID
	listedFor   sharedidentity.UserID

	// findCount counts the material lookups, and refind — when set — is what the
	// store answers from the second one on. `CreateDownload` re-reads the material
	// when its guarded projection write was refused, and that second row is the
	// whole difference between "wait for the preparation already running" and "the
	// video is ready after all"; a store answering the same row twice could not
	// tell those two cases apart, so neither could a test of them.
	findCount int
	refind    *model.Material

	// relations and facts are the two batch reads the library page decorates from.
	// Both default to empty — no relation, no successful output — which is the
	// honest starting state of a new material.
	relations    []repository.MaterialUsageRelation
	relationsFor sharedidentity.UserID
	relationsIDs []int64
	facts        []repository.MaterialOutputFact
	factsIDs     []int64

	// usageMissing is the download gate's refusal. It is opt-in because most
	// download tests are about what happens after the gate; the gate's own test
	// sets it, so a service that stopped asking would answer a queued task there.
	usageMissing bool
	usageChecks  []int64

	// preparing defaults to false for the same reason the other doubles refuse by
	// default: a test that expects the projection to have moved has to say so.
	preparing     bool
	preparingErr  error
	preparedTeam  sharedidentity.TeamID
	preparedFor   int64
	preparedCount int

	// The take-back write is recorded with the exact key and timestamp it was
	// scoped to: that pair is the compare, and a service that passed the wrong
	// one would still move a row and every flag-based assertion would pass.
	repreparing     bool
	repreparingErr  error
	repreparedTeam  sharedidentity.TeamID
	repreparedFor   int64
	repreparedKey   string
	repreparedAt    *time.Time
	repreparedCount int

	// The worker's two writes, recorded in full rather than as a flag: what the
	// service does with each is the part under test — one turns a refused write into
	// a missing material, the other lets it pass — and the facts have to be shown to
	// have travelled untouched to the store.
	readyFacts  repository.VideoFacts
	readyTeam   sharedidentity.TeamID
	readyFor    int64
	readyAt     time.Time
	ready       bool
	readyErr    error
	failedMsg   string
	failedTeam  sharedidentity.TeamID
	failedFor   int64
	failedAt    time.Time
	failed      bool
	failedErr   error
	failedCount int

	// The cancellation's write, recorded the same way. Its flag is not a refusal:
	// the repository's predicate makes "nothing to take back" the ordinary reading,
	// so a double that defaulted either way would have to be told what it meant.
	notPreparedTeam  sharedidentity.TeamID
	notPreparedFor   int64
	notPreparedAt    time.Time
	notPrepared      bool
	notPreparedErr   error
	notPreparedCount int
}

// stubTransfers records the request rather than only its answer, because what
// this module has to get right is the material facts it copies onto the task —
// the executor runs later, on another machine, and cannot read them back.
type stubTransfers struct {
	task  transferdto.Task
	err   error
	last  transferservice.CreateUserDownloadInput
	count int

	// The preparation is recorded separately: it is the other half of the click,
	// and a double that folded it into `last` could not show that a direct download
	// queued no preparation, or that a waiting one queued no facts of its own.
	prepare      transferservice.EnsureMaterialSourcePrepareInput
	prepareTask  transferdto.Task
	prepareErr   error
	prepareCount int

	// downloadStatuses is what LatestUserDownloadStatuses answers, keyed by
	// material_id. A nil map is the honest "no one has ever downloaded anything"
	// default, which keeps every older test meaningfully green.
	downloadStatuses map[int64]string
	// lastStatusQuery records the material_ids handed to LatestUserDownloadStatuses.
	lastStatusQuery []int64

	// attachOK/attachTask stand in for the post-insert hand-over: false is the
	// ordinary "the preparation has not finished yet" answer, and the task body
	// is what the caller must return when the attach did happen.
	attachTask  transferdto.Task
	attachOK    bool
	attachErr   error
	attachIDs   []string
	attachCount int
}

func (s *stubTransfers) EnsureMaterialSourcePrepare(input transferservice.EnsureMaterialSourcePrepareInput) (transferdto.Task, error) {
	s.prepareCount++
	s.prepare = input
	if s.prepareErr != nil {
		return transferdto.Task{}, s.prepareErr
	}
	if s.prepareTask.ID == "" {
		return transferdto.Task{ID: "transfer-prepare-1", Status: "pending"}, nil
	}
	return s.prepareTask, nil
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

func (s *stubTransfers) LatestUserDownloadStatuses(userID service.UserID, materialIDs []int64) (map[int64]string, error) {
	s.lastStatusQuery = append([]int64(nil), materialIDs...)
	return s.downloadStatuses, nil
}

func (s *stubTransfers) AttachCompletedDependency(taskID string) (transferdto.Task, bool, error) {
	s.attachCount++
	s.attachIDs = append(s.attachIDs, taskID)
	if s.attachErr != nil {
		return transferdto.Task{}, false, s.attachErr
	}
	if !s.attachOK {
		return transferdto.Task{}, false, nil
	}
	return s.attachTask, true, nil
}

func testService(store Store) *Service {
	return NewService(store, &stubTransfers{}, &stubLinks{})
}

// stubLinks answers the one question this module asks the storage boundary: the
// signed address of an object key. It records the key it was asked about, for
// the same reason stubTransfers records its input — the key handed over is the
// seam between "the material's own facts" and "where the bucket keeps it", and
// a double that only answered could not show the wrong key being signed. It
// records the context for the same reason: the real issuer signs locally today,
// so an implementation that dropped the caller's context would still compile and
// still pass every other arm here.
type stubLinks struct {
	url string
	err error
	key string
	ctx context.Context

	// missing names the keys the bucket is told not to hold; every other key is
	// present. Absence is opt-in for the same reason refusals are elsewhere: a
	// double that answered "missing" by default would make every older ready-row
	// test exercise the repair branch without saying so.
	missing    map[string]bool
	existsErr  error
	existsKeys []string
	existsCtx  context.Context
}

func (s *stubLinks) PresignObjectURL(ctx context.Context, key string) (string, error) {
	s.key, s.ctx = key, ctx
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func (s *stubLinks) ObjectExists(ctx context.Context, key string) (bool, error) {
	s.existsKeys = append(s.existsKeys, key)
	s.existsCtx = ctx
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return !s.missing[key], nil
}

func (s *memoryStore) FindMaterial(int64) (model.Material, bool, error) {
	s.findCount++
	if s.refind != nil && s.findCount > 1 {
		return *s.refind, s.found, nil
	}
	return s.material, s.found, nil
}
func (s *memoryStore) MarkVideoPreparing(teamID sharedidentity.TeamID, materialID int64, _ time.Time) (bool, error) {
	s.preparedCount++
	s.preparedTeam, s.preparedFor = teamID, materialID
	return s.preparing, s.preparingErr
}
func (s *memoryStore) MarkVideoRepreparing(teamID sharedidentity.TeamID, materialID int64, expectedObjectKey string, expectedPreparedAt *time.Time, _ time.Time) (bool, error) {
	s.repreparedCount++
	s.repreparedTeam, s.repreparedFor = teamID, materialID
	s.repreparedKey, s.repreparedAt = expectedObjectKey, expectedPreparedAt
	return s.repreparing, s.repreparingErr
}
func (s *memoryStore) MarkVideoReady(teamID sharedidentity.TeamID, materialID int64, facts repository.VideoFacts, now time.Time) (bool, error) {
	s.readyTeam, s.readyFor, s.readyFacts, s.readyAt = teamID, materialID, facts, now
	return s.ready, s.readyErr
}
func (s *memoryStore) MarkVideoFailed(teamID sharedidentity.TeamID, materialID int64, message string, now time.Time) (bool, error) {
	s.failedCount++
	s.failedTeam, s.failedFor, s.failedMsg, s.failedAt = teamID, materialID, message, now
	return s.failed, s.failedErr
}
func (s *memoryStore) MarkVideoNotPrepared(teamID sharedidentity.TeamID, materialID int64, now time.Time) (bool, error) {
	s.notPreparedCount++
	s.notPreparedTeam, s.notPreparedFor, s.notPreparedAt = teamID, materialID, now
	return s.notPrepared, s.notPreparedErr
}

func (s *memoryStore) CreateOrRestoreUsage(input repository.CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	s.created = input
	return model.MaterialUsage{ID: 1, TeamID: input.TeamID, MaterialID: input.MaterialID, UserID: input.UserID, Status: model.MaterialUsageActive, CreatedAt: now, UpdatedAt: now}, s.createdUsage, nil
}
func (s *memoryStore) ListMaterials(filter repository.MaterialFilter) ([]model.Material, error) {
	s.filter = filter
	return []model.Material{s.material}, nil
}
func (s *memoryStore) ListUsageRelations(userID sharedidentity.UserID, materialIDs []int64) ([]repository.MaterialUsageRelation, error) {
	s.relationsFor = userID
	s.relationsIDs = append([]int64(nil), materialIDs...)
	return s.relations, nil
}
func (s *memoryStore) ListMaterialOutputFacts(materialIDs []int64) ([]repository.MaterialOutputFact, error) {
	s.factsIDs = append([]int64(nil), materialIDs...)
	return s.facts, nil
}
func (s *memoryStore) HasActiveUsage(materialID int64, _ sharedidentity.UserID) (bool, error) {
	s.usageChecks = append(s.usageChecks, materialID)
	return !s.usageMissing, nil
}
func (s *memoryStore) ListUsages(userID sharedidentity.UserID) ([]model.MaterialUsage, error) {
	s.listedFor = userID
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
func (s *memoryStore) RestoreUsageByID(usageID int64, userID sharedidentity.UserID, _ time.Time) error {
	s.restored = usageID
	s.restoredFor = userID
	return nil
}

// A game is a filter, not a visibility boundary (§5.3.3): an operator may join
// a material in their team even when someone else is responsible for its game.
func TestAddUsageAllowsAnotherGameInTheActorsTeam(t *testing.T) {
	team := service.TeamID(7)
	game := "game-b"
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, createdUsage: true}
	svc := testService(store)
	_, _, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
	if err != nil {
		t.Fatalf("AddUsage() error = %v, want the team's other game to be allowed", err)
	}
	if store.created.MaterialID != 42 || store.created.TeamID != team {
		t.Fatalf("store creation = %+v, want the team's material", store.created)
	}
}

func TestAddUsageRejectsAMaterialInAnotherTeam(t *testing.T) {
	team := service.TeamID(7)
	otherTeam := service.TeamID(8)
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: otherTeam}}
	svc := testService(store)
	_, _, err := svc.AddUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team}, 42)
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

// Detail, add and download share this visibility check. The team is the
// boundary; the actor's responsible games are not, so an operator sees a
// material of another game in the same team and an admin crosses teams. A
// senior is still pinned to their own team.
func TestGetMaterialScopesVisibilityByTeamNotGame(t *testing.T) {
	team := service.TeamID(7)
	otherTeam := service.TeamID(8)
	game := "game-a"
	for _, testCase := range []struct {
		name    string
		actor   service.PublicUser
		wantErr bool
	}{
		{"operator other game in team", service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-b"}}, false},
		{"operator other team", service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &otherTeam}, true},
		{"senior other game in team", service.PublicUser{ID: 2, Role: service.RoleSeniorOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-b"}}, false},
		{"senior other team", service.PublicUser{ID: 2, Role: service.RoleSeniorOperator, Status: service.UserStatusEnabled, TeamID: &otherTeam}, true},
		{"admin across teams", service.PublicUser{ID: 1, Role: service.RoleAdmin, Status: service.UserStatusEnabled}, false},
		{"disabled", service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusDisabled, TeamID: &team}, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}}
			svc := testService(store)
			_, err := svc.GetMaterial(testCase.actor, 42)
			if testCase.wantErr {
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("GetMaterial() error = %v, want ErrForbidden", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetMaterial() error = %v", err)
			}
		})
	}
}

// The library query is scoped by team, not by the actor's responsible games.
// Games travel only because the caller asked to narrow to them, and the viewer
// travels so the store can answer the relation and usage filters.
func TestListMaterialsScopesTheQueryByTeamAndLeavesGamesAsFilters(t *testing.T) {
	team := service.TeamID(7)
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team}}
	svc := testService(store)
	filter := repository.MaterialFilter{GameIDs: []string{"game-a", "game-b"}, Search: "demo", Joined: "not_joined"}
	items, err := svc.ListMaterialsFiltered(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, filter)
	if err != nil {
		t.Fatalf("ListMaterialsFiltered() error = %v", err)
	}
	if len(items) != 1 || store.filter.TeamID == nil || *store.filter.TeamID != team {
		t.Fatalf("items=%+v filter=%+v", items, store.filter)
	}
	if !slices.Equal(store.filter.GameIDs, []string{"game-a", "game-b"}) || store.filter.Search != "demo" || store.filter.Joined != "not_joined" || store.filter.UserID != 9 {
		t.Fatalf("the caller's filters were not carried through: %+v", store.filter)
	}
}

func TestListMaterialsRefusesAnotherTeamsFilter(t *testing.T) {
	team := service.TeamID(7)
	otherTeam := service.TeamID(8)
	store := &memoryStore{found: true}
	svc := testService(store)
	_, err := svc.ListMaterialsFiltered(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team}, repository.MaterialFilter{TeamID: &otherTeam})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListMaterialsFiltered() error = %v, want ErrForbidden", err)
	}
	if store.filter.TeamID != nil {
		t.Fatalf("the store was queried for a refused team: %+v", store.filter)
	}
}

func TestListMaterialsLetsAnAdminChooseTheTargetTeam(t *testing.T) {
	target := service.TeamID(8)
	store := &memoryStore{found: true}
	svc := testService(store)
	if _, err := svc.ListMaterialsFiltered(service.PublicUser{ID: 1, Role: service.RoleAdmin, Status: service.UserStatusEnabled}, repository.MaterialFilter{TeamID: &target}); err != nil {
		t.Fatalf("ListMaterialsFiltered() error = %v", err)
	}
	if store.filter.TeamID == nil || *store.filter.TeamID != target || store.filter.UserID != 1 {
		t.Fatalf("admin filter = %+v, want the named team and the admin as viewer", store.filter)
	}
}

// An admin who names no team reads across teams. The returned rows carry their
// own team, so a store query that silently pinned one team would hide the rest
// without any row looking wrong.
func TestListMaterialsLetsAnAdminReadAcrossTeamsWhenNoTeamIsNamed(t *testing.T) {
	store := &memoryStore{found: true}
	svc := testService(store)
	if _, err := svc.ListMaterialsFiltered(service.PublicUser{ID: 1, Role: service.RoleAdmin, Status: service.UserStatusEnabled}, repository.MaterialFilter{}); err != nil {
		t.Fatalf("ListMaterialsFiltered() error = %v", err)
	}
	if store.filter.TeamID != nil {
		t.Fatalf("admin filter = %+v, want no team scope", store.filter)
	}
}

// A senior operator reads every game of their own team. The games they are
// responsible for narrow a request when they ask, but they must not narrow the
// team behind their back (§5.3.3); another team stays refused and is not even
// queried.
func TestListMaterialsLetsASeniorReadEveryGameOfTheirTeam(t *testing.T) {
	team := service.TeamID(7)
	otherTeam := service.TeamID(8)
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team}}
	svc := testService(store)
	senior := service.PublicUser{ID: 2, Role: service.RoleSeniorOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}

	if _, err := svc.ListMaterialsFiltered(senior, repository.MaterialFilter{}); err != nil {
		t.Fatalf("ListMaterialsFiltered() error = %v", err)
	}
	if store.filter.TeamID == nil || *store.filter.TeamID != team || len(store.filter.GameIDs) != 0 {
		t.Fatalf("senior filter = %+v, want the whole team and no game scope", store.filter)
	}

	store.factsIDs = nil
	if _, err := svc.ListMaterialsFiltered(senior, repository.MaterialFilter{TeamID: &otherTeam}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListMaterialsFiltered(other team) error = %v, want ErrForbidden", err)
	}
	if len(store.factsIDs) != 0 {
		t.Fatalf("the store was queried for a refused team: %v", store.factsIDs)
	}
}

// The three usage dimensions come from batch reads: one relation row per
// material for the viewer, one successful-output count per operator. A
// historical output with no operator still counts as use but is not a person,
// and ordinary operators get counts without names.
func TestListMaterialsDecoratesTheViewersRelationAndTheTeamsUsage(t *testing.T) {
	team := service.TeamID(7)
	material := model.Material{ID: 42, TeamID: team}
	added := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	store := &memoryStore{
		found:    true,
		material: material,
		relations: []repository.MaterialUsageRelation{
			{MaterialID: 42, Status: model.MaterialUsageRemoved, CreatedAt: added},
		},
		facts: []repository.MaterialOutputFact{
			{MaterialID: 42, OperatorID: ptrInt64(9), OutputCount: 2},
			{MaterialID: 42, OperatorID: ptrInt64(11), OutputCount: 1},
			{MaterialID: 42, OutputCount: 3},
		},
	}
	svc := testService(store)

	items, err := svc.ListMaterials(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team}, "")
	if err != nil {
		t.Fatalf("ListMaterials() error = %v", err)
	}
	item := items[0]
	if item.UsageStatus != model.MaterialUsageRemoved || item.AddedAt == nil || !item.AddedAt.Equal(added) {
		t.Fatalf("relation projection = %+v, want removed with its original added time", item)
	}
	if item.MyUsage != model.MaterialMyUsageUsed {
		t.Fatalf("MyUsage = %q, want used: the viewer has a successful output", item.MyUsage)
	}
	if item.Usage == nil || item.Usage.OutputCount != 6 || item.Usage.OperatorCount != 2 {
		t.Fatalf("Usage = %+v, want 6 outputs by 2 known operators", item.Usage)
	}
	if item.Usage.Operators != nil {
		t.Fatalf("ordinary operators must not receive the operator names: %+v", item.Usage.Operators)
	}
	if !slices.Equal(store.factsIDs, []int64{42}) || store.relationsFor != 9 || !slices.Equal(store.relationsIDs, []int64{42}) {
		t.Fatalf("batch reads were asked for %v / %d %v", store.factsIDs, store.relationsFor, store.relationsIDs)
	}

	svc.operators = func() ([]service.PublicUser, error) {
		return []service.PublicUser{{ID: 9, Username: "operator-a"}, {ID: 11, Nickname: "运营乙"}}, nil
	}
	items, err = svc.ListMaterials(service.PublicUser{ID: 1, Role: service.RoleSeniorOperator, Status: service.UserStatusEnabled, TeamID: &team}, "")
	if err != nil {
		t.Fatalf("ListMaterials() error = %v", err)
	}
	usage := items[0].Usage
	if usage == nil || usage.OperatorCount != 2 || len(usage.Operators) != 2 {
		t.Fatalf("senior usage = %+v, want both known operators", usage)
	}
	if usage.Operators[0].Name != "operator-a" || usage.Operators[1].Name != "运营乙" {
		t.Fatalf("operator names = %+v, want the directory names", usage.Operators)
	}

	items, err = svc.ListMaterials(service.PublicUser{ID: 1, Role: service.RoleAdmin, Status: service.UserStatusEnabled}, "")
	if err != nil {
		t.Fatalf("ListMaterials() error = %v", err)
	}
	if items[0].Usage == nil || len(items[0].Usage.Operators) != 2 || items[0].Usage.Operators[1].Name != "运营乙" {
		t.Fatalf("admin usage = %+v, want the same directory names", items[0].Usage)
	}
}

// A material nobody has produced from yet still carries a summary — zero
// counts — so the page can render the third column without guessing from an
// absent field.
func TestListMaterialsAnswersAnUnusedMaterialWithZeroCounts(t *testing.T) {
	team := service.TeamID(7)
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team}}
	svc := testService(store)
	items, err := svc.ListMaterials(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team}, "")
	if err != nil {
		t.Fatalf("ListMaterials() error = %v", err)
	}
	item := items[0]
	if item.MyUsage != model.MaterialMyUsageUnused || item.UsageStatus != "" || item.AddedAt != nil {
		t.Fatalf("unused projection = %+v, want unused and no relation", item)
	}
	if item.Usage == nil || item.Usage.OutputCount != 0 || item.Usage.OperatorCount != 0 || len(item.Usage.Operators) != 0 {
		t.Fatalf("Usage = %+v, want an all-zero summary", item.Usage)
	}
}

func ptrInt64(value int64) *int64 { return &value }

func usageIDValue(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
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

// The download lifecycle is one user's view of one material, derived from the
// newest user_download task. pending/running and failed/cancelled each collapse
// into one bucket (the user's rulings: "排队到传输都算下载中", "准备失败和本地
// 下载失败都属于下载失败"); any other value — or none — leaves the field empty
// so the UI falls back to video_status.
func TestDownloadStatusOfMapsRawTaskStatusesToTheUserFacingBuckets(t *testing.T) {
	for raw, want := range map[string]string{
		"pending":   "downloading",
		"running":   "downloading",
		"success":   "downloaded",
		"failed":    "failed",
		"cancelled": "failed",
		"":          "",
		"mystery":   "",
	} {
		if got := downloadStatusOf(raw); got != want {
			t.Fatalf("downloadStatusOf(%q) = %q, want %q", raw, got, want)
		}
	}
}

// ListMyMaterials asks for the newest user_download status per visible material
// in one batch and writes the derived state onto each usage, leaving the
// material's own cloud-side video_status untouched.
func TestListMyMaterialsFillsTheDerivedDownloadStatusPerUsage(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	material42 := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady}
	material30 := model.Material{ID: 30, TeamID: team, GameID: &game, VideoStatus: model.VideoReady}
	store := &memoryStore{
		found:    true,
		material: material42,
		// the second FindMaterial call answers the second usage's material.
		refind: &material30,
		usages: []model.MaterialUsage{
			{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive},
			{ID: 6, TeamID: team, MaterialID: 30, UserID: 9, Status: model.MaterialUsageActive},
		},
	}
	transfers := &stubTransfers{downloadStatuses: map[int64]string{42: "success", 30: "pending"}}
	svc := NewService(store, transfers, &stubLinks{})
	items, err := svc.ListMyMaterials(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}})
	if err != nil {
		t.Fatalf("ListMyMaterials() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].DownloadStatus != "downloaded" || items[1].DownloadStatus != "downloading" {
		t.Fatalf("DownloadStatus = %q / %q, want downloaded / downloading", items[0].DownloadStatus, items[1].DownloadStatus)
	}
	if items[0].Material.VideoStatus != model.VideoReady || items[1].Material.VideoStatus != model.VideoReady {
		t.Fatalf("material video_status was mutated, want it untouched")
	}
	if !slices.Equal(transfers.lastStatusQuery, []int64{42, 30}) {
		t.Fatalf("LatestUserDownloadStatuses was asked for %v, want [42 30]", transfers.lastStatusQuery)
	}
}

// Both lists identify a material by its cover and its id (CHG-20260930-069), so
// the two source links have to survive the service untouched. The service has
// no reason to drop them today, and that is exactly why the check is here: a
// future field whitelist or response DTO introduced between the store and the
// route would otherwise strip them silently, and both pages would fall back to
// the placeholder cover with nothing failing.
func TestBothListsCarryTheSourceRowLinksWithoutFiltering(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	material := model.Material{ID: 42, TeamID: team, GameID: &game, CoverURL: "https://cover/42.jpg", AuthorHomeURL: "https://author/42"}
	operator := service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}
	store := &memoryStore{found: true, material: material, usages: []model.MaterialUsage{{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive}}}
	svc := testService(store)

	items, err := svc.ListMaterials(operator, "")
	if err != nil {
		t.Fatalf("ListMaterials() error = %v", err)
	}
	if len(items) != 1 || items[0].CoverURL != material.CoverURL || items[0].AuthorHomeURL != material.AuthorHomeURL {
		t.Fatalf("the library list returned %+v, want the source links carried through", items)
	}

	usages, err := svc.ListMyMaterials(operator)
	if err != nil {
		t.Fatalf("ListMyMaterials() error = %v", err)
	}
	if len(usages) != 1 || usages[0].Material == nil || usages[0].Material.CoverURL != material.CoverURL || usages[0].Material.AuthorHomeURL != material.AuthorHomeURL {
		t.Fatalf("the my-materials list returned %+v, want the source links carried through", usages)
	}
}

// The video address is handed out on a stricter contract than the material
// body: the actor's scope is checked first (an out-of-scope material must not
// be told where the bucket keeps its video — that is a 404/403, not a 409), and
// only a `ready` material has an address at all. The not-ready refusal reuses
// `ErrMaterialUnavailable`, which the route already maps to the 409 the drawer
// reads as "try again once preparation finishes".
func TestVideoURLChecksScopeThenReadinessBeforeComposingTheAddress(t *testing.T) {
	ctx := context.Background()
	team := service.TeamID(7)
	game := "game-a"
	size := int64(4096)
	ready := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady, SourceObjectKey: "materials/42/x.mp4", VideoSizeBytes: &size, VideoSHA256: "0f343b0931126a20f133d67c2b018a3b"}
	operator := service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}

	// Out of scope: the material's team is the boundary, and this actor is in
	// another one.
	otherTeam := service.TeamID(8)
	store := &memoryStore{found: true, material: ready}
	svc := testService(store)
	if _, err := svc.VideoURL(ctx, service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &otherTeam, GameIDs: []string{"game-a"}}, 42); !errors.Is(err, ErrForbidden) {
		t.Fatalf("out-of-scope VideoURL() error = %v, want ErrForbidden", err)
	}

	// Missing: same answer the single-material read gives.
	store = &memoryStore{found: false}
	svc = testService(store)
	if _, err := svc.VideoURL(ctx, operator, 42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing VideoURL() error = %v, want ErrNotFound", err)
	}

	// Visible but not ready: 409, not an address and not a 404 — the material
	// exists and the answer changes on its own once preparation finishes.
	for _, status := range []model.VideoStatus{model.VideoNotDownloaded, model.VideoDownloading, model.VideoFailed} {
		unready := ready
		unready.VideoStatus = status
		store = &memoryStore{found: true, material: unready}
		svc = testService(store)
		if _, err := svc.VideoURL(ctx, operator, 42); !errors.Is(err, ErrMaterialUnavailable) {
			t.Fatalf("%s VideoURL() error = %v, want ErrMaterialUnavailable", status, err)
		}
	}

	// Ready: the address is the linker's answer for the material's own object
	// key, and nothing wider.
	links := &stubLinks{url: "http://127.0.0.1:9000/wt-media/dev/materials/42/x.mp4?sig=stub"}
	store = &memoryStore{found: true, material: ready}
	svc = NewService(store, &stubTransfers{}, links)
	url, err := svc.VideoURL(ctx, operator, 42)
	if err != nil {
		t.Fatalf("ready VideoURL() error = %v", err)
	}
	if url != links.url {
		t.Fatalf("VideoURL() = %q, want the signed address %q", url, links.url)
	}
	if links.key != ready.SourceObjectKey {
		t.Fatalf("the address was signed for key %q, want the material's own %q", links.key, ready.SourceObjectKey)
	}
}

// The issuer the wired today signs locally, so it cannot fail on a context —
// which means a `VideoURL` that quietly passed `context.Background()` instead
// of the caller's context would still satisfy every other arm in this file. The
// arm exists to be the one that goes red if it does: a request that is cancelled
// between the store read and the signature must reach the signer cancelled.
func TestVideoURLHandsTheCallersContextToTheLinker(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	size := int64(4096)
	ready := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady, SourceObjectKey: "materials/42/x.mp4", VideoSizeBytes: &size, VideoSHA256: "0f343b0931126a20f133d67c2b018a3b"}
	links := &stubLinks{url: "http://unused"}
	svc := NewService(&memoryStore{found: true, material: ready}, &stubTransfers{}, links)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.VideoURL(ctx, service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42); err != nil {
		t.Fatalf("VideoURL() error = %v", err)
	}
	if links.ctx == nil {
		t.Fatal("the linker was called with no context at all")
	}
	if links.ctx.Err() == nil {
		t.Fatal("the linker was called with a live context, want the caller's cancelled one")
	}
}

// A `ready` row missing part of its facts is this module's own inconsistency,
// and the address must not be an exception to the promise the projection makes:
// the fault surfaces as an error, not as a URL composed from a half-empty key.
func TestVideoURLReportsAReadyRowWithIncompleteFactsAsAFault(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	incomplete := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady, SourceObjectKey: "materials/42/x.mp4"}
	store := &memoryStore{found: true, material: incomplete}
	links := &stubLinks{url: "http://unused"}
	svc := NewService(store, &stubTransfers{}, links)
	if _, err := svc.VideoURL(context.Background(), service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42); err == nil {
		t.Fatal("VideoURL() succeeded for a ready row with no size or hash, want a fault")
	}
	if links.key != "" {
		t.Fatalf("a key was handed to the linker despite the incomplete facts: %q", links.key)
	}
}

// The linker is the storage boundary; its refusal is not this module's to
// re-classify, it is the route's 500 with the reason kept in the log. The
// refusal this route actually meets in production is an unconfigured or
// unreachable object store, so the arm keeps the issuer's own message intact
// rather than replacing it with a generic one.
func TestVideoURLPropagatesTheLinkerRefusal(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	size := int64(4096)
	ready := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady, SourceObjectKey: "materials/42/x.mp4", VideoSizeBytes: &size, VideoSHA256: "0f343b0931126a20f133d67c2b018a3b"}
	store := &memoryStore{found: true, material: ready}
	svc := NewService(store, &stubTransfers{}, &stubLinks{err: errors.New("minting refused")})
	if _, err := svc.VideoURL(context.Background(), service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42); err == nil || !strings.Contains(err.Error(), "minting refused") {
		t.Fatalf("VideoURL() error = %v, want the linker's refusal to propagate", err)
	}
}

// `ready` says the row was written with all its facts, not that the bucket still
// holds the object. An address signed for a gone object opens a 404 in the
// drawer, so the same 409 the download route gives is the answer here.
func TestVideoURLRefusesAReadyMaterialWhoseObjectIsGone(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	size := int64(4096)
	ready := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady, SourceObjectKey: "materials/42/x.mp4", VideoSizeBytes: &size, VideoSHA256: "0f343b0931126a20f133d67c2b018a3b"}
	links := &stubLinks{url: "http://unused", missing: map[string]bool{ready.SourceObjectKey: true}}
	svc := NewService(&memoryStore{found: true, material: ready}, &stubTransfers{}, links)
	if _, err := svc.VideoURL(context.Background(), service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42); !errors.Is(err, ErrMaterialUnavailable) {
		t.Fatalf("VideoURL() error = %v, want ErrMaterialUnavailable", err)
	}
	if links.key != "" {
		t.Fatalf("an address was signed for a missing object: %q", links.key)
	}
}

// A bucket that cannot answer is a fault, not "missing": the drawer must not be
// told the video is temporarily unavailable because the storage was unreachable,
// and no address may be signed on an unknown answer.
func TestVideoURLReportsABucketCheckFailure(t *testing.T) {
	team := service.TeamID(7)
	game := "game-a"
	size := int64(4096)
	ready := model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoReady, SourceObjectKey: "materials/42/x.mp4", VideoSizeBytes: &size, VideoSHA256: "0f343b0931126a20f133d67c2b018a3b"}
	links := &stubLinks{url: "http://unused", existsErr: errors.New("storage unreachable")}
	svc := NewService(&memoryStore{found: true, material: ready}, &stubTransfers{}, links)
	_, err := svc.VideoURL(context.Background(), service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team, GameIDs: []string{"game-a"}}, 42)
	if err == nil || !strings.Contains(err.Error(), "storage unreachable") {
		t.Fatalf("VideoURL() error = %v, want the bucket failure to propagate", err)
	}
	if errors.Is(err, ErrMaterialUnavailable) {
		t.Fatal("unreachable storage must not be reported as an unavailable video")
	}
	if links.key != "" {
		t.Fatalf("an address was signed on an unknown bucket answer: %q", links.key)
	}
}

func TestRemoveUsageChecksTheMaterialScopeBeforeSoftRemoval(t *testing.T) {
	team := service.TeamID(7)
	otherTeam := service.TeamID(8)
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team}, usages: []model.MaterialUsage{usage}}
	svc := testService(store)
	err := svc.RemoveUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &otherTeam}, 5)
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

// The page shows a given-up relation as a row of its own, with the state and the
// moment it was given up, and offers 恢复使用 on it (CHG-20260930-069 task 15).
// None of that is reachable while the list drops every row that is not `active`,
// so the removed row has to survive the whole way through the service — the
// relation's own state and timestamp included, not just the material it embeds.
//
// Both rows are asked for in one query about the acting user. The scope is
// asserted on the argument, because a list read for the wrong user returns rows
// that look perfectly ordinary.
func TestListMyMaterialsKeepsTheGivenUpRelationWithItsOwnState(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	givenUpAt := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	store := &memoryStore{
		found:    true,
		material: model.Material{ID: 42, TeamID: team, GameID: &game},
		usages: []model.MaterialUsage{
			{ID: 6, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageRemoved, RemovedAt: &givenUpAt},
			{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive},
		},
	}
	items, err := testService(store).ListMyMaterials(scopedActor(team, game))
	if err != nil {
		t.Fatalf("ListMyMaterials() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want the given-up relation kept alongside the one in use", items)
	}
	if items[0].Status != model.MaterialUsageRemoved || items[0].RemovedAt == nil || !items[0].RemovedAt.Equal(givenUpAt) {
		t.Fatalf("the given-up relation lost its own state: %+v", items[0])
	}
	if items[0].Material == nil || items[0].Material.ID != 42 {
		t.Fatalf("a given-up relation is still a relation to a material: %+v", items[0])
	}
	if items[1].Status != model.MaterialUsageActive {
		t.Fatalf("the relation in use came back as %q", items[1].Status)
	}
	if store.listedFor != 9 {
		t.Fatalf("the list was read for user %d, want the acting user 9", store.listedFor)
	}
}

func TestRestoreUsageChecksTheMaterialScopeBeforeRestoration(t *testing.T) {
	team := service.TeamID(7)
	otherTeam := service.TeamID(8)
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageRemoved}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team}, usages: []model.MaterialUsage{usage}}
	err := testService(store).RestoreUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &otherTeam}, 5)
	if !errors.Is(err, ErrForbidden) || store.restored != 0 {
		t.Fatalf("err=%v restored=%d", err, store.restored)
	}
}

// The same split the removal already keeps: "this row is not yours" and "this
// material does not exist" are published as different errcodes, so they have to
// stay different sentinels here too.
func TestRestoreUsageReportsAMissingUsageSeparatelyFromAMissingMaterial(t *testing.T) {
	team := service.TeamID(7)
	store := &memoryStore{found: false}
	err := testService(store).RestoreUsage(service.PublicUser{ID: 9, Role: service.RoleOperator, Status: service.UserStatusEnabled, TeamID: &team}, 5)
	if !errors.Is(err, ErrUsageNotFound) {
		t.Fatalf("RestoreUsage() error = %v, want ErrUsageNotFound", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing usage must not be reported as a missing material: %v", err)
	}
	if store.restored != 0 {
		t.Fatalf("nothing may be restored when the usage is not the actor's: %d", store.restored)
	}
}

// Scoped to the acting user for the same reason the removal is: one operator must
// not be able to put another operator's given-up row back into use. The ID alone
// would pass even if the user never reached the store.
func TestRestoreUsageScopesTheRestorationToTheActingUser(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageRemoved}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, usages: []model.MaterialUsage{usage}}
	if err := testService(store).RestoreUsage(scopedActor(team, game), 5); err != nil {
		t.Fatalf("RestoreUsage() error = %v", err)
	}
	if store.restored != 5 || store.restoredFor != 9 {
		t.Fatalf("restored=%d restoredFor=%d, want 5 and 9", store.restored, store.restoredFor)
	}
}

// Restoring a relation that is already in use is not a refusal. The click asks
// for "this is in use again", which is already true — and the second click of a
// double-click arrives before the list has reloaded, so answering it with an
// error would put a failure message on a successful action.
//
// This does not pin *where* that is decided: the service may answer from the row
// it already read, and the conditional `UPDATE` is the backstop for the race the
// read cannot see. Both are the same answer to the user, so the assertion is that
// answer rather than which of the two produced it.
func TestRestoreUsageTreatsARelationAlreadyInUseAsDone(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	usage := model.MaterialUsage{ID: 5, TeamID: team, MaterialID: 42, UserID: 9, Status: model.MaterialUsageActive}
	store := &memoryStore{found: true, material: model.Material{ID: 42, TeamID: team, GameID: &game}, usages: []model.MaterialUsage{usage}}
	if err := testService(store).RestoreUsage(scopedActor(team, game), 5); err != nil {
		t.Fatalf("RestoreUsage() error = %v, want the relation it was asked to reach", err)
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

// stubResolveGame stands in for the identity store call `CreateDownload` makes to
// name the game a download is filed under; the real resolver hits the global DB,
// which no unit test here has.
var stubResolveGame = func(gameID string) (string, bool, error) {
	return "三角洲行动", true, nil
}

// A download belongs to the user who clicked, not to the machine that happens to
// be polling: any machine that user logs in on may claim the task, so the only
// identity the route puts on it is the actor.
func TestCreateDownloadAddressesTheTaskToTheActor(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := readyMaterial(team, game)
	store := &memoryStore{found: true, material: material}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame
	now := time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	task, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.count != 1 {
		t.Fatalf("transfer calls = %d, want 1", transfers.count)
	}
	got := transfers.last
	if got.RequestedBy != 9 || got.TeamID != team || got.AssetID != 42 {
		t.Fatalf("transfer input = %+v", got)
	}
	// The executor runs later, on another machine, and may not read `materials`:
	// everything it needs to name, size and verify the file has to be on the task.
	if got.AssetTitle != "示例视频" || got.GameName != "三角洲行动" || got.SourceObjectKey != "materials/42/aaaaaaaa.mp4" || got.TotalBytes != 2048 || got.ExpectedSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("the material facts must travel with the task: %+v", got)
	}
	if task.ID != "transfer-1" {
		t.Fatalf("task = %+v, want the transfer module's answer", task)
	}
}

// The library is team-public, but the original video is personal: downloading
// it is an action from "My Materials" (§5.3.4). A direct API call by a viewer
// with no active relation must fail closed, before anything is queued.
func TestCreateDownloadRequiresTheActorsActiveUsage(t *testing.T) {
	team, game := service.TeamID(7), "game-a"

	store := &memoryStore{found: true, material: readyMaterial(team, game), usageMissing: true}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame
	if _, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42); !errors.Is(err, ErrUsageForbidden) {
		t.Fatalf("CreateDownload() error = %v, want ErrUsageForbidden", err)
	}
	if transfers.count != 0 || transfers.prepareCount != 0 || store.preparedCount != 0 {
		t.Fatalf("a refused download queued work: downloads=%d preparations=%d projection=%d", transfers.count, transfers.prepareCount, store.preparedCount)
	}
	if !slices.Equal(store.usageChecks, []int64{42}) {
		t.Fatalf("usage checks = %v, want the material asked about", store.usageChecks)
	}

	// The allowed arm is what makes the refusal meaningful: the same viewer with
	// an active relation gets the task, and the relation was actually consulted.
	store = &memoryStore{found: true, material: readyMaterial(team, game)}
	transfers = &stubTransfers{}
	svc = NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame
	if _, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42); err != nil {
		t.Fatalf("CreateDownload() error = %v, want the relation to allow it", err)
	}
	if transfers.count != 1 || !slices.Equal(store.usageChecks, []int64{42}) {
		t.Fatalf("downloads=%d usage checks=%v, want one download after the relation check", transfers.count, store.usageChecks)
	}
}

// The refusals, each asserted separately because they are different things to
// tell the user: not yours to see, not ready yet, not there at all. Collapsing
// any two of them would answer 404 or 500 where the frozen contract publishes a
// 409.
func TestCreateDownloadRefusesBeforeItQueuesAnything(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	actor := scopedActor(team, game)

	for _, testCase := range []struct {
		name        string
		material    model.Material
		found       bool
		transferErr error
		want        error
	}{
		{
			name:        "the transfer module rejects the input",
			material:    readyMaterial(team, game),
			found:       true,
			transferErr: transferservice.ErrInvalidInput,
			want:        ErrInvalidInput,
		},
		{
			name:     "outside the actor's team",
			material: func() model.Material { m := readyMaterial(service.TeamID(8), "game-b"); return m }(),
			found:    true,
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
			want:  ErrMaterialUnavailable,
		},
		{
			name:     "material does not exist",
			material: model.Material{},
			found:    false,
			want:     ErrNotFound,
		},
	} {
		store := &memoryStore{found: testCase.found, material: testCase.material}
		transfers := &stubTransfers{err: testCase.transferErr}
		svc := NewService(store, transfers, &stubLinks{})
		svc.resolveGame = stubResolveGame

		if _, err := svc.CreateDownload(context.Background(), actor, 42); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: error = %v, want %v", testCase.name, err, testCase.want)
		}
		// The one arm that legitimately reaches the transfer module is the one
		// where that module itself refused; every other arm must stop before it.
		if testCase.transferErr == nil && transfers.count != 0 {
			t.Fatalf("%s: refused download still queued %d task(s)", testCase.name, transfers.count)
		}
		// A refusal is a refusal of the whole click: no preparation may be left
		// behind for a request the user was told did not happen, and the projection
		// must not read `downloading` with nothing on its way.
		if transfers.prepareCount != 0 || store.preparedCount != 0 {
			t.Fatalf("%s: refused download still queued a preparation (%d) or moved the projection (%d)", testCase.name, transfers.prepareCount, store.preparedCount)
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
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	_, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err == nil {
		t.Fatal("a ready material with no hash must not be queued for download")
	}
	if errors.Is(err, ErrMaterialUnavailable) {
		t.Fatalf("error = %v, want an internal fault rather than a conflict the user is told to retry", err)
	}
	if transfers.count != 0 {
		t.Fatal("an inconsistent row must not reach the transfer module")
	}
}

func pendingMaterial(team service.TeamID, game string) model.Material {
	material := readyMaterial(team, game)
	material.VideoStatus = model.VideoNotDownloaded
	material.SourceObjectKey = ""
	material.VideoSizeBytes = nil
	material.VideoSHA256 = ""
	material.SourceURL = "https://example.invalid/video/42"
	return material
}

// The one-click rule: a material with no prepared video is not refused, it is
// queued — and what is queued is the *same kind* of task the prepared case gets,
// held back by the preparation it names. The user sees one download either way.
//
// The facts asserted as empty are the point of `dependency_task_id`: a task that
// carried a placeholder hash or a zero size would be a task an executor could
// claim and then fail to verify against, which is worse than one it cannot claim.
func TestCreateDownloadQueuesAWaitingTaskForAMaterialWithNoPreparedVideo(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	store := &memoryStore{found: true, material: pendingMaterial(team, game), preparing: true}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	task, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.prepareCount != 1 || transfers.count != 1 {
		t.Fatalf("prepare calls = %d, download calls = %d; the click is one upload of each", transfers.prepareCount, transfers.count)
	}
	got := transfers.prepare
	if got.TeamID != team || got.AssetID != 42 || got.RequestedBy != 9 || got.AssetTitle != "示例视频" {
		t.Fatalf("prepare input = %+v", got)
	}
	// The preparation is keyed on the material, not on the user who happened to
	// click: nothing about this request may reach the shared task.
	if transfers.last.DependencyTaskID != "transfer-prepare-1" {
		t.Fatalf("dependency = %q, want the preparation the transfer module answered with", transfers.last.DependencyTaskID)
	}
	if transfers.last.SourceObjectKey != "" || transfers.last.TotalBytes != 0 || transfers.last.ExpectedSHA256 != "" {
		t.Fatalf("a waiting download must carry no facts of its own: %+v", transfers.last)
	}
	if transfers.last.RequestedBy != 9 {
		t.Fatalf("a waiting download still belongs to the user who clicked: %+v", transfers.last)
	}
	if store.preparedCount != 1 || store.preparedTeam != team || store.preparedFor != 42 {
		t.Fatalf("the projection must move with the task: count=%d team=%d material=%d", store.preparedCount, store.preparedTeam, store.preparedFor)
	}
	if task.ID != "transfer-1" {
		t.Fatalf("task = %+v, want the transfer module's answer", task)
	}
}

// A prepared material queues no preparation. Without this arm the waiting path
// could be taken for every click and the tests above would still pass, having
// only ever exercised one branch.
func TestCreateDownloadQueuesNoPreparationForAPreparedMaterial(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	store := &memoryStore{found: true, material: readyMaterial(team, game), preparing: true}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	if _, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42); err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.prepareCount != 0 {
		t.Fatalf("a prepared material queued %d preparation(s)", transfers.prepareCount)
	}
	if transfers.last.DependencyTaskID != "" {
		t.Fatalf("dependency = %q, want none for a material that is already prepared", transfers.last.DependencyTaskID)
	}
	if store.preparedCount != 0 {
		t.Fatalf("the projection of a ready material was moved %d time(s)", store.preparedCount)
	}
}

// The guarded write can refuse, and the two reasons need different tasks. This is
// the arm where guessing wrong strands a download: a video that became ready
// between the read and the write must be fetched now, not held back behind a
// preparation nobody needs any more.
func TestCreateDownloadFetchesDirectlyWhenTheVideoBecameReadyMidRequest(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	fresh := readyMaterial(team, game)
	store := &memoryStore{found: true, material: pendingMaterial(team, game), preparing: false, refind: &fresh}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	if _, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42); err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if store.findCount != 2 {
		t.Fatalf("material lookups = %d, want a re-read after the refused projection write", store.findCount)
	}
	if transfers.last.DependencyTaskID != "" {
		t.Fatalf("dependency = %q, want none: the video is ready now", transfers.last.DependencyTaskID)
	}
	if transfers.last.SourceObjectKey != "materials/42/aaaaaaaa.mp4" || transfers.last.TotalBytes != 2048 || transfers.last.ExpectedSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("the facts must come from the fresh row: %+v", transfers.last)
	}
}

// The other reason the guarded write refuses: a preparation is already running
// for this material, because another operator clicked first. That is the case the
// wait was designed for, and it must still wait — not fall through to a direct
// download with empty facts, which the transfer module would refuse.
func TestCreateDownloadStillWaitsWhenAPreparationIsAlreadyRunning(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	stillPending := pendingMaterial(team, game)
	stillPending.VideoStatus = model.VideoDownloading
	store := &memoryStore{found: true, material: pendingMaterial(team, game), preparing: false, refind: &stillPending}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	task, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.last.DependencyTaskID != "transfer-prepare-1" {
		t.Fatalf("dependency = %q, want the outstanding preparation", transfers.last.DependencyTaskID)
	}
	if task.ID != "transfer-1" {
		t.Fatalf("task = %+v", task)
	}
}

// A ready row whose object is gone is not downloaded and not refused: the same
// click reprepares the material and queues the download behind that preparation.
// The take-back is the key-and-timestamp compare, so the exact pair the row was
// read with is what the assertion pins — a service that passed a zero timestamp
// would still move the row in the fake and fail in production.
func TestCreateDownloadRepreparesAReadyMaterialWhoseObjectIsGone(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	preparedAt := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	material := readyMaterial(team, game)
	material.SourceURL = "https://www.douyin.com/video/42"
	material.VideoPreparedAt = &preparedAt
	store := &memoryStore{found: true, material: material, repreparing: true}
	links := &stubLinks{missing: map[string]bool{material.SourceObjectKey: true}}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, links)
	svc.resolveGame = stubResolveGame

	task, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.prepareCount != 1 || transfers.count != 1 {
		t.Fatalf("prepare calls = %d, download calls = %d", transfers.prepareCount, transfers.count)
	}
	if store.repreparedCount != 1 || store.repreparedTeam != team || store.repreparedFor != 42 {
		t.Fatalf("take-back count=%d team=%d material=%d", store.repreparedCount, store.repreparedTeam, store.repreparedFor)
	}
	if store.repreparedKey != material.SourceObjectKey || store.repreparedAt == nil || !store.repreparedAt.Equal(preparedAt) {
		t.Fatalf("take-back scope = key %q prepared_at %v, want %q / %v", store.repreparedKey, store.repreparedAt, material.SourceObjectKey, preparedAt)
	}
	if store.preparedCount != 0 {
		t.Fatalf("a ready row must be taken back by the compare, not the ordinary marker: %d", store.preparedCount)
	}
	if transfers.last.DependencyTaskID != "transfer-prepare-1" {
		t.Fatalf("dependency = %q, want the preparation", transfers.last.DependencyTaskID)
	}
	if transfers.last.SourceObjectKey != "" || transfers.last.TotalBytes != 0 || transfers.last.ExpectedSHA256 != "" {
		t.Fatalf("a waiting download must carry no facts of its own: %+v", transfers.last)
	}
	if task.ID != "transfer-1" {
		t.Fatalf("task = %+v", task)
	}
}

// The take-back can refuse because a preparation finished between the read and
// the compare. The fresh row is the only source that can tell that apart from
// "another preparation is still running", and guessing wrong strands a download
// behind a preparation nobody needs any more.
func TestCreateDownloadReReadsAfterTheTakeBackWasRefused(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	gone := readyMaterial(team, game)
	gone.SourceURL = "https://www.douyin.com/video/42"
	fresh := readyMaterial(team, game)
	fresh.SourceObjectKey = "materials/42/bbbbbbbb.mp4"

	for _, tc := range []struct {
		name      string
		refreshed model.Material
		missing   map[string]bool
		want      error
		wantFacts bool
	}{
		{"ready again with the object present", fresh, nil, nil, true},
		{"ready again but the object is still gone", fresh, map[string]bool{fresh.SourceObjectKey: true}, ErrMaterialUnavailable, false},
		{"still being prepared", model.Material{ID: 42, TeamID: team, GameID: &game, VideoStatus: model.VideoDownloading, SourceURL: gone.SourceURL}, nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refreshed := tc.refreshed
			store := &memoryStore{found: true, material: gone, repreparing: false, refind: &refreshed}
			links := &stubLinks{missing: map[string]bool{gone.SourceObjectKey: true}}
			for key := range tc.missing {
				links.missing[key] = true
			}
			transfers := &stubTransfers{}
			svc := NewService(store, transfers, links)
			svc.resolveGame = stubResolveGame

			task, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("error = %v, want %v", err, tc.want)
				}
				if transfers.count != 0 {
					t.Fatalf("a refused click queued %d task(s)", transfers.count)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateDownload() error = %v", err)
			}
			if store.findCount != 2 {
				t.Fatalf("material lookups = %d, want a re-read after the refused take-back", store.findCount)
			}
			if !tc.wantFacts {
				if transfers.last.DependencyTaskID != "transfer-prepare-1" {
					t.Fatalf("dependency = %q, want the outstanding preparation", transfers.last.DependencyTaskID)
				}
				if task.ID != "transfer-1" {
					t.Fatalf("task = %+v", task)
				}
				return
			}
			if transfers.last.DependencyTaskID != "" {
				t.Fatalf("dependency = %q, want none: the fresh row is ready", transfers.last.DependencyTaskID)
			}
			if transfers.last.SourceObjectKey != fresh.SourceObjectKey || transfers.last.TotalBytes != 2048 || transfers.last.ExpectedSHA256 != fresh.VideoSHA256 {
				t.Fatalf("the facts must come from the fresh row: %+v", transfers.last)
			}
		})
	}
}

// A bucket that cannot answer is not "the object is missing": treating an
// outage as a missing object would queue a preparation for a material that is
// fine, and the route would report a download in progress that nobody needs.
func TestCreateDownloadReportsABucketCheckFailure(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	store := &memoryStore{found: true, material: readyMaterial(team, game)}
	links := &stubLinks{existsErr: errors.New("storage unreachable")}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, links)
	svc.resolveGame = stubResolveGame

	_, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err == nil || !strings.Contains(err.Error(), "storage unreachable") {
		t.Fatalf("error = %v, want the bucket failure to propagate", err)
	}
	if transfers.prepareCount != 0 || transfers.count != 0 || store.repreparedCount != 0 {
		t.Fatalf("a bucket failure must queue nothing: prepare=%d download=%d take-back=%d", transfers.prepareCount, transfers.count, store.repreparedCount)
	}
}

// Ready, object gone, and no page address: there is nothing to re-resolve, so
// the only honest answer is the 409 the drawer already reads as "no prepared
// video right now". A preparation task here could only fail.
func TestCreateDownloadRefusesAReadyMaterialWhoseObjectIsGoneAndHasNoSource(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := readyMaterial(team, game)
	store := &memoryStore{found: true, material: material}
	links := &stubLinks{missing: map[string]bool{material.SourceObjectKey: true}}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, links)
	svc.resolveGame = stubResolveGame

	if _, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42); !errors.Is(err, ErrMaterialUnavailable) {
		t.Fatalf("error = %v, want ErrMaterialUnavailable", err)
	}
	if transfers.prepareCount != 0 || transfers.count != 0 {
		t.Fatalf("nothing may be queued: prepare=%d download=%d", transfers.prepareCount, transfers.count)
	}
}

// The preparation may finish between `Ensure` and the insert; the live hand-over
// then had no row to update. The route attaches afterwards, and when that works
// the client must receive the attached body — the one that now carries the
// object key — rather than the pending row it would otherwise be stuck with.
func TestCreateDownloadReturnsTheAttachedTaskBody(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	store := &memoryStore{found: true, material: pendingMaterial(team, game), preparing: true}
	transfers := &stubTransfers{attachOK: true, attachTask: transferdto.Task{ID: "transfer-1", Status: "pending", TotalBytes: 2048}}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	task, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.attachCount != 1 || len(transfers.attachIDs) != 1 || transfers.attachIDs[0] != "transfer-1" {
		t.Fatalf("attach calls = %v, want the created task", transfers.attachIDs)
	}
	if task.TotalBytes != 2048 {
		t.Fatalf("task = %+v, want the attached body", task)
	}
}

// A direct download queues nothing, so there is no dependency to attach after
// it. Without this arm the attach could be called unconditionally and every
// other test would still pass.
func TestCreateDownloadDoesNotAttachWithoutADependency(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	store := &memoryStore{found: true, material: readyMaterial(team, game)}
	transfers := &stubTransfers{attachOK: true}
	svc := NewService(store, transfers, &stubLinks{})
	svc.resolveGame = stubResolveGame

	if _, err := svc.CreateDownload(context.Background(), scopedActor(team, game), 42); err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if transfers.attachCount != 0 {
		t.Fatalf("a direct download attached %d dependency(ies)", transfers.attachCount)
	}
}

// verifiedVideoFacts is what a worker hands the service once its download has been
// verified. It is built here rather than reused from the repository's tests, which
// are a different package: this is the shape the service promises to pass on.
func verifiedVideoFacts() repository.VideoFacts {
	return repository.VideoFacts{
		ObjectKey: "materials/42/8f14e45fceea167a5a36dedd4bea2543a1b2c3d4e5f60718293a4b5c6d7e8f90.mp4",
		SizeBytes: 1048576,
		SHA256:    "8f14e45fceea167a5a36dedd4bea2543a1b2c3d4e5f60718293a4b5c6d7e8f90",
		Media:     []byte(`{"container":"mp4"}`),
	}
}

// The facts reach the store unchanged, and the write is scoped by the team from
// the task row rather than by any actor — the worker has no session to read one
// from. A service that dropped the team and wrote by material id alone would
// prepare another team's material with this team's task.
func TestMarkVideoReadyWritesTheFactsItWasGivenUnderTheTasksTeam(t *testing.T) {
	store := &memoryStore{ready: true}
	svc := testService(store)
	now := time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	facts := verifiedVideoFacts()

	if err := svc.MarkVideoReady(sharedidentity.TeamID(7), 42, facts); err != nil {
		t.Fatalf("MarkVideoReady() error = %v", err)
	}
	if store.readyTeam != sharedidentity.TeamID(7) || store.readyFor != 42 {
		t.Fatalf("scope = team %d material %d, want team 7 material 42", store.readyTeam, store.readyFor)
	}
	if store.readyFacts.ObjectKey != facts.ObjectKey || store.readyFacts.SHA256 != facts.SHA256 || store.readyFacts.SizeBytes != facts.SizeBytes || string(store.readyFacts.Media) != string(facts.Media) {
		t.Fatalf("facts = %+v, want them passed on unchanged", store.readyFacts)
	}
	if !store.readyAt.Equal(now) {
		t.Fatalf("written at %v, want the service's clock %v", store.readyAt, now)
	}
}

// A write that changed no row is the material being gone, and it is an error for
// the worker: a task that reported success here would leave a download waiting on a
// video nobody is preparing. The reading is `ErrNotFound` rather than a new
// sentinel because that is what it is — the scope names no material.
func TestMarkVideoReadyReportsAWriteThatChangedNoRowAsAMissingMaterial(t *testing.T) {
	store := &memoryStore{ready: false}
	svc := testService(store)

	err := svc.MarkVideoReady(sharedidentity.TeamID(7), 42, verifiedVideoFacts())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkVideoReady() error = %v, want ErrNotFound", err)
	}
}

func TestMarkVideoReadyPassesOnAStoreFailure(t *testing.T) {
	failure := errors.New("connection reset")
	store := &memoryStore{ready: true, readyErr: failure}
	svc := testService(store)

	err := svc.MarkVideoReady(sharedidentity.TeamID(7), 42, verifiedVideoFacts())
	if !errors.Is(err, failure) {
		t.Fatalf("MarkVideoReady() error = %v, want the store's failure", err)
	}
}

// A refused failure write is not an error, unlike a refused readiness one. The
// statement refuses a material that is already `ready`, and that is the case the
// refusal exists for: a retry failing after a slower earlier attempt succeeded. The
// task still fails; the material keeps its video. A service that turned this into an
// error would make the worker report a fault it cannot act on.
func TestMarkVideoFailedLetsARefusedWritePass(t *testing.T) {
	store := &memoryStore{failed: false}
	svc := testService(store)
	now := time.Date(2026, 9, 26, 11, 30, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if err := svc.MarkVideoFailed(sharedidentity.TeamID(7), 42, "source http status 404"); err != nil {
		t.Fatalf("MarkVideoFailed() error = %v, want nil for a material that is already ready", err)
	}
	if store.failedTeam != sharedidentity.TeamID(7) || store.failedFor != 42 || store.failedMsg != "source http status 404" {
		t.Fatalf("write = team %d material %d message %q", store.failedTeam, store.failedFor, store.failedMsg)
	}
	if !store.failedAt.Equal(now) {
		t.Fatalf("written at %v, want the service's clock %v", store.failedAt, now)
	}
}

func TestMarkVideoFailedPassesOnAStoreFailure(t *testing.T) {
	failure := errors.New("connection reset")
	store := &memoryStore{failed: true, failedErr: failure}
	svc := testService(store)

	if err := svc.MarkVideoFailed(sharedidentity.TeamID(7), 42, "boom"); !errors.Is(err, failure) {
		t.Fatalf("MarkVideoFailed() error = %v, want the store's failure", err)
	}
}

// The worker resolves the source from the platform and the provider's own content
// id, both of which are stable, rather than from the address snapshot, which is a
// signed URL and is expired by the time anything downloads it.
//
// The provider's id and this row's own `source_contents` id are two different
// numbers, and the fixture keeps them different on purpose. A fixture that set
// only one of them could not tell which one the worker is about to ask the
// provider for -- which is exactly how the two came to be confused: the worker
// asked for the local row id and every provider answered "no such video".
func TestPreparationSourceCarriesThePlatformContentTheWorkerResolvesFrom(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := pendingMaterial(team, game)
	material.Platform = "douyin"
	// The local `source_contents` row id, which is *not* an id the provider knows.
	material.SourceContentID = 42
	material.PlatformContentID = "7123456789012345678"
	store := &memoryStore{found: true, material: material}

	source, err := testService(store).PreparationSource(team, 42)
	if err != nil {
		t.Fatalf("PreparationSource() error = %v", err)
	}
	if source.Platform != "douyin" || source.ContentID != 7123456789012345678 {
		t.Fatalf("source = %+v, want the platform and the provider's content id", source)
	}
	if source.ContentID == material.SourceContentID {
		t.Fatal("the provider's content id was taken from the local row id")
	}
	if source.TeamID != team || source.MaterialID != 42 {
		t.Fatalf("source scope = team %d material %d, want team 7 material 42", source.TeamID, source.MaterialID)
	}
}

// A material in another team reads as missing rather than as forbidden. The worker
// is not acting on anyone's behalf and has nothing to be refused *as*; what it is
// being told is that its task does not name a material this team has, which is the
// same answer an id that never existed gets. A distinct "forbidden" here would
// invite a caller to try a different team.
func TestPreparationSourceRefusesAMaterialOutsideTheTasksTeam(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := pendingMaterial(team, game)
	material.Platform = "douyin"
	material.SourceContentID = 7123456789012345678
	store := &memoryStore{found: true, material: material}

	_, err := testService(store).PreparationSource(service.TeamID(8), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("PreparationSource() error = %v, want ErrNotFound", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Fatal("another team's material must not be reported as forbidden to a caller with no actor")
	}
}

// A material with no platform or no content id cannot be re-resolved, and the
// answer says that rather than "not found": the material is there, and the reason a
// preparation cannot run is a property of the row. The worker's failure message is
// the only place an operator sees why, so the two are kept apart.
func TestPreparationSourceRefusesAMaterialWithNothingToResolveFrom(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	for _, testCase := range []struct {
		name      string
		platform  string
		contentID string
	}{
		{"no platform", "", "7123456789012345678"},
		{"no content id", "douyin", ""},
		{"blank content id", "douyin", "   "},
		{"neither", "  ", ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			material := pendingMaterial(team, game)
			material.Platform = testCase.platform
			material.PlatformContentID = testCase.contentID
			// The local row id is present throughout, so a refusal can only come
			// from the platform's own id being absent: the two are not substitutes.
			material.SourceContentID = 42
			store := &memoryStore{found: true, material: material}

			_, err := testService(store).PreparationSource(team, 42)
			if err == nil {
				t.Fatal("error = nil, want a refusal naming what the material lacks")
			}
			if errors.Is(err, ErrNotFound) {
				t.Fatalf("error = %v, want a reason about the row rather than a missing material", err)
			}
			if !strings.Contains(err.Error(), "no platform source") {
				t.Fatalf("error = %v, want it to name what is missing", err)
			}
		})
	}
}

// A platform content id that is not a number cannot be asked for by id, and the
// worker sends the id rather than the short link. Refusing here names the row;
// sending it anyway would spend a provider call to be told the video does not
// exist, which reads the same as a video that really is gone.
func TestPreparationSourceRefusesAPlatformContentIdThatIsNotAnId(t *testing.T) {
	team, game := service.TeamID(7), "game-a"
	material := pendingMaterial(team, game)
	material.Platform = "douyin"
	material.SourceContentID = 42
	material.PlatformContentID = "https://www.douyin.com/video/7123456789012345678"
	store := &memoryStore{found: true, material: material}

	_, err := testService(store).PreparationSource(team, 42)
	if err == nil {
		t.Fatal("error = nil, want a refusal naming the unusable content id")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want a reason about the row rather than a missing material", err)
	}
	if !strings.Contains(err.Error(), "no platform source") {
		t.Fatalf("error = %v, want it to name what is missing", err)
	}
}

func TestPreparationSourceRefusesAnIncompleteScope(t *testing.T) {
	store := &memoryStore{found: true, material: pendingMaterial(service.TeamID(7), "game-a")}
	for _, testCase := range []struct {
		name       string
		teamID     service.TeamID
		materialID int64
	}{
		{"no team", 0, 42},
		{"no material", 7, 0},
	} {
		if _, err := testService(store).PreparationSource(testCase.teamID, testCase.materialID); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("PreparationSource %s error = %v, want ErrInvalidInput", testCase.name, err)
		}
	}
}
