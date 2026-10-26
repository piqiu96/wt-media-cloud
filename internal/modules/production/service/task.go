package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

var (
	ErrComposeTaskInvalid        = errors.New("compose task input is invalid")
	ErrComposeTaskForbidden      = errors.New("compose task operation is forbidden")
	ErrComposeTaskNotFound       = errors.New("compose task not found")
	ErrComposeTaskNotCancellable = errors.New("compose task is not cancellable")
	ErrComposeTaskNotRetryable   = errors.New("compose task is not retryable")
	ErrComposeTaskConflict       = errors.New("compose task conflicts with a task this material already has")
)

type CreateComposeTaskItemInput struct {
	MaterialID int64  `json:"material_id"`
	RequestKey string `json:"request_key"`
}

type CreateComposeTasksInput struct {
	StrategyID int64 `json:"strategy_id"`
	// OperatorID is optional on the wire; when present it must name the
	// strategy's own operator, because a task snapshots that assignment.
	OperatorID int64 `json:"operator_id"`
	// Name is the operator's label for this batch. Blank keeps the derived
	// material · strategy name; otherwise the whole batch shares it, numbered
	// in submission order (model.ComposeTaskBatchDisplayName).
	Name           string                       `json:"name"`
	ConfigOverride json.RawMessage              `json:"config_override"`
	Items          []CreateComposeTaskItemInput `json:"items"`
}

type ListComposeTasksInput struct {
	Status        string    `json:"status"`
	TriggerType   string    `json:"trigger_type"`
	StrategyID    int64     `json:"compose_strategy_id"`
	TeamID        int64     `json:"team_id"`
	OperatorID    int64     `json:"operator_id"`
	OperatorScope string    `json:"operator_scope"`
	CreatedFrom   time.Time `json:"created_from"`
	CreatedBefore time.Time `json:"created_before"`
}

// CreateComposeTaskFailure is one rejected item of a batch command. Err is one
// of the named compose task sentinels so the handler maps it to the contract
// errcode; Message is the per-item reason the page shows.
type CreateComposeTaskFailure struct {
	MaterialID int64
	RequestKey string
	Err        error
	Message    string
}

type CreateComposeTasksResult struct {
	Created []model.ComposeTask
	Failed  []CreateComposeTaskFailure
}

type ComposeTaskRepository interface {
	Find(int64) (model.ComposeTask, bool, error)
	FindByRequestKey(string) (model.ComposeTask, bool, error)
	List(repository.ComposeTaskFilter) ([]model.ComposeTask, error)
	CreateForUsage(model.ComposeTask, string) (model.ComposeTask, bool, error)
	CancelAudited(model.ComposeTask, int64, string) (bool, error)
	RequestComposeCancel(taskID, actorID int64, action string) (bool, error)
	Retry(model.ComposeTask, int64, string) (bool, error)
	ListAttempts(int64) ([]model.ComposeTaskAttempt, error)
	MaterialStates(int64, []int64) ([]model.ComposeMaterialState, error)
}

type composeTaskStrategySource interface {
	Find(int64) (model.ComposeStrategy, bool, error)
	FindTemplate(int64, uint32) (model.ComposeTemplate, bool, error)
}

type composeTaskMaterialSource interface {
	FindMaterial(int64) (model.Material, bool, error)
}

type ComposeTaskService struct {
	tasks      ComposeTaskRepository
	strategies composeTaskStrategySource
	materials  composeTaskMaterialSource
	operators  func() ([]identity.PublicUser, error)
}

func NewComposeTaskService(tasks ComposeTaskRepository, strategies composeTaskStrategySource, materials composeTaskMaterialSource) *ComposeTaskService {
	return &ComposeTaskService{tasks: tasks, strategies: strategies, materials: materials}
}

func wiredComposeTaskService() *ComposeTaskService {
	service := NewComposeTaskService(repository.ComposeTaskStore{}, repository.StrategyStore{}, mysqlStore{})
	service.operators = identity.ListUsers
	return service
}

