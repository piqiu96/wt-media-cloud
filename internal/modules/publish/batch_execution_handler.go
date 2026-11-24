package publish

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
)

func decodeBatchCommand(c *app.RequestContext, out any, required ...string) error {
	if err := decodeBatch(c, out); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(c.Request.Body(), &fields); err != nil {
		return service.ErrInvalid
	}
	return requireBatchFields(fields, required...)
}

func PreviewBatchExecution(ctx context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.BatchPreviewInput
	if err := decodeBatchCommand(c, &in, "expected_revision", "row_keys"); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().PreviewBatchExecution(ctx, a, id, in)
	respondBatch(c, result, err)
}
func StartBatch(ctx context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in model.BatchStartInput
	if err := decodeBatchCommand(c, &in, "request_key", "expected_revision", "included_row_keys", "excluded_row_keys", "allow_shortfall", "expected_shortfalls"); err != nil {
		failBatch(c, err)
		return
	}
	var raw struct {
		Expect json.RawMessage `json:"expected_shortfalls"`
	}
	if json.Unmarshal(c.Request.Body(), &raw) != nil || requireBatchObjects(raw.Expect, "category_id", "shortfall") != nil {
		failBatch(c, service.ErrInvalid)
		return
	}
	result, err := service.Wired().StartBatch(ctx, a, id, in)
	respondBatch(c, result, err)
}
func batchReasonCommand(c *app.RequestContext, action string) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.BatchReasonInput
	if err := decodeBatchCommand(c, &in, "expected_revision", "reason"); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().BatchReasonCommand(a, id, action, in)
	if action == "stop" {
		respondBatch(c, result, err)
		return
	}
	respondBatch(c, result.BatchControlResult, err)
}
func PauseBatch(_ context.Context, c *app.RequestContext) { batchReasonCommand(c, "pause") }
func StopBatch(_ context.Context, c *app.RequestContext)  { batchReasonCommand(c, "stop") }
func ResumeBatch(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.BatchResumeInput
	if err := decodeBatchCommand(c, &in, "request_key", "expected_revision", "expected_pause_id", "acknowledged_exceptions"); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().ResumeBatch(a, id, in)
	respondBatch(c, result, err)
}
func FinishBatch(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.BatchFinishInput
	if err := decodeBatchCommand(c, &in, "expected_revision", "expected_failed_tasks", "reason"); err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().FinishBatch(a, id, in)
	respondBatch(c, result, err)
}
func ListBatchTasks(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	page, size, err := batchPage(c)
	if err != nil {
		failBatch(c, err)
		return
	}
	account, err := batchQueryUint(c, "media_account_id", false)
	if err != nil {
		failBatch(c, err)
		return
	}
	result, err := service.Wired().BatchTasks(a, id, service.BatchTaskFilter{Page: page, PageSize: size, AccountID: account, Status: c.Query("status"), Search: c.Query("search")})
	respondBatch(c, result, err)
}
