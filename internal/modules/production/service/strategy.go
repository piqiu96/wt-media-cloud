package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
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
	TeamID          int64                       `json:"team_id"`
	GameID          string                      `json:"game_id"`
	Name            string                      `json:"name"`
	TemplateID      int64                       `json:"compose_template_id"`
	TemplateVersion uint32                      `json:"compose_template_version"`
	Config          json.RawMessage             `json:"config"`
	ExecutionMode   string                      `json:"execution_mode"`
	Schedule        *model.ComposeSchedule      `json:"schedule"`
	PriorityRules   []model.ComposePriorityRule `json:"priority_rules"`
	SingleLimit     int                         `json:"single_limit"`
	DailyLimit      int                         `json:"daily_limit"`
}

type UpdateStrategyInput struct {
	Name          string                      `json:"name"`
	Config        json.RawMessage             `json:"config"`
	ExecutionMode string                      `json:"execution_mode"`
	Schedule      *model.ComposeSchedule      `json:"schedule"`
	PriorityRules []model.ComposePriorityRule `json:"priority_rules"`
	SingleLimit   int                         `json:"single_limit"`
	DailyLimit    int                         `json:"daily_limit"`
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
	now         func() time.Time
}

func NewStrategyService(store StrategyRepository, resolveGame func(string) (identity.OperationGame, bool, error)) *StrategyService {
	return &StrategyService{store: store, resolveGame: resolveGame, now: time.Now}
}

func wiredStrategyService() *StrategyService {
	return NewStrategyService(repository.StrategyStore{}, identity.ResolveGame)
}

// ComposeStrategyListQuery carries the list page's filters. Pagination stays
// client-side; the repository caps the result at 200 rows.
type ComposeStrategyListQuery struct {
	Search     string
	GameID     string
	TemplateID int64
	Status     string
}

func ListComposeStrategies(actor identity.PublicUser, query ComposeStrategyListQuery) ([]model.ComposeStrategy, error) {
	return wiredStrategyService().List(actor, query)
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

func (s *StrategyService) List(actor identity.PublicUser, query ComposeStrategyListQuery) ([]model.ComposeStrategy, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrStrategyForbidden
	}
	if query.Status != "" && query.Status != model.ComposeStrategyEnabled && query.Status != model.ComposeStrategyDisabled {
		return nil, fmt.Errorf("%w: unknown status %s", ErrStrategyInvalid, query.Status)
	}
	filter := repository.StrategyFilter{
		ActiveOnly: actor.Role == identity.RoleOperator,
		Search:     strings.TrimSpace(query.Search),
		GameID:     strings.TrimSpace(query.GameID),
		TemplateID: query.TemplateID,
		Status:     query.Status,
	}
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
	if err := validateStrategyExecution(input.ExecutionMode, input.Schedule, input.SingleLimit, input.DailyLimit); err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := model.ValidateComposePriorityRules(input.PriorityRules); err != nil {
		return model.ComposeStrategy{}, fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
	}
	rulesJSON, err := json.Marshal(input.PriorityRules)
	if err != nil {
		return model.ComposeStrategy{}, fmt.Errorf("%w: priority_rules are invalid", ErrStrategyInvalid)
	}
	item := model.ComposeStrategy{TeamID: teamID, GameID: input.GameID, TemplateID: input.TemplateID, TemplateVersion: input.TemplateVersion, Name: strings.TrimSpace(input.Name), Config: append(json.RawMessage(nil), input.Config...), ExecutionMode: input.ExecutionMode, Schedule: input.Schedule, PriorityRules: input.PriorityRules, RulesJSON: rulesJSON, SingleLimit: input.SingleLimit, DailyLimit: input.DailyLimit, Status: model.ComposeStrategyEnabled, CreatedBy: int64(actor.ID), UpdatedBy: int64(actor.ID)}
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.store.CreateAudited(item, "compose_strategy.create")
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
	if err := validateStrategyExecution(input.ExecutionMode, input.Schedule, input.SingleLimit, input.DailyLimit); err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := model.ValidateComposePriorityRules(input.PriorityRules); err != nil {
		return model.ComposeStrategy{}, fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
	}
	rulesJSON, err := json.Marshal(input.PriorityRules)
	if err != nil {
		return model.ComposeStrategy{}, fmt.Errorf("%w: priority_rules are invalid", ErrStrategyInvalid)
	}
	item.Name, item.Config, item.UpdatedBy = strings.TrimSpace(input.Name), append(json.RawMessage(nil), input.Config...), int64(actor.ID)
	item.ExecutionMode, item.Schedule, item.PriorityRules, item.RulesJSON = input.ExecutionMode, input.Schedule, input.PriorityRules, rulesJSON
	item.SingleLimit, item.DailyLimit = input.SingleLimit, input.DailyLimit
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
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
	item.ID, item.Name, item.Status, item.CreatedBy, item.UpdatedBy = 0, strings.TrimSpace(name), model.ComposeStrategyEnabled, int64(actor.ID), int64(actor.ID)
	item.ExecutionMode, item.Schedule, item.ScheduleJSON, item.NextRunAt = model.ComposeExecutionManual, nil, nil, nil
	return s.store.CreateAudited(item, "compose_strategy.copy")
}

