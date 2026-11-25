package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	images "github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/errcode"
	"io"
	"strconv"
	"time"
)

func RegisterRoutes(h *server.Hertz) {
	registerBatchRoutes(h)
	registerPlanRoutes(h)
	h.GET("/api/v1/publication", List)
	h.GET("/api/v1/publication/output", OutputCandidates)
	h.GET("/api/v1/publication/request", FindByRequest)
	h.POST("/api/v1/publication", Create)
	h.POST("/api/v1/publication/generate", GeneratePublicationContent)
	h.GET("/api/v1/publication/:id", Get)
	h.PUT("/api/v1/publication/:id/content", Edit)
	h.POST("/api/v1/publication/:id/start", Start)
	h.POST("/api/v1/publication/:id/resolve", Resolve)
	h.POST("/api/v1/publication/:id/retry", Retry)
	h.POST("/api/v1/publication/:id/cancel", Cancel)
	h.POST("/api/v1/publication/:id/abandon", Abandon)
	h.GET("/api/v1/publication/:id/copy", Copy)
	h.GET("/api/v1/cloud-agent/publication-task/:id/attempt/:attempt_id", localagent.RequireAgent(localagent.ReportSafeMode), OriginalAttempt)
	h.GET("/api/v1/publication/:id/attempt", Attempt)
	h.GET("/api/v1/cloud-agent/publication-task/:id/attempt/:attempt_id/scene", localagent.RequireAgent(localagent.StrictMode), GetHumanScene)
	h.POST("/api/v1/cloud-agent/publication-task/:id/resume", localagent.RequireAgent(localagent.StrictMode), Resume)
	h.POST("/api/v1/cloud-agent/publication-task/check/claim", localagent.RequireAgent(localagent.ReportSafeMode), ClaimPublicCheck)
	h.POST("/api/v1/cloud-agent/publication-task/:id/check", localagent.RequireAgent(localagent.ReportSafeMode), ObservePublication)
	h.POST("/api/v1/cloud-agent/publication-task/claim", localagent.RequireAgent(localagent.StrictMode), Claim)
	h.POST("/api/v1/cloud-agent/publication-task/:id/report", localagent.RequireAgent(localagent.ReportSafeMode), Report)
	h.POST("/api/v1/cloud-agent/publication-task/:id/complete", localagent.RequireAgent(localagent.ReportSafeMode), Complete)
}
func fail(c *app.RequestContext, err error) {
	if failBatchControl(c, err) {
		return
	}
	switch {
	case errors.Is(err, images.ErrInvalid), errors.Is(err, service.ErrInvalid), errors.Is(err, files.ErrInvalid), errors.Is(err, production.ErrComposeOutputInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, errcode.PublishInvalid, "请检查发布资源并确认文案、封面和声明", "publication_invalid")
	case errors.Is(err, images.ErrNotFound), errors.Is(err, service.ErrNotFound), errors.Is(err, files.ErrNotFound), errors.Is(err, accounts.ErrNotFound), errors.Is(err, accounts.ErrForbidden), errors.Is(err, production.ErrComposeOutputNotFound), errors.Is(err, production.ErrComposeOutputForbidden):
		api.FailureNamed(c, consts.StatusNotFound, errcode.PublishNotFound, "任务或资源不存在，或没有访问权限", "publication_not_found")
	case errors.Is(err, service.ErrConflict):
		var conflict *service.ConflictError
		var details any
		if errors.As(err, &conflict) {
			details = conflict
		}
		api.FailureCode(c, consts.StatusConflict, errcode.PublishConflict, "任务或输入已变化，请重新加载后编辑", "publication_conflict", details, false, "reload")
	case errors.Is(err, service.ErrOccupied):
		api.FailureNamed(c, consts.StatusConflict, errcode.PublishOutputOccupied, "该成片已被当前平台的任务占用", "publication_output_occupied")
	case errors.Is(err, service.ErrClosed):
		api.FailureNamed(c, consts.StatusConflict, errcode.PublishPlatformClosed, "当前平台尚未开放该操作", "publication_platform_closed")
	default:
		api.FailureNamed(c, consts.StatusServiceUnavailable, errcode.PublishUnavailable, "操作尚未完成，请保留当前内容并重试", "publication_unavailable")
	}
}
func decode(c *app.RequestContext, in any) error {
	d := json.NewDecoder(bytes.NewReader(c.Request.Body()))
	d.DisallowUnknownFields()
	if err := d.Decode(in); err != nil {
		return service.ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return service.ErrInvalid
	}
	return nil
}
func taskID(c *app.RequestContext) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, service.ErrInvalid
	}
	return id, nil
}
func List(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	f := service.Filter{Page: 1, PageSize: 20, Search: c.Query("search"), Status: c.Query("status"), Platform: c.Query("platform")}
	if raw := c.Query("category_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			fail(c, service.ErrInvalid)
			return
		}
		f.CategoryID = id
	}
	for key, target := range map[string]**time.Time{"created_from": &f.CreatedFrom, "created_before": &f.CreatedBefore} {
		if raw := c.Query(key); raw != "" {
			v, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				fail(c, service.ErrInvalid)
				return
			}
			*target = &v
		}
	}
	for key, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize} {
		if raw := c.Query(key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				fail(c, service.ErrInvalid)
				return
			}
			*target = n
		}
	}
	s := service.Wired()
	items, total, err := s.List(a, f)
	if err != nil {
		fail(c, err)
		return
	}
	summary, err := s.Summary(a, f)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, map[string]any{"summary": summary, "items": items, "total": total, "page": f.Page, "page_size": f.PageSize})
}
func FindByRequest(_ context.Context, c *app.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	task, err := service.Wired().FindByRequest(actor, c.Query("request_key"))
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, task)
}

