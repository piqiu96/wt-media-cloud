package publish

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/deepseek"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
)

func batchGenerationRequest(c *app.RequestContext) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(c.Request.Body(), &fields); err != nil {
		return false
	}
	_, found := fields["items"]
	return found
}

func generateBatchContent(c *app.RequestContext, a identity.PublicUser) {
	var in struct {
		Items []service.GenerationInput `json:"items"`
	}
	if err := decodeBatch(c, &in); err != nil {
		failBatch(c, err)
		return
	}
	if len(in.Items) == 0 || len(in.Items) > api.MaxStreamItems {
		failBatch(c, service.ErrInvalid)
		return
	}
	seen := make(map[string]bool, len(in.Items))
	jobs := make([]api.StreamJob, 0, len(in.Items))
	for _, item := range in.Items {
		if err := service.ValidateBatchGeneration(a, item); err != nil {
			failBatch(c, err)
			return
		}
		if seen[item.RowKey] {
			failBatch(c, service.ErrInvalid)
			return
		}
		seen[item.RowKey] = true
		jobs = append(jobs, api.StreamJob{
			Item: api.StreamItem{RowKey: item.RowKey, InputRevision: item.InputRevision, FieldRevisions: item.FieldRevisions}, Stage: "generating",
			Run: func(ctx context.Context) (any, error) {
				run, err := service.Wired().PreparePublicationGeneration(a, item)
				if errors.Is(err, service.ErrInvalid) {
					return nil, &api.StreamError{Code: "publication_invalid", Message: "候选成片或文案输入已变化"}
				}
				if errors.Is(err, service.ErrNotFound) || errors.Is(err, production.ErrComposeOutputNotFound) || errors.Is(err, production.ErrComposeOutputForbidden) {
					return nil, &api.StreamError{Code: "publication_not_found", Message: "成片不可用或没有访问权限"}
				}
				if err != nil {
					return nil, err
				}
				return run(ctx)
			},
		})
	}
	if !deepseek.Get().Ready() {
		api.FailureNamed(c, 503, errcode.PublishUnavailable, "AI 文案服务未配置，请联系管理员", "publication_unavailable")
		return
	}
	api.StreamManyWithTimeout(c, int64(a.ID), 180*time.Second, jobs)
}
