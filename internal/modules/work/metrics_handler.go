package work

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
	"io"
	"strconv"
	"time"
)

func metricID(c *app.RequestContext) (uint64, error) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil || id == 0 {
		return 0, service.ErrInvalid
	}
	return id, nil
}
func RefreshMetrics(_ context.Context, c *app.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := metricID(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := service.WiredMetrics().Request(a, id, time.Now().UTC())
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func ClaimMetrics(_ context.Context, c *app.RequestContext) {
	a, ok := localagent.AgentAuthFrom(c)
	if !ok {
		return
	}
	v, e := service.WiredMetrics().Claim(a.User, time.Now().UTC())
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func ReportMetrics(_ context.Context, c *app.RequestContext) {
	a, ok := localagent.AgentAuthFrom(c)
	if !ok {
		return
	}
	id, e := metricID(c)
	if e != nil {
		fail(c, e)
		return
	}
	var in model.MetricsReport
	body := c.Request.Body()
	if len(body) > 4096 {
		fail(c, service.ErrInvalid)
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF {
		fail(c, service.ErrInvalid)
		return
	}
	v, e := service.WiredMetrics().Report(a.Claims.UserID, id, in, time.Now().UTC())
	if errors.Is(e, repository.ErrMetricsLease) {
		api.FailureNamed(c, 409, 10001, "指标更新已过期，请重新刷新", "work_metrics_lease_lost")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
