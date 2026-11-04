package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
)

type outputPageRepository interface {
	Page(context.Context, repository.ComposeOutputFilter, int, int) (repository.ComposeOutputPage, error)
	MaterialPage(context.Context, repository.ComposeOutputFilter, int, int) (repository.ComposeOutputMaterialPage, error)
	ObjectRefs(context.Context, repository.ComposeOutputFilter) ([]repository.ComposeOutputObjectRef, error)
}
type outputTemplateRepository interface {
	OriginalTemplate(string, uint32) (model.ComposeTemplate, bool, error)
}

func outputDownloadTitle(o model.CompositeOutput) string {
	if o.Name != "" {
		return o.Name
	}
	return fmt.Sprintf("成片-%d", o.ID)
}
func (s *ComposeOutputService) outputFilter(actor identity.PublicUser, input ListComposeOutputsInput) (repository.ComposeOutputFilter, bool, error) {
	f := repository.ComposeOutputFilter{Source: strings.TrimSpace(input.Source), Search: strings.TrimSpace(input.Search), CreatedFrom: input.CreatedFrom, CreatedBefore: input.CreatedBefore, MaterialID: input.MaterialID, BatchID: input.BatchID, Platform: input.Platform, TriggerType: input.TriggerType, PublishStatus: input.PublishStatus}
	invalid := func() (repository.ComposeOutputFilter, bool, error) { return f, false, ErrComposeOutputInvalid }
	if actor.Status != identity.UserStatusEnabled {
		return f, false, ErrComposeOutputForbidden
	}
	if input.Page < 0 || (input.Page > 0 && input.Page-1 > math.MaxInt/100) || input.PageSize < 0 || input.PageSize > 100 || input.MaterialID < 0 || input.BatchID < 0 || input.CategoryID < 0 || len([]rune(f.Source)) > 100 || len([]rune(f.Search)) > 100 {
		return invalid()
	}
	if f.Platform != "" && !slices.Contains([]string{"bilibili", "baidu", "xiaohongshu"}, f.Platform) {
		return invalid()
	}
	if input.VideoStatus != "" && !slices.Contains([]string{"ready", "failed", "unknown"}, input.VideoStatus) {
		return invalid()
	}
	if f.TriggerType != "" && !slices.Contains([]string{"manual", "immediate", "scheduled"}, f.TriggerType) {
		return invalid()
	}
	if f.PublishStatus != "" && !slices.Contains([]string{"pending_publish", "published", "archived"}, f.PublishStatus) {
		return invalid()
	}
	if !f.CreatedFrom.IsZero() && !f.CreatedBefore.IsZero() && !f.CreatedFrom.Before(f.CreatedBefore) {
		return invalid()
	}
	scope, err := resolveComposeListScope(actor, input.TeamID, input.OperatorID, input.OperatorScope)
	if err != nil {
		if errors.Is(err, errComposeScopeForbidden) {
			return f, false, ErrComposeOutputForbidden
		}
		return invalid()
	}
	f.TeamID = scope.TeamID
	f.OperatorID = scope.OperatorID
	ids := append([]identity.CategoryID(nil), input.CategoryIDs...)
	if input.CategoryID > 0 {
		if len(ids) > 0 && !slices.Contains(ids, input.CategoryID) {
			return f, true, nil
		}
		ids = []identity.CategoryID{input.CategoryID}
	}
	for _, id := range ids {
		if id <= 0 {
			return invalid()
		}
		if actor.Role == identity.RoleOperator && !slices.Contains(actor.CategoryIDs, id) {
			return f, false, ErrComposeOutputForbidden
		}
	}
	if actor.Role == identity.RoleOperator && len(ids) == 0 {
		ids = append(ids, actor.CategoryIDs...)
		if len(ids) == 0 {
			return f, true, nil
		}
	}
	f.CategoryIDs = ids
	return f, false, nil
}
func outputPage(input ListComposeOutputsInput) (int, int) {
	p, n := input.Page, input.PageSize
	if p == 0 {
		p = 1
	}
	if n == 0 {
		n = 20
	}
	return p, n
}
func ListComposeOutputPage(ctx context.Context, actor identity.PublicUser, input ListComposeOutputsInput) (repository.ComposeOutputPage, error) {
	return wiredComposeOutputService().Page(ctx, actor, input)
}
func ListComposeOutputMaterials(ctx context.Context, actor identity.PublicUser, input ListComposeOutputsInput) (repository.ComposeOutputMaterialPage, error) {
	return wiredComposeOutputService().MaterialPage(ctx, actor, input)
}
func (s *ComposeOutputService) Page(ctx context.Context, actor identity.PublicUser, input ListComposeOutputsInput) (repository.ComposeOutputPage, error) {
	p, n := outputPage(input)
	result := repository.ComposeOutputPage{Items: []model.CompositeOutput{}, Page: p, PageSize: n}
	f, empty, err := s.outputFilter(actor, input)
	if err != nil || empty {
		return result, err
	}
	repo, ok := s.outputs.(outputPageRepository)
	if !ok {
		return result, errors.New("output paging unavailable")
	}
	if input.VideoStatus != "" {
		f, err = s.filterVideo(ctx, repo, f, input.VideoStatus)
		if err != nil {
			return result, err
		}
	}
	result, err = repo.Page(ctx, f, p, n)
	if err != nil {
		return result, err
	}
	result.Items, err = s.withOperatorNames(result.Items)
	if err != nil {
		return result, err
	}
	for i := range result.Items {
		if err = s.decorateOutput(ctx, actor, &result.Items[i]); err != nil {
			return repository.ComposeOutputPage{}, err
		}
	}
	return result, nil
}
func (s *ComposeOutputService) MaterialPage(ctx context.Context, actor identity.PublicUser, input ListComposeOutputsInput) (repository.ComposeOutputMaterialPage, error) {
	p, n := outputPage(input)
	result := repository.ComposeOutputMaterialPage{Items: []repository.ComposeOutputMaterial{}, Page: p, PageSize: n}
	f, empty, err := s.outputFilter(actor, input)
	if err != nil || empty {
		return result, err
	}
	repo, ok := s.outputs.(outputPageRepository)
	if !ok {
		return result, errors.New("output paging unavailable")
	}
	if input.VideoStatus != "" {
		f, err = s.filterVideo(ctx, repo, f, input.VideoStatus)
		if err != nil {
			return result, err
		}
	}
	return repo.MaterialPage(ctx, f, p, n)
}
func (s *ComposeOutputService) requireObject(ctx context.Context, o model.CompositeOutput) error {
	status, err := s.checkObject(ctx, o.ObjectKey)
	if err != nil {
		return err
	}
	if status != "ready" {
		return ErrComposeOutputUnavailable
	}
	return nil
}
func (s *ComposeOutputService) checkObject(ctx context.Context, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "unknown", nil
	}
	exists, err := s.links.ObjectExists(ctx, key)
	if err != nil {
		return "unknown", ErrComposeOutputVerificationFailed
	}
	if exists {
		return "ready", nil
	}
	return "failed", nil
}
func (s *ComposeOutputService) filterVideo(ctx context.Context, repo outputPageRepository, f repository.ComposeOutputFilter, want string) (repository.ComposeOutputFilter, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	refs, err := repo.ObjectRefs(ctx, f)
	if err != nil {
		return f, err
	}
	jobs := make(chan repository.ComposeOutputObjectRef)
	ids := []int64{}
	var mu sync.Mutex
	var first error
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				status, err := s.checkObject(ctx, r.ObjectKey)
				mu.Lock()
				if err != nil && first == nil {
					first = err
					cancel()
				}
				if err == nil && status == want {
					ids = append(ids, r.ID)
				}
				mu.Unlock()
			}
		}()
	}