func ListComposeTasks(actor identity.PublicUser, input ListComposeTasksInput) ([]model.ComposeTask, error) {
	return wiredComposeTaskService().List(actor, input)
}

func GetComposeTask(actor identity.PublicUser, id int64) (model.ComposeTask, error) {
	return wiredComposeTaskService().Get(actor, id)
}

func CreateComposeTasks(actor identity.PublicUser, input CreateComposeTasksInput) (CreateComposeTasksResult, error) {
	return wiredComposeTaskService().Create(actor, input)
}

func CancelComposeTask(actor identity.PublicUser, id int64) (model.ComposeTask, error) {
	return wiredComposeTaskService().Cancel(actor, id)
}

// Retry requeues one failed or cancelled task for another attempt (M4-G, PRD
// §5.8.5): visibility first, then the senior-operator gate — retry is a §5.10.3
// senior capability, a plain operator only reads. The repository's guarded
// update refuses anything but a cancelled row or a failed row other than
// execution_uncertain; a refusal answers compose_task_not_retryable, unless a
// concurrent retry already put the task back to pending, which answers with
// that row. The snapshot never changes — operators who need different
// parameters create a new task.
func (s *ComposeTaskService) Retry(actor identity.PublicUser, id int64) (model.ComposeTask, error) {
	item, found, err := s.tasks.Find(id)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if !found || !canReadComposeTask(actor, item) {
		return model.ComposeTask{}, ErrComposeTaskNotFound
	}
	if actor.Role != identity.RoleAdmin && actor.Role != identity.RoleSeniorOperator {
		return model.ComposeTask{}, ErrComposeTaskForbidden
	}
	ok, err := s.tasks.Retry(item, int64(actor.ID), "compose_task.retry")
	if err != nil {
		return model.ComposeTask{}, err
	}
	stored, found, err := s.tasks.Find(id)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if !found {
		return model.ComposeTask{}, ErrComposeTaskNotFound
	}
	if !ok && stored.Status != model.ComposeTaskPending {
		return model.ComposeTask{}, ErrComposeTaskNotRetryable
	}
	return stored, nil
}

func (s *ComposeTaskService) ListAttempts(actor identity.PublicUser, id int64) ([]model.ComposeTaskAttempt, error) {
	if _, err := s.Get(actor, id); err != nil {
		return nil, err
	}
	return s.tasks.ListAttempts(id)
}

func RetryComposeTask(actor identity.PublicUser, id int64) (model.ComposeTask, error) {
	return wiredComposeTaskService().Retry(actor, id)
}

func ListComposeTaskAttempts(actor identity.PublicUser, id int64) ([]model.ComposeTaskAttempt, error) {
	return wiredComposeTaskService().ListAttempts(actor, id)
}

