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
)

type CreateComposeTaskItemInput struct {
	MaterialID int64  `json:"material_id"`
	RequestKey string `json:"request_key"`
}

type CreateComposeTasksInput struct {
	StrategyID     int64                        `json:"strategy_id"`
	ConfigOverride json.RawMessage              `json:"config_override"`
	Items          []CreateComposeTaskItemInput `json:"items"`
}

type ListComposeTasksInput struct {
	Status        string    `json:"status"`
	TriggerType   string    `json:"trigger_type"`
	StrategyID    int64     `json:"compose_strategy_id"`
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
	RetryFailed(model.ComposeTask, int64, string) (bool, error)
	ListAttempts(int64) ([]model.ComposeTaskAttempt, error)
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
}

func NewComposeTaskService(tasks ComposeTaskRepository, strategies composeTaskStrategySource, materials composeTaskMaterialSource) *ComposeTaskService {
	return &ComposeTaskService{tasks: tasks, strategies: strategies, materials: materials}
}

func wiredComposeTaskService() *ComposeTaskService {
	return NewComposeTaskService(repository.ComposeTaskStore{}, repository.StrategyStore{}, mysqlStore{})
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

// Retry requeues one failed task for another attempt (M4-G, PRD §5.8.5):
// visibility first, then the senior-operator gate — retry is a §5.10.3
// senior capability, a plain operator only reads. The repository's guarded
// update refuses a non-failed row or an error code outside the retryable
// set; either refusal answers compose_task_not_retryable. The snapshot never
// changes — operators who need different parameters create a new task.
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
	ok, err := s.tasks.RetryFailed(item, int64(actor.ID), "compose_task.retry")
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
	if strategy.Status != "active" {
		return CreateComposeTasksResult{}, fmt.Errorf("%w: strategy is not active", ErrComposeTaskInvalid)
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
	for _, item := range input.Items {
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
		task := model.ComposeTask{
			TeamID: strategy.TeamID, GameID: strategy.GameID, TriggerType: "manual",
			MaterialID: item.MaterialID, StrategyID: strategy.ID,
			TemplateCode: template.Code, TemplateVersion: strategy.TemplateVersion,
			ConfigSnapshot: merged, Status: model.ComposeTaskPending,
			RequestKey: key, CreatedBy: int64(actor.ID),
		}
		created, written, err := s.tasks.CreateForUsage(task, "compose_task.create")
		if err != nil {
			return CreateComposeTasksResult{}, err
		}
		if !written {
			res.Failed = append(res.Failed, CreateComposeTaskFailure{MaterialID: item.MaterialID, RequestKey: key, Err: ErrComposeTaskInvalid, Message: "素材不在我的素材或已移出"})
			continue
		}
		res.Created = append(res.Created, created)
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
	return item, nil
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
	if actor.Role != identity.RoleAdmin {
		if actor.Role != identity.RoleOperator && actor.Role != identity.RoleSeniorOperator {
			return nil, ErrComposeTaskForbidden
		}
		if actor.TeamID == nil || len(actor.GameIDs) == 0 {
			return []model.ComposeTask{}, nil
		}
		filter.TeamID = int64(*actor.TeamID)
		filter.GameIDs = actor.GameIDs
		if actor.Role == identity.RoleOperator {
			filter.CreatedBy = int64(actor.ID)
		}
	}
	return s.tasks.List(filter)
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
// their team-and-game scope, operators only their own tasks.
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
	inGame := false
	for _, gameID := range actor.GameIDs {
		if gameID == item.GameID {
			inGame = true
		}
	}
	if !inGame {
		return false
	}
	if actor.Role == identity.RoleOperator {
		return item.CreatedBy == int64(actor.ID)
	}
	return actor.Role == identity.RoleSeniorOperator
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
