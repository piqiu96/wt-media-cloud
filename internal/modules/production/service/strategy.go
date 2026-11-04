package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
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
	TeamID          int64                        `json:"team_id"`
	CategoryID      identity.CategoryID          `json:"category_id"`
	OperatorID      int64                        `json:"operator_id"`
	Name            string                       `json:"name"`
	TemplateID      int64                        `json:"compose_template_id"`
	TemplateVersion uint32                       `json:"compose_template_version"`
	Config          json.RawMessage              `json:"config"`
	OutputSettings  []model.ComposeOutputSetting `json:"output_settings"`
	ExecutionMode   string                       `json:"execution_mode"`
	Schedule        *model.ComposeSchedule       `json:"schedule"`
	PriorityRules   []model.ComposePriorityRule  `json:"priority_rules"`
	SingleLimit     int                          `json:"single_limit"`
	DailyLimit      int                          `json:"daily_limit"`
}

type UpdateStrategyInput struct {
	OperatorID     int64                        `json:"operator_id"`
	Name           string                       `json:"name"`
	Config         json.RawMessage              `json:"config"`
	OutputSettings []model.ComposeOutputSetting `json:"output_settings"`
	ExecutionMode  string                       `json:"execution_mode"`
	Schedule       *model.ComposeSchedule       `json:"schedule"`
	PriorityRules  []model.ComposePriorityRule  `json:"priority_rules"`
	SingleLimit    int                          `json:"single_limit"`
	DailyLimit     int                          `json:"daily_limit"`
}

type StrategyRepository interface {
	List(repository.StrategyFilter) ([]model.ComposeStrategy, error)
	Find(int64) (model.ComposeStrategy, bool, error)
	FindTemplate(int64, uint32) (model.ComposeTemplate, bool, error)
	CreateAudited(model.ComposeStrategy, string) (model.ComposeStrategy, error)
	UpdateAudited(model.ComposeStrategy, string) (model.ComposeStrategy, error)
}

type StrategyService struct {
	store           StrategyRepository
	resolveCategory func(identity.CategoryID) (identity.Category, bool, error)
	operators       func() ([]identity.PublicUser, error)
	now             func() time.Time
}

func NewStrategyService(store StrategyRepository, resolveCategory func(identity.CategoryID) (identity.Category, bool, error)) *StrategyService {
	return &StrategyService{store: store, resolveCategory: resolveCategory, now: time.Now}
}

func wiredStrategyService() *StrategyService {
	service := NewStrategyService(repository.StrategyStore{}, identity.ResolveCategory)
	service.operators = identity.ListUsers
	return service
}

