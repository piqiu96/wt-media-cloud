package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transfermodel "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

var (
	ErrComposeOutputUnavailable        = errors.New("compose output object is unavailable")
	ErrComposeOutputVerificationFailed = errors.New("compose output verification failed")
	ErrComposeOutputInvalid            = errors.New("compose output input is invalid")
	ErrComposeOutputForbidden          = errors.New("compose output operation is forbidden")
	ErrComposeOutputNotFound           = errors.New("compose output not found")
	ErrComposeOutputNotArchivable      = errors.New("compose output is not archivable")
)

type ListComposeOutputsInput struct {
	Page          int                   `json:"page"`
	PageSize      int                   `json:"page_size"`
	MaterialID    int64                 `json:"material_id"`
	BatchID       int64                 `json:"batch_id"`
	Platform      string                `json:"platform"`
	VideoStatus   string                `json:"video_status"`
	CategoryIDs   []identity.CategoryID `json:"category_ids"`
	PublishStatus string                `json:"publish_status"`
	Source        string                `json:"source"`
	Search        string                `json:"search"`
	TriggerType   string                `json:"trigger_type"`
	CategoryID    identity.CategoryID   `json:"category_id"`
	TeamID        int64                 `json:"team_id"`
	OperatorID    int64                 `json:"operator_id"`
	OperatorScope string                `json:"operator_scope"`
	CreatedFrom   time.Time             `json:"created_from"`
	CreatedBefore time.Time             `json:"created_before"`
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
	ObjectExists(ctx context.Context, key string) (bool, error)
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
	TemplateSchema  json.RawMessage `json:"template_schema"`
}

type ComposeOutputTraceTask struct {
	BatchID         *int64                  `json:"batch_id"`
	SettingKey      string                  `json:"setting_key"`
	SettingName     string                  `json:"setting_name"`
	Platforms       []string                `json:"platforms"`
	TemplateCode    string                  `json:"template_code"`
	TemplateVersion uint32                  `json:"template_version"`
	ID              int64                   `json:"id"`
	Status          model.ComposeTaskStatus `json:"status"`
	ConfigSnapshot  json.RawMessage         `json:"config_snapshot"`
	CreatedAt       time.Time               `json:"created_at"`
}

