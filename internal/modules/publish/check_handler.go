package publish

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func checkActor(c *app.RequestContext, strict bool) (model.CheckActor, bool) {
	a, ok := localagent.AgentAuthFrom(c)
	if !ok {
		fail(c, service.ErrNotFound)
		return model.CheckActor{}, false
	}
	_, failure, err := localagent.CheckDesktopSession(a.Claims)
	if err != nil {
		fail(c, err)
		return model.CheckActor{}, false
	}
	if strict && failure != nil {
		api.FailureCode(c, 401, 11001, failure.Message, failure.Code, nil, false, failure.RecoveryAction)
		return model.CheckActor{}, false
	}
	return model.CheckActor{UserID: a.Claims.UserID, SessionID: a.Claims.SessionID, Current: failure == nil}, true
}

func ClaimPublicCheck(_ context.Context, c *app.RequestContext) {
	var in model.CheckClaimInput
	if len(c.Request.Body()) == 0 {
		fail(c, repository.ErrAgentUpgrade)
		return
	}
	if err := decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(c.Request.Body(), &fields) != nil {
		fail(c, service.ErrInvalid)
		return
	}
	if _, present := fields["settle_unreceived"]; present {
		if !in.SettleUnreceived || len(fields) != 2 {
			fail(c, service.ErrInvalid)
			return
		}
	} else if !in.Capacity.Valid() {
		fail(c, repository.ErrAgentUpgrade)
		return
	}
	actor, ok := checkActor(c, !in.SettleUnreceived)
	if !ok {
		return
	}
	v, err := service.WiredPublicCheck().ClaimV2(actor, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
func ObservePublication(_ context.Context, c *app.RequestContext) {
	id, err := taskID(c)
	if err != nil {
		fail(c, err)
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(c.Request.Body(), &fields) != nil {
		fail(c, service.ErrInvalid)
		return
	}
	if _, release := fields["release_only"]; release {
		var in model.PublicCheckRelease
		if err = decode(c, &in); err != nil {
			fail(c, err)
			return
		}
		actor, ok := checkActor(c, false)
		if !ok {
			return
		}
		v, e := service.WiredPublicCheck().Release(actor, id, in)
		if e != nil {
			fail(c, e)
			return
		}
		api.Success(c, v)
		return
	}
	var in model.PublicCheckReport
	if err = decode(c, &in); err != nil {
		fail(c, err)
		return
	}
	if in.ReservationID == 0 {
		fail(c, repository.ErrAgentUpgrade)
		return
	}
	actor, ok := checkActor(c, true)
	if !ok {
		return
	}
	v, err := service.WiredPublicCheck().Observe(actor.UserID, id, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, v)
}