// ComposeStrategyListQuery carries the authorized strategy page filters.
type ComposeStrategyListQuery struct {
	Page       int
	PageSize   int
	TeamID     int64
	OperatorID int64
	Search     string
	CategoryID identity.CategoryID
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
		CategoryID: query.CategoryID,
		TemplateID: query.TemplateID,
		Status:     query.Status,
	}
	if actor.Role != identity.RoleAdmin {
		if actor.Role != identity.RoleOperator && actor.Role != identity.RoleSeniorOperator {
			return nil, ErrStrategyForbidden
		}
		if actor.TeamID == nil {
			return []model.ComposeStrategy{}, nil
		}
		filter.TeamID = int64(*actor.TeamID)
		// A senior operator manages every category of the team; an ordinary
		// operator stays inside the categories assigned to them.
		if actor.Role == identity.RoleOperator {
			if len(actor.CategoryIDs) == 0 {
				return []model.ComposeStrategy{}, nil
			}
			filter.CategoryIDs = actor.CategoryIDs
		}
	}
	if actor.Role == identity.RoleOperator {
		filter.OperatorID = int64(actor.ID)
	}
	items, err := s.store.List(filter)
	if err != nil {
		return nil, err
	}
	return s.withOperatorNames(items)
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
		if err := s.validateOutputSettings(item, true); err != nil {
			if errors.Is(err, ErrStrategyTemplateUnavailable) {
				return model.ComposeStrategy{}, ErrStrategyNotFound
			}
			return model.ComposeStrategy{}, err
		}
	}
	return s.withOperatorName(item)
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
	if !inCategoryScope(actor, teamID, input.CategoryID) {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	if err := validateStrategyFields(input.Name); err != nil {
		return model.ComposeStrategy{}, err
	}
	category, found, err := s.resolveCategory(input.CategoryID)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found || !category.Enabled {
		return model.ComposeStrategy{}, fmt.Errorf("%w: category is unavailable", ErrStrategyInvalid)
	}
	if input.OperatorID <= 0 {
		return model.ComposeStrategy{}, fmt.Errorf("%w: operator_id is required", ErrStrategyInvalid)
	}
	if _, ok, err := resolveComposeOperator(s.operators, teamID, input.CategoryID, input.OperatorID); err != nil {
		return model.ComposeStrategy{}, err
	} else if !ok {
		return model.ComposeStrategy{}, fmt.Errorf("%w: operator must be an enabled team operator responsible for the category", ErrStrategyInvalid)
	}
	settings, settingsJSON, config, err := s.prepareOutputSettings(input.TemplateID, input.TemplateVersion, input.OutputSettings, input.Config, true)
	if err != nil {
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
	operatorID := input.OperatorID
	item := model.ComposeStrategy{TeamID: teamID, CategoryID: input.CategoryID, OperatorID: &operatorID, TemplateID: input.TemplateID, TemplateVersion: input.TemplateVersion, Name: strings.TrimSpace(input.Name), Config: config, OutputSettings: settings, OutputSettingsJSON: settingsJSON, ExecutionMode: input.ExecutionMode, Schedule: input.Schedule, PriorityRules: input.PriorityRules, RulesJSON: rulesJSON, SingleLimit: input.SingleLimit, DailyLimit: input.DailyLimit, Status: model.ComposeStrategyEnabled, CreatedBy: int64(actor.ID), UpdatedBy: int64(actor.ID)}
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	stored, err := s.store.CreateAudited(item, "compose_strategy.create")
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.withOperatorName(stored)
}

func (s *StrategyService) Update(actor identity.PublicUser, id int64, input UpdateStrategyInput) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := validateStrategyFields(input.Name); err != nil {
		return model.ComposeStrategy{}, err
	}
	operatorID := int64(0)
	if item.OperatorID != nil {
		operatorID = *item.OperatorID
	}
	if input.OperatorID > 0 {
		operatorID = input.OperatorID
	}
	// An enabled strategy must stay runnable: an edit that leaves it enabled
	// with no valid assignee would be a strategy that can never execute.
	if item.Status == model.ComposeStrategyEnabled {
		if _, ok, err := resolveComposeOperator(s.operators, item.TeamID, item.CategoryID, operatorID); err != nil {
			return model.ComposeStrategy{}, err
		} else if !ok {
			return model.ComposeStrategy{}, fmt.Errorf("%w: operator must be an enabled team operator responsible for the category", ErrStrategyInvalid)
		}
	}
	groups := input.OutputSettings
	if groups == nil && len(item.OutputSettingsJSON) > 0 {
		groups = item.EffectiveOutputSettings()
	}
	settings, settingsJSON, config, err := s.prepareOutputSettings(item.TemplateID, item.TemplateVersion, groups, input.Config, false)
	if err != nil {
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
	item.Name, item.Config, item.UpdatedBy = strings.TrimSpace(input.Name), config, int64(actor.ID)
	item.OutputSettings, item.OutputSettingsJSON = settings, settingsJSON
	item.ExecutionMode, item.Schedule, item.PriorityRules, item.RulesJSON = input.ExecutionMode, input.Schedule, input.PriorityRules, rulesJSON
	item.SingleLimit, item.DailyLimit = input.SingleLimit, input.DailyLimit
	if input.OperatorID > 0 {
		value := input.OperatorID
		item.OperatorID = &value
	}
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	stored, err := s.store.UpdateAudited(item, "compose_strategy.update")
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.withOperatorName(stored)
}

func (s *StrategyService) Copy(actor identity.PublicUser, id int64, name string) (model.ComposeStrategy, error) {
	item, err := s.managed(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if err := validateStrategyFields(name); err != nil {
		return model.ComposeStrategy{}, err
	}
	if item.OperatorID == nil {
		return model.ComposeStrategy{}, fmt.Errorf("%w: assign an operator before copying", ErrStrategyInvalid)
	}
	if _, ok, err := resolveComposeOperator(s.operators, item.TeamID, item.CategoryID, *item.OperatorID); err != nil {
		return model.ComposeStrategy{}, err
	} else if !ok {
		return model.ComposeStrategy{}, fmt.Errorf("%w: the strategy's operator is no longer available", ErrStrategyInvalid)
	}
	if err := s.validateOutputSettings(item, true); err != nil {
		return model.ComposeStrategy{}, err
	}
	item.ID, item.Name, item.Status, item.CreatedBy, item.UpdatedBy = 0, strings.TrimSpace(name), model.ComposeStrategyEnabled, int64(actor.ID), int64(actor.ID)
	item.ExecutionMode, item.Schedule, item.ScheduleJSON, item.NextRunAt = model.ComposeExecutionManual, nil, nil, nil
	stored, err := s.store.CreateAudited(item, "compose_strategy.copy")
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.withOperatorName(stored)
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
		if err := s.validateOutputSettings(item, true); err != nil {
			return model.ComposeStrategy{}, err
		}
		if err := model.ValidateComposePriorityRules(item.PriorityRules); err != nil {
			return model.ComposeStrategy{}, fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
		}
		operatorID := int64(0)
		if item.OperatorID != nil {
			operatorID = *item.OperatorID
		}
		if _, ok, err := resolveComposeOperator(s.operators, item.TeamID, item.CategoryID, operatorID); err != nil {
			return model.ComposeStrategy{}, err
		} else if !ok {
			return model.ComposeStrategy{}, fmt.Errorf("%w: assign an available operator before enabling", ErrStrategyInvalid)
		}
	}
	item.Status, item.UpdatedBy = status, int64(actor.ID)
	item.NextRunAt, err = nextRunForStrategy(item, s.now())
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	stored, err := s.store.UpdateAudited(item, "compose_strategy.status")
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.withOperatorName(stored)
}

func (s *StrategyService) managed(actor identity.PublicUser, id int64) (model.ComposeStrategy, error) {
	if !canManageStrategies(actor) {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	item, found, err := s.store.Find(id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found || !inCategoryScope(actor, item.TeamID, item.CategoryID) {
		return model.ComposeStrategy{}, ErrStrategyNotFound
	}
	return item, nil
}

func canManageStrategies(actor identity.PublicUser) bool {
	return actor.Status == identity.UserStatusEnabled && (actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin)
}

func inCategoryScope(actor identity.PublicUser, teamID int64, categoryID identity.CategoryID) bool {
	if actor.Status != identity.UserStatusEnabled {
		return false
	}
	if actor.Role == identity.RoleAdmin {
		return true
	}
	if actor.TeamID == nil || int64(*actor.TeamID) != teamID {
		return false
	}
	// Senior operators manage every category of their team; the category only binds
	// the strategy's operator, not the person who manages the strategy.
	if actor.Role == identity.RoleSeniorOperator {
		return true
	}
	for _, id := range actor.CategoryIDs {
		if id == categoryID {
			return true
		}
	}
	return false
}

func canReadStrategy(actor identity.PublicUser, item model.ComposeStrategy) bool {
	if !inCategoryScope(actor, item.TeamID, item.CategoryID) {
		return false
	}
	if actor.Role == identity.RoleOperator {
		return item.Status == model.ComposeStrategyEnabled && item.OperatorID != nil && *item.OperatorID == int64(actor.ID)
	}
	return actor.Role == identity.RoleSeniorOperator || actor.Role == identity.RoleAdmin
}

func (s *StrategyService) withOperatorName(item model.ComposeStrategy) (model.ComposeStrategy, error) {
	items, err := s.withOperatorNames([]model.ComposeStrategy{item})
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return items[0], nil
}

// withOperatorNames resolves every row's operator in one directory read. Rows
// without an operator keep an empty name: the page shows 未记录 rather than
// presenting the creator as the operator.
func (s *StrategyService) withOperatorNames(items []model.ComposeStrategy) ([]model.ComposeStrategy, error) {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item.OperatorID != nil {
			ids = append(ids, *item.OperatorID)
		}
	}
	names, err := composeOperatorNameLookup(s.operators, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].OutputSettings = items[i].EffectiveOutputSettings()
		if items[i].OperatorID != nil {
			items[i].OperatorName = names[*items[i].OperatorID]
		}
	}
	return items, nil
}

func validateStrategyFields(name string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(name)); n == 0 || n > 120 {
		return fmt.Errorf("%w: name must contain 1-120 characters", ErrStrategyInvalid)
	}
	return nil
}