send:
	for _, r := range refs {
		select {
		case jobs <- r:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return f, first
	}
	if ctx.Err() != nil {
		return f, ErrComposeOutputVerificationFailed
	}
	f.RestrictIDs = true
	f.OutputIDs = ids
	return f, nil
}
func (s *ComposeOutputService) originalTemplate(o model.CompositeOutput) (model.ComposeTemplate, bool, error) {
	repo, ok := s.outputs.(outputTemplateRepository)
	if !ok {
		return model.ComposeTemplate{}, false, nil
	}
	return repo.OriginalTemplate(o.TemplateCode, o.TemplateVersion)
}
func safeOutputSchema(raw json.RawMessage) json.RawMessage {
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return json.RawMessage(`{}`)
	}
	var clean func(any)
	clean = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for key, value := range x {
				if key == "guide_video_url" {
					if prop, ok := value.(map[string]any); ok {
						delete(prop, "default")
						delete(prop, "examples")
						delete(prop, "example")
						delete(prop, "const")
						delete(prop, "enum")
					}
				}
				clean(value)
			}
		case []any:
			for _, v := range x {
				clean(v)
			}
		}
	}
	clean(schema)
	out, _ := json.Marshal(schema)
	return out
}
func (s *ComposeOutputService) decorateOutput(ctx context.Context, actor identity.PublicUser, o *model.CompositeOutput) error {
	status, err := s.checkObject(ctx, o.ObjectKey)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	o.Video = model.ComposeOutputVideo{Status: status, CheckedAt: &now}
	o.Actions = model.ComposeOutputActions{Download: status == "ready", RemakeReason: "原始制作信息不完整，请从素材新建"}
	if status == "failed" {
		o.Video.Reason = "成片文件不可用"
	} else if status == "unknown" {
		o.Video.Reason = "成片文件位置未记录"
	}
	o.Actions.DownloadReason = o.Video.Reason
	if !canDownloadLocally(actor) {
		o.Actions.Download = false
		o.Actions.DownloadReason = "仅运营和高级运营可下载到本机"
	}
	task, found, err := s.tasks.Find(o.TaskID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if _, err = repository.ManualSourceSetting(*o, task); err != nil {
		return nil
	}
	strategy, found, err := s.strategies.Find(o.StrategyID)
	if err != nil {
		return err
	}
	if !found || !canReadStrategy(actor, strategy) || repository.ValidateManualSourceStrategy(*o, strategy) != nil {
		o.Actions.RemakeReason = "原策略不可用或归属已变化"
		return nil
	}
	template, found, err := s.originalTemplate(*o)
	if err != nil {
		return err
	}
	definition, deployedErr := executor.DeployedDefinition(o.TemplateCode, o.TemplateVersion)
	if !found || deployedErr != nil || template.Status != "active" || template.ExecutorID != definition.ExecutorID || !sameJSON(template.Schema, definition.Schema) {
		o.Actions.RemakeReason = "原模板版本不可用"
		return nil
	}
	if _, validationErr := definition.Executor.ValidateParams(task.ConfigSnapshot); validationErr != nil {
		o.Actions.RemakeReason = "原制作参数无法执行"
		return nil
	}
	_, valid, err := resolveComposeOperator(s.operators, o.TeamID, o.CategoryID, *o.OperatorID)
	if err != nil {
		return err
	}
	if !valid {
		o.Actions.RemakeReason = "原负责人不可用"
		return nil
	}
	material, found, err := s.materials.FindMaterial(o.MaterialID)
	if err != nil {
		return err
	}
	if !found || int64(material.TeamID) != o.TeamID || material.CategoryID == nil || *material.CategoryID != o.CategoryID {
		o.Actions.RemakeReason = "原素材不可用"
		return nil
	}
	// Availability is a source fact; it is independent of the output object's presence.
	if checker, ok := s.materials.(interface {
		OutputInputAvailable(model.Material) (bool, error)
	}); ok {
		available, err := checker.OutputInputAvailable(material)
		if err != nil {
			return ErrComposeOutputVerificationFailed
		}
		if !available {
			o.Actions.RemakeReason = "原素材不可获取"
			return nil
		}
	} else {
		o.Actions.RemakeReason = "原素材可获取状态未知"
		return nil
	}
	o.Actions.Remake = true
	o.Actions.RemakeReason = ""
	return nil
}
func (mysqlStore) OutputInputAvailable(material model.Material) (bool, error) {
	if strings.TrimSpace(material.SourceURL) == "" && material.VideoStatus != model.VideoReady {
		return false, nil
	}
	available, _, err := newWiredService().materialSelectionAvailable(material)
	return available, err
}