type ComposeOutputPublishingPlatform struct {
	Platform string  `json:"platform"`
	Status   *string `json:"status"`
}
type ComposeOutputPublishing struct {
	Available bool                              `json:"available"`
	Reason    string                            `json:"reason"`
	Platforms []ComposeOutputPublishingPlatform `json:"platforms"`
}
type ComposeOutputTrace struct {
	Publishing    ComposeOutputPublishing    `json:"publishing"`
	Material      ComposeOutputTraceMaterial `json:"material"`
	MaterialUsage *ComposeOutputTraceUsage   `json:"material_usage"`
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
	// resolveCategory names the category a download is filed under, stubbed by the
	// unit tests the same way the material download's is.
	resolveCategory func(categoryID identity.CategoryID) (string, bool, error)
	now             func() time.Time
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
		resolveCategory: func(categoryID identity.CategoryID) (string, bool, error) {
			category, ok, err := identity.ResolveCategory(categoryID)
			return category.Name, ok, err
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

// List scopes the page exactly like the compose-task list — admins the target
// team or every team, seniors their whole team, operators only what they
// created inside the categories they cover (PRD §5.10.2/§5.10.3). A caller without
// a scope answers an empty page rather than an error, because the scope is
// legitimate and simply contains nothing.
func (s *ComposeOutputService) List(actor identity.PublicUser, input ListComposeOutputsInput) ([]model.CompositeOutput, error) {
	filter, empty, err := s.outputFilter(actor, input)
	if err != nil {
		return nil, err
	}
	if empty {
		return []model.CompositeOutput{}, nil
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
// projection: mutable source labels are joined for display; execution parameters
// and template binding always come from the original task snapshot.
func (s *ComposeOutputService) Get(actor identity.PublicUser, id int64) (ComposeOutputDetail, error) {
	output, err := s.readable(actor, id)
	if err != nil {
		return ComposeOutputDetail{}, err
	}
	detail := ComposeOutputDetail{Output: output, Trace: ComposeOutputTrace{ComposeTask: ComposeOutputTraceTask{Platforms: []string{}}}}
	material, _, err := s.materials.FindMaterial(output.MaterialID)
	if err != nil {
		return detail, err
	}
	detail.Trace.Material = ComposeOutputTraceMaterial{ID: output.MaterialID, Title: material.Title}
	if output.MaterialUsageID != nil {
		usage, found, err := s.usages.FindUsage(*output.MaterialUsageID)
		if err != nil {
			return detail, err
		}
		if found {
			detail.Trace.MaterialUsage = &ComposeOutputTraceUsage{ID: usage.ID, Status: string(usage.Status)}
		}
	}
	strategy, _, err := s.strategies.Find(output.StrategyID)
	if err != nil {
		return detail, err
	}
	task, found, err := s.tasks.Find(output.TaskID)
	if err != nil {
		return detail, err
	}
	detail.Trace.Strategy = ComposeOutputTraceStrategy{ID: output.StrategyID, Name: strategy.Name, TemplateCode: output.TemplateCode, TemplateVersion: output.TemplateVersion, ConfigSnapshot: json.RawMessage(`{}`), TemplateSchema: json.RawMessage(`{}`)}
	if found {
		safe := model.SafeComposeSnapshot(task.ConfigSnapshot)
		detail.Trace.Strategy.ConfigSnapshot = safe
		detail.Trace.ComposeTask = ComposeOutputTraceTask{ID: task.ID, Status: task.Status, ConfigSnapshot: safe, CreatedAt: task.CreatedAt, BatchID: task.BatchID, SettingKey: task.SettingKey, SettingName: task.SettingName, Platforms: []string{}, TemplateCode: task.TemplateCode, TemplateVersion: task.TemplateVersion}
		_ = json.Unmarshal(task.PlatformsJSON, &detail.Trace.ComposeTask.Platforms)
		detail.Output.BatchID = task.BatchID
		detail.Output.SettingKey = task.SettingKey
		detail.Output.SettingName = task.SettingName
		detail.Output.Platforms = detail.Trace.ComposeTask.Platforms
	}
	detail.Output.MaterialTitle = material.Title
	detail.Output.AuthorName = material.AuthorName
	detail.Output.StrategyName = strategy.Name
	template, templateFound, err := s.originalTemplate(output)
	if err != nil {
		return detail, err
	}
	if templateFound {
		detail.Trace.Strategy.TemplateSchema = safeOutputSchema(template.Schema)
	}
	if err := s.decorateOutput(context.Background(), actor, &detail.Output); err != nil {
		return detail, err
	}
	detail.Trace.Publishing = ComposeOutputPublishing{Available: false, Reason: "发布能力尚未提供", Platforms: []ComposeOutputPublishingPlatform{}}
	for _, p := range detail.Output.Platforms {
		detail.Trace.Publishing.Platforms = append(detail.Trace.Publishing.Platforms, ComposeOutputPublishingPlatform{Platform: p})
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
// not part of any body. The stored success fact is followed by a fresh object
// check, since the storage object can disappear after production.
func (s *ComposeOutputService) VideoURL(ctx context.Context, actor identity.PublicUser, id int64) (string, error) {
	output, err := s.readable(actor, id)
	if err != nil {
		return "", err
	}
	if err := s.requireObject(ctx, output); err != nil {
		return "", err
	}
	return s.links.PresignObjectURL(ctx, output.ObjectKey)
}

// CreateOutputDownload queues the actor's download of one finished compose
// output onto their own Local Agent — the same `user_download` task a material
// download queues, with `composite_output` as the type.
//
// The immutable asset name is passed as the transfer title. The Local Agent
// reports its actual completed filename through the existing transfer domain.
// Object presence is checked again before queueing the transfer.
//
// The refusal mirrors the material download's: the scope gate answers not-found
// for an output the actor cannot see. The task carries no machine: any machine
// the user is logged in on can claim it.
func (s *ComposeOutputService) CreateOutputDownload(actor identity.PublicUser, id int64) (transferdto.Task, error) {
	if !canDownloadLocally(actor) {
		return transferdto.Task{}, ErrComposeOutputForbidden
	}
	output, err := s.readable(actor, id)
	if err != nil {
		return transferdto.Task{}, err
	}
	if err := s.requireObject(context.Background(), output); err != nil {
		return transferdto.Task{}, err
	}
	categoryName := ""
	if output.CategoryID > 0 {
		// Same swallow as the material path's `categoryNameOf`: a category that cannot be
		// resolved names no file rather than failing the download.
		if name, ok, err := s.resolveCategory(output.CategoryID); err == nil && ok {
			categoryName = name
		}
	}
	task, err := s.transfers.CreateUserDownload(transferservice.CreateUserDownloadInput{
		TeamID:          identity.TeamID(output.TeamID),
		AssetType:       transfermodel.AssetCompositeOutput,
		AssetID:         output.ID,
		AssetTitle:      outputDownloadTitle(output),
		CategoryName:    categoryName,
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
// everything, seniors every category of their team, operators only their own.
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
	inCategory := false
	for _, categoryID := range actor.CategoryIDs {
		if categoryID == item.CategoryID {
			inCategory = true
		}
	}
	if !inCategory {
		return false
	}
	// An absent owner is a management-only historical asset; creator is not ownership.
	return item.OperatorID != nil && *item.OperatorID == int64(actor.ID)
}
