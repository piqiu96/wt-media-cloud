package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transfermodel "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
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
	PublishStatus string    `json:"publish_status"`
	Source        string    `json:"source"`
	Search        string    `json:"search"`
	TriggerType   string    `json:"trigger_type"`
	TeamID        int64     `json:"team_id"`
	OperatorID    int64     `json:"operator_id"`
	OperatorScope string    `json:"operator_scope"`
	CreatedFrom   time.Time `json:"created_from"`
	CreatedBefore time.Time `json:"created_before"`
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
	operators  func() ([]identity.PublicUser, error)
	// transfers is the download path's only dependency: a consumer-side
	// interface narrowed to the one call a download of an already-verified object
	// needs — an output is never prepared in the cloud, so this service has no
	// reason to name the preparation half of TransferCreator.
	transfers outputTransferCreator
	// resolveGame names the game a download is filed under, stubbed by the
	// unit tests the same way the material download's is.
	resolveGame func(gameID string) (string, bool, error)
	now         func() time.Time
}

// outputTransferCreator is the slice of TransferCreator a download of an
// already-verified object needs: queue the task, nothing else.
type outputTransferCreator interface {
	CreateUserDownload(transferservice.CreateUserDownloadInput) (transferdto.Task, error)
}

func NewComposeOutputService(outputs ComposeOutputRepository, materials composeOutputTraceMaterialSource, usages composeOutputTraceUsageSource, strategies composeOutputTraceStrategySource, tasks composeOutputTraceTaskSource, links composeOutputLinker, transfers outputTransferCreator) *ComposeOutputService {
	return &ComposeOutputService{
		outputs: outputs, materials: materials, usages: usages, strategies: strategies, tasks: tasks, links: links,
		transfers: transfers, now: time.Now,
		resolveGame: func(gameID string) (string, bool, error) {
			game, ok, err := identity.ResolveGame(gameID)
			return game.Name, ok, err
		},
	}
}

