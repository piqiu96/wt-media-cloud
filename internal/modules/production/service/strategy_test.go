package service

import (
	"encoding/json"
	"errors"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type strategyMemoryStore struct {
	items      []model.ComposeStrategy
	template   model.ComposeTemplate
	lastFilter repository.StrategyFilter
	writes     int
}

func (s *strategyMemoryStore) List(filter repository.StrategyFilter) ([]model.ComposeStrategy, error) {
	s.lastFilter = filter
	return s.items, nil
}
func (s *strategyMemoryStore) Find(id int64) (model.ComposeStrategy, bool, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return model.ComposeStrategy{}, false, nil
}
func (s *strategyMemoryStore) FindTemplate(id int64, version uint32) (model.ComposeTemplate, bool, error) {
	return s.template, s.template.ID == id && s.template.Version == version, nil
}
func (s *strategyMemoryStore) CreateAudited(item model.ComposeStrategy, action string) (model.ComposeStrategy, error) {
	s.writes++
	item.ID = int64(len(s.items) + 1)
	s.items = append(s.items, item)
	return item, nil
}
func (s *strategyMemoryStore) UpdateAudited(item model.ComposeStrategy, action string) (model.ComposeStrategy, error) {
	s.writes++
	for i := range s.items {
		if s.items[i].ID == item.ID {
			s.items[i] = item
			return item, nil
		}
	}
	return item, nil
}

func strategyFixture() (*StrategyService, *strategyMemoryStore, identity.PublicUser) {
	team := identity.TeamID(7)
	definition := executor.BasicCutV1()
	store := &strategyMemoryStore{template: model.ComposeTemplate{ID: 3, Code: definition.Code, Version: definition.Version, Schema: definition.Schema, ExecutorID: definition.ExecutorID, Status: "active"}}
	service := NewStrategyService(store, func(string) (identity.OperationGame, bool, error) {
		return identity.OperationGame{ID: "delta", Status: identity.GameStatusEnabled}, true, nil
	})
	actor := identity.PublicUser{ID: 9, Username: "senior9", Nickname: "九号", Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	service.operators = func() ([]identity.PublicUser, error) { return []identity.PublicUser{actor}, nil }
	return service, store, actor
}

func operatorIDPtr(id int64) *int64 { return &id }

func strategyRulesFixture() []model.ComposePriorityRule {
	return []model.ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []model.ComposeCondition{{Field: "like_count", Operator: "gte", Value: json.RawMessage(`0`)}}}}
}