func validateStrategyExecution(mode string, schedule *model.ComposeSchedule, singleLimit, dailyLimit int) error {
	if singleLimit < 1 || singleLimit > 100 || dailyLimit < 0 || dailyLimit > 1000 || (mode == model.ComposeExecutionScheduled && dailyLimit < singleLimit) {
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

func (s *StrategyService) prepareOutputSettings(templateID int64, version uint32, groups []model.ComposeOutputSetting, legacy json.RawMessage, active bool) ([]model.ComposeOutputSetting, json.RawMessage, json.RawMessage, error) {
	legacyInput := groups == nil
	if groups == nil {
		groups = (model.ComposeStrategy{Config: legacy}).EffectiveOutputSettings()
	}
	normalized, err := model.NormalizeComposeOutputSettings(groups)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %v", ErrStrategyInvalid, err)
	}
	var first json.RawMessage
	for i, group := range normalized {
		raw, err := json.Marshal(group.Params)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%w: params cannot be encoded", ErrStrategyInvalid)
		}
		if err = s.validateConfig(templateID, version, raw, active); err != nil {
			return nil, nil, nil, err
		}
		if i == 0 {
			first = raw
		}
	}
	if legacyInput {
		first = append(json.RawMessage(nil), legacy...)
	}
	encoded, err := json.Marshal(normalized)
	return normalized, encoded, first, err
}
func (s *StrategyService) validateOutputSettings(item model.ComposeStrategy, active bool) error {
	_, _, _, err := s.prepareOutputSettings(item.TemplateID, item.TemplateVersion, item.EffectiveOutputSettings(), item.Config, active)
	return err
}

