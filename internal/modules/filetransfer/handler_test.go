package filetransfer

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Every route the module publishes, in the order the router installs them, with
// the family it belongs to. The two families answer the same 401 to a caller with
// no credential — one because a session is missing, the other because a node
// bearer token is — and that is the point: a request naming no caller cannot be
// told apart from a request naming a path nobody registered unless the routes are
// actually bound.
func TestRegisterRoutesBindsTransferEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	routes := []struct {
		family string
		method string
		path   string
	}{
		{"session", consts.MethodGet, "/api/v1/file-transfer-tasks"},
		{"session", consts.MethodPost, "/api/v1/file-transfer-tasks/task-1/cancel"},
		{"session", consts.MethodPost, "/api/v1/file-transfer-tasks/task-1/retry"},
		{"executor", consts.MethodPost, "/api/v1/cloud-agent/file-transfer-tasks/claim"},
		{"executor", consts.MethodPost, "/api/v1/cloud-agent/file-transfer-tasks/task-1/heartbeat"},
		{"executor", consts.MethodPost, "/api/v1/cloud-agent/file-transfer-tasks/task-1/progress"},
		{"executor", consts.MethodPost, "/api/v1/cloud-agent/file-transfer-tasks/task-1/complete"},
	}

	// The control that keeps the loop honest: an engine where every path answers
	// 401 would let a renamed or dropped route pass as bound. Unregistered paths
	// have to answer 404, so the assertion below distinguishes the two.
	unbound := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/file-transfer-tasks/nothing-here", nil)
	if unbound.Result().StatusCode() != consts.StatusNotFound {
		t.Fatalf("unbound path status = %d, want 404: this engine cannot tell a bound route from an unbound one", unbound.Result().StatusCode())
	}

	for _, route := range routes {
		// A nil body on purpose, including for the four POSTs: the executor
		// handlers read the credential before they decode, so the credential-less
		// answer does not depend on the body being parseable.
		response := ut.PerformRequest(engine.Engine, route.method, route.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s (%s route) status = %d body=%s, want 401", route.method, route.path, route.family, response.Result().StatusCode(), response.Result().Body())
		}
	}
}

// The executor handlers take their credential out of the Authorization header and
// nowhere else, and this pins the accepted shape. `Bearer` with no value, a bare
// token and a different scheme all have to be refused, because a route that
// accepted any of them would be authenticating on the presence of a header.
func TestBearerCredentialAcceptsOnlyABearerValue(t *testing.T) {
	for _, testCase := range []struct {
		header     string
		credential string
		ok         bool
	}{
		{"Bearer node-secret", "node-secret", true},
		{"Bearer   node-secret  ", "node-secret", true},
		{"Bearer ", "", false},
		{"Bearer", "", false},
		{"node-secret", "", false},
		{"Basic node-secret", "", false},
		{"", "", false},
	} {
		credential, ok := bearerCredential(testCase.header)
		if ok != testCase.ok || credential != testCase.credential {
			t.Fatalf("bearerCredential(%q) = (%q, %t), want (%q, %t)", testCase.header, credential, ok, testCase.credential, testCase.ok)
		}
	}
}
