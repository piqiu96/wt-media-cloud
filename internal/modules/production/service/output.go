package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

var (
	ErrComposeOutputInvalid       = errors.New("compose output input is invalid")
	ErrComposeOutputForbidden     = errors.New("compose output operation is forbidden")
	ErrComposeOutputNotFound      = errors.New("compose output not found")
	ErrComposeOutputNotArchivable = errors.New("compose output is not archivable")
)

type ListComposeOutputsInput struct {
	PublishStatus string `json:"publish_status"`
}

// ComposeOutputRepository is the my-outputs read and the one actor-facing
// write. The row itself is written by the worker's success transaction only.
type ComposeOutputRepository interface {
	List(repository.ComposeOutputFilter) ([]model.CompositeOutput, error)
	Find(int64) (model.CompositeOutput, bool, error)
	Archive(id, actorID int64, now time.Time) (bool, error)
}

type composeOutputTraceMaterialSource interface {
	FindMaterial(int64) (model.Material, bool, error)
}

type composeOutputTraceUsageSource interface {
	FindUsage(int64) (model.MaterialUsage, bool, error)
}

type composeOutputTraceStrategySource interface {
	Find(int64) (model.ComposeStrategy, bool, error)
}

type composeOutputTraceTaskSource interface {
	Find(int64) (model.ComposeTask, bool, error)
}

// composeOutputLinker mints the preview grant; the object address itself never
// enters a body.
type composeOutputLinker interface {
	PresignObjectURL(ctx context.Context, key string) (string, error)
}

// ComposeOutputTraceMaterial is the trace's material projection: the page
// names the source, it does not re-render the material list.
type ComposeOutputTraceMaterial struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type ComposeOutputTraceUsage struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type ComposeOutputTraceStrategy struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	TemplateCode    string          `json:"template_code"`
	TemplateVersion uint32          `json:"template_version"`
	ConfigSnapshot  json.RawMessage `json:"config_snapshot"`
}

type ComposeOutputTraceTask struct {
	ID             int64                   `json:"id"`
	Status         model.ComposeTaskStatus `json:"status"`
	ConfigSnapshot json.RawMessage         `json:"config_snapshot"`
	CreatedAt      time.Time               `json:"created_at"`
}

type ComposeOutputTrace struct {
	Material      ComposeOutputTraceMaterial `json:"material"`
	MaterialUsage ComposeOutputTraceUsage    `json:"material_usage"`
	Strategy      ComposeOutputTraceStrategy `json:"strategy"`
	ComposeTask   ComposeOutputTraceTask     `json:"compose_task"`
}

type ComposeOutputDetail struct {
	Output model.CompositeOutput `json:"output"`
	Trace  ComposeOutputTrace    `json:"trace"`
}

type ComposeOutputService struct {
	outputs    ComposeOutputRepository
	materials  composeOutputTraceMaterialSource
	usages     composeOutputTraceUsageSource
	strategies composeOutputTraceStrategySource
	tasks      composeOutputTraceTaskSource
	links      composeOutputLinker
	now        func() time.Time
}

func NewComposeOutputService(outputs ComposeOutputRepository, materials composeOutputTraceMaterialSource, usages composeOutputTraceUsageSource, strategies composeOutputTraceStrategySource, tasks composeOutputTraceTaskSource, links composeOutputLinker) *ComposeOutputService {
	return &ComposeOutputService{outputs: outputs, materials: materials, usages: usages, strategies: strategies, tasks: tasks, links: links, now: time.Now}
}

func wiredComposeOutputService() *ComposeOutputService {
	return NewComposeOutputService(repository.ComposeOutputStore{}, mysqlStore{}, mysqlStore{}, repository.StrategyStore{}, repository.ComposeTaskStore{}, productionObjectLinker{})
}

func ListComposeOutputs(actor identity.PublicUser, input ListComposeOutputsInput) ([]model.CompositeOutput, error) {
	return wiredComposeOutputService().List(actor, input)
}

func GetComposeOutput(actor identity.PublicUser, id int64) (ComposeOutputDetail, error) {
	return wiredComposeOutputService().Get(actor, id)
}

func ArchiveComposeOutput(actor identity.PublicUser, id int64) (model.CompositeOutput, error) {
	return wiredComposeOutputService().Archive(actor, id)
}

func ComposeOutputVideoURL(ctx context.Context, actor identity.PublicUser, id int64) (string, error) {
	return wiredComposeOutputService().VideoURL(ctx, actor, id)
}