func Create(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.CreateInput
	if err := decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Create(ctx, a, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Created(c, v)
}
func Get(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, err := taskID(c)
	if err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Get(a, id)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
func Edit(ctx context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, err := taskID(c)
	if err != nil {
		fail(c, err)
		return
	}
	var in service.EditInput
	if err := decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	v, err := service.Wired().Edit(ctx, a, id, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}

func failBatchControl(c *app.RequestContext, err error) bool {
	var code int
	var name, message, action string
	var retryable bool
	var item *repository.BatchExecutionConflict
	var planConflict *repository.PlanConflictError
	var details any
	if errors.As(err, &planConflict) {
		api.FailureCode(c, consts.StatusConflict, errcode.PublishConflict, "计划候选存在跨批重复成片", "publication_conflict", planConflict, false, "reload")
		return true
	}
	if errors.As(err, &item) {
		details = item
	}
	switch {
	case errors.Is(err, repository.ErrParentBlocked):
		code, name, message, action = errcode.PublishBatchParentBlocked, "publication_batch_parent_blocked", "当前批次或执行组不允许此操作", "reload"
	case errors.Is(err, repository.ErrShortfallChanged):
		code, name, message, action = errcode.PublishBatchShortfallChanged, "publication_batch_shortfall_changed", "当前创建缺口已变化，请重新检查并确认", "reload"
	case errors.Is(err, repository.ErrPauseChanged):
		code, name, message, action = errcode.PublishBatchPauseChanged, "publication_batch_pause_changed", "暂停或异常依据已变化，请重新确认", "reload"
	case errors.Is(err, repository.ErrUnresolved):
		code, name, message, action = errcode.PublishBatchUnresolved, "publication_batch_unresolved", "仍有未决任务或失败集合已变化", "reload"
	case errors.Is(err, repository.ErrCapacityWait):
		code, name, message, action, retryable = errcode.PublishCapacityWait, "publication_capacity_wait", "自动执行名额已满，请稍后继续原现场", "retry", true
	case errors.Is(err, repository.ErrAgentUpgrade):
		code, name, message, action = errcode.PublishAgentUpgradeRequired, "publication_agent_upgrade_required", "当前 Agent 版本需要升级后执行发布", "none"
	case errors.Is(err, repository.ErrProfileUse):
		code, name, message, action = errcode.PublishProfileUseConflict, "publication_profile_use_conflict", "原 Profile 使用记录或安全关闭依据已变化", "reload"
	case errors.Is(err, repository.ErrAdmissionClosed):
		code, name, message, action, retryable = errcode.PublishAdmissionClosed, "publication_admission_closed", "发布正等待既有核验与窗口收尾", "retry", true
	case item != nil && errors.Is(err, repository.ErrOccupied):
		code, name, message, action = errcode.PublishOutputOccupied, "publication_output_occupied", "该成片已被当前平台的任务占用", "reload"
	case item != nil && errors.Is(err, repository.ErrConflict):
		code, name, message, action = errcode.PublishConflict, "publication_conflict", "发布条件已变化，请重新检查", "reload"
	default:
		return false
	}
	api.FailureCode(c, consts.StatusConflict, code, message, name, details, retryable, action)
	return true
}
