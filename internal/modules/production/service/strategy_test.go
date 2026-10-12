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
	actor := identity.PublicUser{ID: 9, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	return service, store, actor
}

func TestStrategyCreateRejectsUnknownConfigAndInactiveTemplate(t *testing.T) {
	svc, store, actor := strategyFixture()
	input := CreateStrategyInput{GameID: "delta", Name: "切段", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12,"bgm":"x"}`)}
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
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", Status: "active", CreatedBy: 99}}
	actor.Role = identity.RoleOperator
	items, err := svc.List(actor)
	if err != nil || len(items) != 1 || store.lastFilter.TeamID != 7 || !store.lastFilter.ActiveOnly {
		t.Fatalf("items=%v filter=%+v err=%v", items, store.lastFilter, err)
	}
	_, err = svc.Create(actor, CreateStrategyInput{})
	if !errors.Is(err, ErrStrategyForbidden) || store.writes != 0 {
		t.Fatalf("write error=%v writes=%d", err, store.writes)
	}
}

func TestStrategySeniorCannotWriteOutsideGame(t *testing.T) {
	svc, store, actor := strategyFixture()
	_, err := svc.Create(actor, CreateStrategyInput{GameID: "other", Name: "x", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`)})
	if !errors.Is(err, ErrStrategyForbidden) || store.writes != 0 {
		t.Fatalf("error=%v writes=%d", err, store.writes)
	}
}

func TestStrategyUpdateKeepsBindingAndCopyRequiresActiveTemplate(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "原策略", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "active", CreatedBy: 99}}
	updated, err := svc.Update(actor, 4, UpdateStrategyInput{Name: "新策略", Config: json.RawMessage(`{"start_seconds":6,"end_seconds":12}`)})
	if err != nil || updated.TemplateID != 3 || updated.TemplateVersion != 1 || updated.CreatedBy != 99 || updated.Name != "新策略" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	store.template.Status = "inactive"
	_, err = svc.Copy(actor, 4, "副本")
	if !errors.Is(err, ErrStrategyTemplateUnavailable) || store.writes != 1 {
		t.Fatalf("copy error=%v writes=%d", err, store.writes)
	}
	if _, err = svc.SetStatus(actor, 4, "inactive"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetStatus(actor, 4, "active"); !errors.Is(err, ErrStrategyTemplateUnavailable) {
		t.Fatalf("activation error=%v", err)
	}
}

func TestStrategyOperatorCannotReadInactiveTemplate(t *testing.T) {
	svc, store, actor := strategyFixture()
	store.items = []model.ComposeStrategy{{ID: 4, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "active"}}
	actor.Role = identity.RoleOperator
	store.template.Status = "inactive"
	_, err := svc.Get(actor, 4)
	if !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("get error=%v", err)
	}
}
