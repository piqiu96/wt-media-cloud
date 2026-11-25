package publish

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"strconv"
)

func decodePlanCommand(c *app.RequestContext, out any, required ...string) error {
	if e := decodePlan(c, out); e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(c.Request.Body(), &fields) != nil {
		return service.ErrInvalid
	}
	if e := requireBatchFields(fields, required...); e != nil {
		return e
	}
	switch out.(type) {
	case *model.StartPlanInput, *model.PlanAdvanceInput:
		var value any
		if json.Unmarshal(c.Request.Body(), &value) != nil || !canonicalPlanCommandIDs(value) {
			return service.ErrInvalid
		}
	}
	return nil
}

// Proof-bearing commands must satisfy the wire ID schema before canonical hashing.
func canonicalPlanCommandIDs(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch key {
			case "expected_revision", "first_batch_id", "batch_id", "current_batch_id", "next_batch_id", "category_id":
				text, ok := item.(string)
				if !ok {
					return false
				}
				id, e := strconv.ParseUint(text, 10, 64)
				if e != nil || id == 0 || strconv.FormatUint(id, 10) != text {
					return false
				}
			}
			if !canonicalPlanCommandIDs(item) {
				return false
			}
		}
	case []any:
		for _, item := range v {
			if !canonicalPlanCommandIDs(item) {
				return false
			}
		}
	}
	return true
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
	var rawConfirmations []struct {
		Shortfalls json.RawMessage `json:"expected_shortfalls"`
	}
	if json.Unmarshal(raw.Confirmations, &rawConfirmations) != nil {
		failBatch(c, service.ErrInvalid)
		return
	}
	for _, confirmation := range rawConfirmations {
		if requireBatchObjects(confirmation.Shortfalls, "category_id", "shortfall") != nil {
			failBatch(c, service.ErrInvalid)
			return
		}
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

func FinishPlan(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in model.FinishPlanInput
	if e := decodePlanCommand(c, &in, "expected_revision", "expected_shortfalls", "end_reason", "end_note"); e != nil {
		failBatch(c, e)
		return
	}
	var raw struct {
		Shortfalls json.RawMessage `json:"expected_shortfalls"`
	}
	json.Unmarshal(c.Request.Body(), &raw)
	if requireBatchObjects(raw.Shortfalls, "platform", "category_id", "shortfall") != nil {
		failBatch(c, service.ErrInvalid)
		return
	}
	out, e := service.Wired().FinishPlan(a, id, in)
	respondBatch(c, out, e)
}
