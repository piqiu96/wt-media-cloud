package publish

import (
	"context"

	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func ClaimPublicCheck(_ context.Context, c *app.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		fail(c, service.ErrNotFound)
		return
	}
	v, err := service.WiredPublicCheck().Claim(auth.Claims.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
func ObservePublication(_ context.Context, c *app.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		fail(c, service.ErrNotFound)
		return
	}
	id, err := taskID(c)
	if err != nil {
		fail(c, err)
		return
	}
	var in model.PublicCheckReport
	if err = decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	v, err := service.WiredPublicCheck().Observe(auth.Claims.UserID, id, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
