package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

var (
	ErrStrategyInvalid             = errors.New("compose strategy input is invalid")
	ErrStrategyForbidden           = errors.New("compose strategy operation is forbidden")
	ErrStrategyNotFound            = errors.New("compose strategy not found")
	ErrStrategyTemplateUnavailable = errors.New("compose template version is unavailable")
)

type CreateStrategyInput struct {
	TeamID          int64           `json:"team_id"`
	GameID          string          `json:"game_id"`
	Name            string          `json:"name"`
	TemplateID      int64           `json:"compose_template_id"`
	TemplateVersion uint32          `json:"compose_template_version"`
	Config          json.RawMessage `json:"config"`
}

type UpdateStrategyInput struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type StrategyRepository interface {
	List(repository.StrategyFilter) ([]model.ComposeStrategy, error)
	Find(int64) (model.ComposeStrategy, bool, error)
	FindTemplate(int64, uint32) (model.ComposeTemplate, bool, error)
	CreateAudited(model.ComposeStrategy, string) (model.ComposeStrategy, error)
	UpdateAudited(model.ComposeStrategy, string) (model.ComposeStrategy, error)
}

type StrategyService struct {
	store       StrategyRepository
	resolveGame func(string) (identity.OperationGame, bool, error)
}

func NewStrategyService(store StrategyRepository, resolveGame func(string) (identity.OperationGame, bool, error)) *StrategyService {
	return &StrategyService{store: store, resolveGame: resolveGame}
}

func wiredStrategyService() *StrategyService {
	return NewStrategyService(repository.StrategyStore{}, identity.ResolveGame)
}

func ListComposeStrategies(actor identity.PublicUser) ([]model.ComposeStrategy, error) {
	return wiredStrategyService().List(actor)
}
func GetComposeStrategy(actor identity.PublicUser, id int64) (model.ComposeStrategy, error) {
	return wiredStrategyService().Get(actor, id)
}
func CreateComposeStrategy(actor identity.PublicUser, input CreateStrategyInput) (model.ComposeStrategy, error) {
	return wiredStrategyService().Create(actor, input)
}
func UpdateComposeStrategy(actor identity.PublicUser, id int64, input UpdateStrategyInput) (model.ComposeStrategy, error) {
	return wiredStrategyService().Update(actor, id, input)
}
func CopyComposeStrategy(actor identity.PublicUser, id int64, name string) (model.ComposeStrategy, error) {
	return wiredStrategyService().Copy(actor, id, name)
}
func SetComposeStrategyStatus(actor identity.PublicUser, id int64, status string) (model.ComposeStrategy, error) {
	return wiredStrategyService().SetStatus(actor, id, status)
}

func (s *StrategyService) List(actor identity.PublicUser) ([]model.ComposeStrategy, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrStrategyForbidden
	}
	filter := repository.StrategyFilter{ActiveOnly: actor.Role == identity.RoleOperator}
	if actor.Role != identity.RoleAdmin {
		if actor.Role != identity.RoleOperator && actor.Role != identity.RoleSeniorOperator {
			return nil, ErrStrategyForbidden
		}
		if actor.TeamID == nil || len(actor.GameIDs) == 0 {
			return []model.ComposeStrategy{}, nil
		}
		filter.TeamID = int64(*actor.TeamID)
		filter.GameIDs = actor.GameIDs
	}
	return s.store.List(filter)
}

