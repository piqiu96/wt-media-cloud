package publish

import (
	"context"
	"errors"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	images "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
)

const batchPayloadLimit = 1 << 20

func registerBatchRoutes(h *server.Hertz) {
	h.GET("/api/v1/publication-batch", ListBatches)
	h.POST("/api/v1/publication-batch", CreateBatch)
	h.POST("/api/v1/publication-batch/allocate", AllocateBatch)
	h.POST("/api/v1/publication-batch/validate", ValidateBatch)
	h.GET("/api/v1/publication-batch/:id", GetBatch)
	h.POST("/api/v1/publication-batch/:id/execution-preview", PreviewBatchExecution)
	h.POST("/api/v1/publication-batch/:id/start", StartBatch)
	h.POST("/api/v1/publication-batch/:id/pause", PauseBatch)
	h.POST("/api/v1/publication-batch/:id/resume", ResumeBatch)
	h.POST("/api/v1/publication-batch/:id/stop", StopBatch)
	h.POST("/api/v1/publication-batch/:id/finish", FinishBatch)
	h.GET("/api/v1/publication-batch/:id/task", ListBatchTasks)
	h.GET("/api/v1/publication-batch/:id/copy", CopyBatch)
	h.GET("/api/v1/publication-batch/:id/record", ListBatchRecords)
	h.PUT("/api/v1/publication-batch/:id/configuration", EditBatchConfiguration)
	h.GET("/api/v1/publication/account", ListBatchAccounts)
	h.GET("/api/v1/publication/resource", GetBatchResources)
}
func decodeBatch(c *app.RequestContext, out any) error {
	if len(c.Request.Body()) > batchPayloadLimit {
		return service.ErrInvalid
	}
	if err := decode(c, out); err != nil {
		return err
	}
	return checkBatchRequestFields(c.Request.Body(), out)
}

func failBatch(c *app.RequestContext, err error) {
	if failBatchControl(c, err) {
		return
	}
	var conflict *service.BatchConflictError
	var field *service.BatchInputError
	var details any
	invalidMessage := "请检查批次目标、本人资源与设置"
	if errors.As(err, &field) {
		details = field
		invalidMessage = field.Issue.Message
	}
	switch {
	case errors.Is(err, service.ErrConflict):
		if errors.As(err, &conflict) {
			details = conflict
		}
		api.FailureCode(c, consts.StatusConflict, errcode.PublishConflict, "批次配置已变化或不可编辑，请重新加载", "publication_conflict", details, false, "reload")
	case errors.Is(err, service.ErrNotFound), errors.Is(err, accounts.ErrNotFound), errors.Is(err, accounts.ErrForbidden), errors.Is(err, files.ErrNotFound), errors.Is(err, files.ErrForbidden), errors.Is(err, images.ErrNotFound), errors.Is(err, production.ErrComposeOutputNotFound), errors.Is(err, production.ErrComposeOutputForbidden):
		api.FailureCode(c, consts.StatusNotFound, errcode.PublishNotFound, "批次或资源不存在，或没有访问权限", "publication_not_found", details, false, "none")
	case errors.Is(err, service.ErrInvalid), errors.Is(err, accounts.ErrInvalidInput), errors.Is(err, files.ErrInvalid), errors.Is(err, images.ErrInvalid), errors.Is(err, production.ErrComposeOutputInvalid):
		api.FailureCode(c, consts.StatusBadRequest, errcode.PublishInvalid, invalidMessage, "publication_invalid", details, false, "none")
	default:
		fail(c, err)
	}
}
func batchQueryUint(c *app.RequestContext, name string, required bool) (uint64, error) {
	raw := c.Query(name)
	if raw == "" && !required {
		return 0, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 || strconv.FormatUint(v, 10) != raw {
		return 0, service.ErrInvalid
	}
	return v, nil
}
func batchPage(c *app.RequestContext) (int, int, error) {
	page, size := 1, 20
	for _, option := range []struct {
		name string
		dest *int
	}{{"page", &page}, {"page_size", &size}} {
		if raw := c.Query(option.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				return 0, 0, service.ErrInvalid
			}
			*option.dest = n
		}
	}
	if size > 100 {
		return 0, 0, service.ErrInvalid
	}
	return page, size, nil
}
func batchCategory(c *app.RequestContext, required bool) (int64, error) {
	id, err := batchQueryUint(c, "category_id", required)
	if err != nil || uint64(int64(id)) != id || int64(id) < 0 {
		return 0, service.ErrInvalid
	}
	return int64(id), nil
}

func ListBatches(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	page, size, err := batchPage(c)
	if err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().ListBatches(a, service.BatchFilter{Page: page, PageSize: size, Search: c.Query("search"), Platform: c.Query("platform"), ConfigurationStatus: c.Query("configuration_status"), ExecutionStatus: c.Query("execution_status"), RequestKey: c.Query("request_key")})
	respondBatch(c, result, err)
}
func GetBatch(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	result, err := service.Wired().GetBatch(a, id)
	respondBatch(c, result, err)
}
func CreateBatch(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.CreateBatchInput
	if err := decodeBatch(c, &in); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().CreateBatch(ctx, a, in)
	if err != nil {
		failBatch(c, err)
		return
	}
	api.Created(c, result)
}
func EditBatchConfiguration(ctx context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.EditBatchInput
	if err := decodeBatch(c, &in); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().EditBatch(ctx, a, id, in)
	respondBatch(c, result, err)
}
func CopyBatch(ctx context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	rev, err := batchQueryUint(c, "expected_revision", true)
	if err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().CopyBatch(ctx, a, id, rev)
	respondBatch(c, result, err)
}
func ListBatchRecords(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	page, size, err := batchPage(c)
	if err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().BatchRecords(a, id, page, size)
	respondBatch(c, result, err)
}
func AllocateBatch(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.BatchAllocationInput
	if err := decodeBatch(c, &in); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().AllocateBatch(ctx, a, in)
	respondBatch(c, result, err)
}
func ValidateBatch(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.ValidateBatchInput
	if err := decodeBatch(c, &in); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().ValidateBatch(ctx, a, in)
	respondBatch(c, result, err)
}
func ListBatchAccounts(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	page, size, err := batchPage(c)
	if err != nil {
		failBatch(c, err)
		return
	}
	category, err := batchCategory(c, false)
	if err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().AccountCandidates(a, c.Query("platform"), category, c.Query("search"), page, size)
	respondBatch(c, result, err)
}
func GetBatchResources(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	category, err := batchCategory(c, true)
	if err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().BatchResources(ctx, a, c.Query("platform"), category)
	respondBatch(c, result, err)
}
func authorizeBatchID(c *app.RequestContext) (identity.PublicUser, uint64, bool) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return a, 0, false
	}
	id, err := taskID(c)
	if err != nil {
		failBatch(c, err)
		return a, 0, false
	}
	return a, id, true
}
func respondBatch(c *app.RequestContext, result any, err error) {
	if err != nil {
		failBatch(c, err)
		return
	}
	api.Success(c, result)
}