// Create runs the command-level gate once — strategy readable and active,
// template version usable, merged config schema-valid — then creates one task
// per item. A failed item never stops the others; the response splits created
// from failed per item (PRD §5.4.4). The request key is resolved before any
// per-item check so a retried item answers its existing task even if its
// usage has since been removed.
func (s *ComposeTaskService) Create(actor identity.PublicUser, input CreateComposeTasksInput) (CreateComposeTasksResult, error) {
	if actor.Status != identity.UserStatusEnabled || (actor.Role != identity.RoleOperator && actor.Role != identity.RoleSeniorOperator && actor.Role != identity.RoleAdmin) {
		return CreateComposeTasksResult{}, ErrComposeTaskForbidden
	}
	if input.StrategyID <= 0 {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: strategy_id is required", ErrComposeTaskInvalid)
	}
	if len(input.Items) == 0 || len(input.Items) > 50 {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: items must contain 1-50 entries", ErrComposeTaskInvalid)
	}
	// Counted in runes: the limit is what fits the column and reads as a name,
	// and a Chinese label must not be cut three times shorter than a Latin one.
	if name := strings.TrimSpace(input.Name); len([]rune(name)) > 60 {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: name must be at most 60 characters", ErrComposeTaskInvalid)
	}
	keys := make(map[string]struct{}, len(input.Items))
	for _, item := range input.Items {
		key := strings.TrimSpace(item.RequestKey)
		if l := len(key); l < 8 || l > 64 {
			return CreateComposeTasksResult{}, fmt.Errorf("%w: request_key must be 8-64 characters", ErrComposeTaskInvalid)
		}
		if item.MaterialID <= 0 {
			return CreateComposeTasksResult{}, fmt.Errorf("%w: material_id is required", ErrComposeTaskInvalid)
		}
		if _, seen := keys[key]; seen {
			return CreateComposeTasksResult{}, fmt.Errorf("%w: request_key %s repeats inside the command", ErrComposeTaskInvalid, key)
		}
		keys[key] = struct{}{}
	}
	strategy, found, err := s.strategies.Find(input.StrategyID)
	if err != nil {
		return CreateComposeTasksResult{}, err
	}
	if !found || !canReadStrategy(actor, strategy) {
		return CreateComposeTasksResult{}, ErrComposeTaskNotFound
	}
	if strategy.Status != model.ComposeStrategyEnabled {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: strategy is not active", ErrComposeTaskInvalid)
	}
	if strategy.OperatorID == nil {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: strategy has no operator assigned", ErrComposeTaskInvalid)
	}
	if input.OperatorID > 0 && input.OperatorID != *strategy.OperatorID {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: operator_id must match the strategy's operator", ErrComposeTaskInvalid)
	}
	template, err := s.usableTemplate(strategy)
	if err != nil {
		return CreateComposeTasksResult{}, err
	}
	merged, err := s.mergedConfig(template, strategy, input.ConfigOverride)
	if err != nil {
		return CreateComposeTasksResult{}, err
	}
	res := CreateComposeTasksResult{Created: []model.ComposeTask{}, Failed: []CreateComposeTaskFailure{}}
	for index, item := range input.Items {
		key := strings.TrimSpace(item.RequestKey)
		if existing, found, err := s.tasks.FindByRequestKey(key); err != nil {
			return CreateComposeTasksResult{}, err
		} else if found {
			if existing.CreatedBy == int64(actor.ID) {
				res.Created = append(res.Created, existing)
			} else {
				res.Failed = append(res.Failed, CreateComposeTaskFailure{MaterialID: item.MaterialID, RequestKey: key, Err: ErrComposeTaskForbidden, Message: "request_key 已被其他任务占用"})
			}
			continue
		}
		failure, err := s.materialFailure(actor, strategy, item.MaterialID)
		if err != nil {
			return CreateComposeTasksResult{}, err
		}
		if failure != nil {
			failure.RequestKey = key
			res.Failed = append(res.Failed, *failure)
			continue
		}
		material, found, err := s.materials.FindMaterial(item.MaterialID)
		if err != nil {
			return CreateComposeTasksResult{}, err
		}
		if !found {
			return CreateComposeTasksResult{}, ErrComposeTaskNotFound
		}
		task := model.ComposeTask{
			TeamID: strategy.TeamID, GameID: strategy.GameID, OperatorID: strategy.OperatorID, TriggerType: "manual",
			DisplayName: model.ComposeTaskBatchDisplayName(input.Name, index, len(input.Items), material.Title, item.MaterialID, strategy.Name),
			MaterialID:  item.MaterialID, StrategyID: strategy.ID,
			TemplateCode: template.Code, TemplateVersion: strategy.TemplateVersion,
			ConfigSnapshot: merged, Status: model.ComposeTaskPending,
			RequestKey: key, CreatedBy: int64(actor.ID),
		}
		created, written, err := s.tasks.CreateForUsage(task, "compose_task.create")
		if errors.Is(err, repository.ErrComposeTaskMaterialUsed) {
			// The Chinese wording lives with the other compose-task messages in
			// the handler's mapping table, so a batch of five says it five times
			// in one voice.
			res.Failed = append(res.Failed, CreateComposeTaskFailure{MaterialID: item.MaterialID, RequestKey: key, Err: ErrComposeTaskConflict})
			continue
		}
		if err != nil {
			return CreateComposeTasksResult{}, err
		}
		if !written {
			res.Failed = append(res.Failed, CreateComposeTaskFailure{MaterialID: item.MaterialID, RequestKey: key, Err: ErrComposeTaskInvalid, Message: "素材不在我的素材或已移出"})
			continue
		}
		res.Created = append(res.Created, created)
	}
	// The tasks are already written; a directory read failure must not turn a
	// successful create into an error the page would retry. Names fall back to
	// empty and the page still has operator_id.
	if decorated, err := s.withOperatorNames(res.Created); err == nil {
		res.Created = decorated
	}
	return res, nil
}

