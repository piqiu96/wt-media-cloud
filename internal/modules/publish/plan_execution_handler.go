package publish

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
)

func decodePlanCommand(c *app.RequestContext, out any, required ...string) error {
	if e := decodePlan(c, out); e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(c.Request.Body(), &fields) != nil {
		return service.ErrInvalid
	}
	return requireBatchFields(fields, required...)
}
func StartPlan(ctx context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in model.StartPlanInput
	if e := decodePlanCommand(c, &in, "request_key", "expected_revision", "first_batch_id", "batch_confirmations"); e != nil {
		failBatch(c, e)
		return
	}
	var raw struct {
		Confirmations json.RawMessage `json:"batch_confirmations"`
	}
	json.Unmarshal(c.Request.Body(), &raw)
	if requireBatchObjects(raw.Confirmations, "batch_id", "expected_revision", "included_row_keys", "excluded_row_keys", "allow_shortfall", "expected_shortfalls") != nil {
		failBatch(c, service.ErrInvalid)
		return
	}
	out, e := service.Wired().StartPlan(ctx, a, id, in)
	respondBatch(c, out, e)
}
func planReason(c *app.RequestContext, action string) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.PlanReasonInput
	if e := decodePlanCommand(c, &in, "expected_revision", "reason"); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().PlanControl(a, id, action, service.PlanTakeoverInput{ExpectedRevision: in.ExpectedRevision, Reason: in.Reason})
	respondBatch(c, out, e)
}
func PausePlan(_ context.Context, c *app.RequestContext)  { planReason(c, "pause") }
func CancelPlan(_ context.Context, c *app.RequestContext) { planReason(c, "cancel") }
func ResumePlan(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.PlanRevisionInput
	if e := decodePlanCommand(c, &in, "expected_revision"); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().PlanControl(a, id, "resume", service.PlanTakeoverInput{ExpectedRevision: in.ExpectedRevision})
	respondBatch(c, out, e)
}
func TakeoverPlan(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.PlanTakeoverInput
	if e := decodePlanCommand(c, &in, "expected_revision", "current_batch_id", "expected_batch_revision", "reason"); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().PlanControl(a, id, "takeover", in)
	respondBatch(c, out, e)
}
func ConfirmPlanNext(ctx context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in model.PlanAdvanceInput
	if e := decodePlanCommand(c, &in, "request_key", "expected_revision", "current_batch_id", "next_batch_id", "expected_basis_fingerprint", "decision", "reason"); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().AdvancePlan(ctx, a, id, in, false)
	respondBatch(c, out, e)
}
