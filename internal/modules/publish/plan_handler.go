package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/deepseek"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
	"io"
	"strconv"
	"time"
)

func registerPlanRoutes(h *server.Hertz) {
	h.GET("/api/v1/publication-plan", ListPlans)
	h.POST("/api/v1/publication-plan", CreatePlan)
	h.POST("/api/v1/publication-plan/validate", ValidatePlan)
	h.POST("/api/v1/publication-plan/recommend", RecommendPlan)
	h.POST("/api/v1/publication-plan/:id/start", StartPlan)
	h.POST("/api/v1/publication-plan/:id/pause", PausePlan)
	h.POST("/api/v1/publication-plan/:id/resume", ResumePlan)
	h.POST("/api/v1/publication-plan/:id/confirm-next", ConfirmPlanNext)
	h.POST("/api/v1/publication-plan/:id/takeover", TakeoverPlan)
	h.POST("/api/v1/publication-plan/:id/cancel", CancelPlan)
	h.POST("/api/v1/publication-plan/:id/finish", FinishPlan)
	h.GET("/api/v1/publication-plan/:id", GetPlan)
	h.PUT("/api/v1/publication-plan/:id/structure", EditPlan)
	h.GET("/api/v1/publication-plan/:id/batch", ListPlanBatches)
	h.GET("/api/v1/publication-plan/:id/record", ListPlanRecords)
	h.GET("/api/v1/publication-plan/:id/copy", CopyPlan)
}
func decodePlan(c *app.RequestContext, out any) error {
	body := c.Request.Body()
	if len(body) == 0 || len(body) > batchPayloadLimit {
		return service.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return service.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return service.ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return service.ErrInvalid
	}
	required := []string{}
	switch out.(type) {
	case *service.CreatePlanInput:
		required = []string{"request_key", "name", "date_start", "date_end", "note", "goals", "batches"}
	case *service.StructurePlanInput:
		required = []string{"expected_revision", "name", "date_start", "date_end", "note", "goals", "batches"}
	case *service.ValidatePlanInput:
		required = []string{"name", "date_start", "date_end", "note", "goals", "batches"}
	case *service.PlanRecommendationInput:
		required = []string{"goals", "batches", "instruction"}
	}
	for _, key := range required {
		v, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return service.ErrInvalid
		}
	}
	return nil
}
func ListPlans(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	page, size, e := batchPage(c)
	if e != nil {
		failBatch(c, e)
		return
	}
	category, e := batchCategory(c, false)
	if e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().ListPlans(a, repository.PlanFilter{Page: page, PageSize: size, Search: c.Query("search"), Status: c.Query("status"), Platform: c.Query("platform"), CategoryID: category, RequestKey: c.Query("request_key")})
	respondBatch(c, out, e)
}
func GetPlan(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	out, e := service.Wired().GetPlan(a, id)
	respondBatch(c, out, e)
}
func CreatePlan(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.CreatePlanInput
	if e := decodePlan(c, &in); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().CreatePlan(a, in)
	if e != nil {
		failBatch(c, e)
		return
	}
	api.Success(c, out)
}
func EditPlan(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	var in service.StructurePlanInput
	if e := decodePlan(c, &in); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().EditPlan(a, id, in)
	respondBatch(c, out, e)
}
func ValidatePlan(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.ValidatePlanInput
	if e := decodePlan(c, &in); e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().ValidatePlan(a, in)
	respondBatch(c, out, e)
}
func ListPlanBatches(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	page, size, e := batchPage(c)
	if e != nil {
		failBatch(c, e)
		return
	}
	var pending *bool
	if raw := c.Query("pending"); raw != "" {
		if raw != "true" && raw != "false" {
			failBatch(c, service.ErrInvalid)
			return
		}
		v, _ := strconv.ParseBool(raw)
		pending = &v
	}
	out, e := service.Wired().PlanBatches(a, id, page, size, c.Query("execution_status"), pending)
	respondBatch(c, out, e)
}
func ListPlanRecords(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	page, size, e := batchPage(c)
	if e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().PlanRecords(a, id, page, size)
	respondBatch(c, out, e)
}
func CopyPlan(_ context.Context, c *app.RequestContext) {
	a, id, ok := authorizeBatchID(c)
	if !ok {
		return
	}
	revision, e := batchQueryUint(c, "expected_revision", true)
	if e != nil {
		failBatch(c, e)
		return
	}
	out, e := service.Wired().CopyPlan(a, id, revision)
	respondBatch(c, out, e)
}
func RecommendPlan(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.PlanRecommendationInput
	if e := decodePlan(c, &in); e != nil {
		failBatch(c, e)
		return
	}
	run, e := service.Wired().PreparePlanRecommendation(ctx, a, in)
	if e != nil {
		failBatch(c, e)
		return
	}
	if !deepseek.Get().Ready() {
		api.FailureNamed(c, 503, errcode.PublishUnavailable, "AI 拆批服务未配置，请手动拆批或联系管理员", "publication_unavailable")
		return
	}
	api.StreamOneWithTimeout(c, api.StreamItem{UserID: int64(a.ID)}, 180*time.Second, func(ctx context.Context, progress func(string) error) (any, error) {
		if e := progress("generating"); e != nil {
			return nil, e
		}
		return run(ctx)
	})
}