func (s *StrategyService) Get(actor identity.PublicUser, id int64) (model.ComposeStrategy, error) {
	item, found, err := s.store.Find(id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found || !canReadStrategy(actor, item) {
		return model.ComposeStrategy{}, ErrStrategyNotFound
	}
	if actor.Role == identity.RoleOperator {
		if err := s.validateConfig(item.TemplateID, item.TemplateVersion, item.Config, true); err != nil {
			if errors.Is(err, ErrStrategyTemplateUnavailable) {
				return model.ComposeStrategy{}, ErrStrategyNotFound
			}
			return model.ComposeStrategy{}, err
		}
	}
	return item, nil
}

func (s *StrategyService) Create(actor identity.PublicUser, input CreateStrategyInput) (model.ComposeStrategy, error) {
	if !canManageStrategies(actor) {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	teamID := input.TeamID
	if actor.Role == identity.RoleSeniorOperator {
		if actor.TeamID == nil || (teamID != 0 && teamID != int64(*actor.TeamID)) {
			return model.ComposeStrategy{}, ErrStrategyForbidden
		}
		teamID = int64(*actor.TeamID)
	}
	if teamID <= 0 {
		return model.ComposeStrategy{}, fmt.Errorf("%w: team_id is required", ErrStrategyInvalid)
	}
	if !inGameScope(actor, teamID, input.GameID) {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	if err := validateStrategyFields(input.Name); err != nil {
		return model.ComposeStrategy{}, err
	}
	game, found, err := s.resolveGame(input.GameID)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found || game.Status != identity.GameStatusEnabled {
		return model.ComposeStrategy{}, fmt.Errorf("%w: game is unavailable", ErrStrategyInvalid)
	}
	if err := s.validateConfig(input.TemplateID, input.TemplateVersion, input.Config, true); err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.store.CreateAudited(model.ComposeStrategy{TeamID: teamID, GameID: input.GameID, TemplateID: input.TemplateID, TemplateVersion: input.TemplateVersion, Name: strings.TrimSpace(input.Name), Config: append(json.RawMessage(nil), input.Config...), Status: "active", CreatedBy: int64(actor.ID), UpdatedBy: int64(actor.ID)}, "compose_strategy.create")
}

func (s *StrategyService) Update(actor identity.PublicUser, id int64, input UpdateStrategyInput) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := validateStrategyFields(input.Name); err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := s.validateConfig(item.TemplateID, item.TemplateVersion, input.Config, false); err != nil {
		return model.ComposeStrategy{}, err
	}
	item.Name, item.Config, item.UpdatedBy = strings.TrimSpace(input.Name), append(json.RawMessage(nil), input.Config...), int64(actor.ID)
	return s.store.UpdateAudited(item, "compose_strategy.update")
}

func (s *StrategyService) Copy(actor identity.PublicUser, id int64, name string) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := validateStrategyFields(name); err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := s.validateConfig(item.TemplateID, item.TemplateVersion, item.Config, true); err != nil {
		return model.ComposeStrategy{}, err
	}
	item.ID, item.Name, item.Status, item.CreatedBy, item.UpdatedBy = 0, strings.TrimSpace(name), "active", int64(actor.ID), int64(actor.ID)
	return s.store.CreateAudited(item, "compose_strategy.copy")
}

func (s *StrategyService) SetStatus(actor identity.PublicUser, id int64, status string) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if status != "active" && status != "inactive" {
		return model.ComposeStrategy{}, fmt.Errorf("%w: invalid status", ErrStrategyInvalid)
	}
	if status == "active" {
		if err := s.validateConfig(item.TemplateID, item.TemplateVersion, item.Config, true); err != nil {
			return model.ComposeStrategy{}, err
		}
	}
	item.Status, item.UpdatedBy = status, int64(actor.ID)
	return s.store.UpdateAudited(item, "compose_strategy.status")
}

func (s *StrategyService) managed(actor identity.PublicUser, id int64) (model.ComposeStrategy, error) {
	if !canManageStrategies(actor) {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	item, found, err := s.store.Find(id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found || !inGameScope(actor, item.TeamID, item.GameID) {
		return model.ComposeStrategy{}, ErrStrategyNotFound
	}
	return item, nil
}

func canManageStrategies(actor identity.PublicUser) bool {
	return actor.Status == identity.UserStatusEnabled && (actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin)
}

func inGameScope(actor identity.PublicUser, teamID int64, gameID string) bool {
	if actor.Status != identity.UserStatusEnabled {
		return false
	}
	if actor.Role == identity.RoleAdmin {
		return true
	}
	if actor.TeamID == nil || int64(*actor.TeamID) != teamID {
		return false
	}
	for _, id := range actor.GameIDs {
		if id == gameID {
			return true
		}
	}
	return false
}

func canReadStrategy(actor identity.PublicUser, item model.ComposeStrategy) bool {
	if !inGameScope(actor, item.TeamID, item.GameID) {
		return false
	}
	if actor.Role == identity.RoleOperator {
		return item.Status == "active"
	}
	return actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin
}

func validateStrategyFields(name string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(name)); n == 0 || n > 120 {
		return fmt.Errorf("%w: name must contain 1-120 characters", ErrStrategyInvalid)
	}
	return nil
}

func (s *StrategyService) validateConfig(templateID int64, version uint32, config json.RawMessage, requireActive bool) error {
	item, found, err := s.store.FindTemplate(templateID, version)
	if err != nil {
		return err
	}
	definition := executor.BasicCutV1()
	if !found || item.Code != definition.Code || item.Version != definition.Version || item.ExecutorID != definition.ExecutorID || (requireActive && item.Status != "active") || !sameJSON(item.Schema, definition.Schema) {
		return ErrStrategyTemplateUnavailable
	}
	if _, err := definition.Executor.ValidateParams(config); err != nil {
		return fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
	}
	return nil
}

func sameJSON(a, b json.RawMessage) bool {
	var left, right any
	return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
}