func (s *ComposeTaskService) Get(actor identity.PublicUser, id int64) (model.ComposeTask, error) {
	item, found, err := s.tasks.Find(id)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if !found || !canReadComposeTask(actor, item) {
		return model.ComposeTask{}, ErrComposeTaskNotFound
	}
	return s.withOperatorName(item)
}

func (s *ComposeTaskService) List(actor identity.PublicUser, input ListComposeTasksInput) ([]model.ComposeTask, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrComposeTaskForbidden
	}
	filter := repository.ComposeTaskFilter{StrategyID: input.StrategyID, CreatedFrom: input.CreatedFrom, CreatedBefore: input.CreatedBefore}
	if input.TriggerType != "" {
		if input.TriggerType != "manual" && input.TriggerType != "scheduled" {
			return nil, fmt.Errorf("%w: unknown trigger_type %s", ErrComposeTaskInvalid, input.TriggerType)
		}
		filter.TriggerType = input.TriggerType
	}
	if input.Status != "" {
		status := model.ComposeTaskStatus(input.Status)
		switch status {
		case model.ComposeTaskPending, model.ComposeTaskRunning, model.ComposeTaskSuccess, model.ComposeTaskFailed, model.ComposeTaskCancelled:
			filter.Status = status
		default:
			return nil, fmt.Errorf("%w: unknown status %s", ErrComposeTaskInvalid, input.Status)
		}
	}
	scope, err := resolveComposeListScope(actor, input.TeamID, input.OperatorID, input.OperatorScope)
	if err != nil {
		if errors.Is(err, errComposeScopeForbidden) {
			return nil, ErrComposeTaskForbidden
		}
		return nil, fmt.Errorf("%w: %v", ErrComposeTaskInvalid, err)
	}
	filter.TeamID, filter.OperatorID, filter.IncludeUnownedCreatedBy = scope.TeamID, scope.OperatorID, scope.IncludeUnowned
	// Operators stay inside the games they cover; seniors and admins see every
	// game of the team they are allowed to read.
	if actor.Role == identity.RoleOperator {
		filter.GameIDs = actor.GameIDs
	}
	items, err := s.tasks.List(filter)
	if err != nil {
		return nil, err
	}
	return s.withOperatorNames(items)
}

func (s *ComposeTaskService) withOperatorName(item model.ComposeTask) (model.ComposeTask, error) {
	items, err := s.withOperatorNames([]model.ComposeTask{item})
	if err != nil {
		return model.ComposeTask{}, err
	}
	return items[0], nil
}

func (s *ComposeTaskService) withOperatorNames(items []model.ComposeTask) ([]model.ComposeTask, error) {
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
		if items[i].OperatorID != nil {
			items[i].OperatorName = names[*items[i].OperatorID]
		}
	}
	return items, nil
}

// ComposeMaterialStateLimit bounds one material-states read. The create drawer
// sends its whole candidate list in one call; a larger request is a client
// mistake, not a large material library. The handler parses to the same number.
const ComposeMaterialStateLimit = 200

