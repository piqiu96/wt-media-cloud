package proxy

import (
	"errors"
	agentclient "github.com/wt-media/wt-media-cloud/internal/infra/client/agent"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesBindsProxyEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/proxies", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated proxy list status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestRegisterRoutesBindsProfileProxyUnbind(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/proxies/profile-unbind", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated profile unbind status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestProxyAssignmentAcknowledgementClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result agentclient.ProxyAssignmentResult
		err    error
		want   error
	}{
		{"accepted", agentclient.ProxyAssignmentResult{ProfileID: "p1", Accepted: true}, nil, nil},
		{"wrong profile", agentclient.ProxyAssignmentResult{ProfileID: "other", Accepted: true}, nil, errProxyReadback},
		{"missing acknowledgement", agentclient.ProxyAssignmentResult{ProfileID: "p1"}, nil, errProxyReadback},
		{"explicit refusal", agentclient.ProxyAssignmentResult{}, &agentclient.ResponseError{Status: 409}, errProxyRejected},
		{"old agent", agentclient.ProxyAssignmentResult{}, &agentclient.ResponseError{Status: 404}, errProxyAgentUpgrade},
		{"lost response", agentclient.ProxyAssignmentResult{}, errors.New("timeout"), errProxyReadback},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyAssignmentAck(tc.result, tc.err, "p1"); !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