func TestStrategyCreateRejectsUnknownConfigAndInactiveTemplate(t *testing.T) {
	svc, store, actor := strategyFixture()
	input := CreateStrategyInput{GameID: "delta", OperatorID: 9, Name: "切段", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12,"bgm":"x"}`)}
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrStrategyInvalid) {
		t.Fatalf("unknown config error = %v", err)
	}
	input.Config = json.RawMessage(`{"start_seconds":5,"end_seconds":12}`)
	store.template.Status = "inactive"
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrStrategyTemplateUnavailable) {
		t.Fatalf("inactive template error = %v", err)
	}
	if store.writes != 0 {
		t.Fatalf("writes = %d", store.writes)
	}
}

func TestStrategyOperatorReadsAvailableTeamStrategyButCannotWrite(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", Status: "enabled", OperatorID: operatorIDPtr(9), CreatedBy: 99}}
	actor.Role = identity.RoleOperator
	items, err := svc.List(actor, ComposeStrategyListQuery{})
	if err != nil || len(items) != 1 || store.lastFilter.TeamID != 7 || !store.lastFilter.ActiveOnly || len(store.lastFilter.GameIDs) != 1 || store.lastFilter.GameIDs[0] != "delta" || store.lastFilter.OperatorID != 9 {
		t.Fatalf("items=%v filter=%+v err=%v", items, store.lastFilter, err)
	}
	_, err = svc.Create(actor, CreateStrategyInput{})
	if !errors.Is(err, ErrStrategyForbidden) || store.writes != 0 {
		t.Fatalf("write error=%v writes=%d", err, store.writes)
	}
}

func TestStrategySeniorManagesEveryGameOfTheirTeam(t *testing.T) {
	svc, store, actor := strategyFixture()
	other := identity.PublicUser{ID: 10, Username: "operator10", Nickname: "十号", Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: actor.TeamID, GameIDs: []string{"other"}}
	svc.operators = func() ([]identity.PublicUser, error) { return []identity.PublicUser{actor, other}, nil }
	created, err := svc.Create(actor, CreateStrategyInput{GameID: "other", OperatorID: 10, Name: "跨游戏", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2})
	if err != nil || created.GameID != "other" || created.TeamID != 7 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	otherTeam := identity.TeamID(8)
	actor.TeamID = &otherTeam
	_, err = svc.Create(actor, CreateStrategyInput{TeamID: 7, GameID: "other", OperatorID: 10, Name: "x", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2})
	if !errors.Is(err, ErrStrategyForbidden) || store.writes != 1 {
		t.Fatalf("error=%v writes=%d", err, store.writes)
	}
}

func TestStrategySeniorScopeIsTheTeamNotTheirOwnGames(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "other", Status: "enabled", OperatorID: operatorIDPtr(9)}}
	items, err := svc.List(actor, ComposeStrategyListQuery{})
	if err != nil || len(items) != 1 || store.lastFilter.TeamID != 7 || len(store.lastFilter.GameIDs) != 0 {
		t.Fatalf("items=%v filter=%+v err=%v", items, store.lastFilter, err)
	}
	if _, err := svc.Get(actor, 4); err != nil {
		t.Fatalf("senior reads another game of the team: %v", err)
	}
	actor.GameIDs = nil
	if _, err := svc.List(actor, ComposeStrategyListQuery{}); err != nil || len(items) != 1 {
		t.Fatalf("senior without game assignments still lists the team: items=%v err=%v", items, err)
	}
	if _, err := svc.Get(actor, 4); err != nil {
		t.Fatalf("senior without game assignments still reads the team: %v", err)
	}

	actor.Role = identity.RoleOperator
	actor.GameIDs = []string{"delta"}
	if _, err := svc.Get(actor, 4); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("ordinary operator must stay inside their games: %v", err)
	}

	actor.Role = identity.RoleSeniorOperator
	actor.GameIDs = []string{"delta"}
	otherTeam := identity.TeamID(8)
	actor.TeamID = &otherTeam
	if _, err := svc.Get(actor, 4); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("senior must not read another team's strategy: %v", err)
	}
	if _, err = svc.List(actor, ComposeStrategyListQuery{}); err != nil || store.lastFilter.TeamID != 8 {
		t.Fatalf("cross-team list filter=%+v err=%v", store.lastFilter, err)
	}
}

func TestStrategyUpdateKeepsBindingAndCopyRequiresActiveTemplate(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "原策略", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2, Status: "enabled", OperatorID: operatorIDPtr(9), CreatedBy: 99}}
	updated, err := svc.Update(actor, 4, UpdateStrategyInput{Name: "新策略", Config: json.RawMessage(`{"start_seconds":6,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2})
	if err != nil || updated.TemplateID != 3 || updated.TemplateVersion != 1 || updated.CreatedBy != 99 || updated.Name != "新策略" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	store.template.Status = "inactive"
	_, err = svc.Copy(actor, 4, "副本")
	if !errors.Is(err, ErrStrategyTemplateUnavailable) || store.writes != 1 {
		t.Fatalf("copy error=%v writes=%d", err, store.writes)
	}
	if _, err = svc.SetStatus(actor, 4, "disabled"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetStatus(actor, 4, "enabled"); !errors.Is(err, ErrStrategyTemplateUnavailable) {
		t.Fatalf("activation error=%v", err)
	}
}

func TestStrategyOperatorCannotReadInactiveTemplate(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "enabled", OperatorID: operatorIDPtr(9)}}
	actor.Role = identity.RoleOperator
	store.template.Status = "inactive"
	_, err := svc.Get(actor, 4)
	if !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("get error=%v", err)
	}
}

