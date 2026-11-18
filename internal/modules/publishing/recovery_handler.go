package publishing

import (
	"context"
	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"strconv"
)

func Resolve(_ context.Context, c *app.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := taskID(c)
	if e != nil {
		fail(c, e)
		return
	}
	var in model.FailureResolution
	if e = decode(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.WiredRecovery().Resolve(actor, id, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func Retry(_ context.Context, c *app.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := taskID(c)
	if e != nil {
		fail(c, e)
		return
	}
	var in struct {
		Revision uint64 `json:"expected_revision,string"`
	}
	if e = decode(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.WiredRecovery().Retry(actor, id, in.Revision)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func OriginalAttempt(_ context.Context, c *app.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		fail(c, service.ErrNotFound)
		return
	}
	id, e := taskID(c)
	if e != nil {
		fail(c, e)
		return
	}
	attempt, e := strconv.ParseUint(c.Param("attempt_id"), 10, 64)
	if e != nil || attempt == 0 {
		fail(c, service.ErrInvalid)
		return
	}
	v, e := service.WiredRecovery().OriginalAttempt(auth.Claims.UserID, id, attempt)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