// List scopes the page exactly like the compose-task list — admins everything,
// seniors their team-and-game scope, operators only what they created (PRD
// §5.10.2/§5.10.3). A caller without a scope answers an empty page rather than
// an error, because the scope is legitimate and simply contains nothing.
func (s *ComposeOutputService) List(actor identity.PublicUser, input ListComposeOutputsInput) ([]model.CompositeOutput, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrComposeOutputForbidden
	}
	filter := repository.ComposeOutputFilter{}
	if input.PublishStatus != "" {
		switch input.PublishStatus {
		case "pending_publish", "published", "archived":
			filter.PublishStatus = input.PublishStatus
		default:
			return nil, fmt.Errorf("%w: unknown publish_status %s", ErrComposeOutputInvalid, input.PublishStatus)
		}
	}
	if actor.Role != identity.RoleAdmin {
		if actor.Role != identity.RoleOperator && actor.Role != identity.RoleSeniorOperator {
			return nil, ErrComposeOutputForbidden
		}
		if actor.TeamID == nil || len(actor.GameIDs) == 0 {
			return []model.CompositeOutput{}, nil
		}
		filter.TeamID = int64(*actor.TeamID)
		filter.GameIDs = actor.GameIDs
		if actor.Role == identity.RoleOperator {
			filter.CreatedBy = int64(actor.ID)
		}
	}
	return s.outputs.List(filter)
}

// Get reads one output with its source chain. The chain is a read-side
// projection: the referenced rows answer as they are now, while the frozen
// creation facts ride on the output row itself.
func (s *ComposeOutputService) Get(actor identity.PublicUser, id int64) (ComposeOutputDetail, error) {
	output, err := s.readable(actor, id)
	if err != nil {
		return ComposeOutputDetail{}, err
	}
	detail := ComposeOutputDetail{Output: output}
	if material, found, err := s.materials.FindMaterial(output.MaterialID); err != nil {
		return ComposeOutputDetail{}, err
	} else if found {
		detail.Trace.Material = ComposeOutputTraceMaterial{ID: material.ID, Title: material.Title}
	}
	if usage, found, err := s.usages.FindUsage(output.MaterialUsageID); err != nil {
		return ComposeOutputDetail{}, err
	} else if found {
		detail.Trace.MaterialUsage = ComposeOutputTraceUsage{ID: usage.ID, Status: string(usage.Status)}
	}
	if strategy, found, err := s.strategies.Find(output.StrategyID); err != nil {
		return ComposeOutputDetail{}, err
	} else if found {
		detail.Trace.Strategy = ComposeOutputTraceStrategy{ID: strategy.ID, Name: strategy.Name, TemplateCode: output.TemplateCode, TemplateVersion: output.TemplateVersion, ConfigSnapshot: strategy.Config}
	}
	if task, found, err := s.tasks.Find(output.TaskID); err != nil {
		return ComposeOutputDetail{}, err
	} else if found {
		detail.Trace.ComposeTask = ComposeOutputTraceTask{ID: task.ID, Status: task.Status, ConfigSnapshot: task.ConfigSnapshot, CreatedAt: task.CreatedAt}
	}
	return detail, nil
}

// Archive flips a pending-publish output to archived in one conditional write.
// A row the write refused is reloaded and named by what it is now: gone
// answers not-found, published or already-archived answers not-archivable.
func (s *ComposeOutputService) Archive(actor identity.PublicUser, id int64) (model.CompositeOutput, error) {
	if _, err := s.readable(actor, id); err != nil {
		return model.CompositeOutput{}, err
	}
	ok, err := s.outputs.Archive(id, int64(actor.ID), s.now().UTC())
	if err != nil {
		return model.CompositeOutput{}, err
	}
	if !ok {
		if _, found, err := s.outputs.Find(id); err != nil {
			return model.CompositeOutput{}, err
		} else if !found {
			return model.CompositeOutput{}, ErrComposeOutputNotFound
		}
		return model.CompositeOutput{}, ErrComposeOutputNotArchivable
	}
	stored, found, err := s.outputs.Find(id)
	if err != nil {
		return model.CompositeOutput{}, err
	}
	if !found {
		return model.CompositeOutput{}, ErrComposeOutputNotFound
	}
	return stored, nil
}

// VideoURL answers the preview grant — the detail drawer's link, deliberately
// not part of any body. Every output row already has a verified object behind
// it, so unlike the material link there is no readiness refusal.
func (s *ComposeOutputService) VideoURL(ctx context.Context, actor identity.PublicUser, id int64) (string, error) {
	output, err := s.readable(actor, id)
	if err != nil {
		return "", err
	}
	return s.links.PresignObjectURL(ctx, output.ObjectKey)
}

// readable is the one scope gate every route shares, with the compose-task
// visibility shape: out of scope answers not-found so absence and
// forbidance are indistinguishable from the outside.
func (s *ComposeOutputService) readable(actor identity.PublicUser, id int64) (model.CompositeOutput, error) {
	if actor.Status != identity.UserStatusEnabled {
		return model.CompositeOutput{}, ErrComposeOutputForbidden
	}
	output, found, err := s.outputs.Find(id)
	if err != nil {
		return model.CompositeOutput{}, err
	}
	if !found || !canReadComposeOutput(actor, output) {
		return model.CompositeOutput{}, ErrComposeOutputNotFound
	}
	return output, nil
}

// canReadComposeOutput scopes the page like canReadComposeTask: admins see
// everything, seniors their team-and-game scope, operators only their own.
func canReadComposeOutput(actor identity.PublicUser, item model.CompositeOutput) bool {
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