func (s *StrategyService) SetStatus(actor identity.PublicUser, id int64, status string) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if status != model.ComposeStrategyEnabled && status != model.ComposeStrategyDisabled {
		return model.ComposeStrategy{}, fmt.Errorf("%w: invalid status", ErrStrategyInvalid)
	}
	if status == model.ComposeStrategyEnabled {
		if err := s.validateConfig(item.TemplateID, item.TemplateVersion, item.Config, true); err != nil {
			return model.ComposeStrategy{}, err
		}
		if err := model.ValidateComposePriorityRules(item.PriorityRules); err != nil {
			return model.ComposeStrategy{}, fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
		}
	}
	item.Status, item.UpdatedBy = status, int64(actor.ID)
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
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
		return item.Status == model.ComposeStrategyEnabled
	}
	return actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin
}

func validateStrategyFields(name string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(name)); n == 0 || n > 120 {
		return fmt.Errorf("%w: name must contain 1-120 characters", ErrStrategyInvalid)
	}
	return nil
}

func validateStrategyExecution(mode string, schedule *model.ComposeSchedule, singleLimit, dailyLimit int) error {
	if singleLimit < 1 || singleLimit > 100 || dailyLimit < singleLimit || dailyLimit > 1000 {
		return fmt.Errorf("%w: limits must satisfy 1 <= single_limit <= 100 and single_limit <= daily_limit <= 1000", ErrStrategyInvalid)
	}
	switch mode {
	case model.ComposeExecutionManual:
		if schedule != nil {
			return fmt.Errorf("%w: manual execution cannot have a schedule", ErrStrategyInvalid)
		}
	case model.ComposeExecutionScheduled:
		if schedule == nil {
			return fmt.Errorf("%w: scheduled execution requires a schedule", ErrStrategyInvalid)
		}
		return validateComposeSchedule(*schedule)
	default:
		return fmt.Errorf("%w: execution_mode is invalid", ErrStrategyInvalid)
	}
	return nil
}

func (s *StrategyService) validateConfig(templateID int64, version uint32, config json.RawMessage, requireActive bool) error {
	item, found, err := s.store.FindTemplate(templateID, version)
	if err != nil {
		return err
	}
	if !found {
		return ErrStrategyTemplateUnavailable
	}
	definition, err := executor.DeployedDefinition(item.Code, item.Version)
	if err != nil || item.ExecutorID != definition.ExecutorID || (requireActive && item.Status != "active") || !sameJSON(item.Schema, definition.Schema) {
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
