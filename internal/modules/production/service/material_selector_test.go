package service

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"testing"
)

func TestMaterialSelectorPinsOwnerTeamCategoryButUsageViewer(t *testing.T) {
	store := &memoryStore{}
	svc := NewService(store, nil, nil)
	owner := int64(9)
	svc.selectorStrategy = func(int64) (model.ComposeStrategy, bool, error) {
		return model.ComposeStrategy{ID: 7, TeamID: 2, CategoryID: 3, OperatorID: &owner, Status: "enabled"}, true, nil
	}
	svc.operators = func() ([]identity.PublicUser, error) {
		team := identity.TeamID(2)
		return []identity.PublicUser{{ID: 9, TeamID: &team, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{3}}}, nil
	}
	actor := identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	got, _, err := svc.materialPageScope(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeamID == nil || *got.TeamID != 2 || got.CategoryID != 3 || got.OwnerID != 9 || got.UserID != 1 {
		t.Fatalf("filter=%+v", got)
	}
	for _, bad := range []repository.MaterialFilter{{StrategyID: 7, OperatorID: 8}, {OperatorID: 9}, {StrategyID: 7, OperatorID: 9, CategoryID: 4}} {
		if _, _, err := svc.materialPageScope(actor, bad, true); err == nil {
			t.Fatalf("accepted bad context %+v", bad)
		}
	}
	if store.created.MaterialID != 0 {
		t.Fatal("context read created usage")
	}
}

type selectorPageMemory struct {
	*memoryStore
	total int
	owner identity.UserID
}

func (s *selectorPageMemory) MaterialMetricPresence(ids []int64) (map[int64]map[string]bool, error) {
	return map[int64]map[string]bool{s.material.ID: {"like_count": false, "view_count": true, "favorite_count": true}}, nil
}
func (s *selectorPageMemory) CountMaterials(f repository.MaterialFilter) (int, error) {
	s.filter = f
	return s.total, nil
}
func (s *selectorPageMemory) OwnerUsagesForMaterials(owner identity.UserID, ids []int64, status string) ([]model.MaterialUsage, error) {
	s.owner = owner
	if s.usages != nil {
		return s.usages, nil
	}
	return []model.MaterialUsage{{MaterialID: s.material.ID, UserID: owner, Status: model.MaterialUsageActive}}, nil
}
func (s *selectorPageMemory) MaterialHistory(strategy int64, ids []int64) ([]model.ComposeMaterialState, error) {
	return []model.ComposeMaterialState{{MaterialID: s.material.ID, SuccessCount: 2, ActiveCount: 1}}, nil
}
func selectorFixture() (*Service, *selectorPageMemory, *stubTransfers, identity.PublicUser) {
	category := identity.CategoryID(3)
	owner := int64(9)
	team := identity.TeamID(2)
	store := &selectorPageMemory{memoryStore: &memoryStore{material: model.Material{ID: 42, TeamID: team, CategoryID: &category, Status: model.MaterialAvailable, SourceURL: "https://www.douyin.com/video/42"}, found: true}, total: 275}
	transfers := &stubTransfers{}
	svc := NewService(store, transfers, nil)
	svc.selectorStrategy = func(int64) (model.ComposeStrategy, bool, error) {
		return model.ComposeStrategy{ID: 7, TeamID: 2, CategoryID: 3, OperatorID: &owner, Status: "enabled"}, true, nil
	}
	svc.operators = func() ([]identity.PublicUser, error) {
		return []identity.PublicUser{{ID: 9, TeamID: &team, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{3}}}, nil
	}
	return svc, store, transfers, identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
}
func TestMaterialSelectorReadReturnsHistoryAndNeverJoinsOrPrepares(t *testing.T) {
	svc, store, transfers, actor := selectorFixture()
	page, err := svc.ListMaterialPage(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9, Page: 3, PageSize: 20, MyUsage: "unused", TeamUsage: "used"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 275 || page.Page != 3 || page.PageSize != 20 || len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
	row := page.Items[0]
	if row.Selectable == nil || !*row.Selectable || row.StrategyHistory == nil || row.StrategyHistory.SuccessCount != 2 {
		t.Fatalf("row=%+v", row)
	}
	if store.filter.OwnerID != 9 || store.filter.UserID != actor.ID || store.relationsFor != actor.ID {
		t.Fatalf("owner/viewer changed %+v", store.filter)
	}
	if store.created.MaterialID != 0 || transfers.prepareCount != 0 {
		t.Fatal("read changed relation/preparation")
	}
}
func TestMaterialSelectorOnlyConfirmationJoinsOwnerAndPrepares(t *testing.T) {
	svc, store, transfers, actor := selectorFixture()
	usage, _, err := svc.AddUsageForSelection(actor, 42, MaterialSelectorContext{StrategyID: 7, OperatorID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if usage.UserID != 9 || store.created.UserID != 9 || transfers.prepareCount != 1 || transfers.prepare.RequestedBy != actor.ID {
		t.Fatalf("usage=%+v created=%+v preparation=%+v", usage, store.created, transfers.prepare)
	}
	svc, store, transfers, actor = selectorFixture()
	wrong := identity.CategoryID(5)
	store.material.CategoryID = &wrong
	if _, _, err := svc.AddUsageForSelection(actor, 42, MaterialSelectorContext{StrategyID: 7, OperatorID: 9}); err == nil {
		t.Fatal("allowed different category")
	}
	if store.created.MaterialID != 0 || transfers.prepareCount != 0 {
		t.Fatal("unauthorized confirmation mutated")
	}
}
func TestMaterialSelectorDoesNotFilterOnVideoReadinessButRejectsAbsentSource(t *testing.T) {
	svc, store, _, actor := selectorFixture()
	store.material.VideoStatus = model.VideoFailed
	page, err := svc.ListMaterialPage(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9}, false)
	if err != nil || page.Items[0].Selectable == nil || !*page.Items[0].Selectable {
		t.Fatalf("failed video with valid source excluded: %+v %v", page, err)
	}
	store.material.SourceURL = ""
	page, err = svc.ListMaterialPage(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9}, false)
	if err != nil || page.Items[0].Selectable == nil || *page.Items[0].Selectable || page.Items[0].SelectionReason != "source_unavailable" {
		t.Fatalf("absent source admitted %+v %v", page, err)
	}
}
func TestMaterialSelectorOwnerContextDoesNotExpandOrdinaryReadScope(t *testing.T) {
	svc, _, _, actor := selectorFixture()
	team := identity.TeamID(2)
	actor.Role = identity.RoleOperator
	actor.TeamID = &team
	actor.CategoryIDs = []identity.CategoryID{3}
	if _, _, err := svc.materialPageScope(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9}, true); err == nil {
		t.Fatal("ordinary actor read another owner's strategy")
	}
	actor.Role = identity.RoleSeniorOperator
	otherTeam := identity.TeamID(4)
	actor.TeamID = &otherTeam
	if _, _, err := svc.materialPageScope(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9}, true); err == nil {
		t.Fatal("senior crossed team")
	}
}

