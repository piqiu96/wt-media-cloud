package publish

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/deepseek"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
	"time"
)

func GeneratePublicationContent(_ context.Context, c *app.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	if len(c.Request.Body()) > batchPayloadLimit {
		fail(c, service.ErrInvalid)
		return
	}
	if batchGenerationRequest(c) {
		generateBatchContent(c, actor)
		return
	}
	var in service.GenerationInput
	if err := decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	s := service.Wired()
	run, err := s.PreparePublicationGeneration(actor, in)
	if err != nil {
		fail(c, err)
		return
	}
	if !deepseek.Get().Ready() {
		api.FailureNamed(c, 503, errcode.PublishUnavailable, "AI 文案服务未配置，请联系管理员", "publication_unavailable")
		return
	}
	api.StreamOneWithTimeout(c, api.StreamItem{UserID: int64(actor.ID), RowKey: in.RowKey, InputRevision: in.InputRevision}, 180*time.Second, func(ctx context.Context, progress func(string) error) (any, error) {
		if err := progress("generating"); err != nil {
			return nil, err
		}
		return run(ctx)
	})
}