func wiredComposeOutputService() *ComposeOutputService {
	service := NewComposeOutputService(repository.ComposeOutputStore{}, mysqlStore{}, mysqlStore{}, repository.StrategyStore{}, repository.ComposeTaskStore{}, productionObjectLinker{}, productionTransferCreator{})
	service.operators = identity.ListUsers
	return service
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

func CreateOutputDownload(actor identity.PublicUser, id int64) (transferdto.Task, error) {
	return wiredComposeOutputService().CreateOutputDownload(actor, id)
}

// List scopes the page exactly like the compose-task list — admins everything,
// seniors their team-and-game scope, operators only what they created (PRD
// §5.10.2/§5.10.3). A caller without a scope answers an empty page rather than
// an error, because the scope is legitimate and simply contains nothing.
func (s *ComposeOutputService) List(actor identity.PublicUser, input ListComposeOutputsInput) ([]model.CompositeOutput, error) {
	if actor.Status != identity.UserStatusEnabled {
		return nil, ErrComposeOutputForbidden
	}
	source := strings.TrimSpace(input.Source)
	if len([]rune(source)) > 100 {
		return nil, fmt.Errorf("%w: source is too long", ErrComposeOutputInvalid)
	}
	search := strings.TrimSpace(input.Search)
	if len([]rune(search)) > 100 {
		return nil, fmt.Errorf("%w: search is too long", ErrComposeOutputInvalid)
	}
	filter := repository.ComposeOutputFilter{Source: source, Search: search, CreatedFrom: input.CreatedFrom, CreatedBefore: input.CreatedBefore}
	if input.PublishStatus != "" {
		switch input.PublishStatus {
		case "pending_publish", "published", "archived":
			filter.PublishStatus = input.PublishStatus
		default:
			return nil, fmt.Errorf("%w: unknown publish_status %s", ErrComposeOutputInvalid, input.PublishStatus)
		}
	}
	if input.TriggerType != "" {
		switch input.TriggerType {
		case "manual", "scheduled":
			filter.TriggerType = input.TriggerType
		default:
			return nil, fmt.Errorf("%w: unknown trigger_type %s", ErrComposeOutputInvalid, input.TriggerType)
		}
	}
	scope, err := resolveComposeListScope(actor, input.TeamID, input.OperatorID, input.OperatorScope)
	if err != nil {
		if errors.Is(err, errComposeScopeForbidden) {
			return nil, ErrComposeOutputForbidden
		}
		return nil, fmt.Errorf("%w: %v", ErrComposeOutputInvalid, err)
	}
	filter.TeamID, filter.OperatorID, filter.IncludeUnownedCreatedBy = scope.TeamID, scope.OperatorID, scope.IncludeUnowned
	// Operators stay inside the games they cover; seniors and admins see every
	// game of the team they are allowed to read.
	if actor.Role == identity.RoleOperator {
		filter.GameIDs = actor.GameIDs
	}
	items, err := s.outputs.List(filter)
	if err != nil {
		return nil, err
	}
	return s.withOperatorNames(items)
}

func (s *ComposeOutputService) withOperatorNames(items []model.CompositeOutput) ([]model.CompositeOutput, error) {
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
	return s.withOperatorName(stored)
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

// CreateOutputDownload queues the actor's download of one finished compose
// output onto their own Local Agent — the same `user_download` task a material
// download queues, with `composite_output` as the type.
//
// Unlike the material download there is no preparation arm: an output row only
// exists once the compose success transaction wrote a verified object behind
// it, so the object key, the size and the hash are facts of the row rather than
// facts to be produced. That is also why the title is resolved here — the row
// has no title of its own, and the executor names its file from what the task
// carries.
//
// The refusal mirrors the material download's: the scope gate answers not-found
// for an output the actor cannot see. The task carries no machine: any machine
// the user is logged in on can claim it.
func (s *ComposeOutputService) CreateOutputDownload(actor identity.PublicUser, id int64) (transferdto.Task, error) {
	output, err := s.readable(actor, id)
	if err != nil {
		return transferdto.Task{}, err
	}
	gameName := ""
	if output.GameID != "" {
		// Same swallow as the material path's `gameNameOf`: a game that cannot be
		// resolved names no file rather than failing the download.
		if name, ok, err := s.resolveGame(output.GameID); err == nil && ok {
			gameName = name
		}
	}
	task, err := s.transfers.CreateUserDownload(transferservice.CreateUserDownloadInput{
		TeamID:          identity.TeamID(output.TeamID),
		AssetType:       transfermodel.AssetCompositeOutput,
		AssetID:         output.ID,
		AssetTitle:      fmt.Sprintf("成片-%d", output.ID),
		GameName:        gameName,
		SourceObjectKey: output.ObjectKey,
		RequestedBy:     actor.ID,
		TotalBytes:      output.FileSizeBytes,
		ExpectedSHA256:  output.SHA256,
	})
	if err != nil {
		if errors.Is(err, transferservice.ErrInvalidInput) {
			return transferdto.Task{}, ErrInvalidInput
		}
		return transferdto.Task{}, fmt.Errorf("queue user download for compose output %d: %w", output.ID, err)
	}
	return task, nil
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
	return s.withOperatorName(output)
}

func (s *ComposeOutputService) withOperatorName(item model.CompositeOutput) (model.CompositeOutput, error) {
	items, err := s.withOperatorNames([]model.CompositeOutput{item})
	if err != nil {
		return model.CompositeOutput{}, err
	}
	return items[0], nil
}

// canReadComposeOutput scopes the page like canReadComposeTask: admins see
// everything, seniors every game of their team, operators only their own.
func canReadComposeOutput(actor identity.PublicUser, item model.CompositeOutput) bool {
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
	// Same fallback as tasks: a historical row with no operator stays
	// reachable by its creator without being attributed to them.
	return (item.OperatorID != nil && *item.OperatorID == int64(actor.ID)) || (item.OperatorID == nil && item.CreatedBy == int64(actor.ID))
}