// The old List method remains for internal compatibility; HTTP always calls
// Page so no visible strategy directory is silently truncated at 200 rows.
type strategyPageRepository interface {
	Page(context.Context, repository.StrategyFilter, int, int) (repository.ComposeStrategyPage, error)
}

func ListComposeStrategyPage(ctx context.Context, actor identity.PublicUser, query ComposeStrategyListQuery) (repository.ComposeStrategyPage, error) {
	return wiredStrategyService().Page(ctx, actor, query)
}
func (s *StrategyService) Page(ctx context.Context, actor identity.PublicUser, query ComposeStrategyListQuery) (repository.ComposeStrategyPage, error) {
	page, size := query.Page, query.PageSize
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	result := repository.ComposeStrategyPage{Items: []model.ComposeStrategy{}, Page: page, PageSize: size}
	if page < 1 || size < 1 || size > 100 || page-1 > math.MaxInt/size || query.TeamID < 0 || query.OperatorID < 0 || query.CategoryID < 0 || query.TemplateID < 0 || len([]rune(strings.TrimSpace(query.Search))) > 100 {
		return result, ErrStrategyInvalid
	}
	if query.Status != "" && query.Status != model.ComposeStrategyEnabled && query.Status != model.ComposeStrategyDisabled {
		return result, ErrStrategyInvalid
	}
	scope, err := resolveComposeListScope(actor, query.TeamID, query.OperatorID, "")
	if err != nil {
		if errors.Is(err, errComposeScopeForbidden) {
			return result, ErrStrategyForbidden
		}
		return result, ErrStrategyInvalid
	}
	f := repository.StrategyFilter{TeamID: scope.TeamID, OperatorID: scope.OperatorID, ActiveOnly: actor.Role == identity.RoleOperator, Search: strings.TrimSpace(query.Search), CategoryID: query.CategoryID, TemplateID: query.TemplateID, Status: query.Status}
	if actor.Role == identity.RoleOperator {
		if len(actor.CategoryIDs) == 0 {
			return result, nil
		}
		if query.CategoryID > 0 && !slices.Contains(actor.CategoryIDs, query.CategoryID) {
			return result, ErrStrategyForbidden
		}
		f.CategoryIDs = append([]identity.CategoryID(nil), actor.CategoryIDs...)
	}
	store, ok := s.store.(strategyPageRepository)
	if !ok {
		return result, errors.New("strategy paging unavailable")
	}
	result, err = store.Page(ctx, f, page, size)
	if err != nil {
		return result, err
	}
	result.Items, err = s.withOperatorNames(result.Items)
	return result, err
}