// MaterialStates answers the create drawer's 未使用过 / 已使用过 split: what this
// strategy has already done with each candidate material. It is history read
// through the strategy, so it needs exactly the strategy's own visibility and
// nothing more — the material ids only group the counts and never reveal a
// material the actor could not otherwise see.
func (s *ComposeTaskService) MaterialStates(actor identity.PublicUser, strategyID int64, materialIDs []int64) ([]model.ComposeMaterialState, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrComposeTaskForbidden
	}
	if strategyID <= 0 || len(materialIDs) == 0 || len(materialIDs) > ComposeMaterialStateLimit {
		return nil, fmt.Errorf("%w: material_ids must hold 1-%d ids", ErrStrategyInvalid, ComposeMaterialStateLimit)
	}
	strategy, found, err := s.strategies.Find(strategyID)
	if err != nil {
		return nil, err
	}
	if !found || !canReadStrategy(actor, strategy) {
		return nil, ErrStrategyNotFound
	}
	return s.tasks.MaterialStates(strategyID, materialIDs)
}

func ListComposeStrategyMaterialStates(actor identity.PublicUser, strategyID int64, materialIDs []int64) ([]model.ComposeMaterialState, error) {
	return wiredComposeTaskService().MaterialStates(actor, strategyID, materialIDs)
}

// Cancel flips a pending task to cancelled, and asks the worker's controlled
// stop for a running one (spec §5.4): the running row keeps answering running
// with cancel_requested set until the worker has stopped FFmpeg and disposed
// its temp files. Reading and cancelling share the visibility gate — cancel
// needs no separate permission. When a conditional update writes nothing the
// task is reloaded and named by what it is now: gone answers not-found, a
// terminal row answers not-cancellable.
func (s *ComposeTaskService) Cancel(actor identity.PublicUser, id int64) (model.ComposeTask, error) {
	item, found, err := s.tasks.Find(id)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if !found || !canReadComposeTask(actor, item) {
		return model.ComposeTask{}, ErrComposeTaskNotFound
	}
	switch item.Status {
	case model.ComposeTaskPending:
		ok, err := s.tasks.CancelAudited(item, int64(actor.ID), "compose_task.cancel")
		if err != nil {
			return model.ComposeTask{}, err
		}
		stored, found, err := s.tasks.Find(id)
		if err != nil {
			return model.ComposeTask{}, err
		}
		if !found {
			return model.ComposeTask{}, ErrComposeTaskNotFound
		}
		if !ok && stored.Status != model.ComposeTaskCancelled {
			return model.ComposeTask{}, ErrComposeTaskNotCancellable
		}
		return stored, nil
	case model.ComposeTaskRunning:
		requested, err := s.tasks.RequestComposeCancel(id, int64(actor.ID), "compose_task.cancel")
		if err != nil {
			return model.ComposeTask{}, err
		}
		stored, found, err := s.tasks.Find(id)
		if err != nil {
			return model.ComposeTask{}, err
		}
		if !found {
			return model.ComposeTask{}, ErrComposeTaskNotFound
		}
		// The request writes nothing when the flag is already set (answered
		// by the reload) or the worker finished the row in between (terminal,
		// refused below). A running row without the flag after a refused
		// request is a state nothing should be able to produce.
		if stored.Status == model.ComposeTaskRunning && (requested || stored.CancelRequested) {
			return stored, nil
		}
	}
	return model.ComposeTask{}, ErrComposeTaskNotCancellable
}

// canReadComposeTask scopes the page: admins see everything, seniors see
// every game of their team, operators only their own tasks inside the games
// they cover.
func canReadComposeTask(actor identity.PublicUser, item model.ComposeTask) bool {
	if actor.Status != identity.UserStatusEnabled {
		return false
	}
	if actor.Role == identity.RoleAdmin {
		return true
	}
	if actor.TeamID == nil || int64(*actor.TeamID) != item.TeamID {
		return false
	}
	if actor.Role == identity.RoleSeniorOperator {
		return true
	}
	if actor.Role != identity.RoleOperator {
		return false
	}
	inGame := false
	for _, gameID := range actor.GameIDs {
		if gameID == item.GameID {
			inGame = true
		}
	}
	if !inGame {
		return false
	}
	// Historical rows have no operator. Their creator still reaches them,
	// but the row is not attributed to the creator.
	return (item.OperatorID != nil && *item.OperatorID == int64(actor.ID)) || (item.OperatorID == nil && item.CreatedBy == int64(actor.ID))
}