func TestMaterialSelectorConfirmationRepairsMissingCloudObject(t *testing.T) {
	svc, store, transfers, actor := selectorFixture()
	store.material.VideoStatus = model.VideoReady
	store.material.SourceObjectKey = "missing-video"
	svc.links = &stubLinks{missing: map[string]bool{"missing-video": true}}
	_, _, err := svc.AddUsageForSelection(actor, 42, MaterialSelectorContext{StrategyID: 7, OperatorID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if store.repreparedCount != 1 || store.repreparedKey != "missing-video" || transfers.prepareCount != 1 {
		t.Fatalf("missing prepared input was not repaired: reprepared=%d transfer=%d", store.repreparedCount, transfers.prepareCount)
	}
}

func TestMaterialSelectorMyPageUsesOwnerRelationsAndActorSuccessFacts(t *testing.T) {
	svc, store, transfers, actor := selectorFixture()
	owner := int64(9)
	store.facts = []repository.MaterialOutputFact{{MaterialID: 42, OperatorID: &owner, OutputCount: 2}}
	page, err := svc.ListMyMaterialPage(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].UserID != 9 || page.Items[0].Material.MyUsage != "unused" || store.owner != 9 {
		t.Fatalf("owner was confused with viewer: %+v", page)
	}
	if page.Items[0].Material.MetricPresence["like_count"] {
		t.Fatal("missing metric represented as observed zero")
	}
	if len(transfers.lastStatusQuery) > 0 || store.created.MaterialID != 0 || transfers.prepareCount != 0 {
		t.Fatal("owner read accessed local downloads or wrote selection")
	}
	actorID := int64(actor.ID)
	store.facts = append(store.facts, repository.MaterialOutputFact{MaterialID: 42, OperatorID: &actorID, OutputCount: 1})
	page, err = svc.ListMyMaterialPage(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9})
	if err != nil || page.Items[0].Material.MyUsage != "used" || page.Items[0].Material.Usage.OutputCount != 3 {
		t.Fatalf("actor/team success facts missing %+v %v", page, err)
	}
}

func TestNormalMyMaterialPagePreservesRemovedRelationsAndOwnDownloads(t *testing.T) {
	svc, store, transfers, actor := selectorFixture()
	store.usages = []model.MaterialUsage{{MaterialID: 42, UserID: actor.ID, Status: model.MaterialUsageRemoved}}
	transfers.downloadStatuses = map[int64]string{42: "downloaded"}
	page, err := svc.ListMyMaterialPage(actor, repository.MaterialFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if store.owner != actor.ID || store.filter.UsageStatus != "all" || len(page.Items) != 1 || page.Items[0].Status != model.MaterialUsageRemoved || page.Items[0].DownloadStatus != "downloaded" {
		t.Fatalf("normal browsing changed %+v filter=%+v", page, store.filter)
	}
	filter, _, err := svc.materialPageScope(actor, repository.MaterialFilter{StrategyID: 7, OperatorID: 9, UsageStatus: "removed"}, true)
	if err != nil || filter.UsageStatus != "active" {
		t.Fatalf("selector admitted removed relationship %+v %v", filter, err)
	}
}