func TestGuideStrategyStoresDownloadAddress(t *testing.T) {
	svc, store, actor := strategyFixture()
	definition := executor.GuideInsertV1()
	store.template = model.ComposeTemplate{ID: 4, Code: definition.Code, Version: definition.Version, Schema: definition.Schema, ExecutorID: definition.ExecutorID, Status: "active"}
	config := json.RawMessage(`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":2,"guide_duration_seconds":0,"max_duration_seconds":0}`)
	created, err := svc.Create(actor, CreateStrategyInput{GameID: "delta", OperatorID: 9, Name: "引导插入", TemplateID: 4, TemplateVersion: 1, Config: config, ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2})
	if err != nil || string(created.Config) != string(config) {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	if _, err := svc.Create(actor, CreateStrategyInput{GameID: "delta", OperatorID: 9, Name: "无效链接", TemplateID: 4, TemplateVersion: 1, Config: json.RawMessage(`{"guide_video_url":"file:///tmp/guide.mp4","insert_seconds":2,"guide_duration_seconds":0,"max_duration_seconds":0}`)}); !errors.Is(err, ErrStrategyInvalid) {
		t.Fatalf("file URL error = %v", err)
	}
}

func strategyCreateInput() CreateStrategyInput {
	return CreateStrategyInput{
		GameID: "delta", Name: "切段", TemplateID: 3, TemplateVersion: 1,
		Config:        json.RawMessage(`{"start_seconds":5,"end_seconds":12}`),
		ExecutionMode: model.ComposeExecutionManual,
		PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2,
	}
}

func TestStrategyCreateRequiresAnAvailableOperator(t *testing.T) {
	svc, store, actor := strategyFixture()
	input := strategyCreateInput()
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrStrategyInvalid) || store.writes != 0 {
		t.Fatalf("missing operator err=%v writes=%d", err, store.writes)
	}
	input.OperatorID = 10
	if _, err := svc.Create(actor, input); !errors.Is(err, ErrStrategyInvalid) || store.writes != 0 {
		t.Fatalf("unknown operator err=%v writes=%d", err, store.writes)
	}
	input.OperatorID = 9
	created, err := svc.Create(actor, input)
	if err != nil || created.OperatorID == nil || *created.OperatorID != 9 || created.OperatorName != "九号" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
}

func TestStrategyOperatorSeesOnlyAssignedEnabledStrategies(t *testing.T) {
	svc, store, actor := strategyFixture()
	other := int64(10)
	store.items = []model.ComposeStrategy{
		{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "enabled", OperatorID: operatorIDPtr(9)},
		{ID: 5, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "enabled", OperatorID: &other},
		{ID: 6, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "enabled"},
	}
	actor.Role = identity.RoleOperator
	if _, err := svc.List(actor, ComposeStrategyListQuery{}); err != nil {
		t.Fatal(err)
	}
	if store.lastFilter.OperatorID != 9 {
		t.Fatalf("operator list filter = %+v", store.lastFilter)
	}
	if _, err := svc.Get(actor, 4); err != nil {
		t.Fatalf("assigned strategy must be readable: %v", err)
	}
	for _, id := range []int64{5, 6} {
		if _, err := svc.Get(actor, id); !errors.Is(err, ErrStrategyNotFound) {
			t.Fatalf("strategy %d error = %v", id, err)
		}
	}
}

func TestStrategyCopyAndEnableRequireTheAssignedOperator(t *testing.T) {
	svc, store, actor := strategyFixture()
	base := model.ComposeStrategy{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "原策略", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2, Status: "disabled"}
	store.items = []model.ComposeStrategy{base}
	if _, err := svc.Copy(actor, 4, "副本"); !errors.Is(err, ErrStrategyInvalid) || store.writes != 0 {
		t.Fatalf("unassigned copy err=%v writes=%d", err, store.writes)
	}
	if _, err := svc.SetStatus(actor, 4, "enabled"); !errors.Is(err, ErrStrategyInvalid) || store.writes != 0 {
		t.Fatalf("unassigned enable err=%v writes=%d", err, store.writes)
	}
	base.OperatorID = operatorIDPtr(9)
	store.items[0] = base
	copied, err := svc.Copy(actor, 4, "副本")
	if err != nil || copied.OperatorID == nil || *copied.OperatorID != 9 {
		t.Fatalf("copy=%+v err=%v", copied, err)
	}
	if _, err := svc.SetStatus(actor, 4, "enabled"); err != nil {
		t.Fatalf("enable err=%v", err)
	}
}

func TestStrategyUpdateRejectsLeavingAnEnabledStrategyUnassigned(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "原策略", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2, Status: "enabled"}}
	input := UpdateStrategyInput{Name: "新策略", Config: json.RawMessage(`{"start_seconds":6,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, PriorityRules: strategyRulesFixture(), SingleLimit: 1, DailyLimit: 2}
	if _, err := svc.Update(actor, 4, input); !errors.Is(err, ErrStrategyInvalid) || store.writes != 0 {
		t.Fatalf("unassigned enabled update err=%v writes=%d", err, store.writes)
	}
	input.OperatorID = 9
	updated, err := svc.Update(actor, 4, input)
	if err != nil || updated.OperatorID == nil || *updated.OperatorID != 9 {
		t.Fatalf("update=%+v err=%v", updated, err)
	}
}