// materialFailure returns the named rejection for one material, or nil when it
// may feed the strategy. Admins browse globally; everyone else stays inside
// their team, and the material's game must be the strategy's game when the
// source row knows its game.
func (s *ComposeTaskService) materialFailure(actor identity.PublicUser, strategy model.ComposeStrategy, materialID int64) (*CreateComposeTaskFailure, error) {
	material, found, err := s.materials.FindMaterial(materialID)
	if err != nil {
		return nil, err
	}
	invalid := func(message string) *CreateComposeTaskFailure {
		return &CreateComposeTaskFailure{MaterialID: materialID, Err: ErrComposeTaskInvalid, Message: message}
	}
	if !found {
		return invalid("素材不存在"), nil
	}
	if actor.Role != identity.RoleAdmin && (actor.TeamID == nil || material.TeamID != *actor.TeamID) {
		return &CreateComposeTaskFailure{MaterialID: materialID, Err: ErrComposeTaskForbidden, Message: "素材不在你的团队范围内"}, nil
	}
	if material.GameID != nil && *material.GameID != strategy.GameID {
		return invalid("素材的游戏与策略不匹配"), nil
	}
	return nil, nil
}

// usableTemplate resolves the strategy's bound template version and rejects
// anything the deployed executor does not own — the same gate strategy
// creation runs, now replayed at task time.
func (s *ComposeTaskService) usableTemplate(strategy model.ComposeStrategy) (model.ComposeTemplate, error) {
	item, found, err := s.strategies.FindTemplate(strategy.TemplateID, strategy.TemplateVersion)
	if err != nil {
		return model.ComposeTemplate{}, err
	}
	if !found {
		return model.ComposeTemplate{}, ErrStrategyTemplateUnavailable
	}
	definition, err := executor.DeployedDefinition(item.Code, item.Version)
	if err != nil || item.ExecutorID != definition.ExecutorID || item.Status != "active" || !sameJSON(item.Schema, definition.Schema) {
		return model.ComposeTemplate{}, ErrStrategyTemplateUnavailable
	}
	return item, nil
}

// mergedConfig layers the command's optional config_override over the strategy
// config and schema-validates the merge — the merge, not the override alone,
// is what the snapshot stores (ADR-0021).
func (s *ComposeTaskService) mergedConfig(template model.ComposeTemplate, strategy model.ComposeStrategy, override json.RawMessage) (json.RawMessage, error) {
	definition, err := executor.DeployedDefinition(template.Code, template.Version)
	if err != nil {
		return nil, ErrStrategyTemplateUnavailable
	}
	if len(override) == 0 {
		if _, err := definition.Executor.ValidateParams(strategy.Config); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComposeTaskInvalid, err)
		}
		return append(json.RawMessage(nil), strategy.Config...), nil
	}
	var base, extra map[string]any
	if len(strategy.Config) > 0 {
		if err := json.Unmarshal(strategy.Config, &base); err != nil {
			return nil, fmt.Errorf("%w: strategy config is not an object", ErrComposeTaskInvalid)
		}
	}
	if len(override) > 0 {
		if err := json.Unmarshal(override, &extra); err != nil || extra == nil {
			return nil, fmt.Errorf("%w: config_override must be an object", ErrComposeTaskInvalid)
		}
		if _, changesURL := extra["guide_video_url"]; changesURL {
			return nil, fmt.Errorf("%w: guide video address belongs to the strategy", ErrComposeTaskInvalid)
		}
	}
	merged := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	if _, err := definition.Executor.ValidateParams(out); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrComposeTaskInvalid, err)
	}
	return out, nil
}
