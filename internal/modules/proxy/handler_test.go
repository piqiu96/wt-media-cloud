package proxy

import (
	"encoding/json"
	"errors"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
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

func TestAssignmentErrorsAreDistinctAndMatchContract(t *testing.T) {
	for _, tc := range []struct {
		name         string
		err          error
		status, code int
	}{
		{"rejected", errProxyRejected, 409, 23101},
		{"old agent", errProxyAgentUpgrade, 409, 23102},
		{"unconfirmed", errProxyReadback, 503, 30008},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := hertzapp.NewContext(0)
			writeProxyAssignmentError(c, tc.err)
			var body struct {
				Errcode int `json:"errcode"`
			}
			if err := json.Unmarshal(c.Response.Body(), &body); err != nil {
				t.Fatal(err)
			}
			if c.Response.StatusCode() != tc.status || body.Errcode != tc.code {
				t.Fatalf("status=%d code=%d", c.Response.StatusCode(), body.Errcode)
			}
		})
	}
}
