package publish

import (
	"context"
	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func Start(_ context.Context, c *app.RequestContext) {
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
	v, e := service.WiredExecution().Start(actor, id, in.Revision)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func Attempt(_ context.Context, c *app.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, e := taskID(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := service.WiredExecution().Attempt(actor, id)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func Claim(ctx context.Context, c *app.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		fail(c, service.ErrNotFound)
		return
	}
	var in struct {
		Key string `json:"request_key"`
	}
	if e := decode(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.WiredExecution().Claim(ctx, auth.Claims.UserID, in.Key)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func Report(_ context.Context, c *app.RequestContext) {
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
	var in model.ExecutionReport
	if e = decode(c, &in); e != nil {
		fail(c, e)
		return
	}
	current := false
	if in.Stage != "" {
		_, failure, checkErr := localagent.CheckDesktopSession(auth.Claims)
		if checkErr != nil {
			fail(c, checkErr)
			return
		}
		current = failure == nil
	}
	v, e := service.WiredExecution().Report(auth.Claims.UserID, id, in, current)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
func Complete(_ context.Context, c *app.RequestContext) {
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
	var in model.Completion
	if e = decode(c, &in); e != nil {
		fail(c, e)
		return
	}
	v, e := service.WiredExecution().Complete(auth.Claims.UserID, id, in)
	if e != nil {
		fail(c, e)
		return
	}
	api.Success(c, v)
}
