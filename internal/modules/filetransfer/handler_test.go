package filetransfer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
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

// transferErrorCases is this module's whole error surface in one table: the
// sentinel, the answer the handler must give, and the name the frozen contract
// publishes for it. The two tests below read it from opposite ends — one drives
// the handler and reads the response, the other reads
// `contracts/cloud-error-codes/v1/file-transfer.yaml` — so a mapping that is
// internally consistent but names something the contract never declared cannot
// pass both. `errorType` is empty for the arms that carry no name: the frozen
// contract publishes six names and these three failures are not among them.
var transferErrorCases = []struct {
	name      string
	err       error
	status    int
	errcode   int
	errorType string
}{
	{"invalid input", transferservice.ErrInvalidInput, 400, 10001, ""},
	{"unauthenticated node", transferservice.ErrNodeUnauthenticated, 401, 11001, ""},
	{"task not found", transferservice.ErrTaskNotFound, 404, 15101, "transfer_task_not_found"},
	{"task forbidden", transferservice.ErrTaskForbidden, 403, 15102, "transfer_task_forbidden"},
	{"task conflict", transferservice.ErrTaskConflict, 409, 15103, "transfer_task_conflict"},
	{"task cancelled", transferservice.ErrTaskCancelled, 409, 15104, "transfer_task_cancelled"},
	{"integrity failed", transferservice.ErrIntegrityFailed, 422, 15106, "transfer_integrity_failed"},
	{"unmapped failure", errors.New("no sentinel matches this"), 500, 50000, ""},
}

// The mapping asserted on real responses rather than on the source text. Two of
// these arms are 409 and differ only in `error.type`; the 422 is the one the
// contract reserves for an integrity failure, and a regression that folded it
// into the conflict arm would leave every other test in the repository green —
// the service still returns the same sentinel, only the status it becomes changes.
func TestWriteTransferErrorCarriesTheFrozenNames(t *testing.T) {
	for _, testCase := range transferErrorCases {
		engine := server.New()
		engine.POST("/probe", func(_ context.Context, c *hertzapp.RequestContext) {
			writeTransferError(c, testCase.err)
		})
		result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/probe", nil)

		if got := result.Result().StatusCode(); got != testCase.status {
			t.Fatalf("%s: status = %d, want %d", testCase.name, got, testCase.status)
		}
		var body struct {
			ErrCode int `json:"errcode"`
			Error   *struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
			t.Fatalf("%s: decode response: %v", testCase.name, err)
		}
		if body.ErrCode != testCase.errcode {
			t.Fatalf("%s: errcode = %d, want %d", testCase.name, body.ErrCode, testCase.errcode)
		}
		// An arm with no frozen name must omit `error` entirely rather than send an
		// empty type: `"type": ""` reads as "a name is published and it is empty",
		// which is a weaker and wrong promise to a client branching on the field.
		if testCase.errorType == "" {
			if body.Error != nil {
				t.Fatalf("%s: error = %+v, want the key absent", testCase.name, body.Error)
			}
			continue
		}
		if body.Error == nil || body.Error.Type != testCase.errorType {
			t.Fatalf("%s: error.type = %+v, want %q", testCase.name, body.Error, testCase.errorType)
		}
	}
}

// The names above are copies of what `contracts/cloud-error-codes/` publishes, and
// a copy nothing checks is a copy that drifts. This reads the frozen file and
// asserts each name is declared there with the status this module answers, so a
// contract revision that renames one or changes its status fails here rather than
// in an Agent that branches on it.
//
// The status is compared, not just the name: the two 409s are the case that makes
// the name load-bearing, and a name moved to the wrong status would still be
// "declared" under a name-only check.
func TestTheTransferErrorNamesAreTheFrozenOnes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-error-codes", "v1", "file-transfer.yaml"))
	if err != nil {
		t.Fatalf("read frozen error codes: %v", err)
	}
	frozen := string(content)
	checked := 0
	for _, testCase := range transferErrorCases {
		if testCase.errorType == "" {
			continue
		}
		declaration := testCase.errorType + ": {http_status: " + strconv.Itoa(testCase.status) + "}"
		if !strings.Contains(frozen, declaration) {
			t.Errorf("file-transfer.yaml does not declare %s", declaration)
		}
		checked++
	}
	// A guard that read no names would pass against any file at all, including an
	// empty one. Five are expected: the six frozen names less
	// `local_transfer_node_unavailable`, which production answers, not this module.
	if checked != 5 {
		t.Fatalf("checked %d names, want 5: the loop is no longer covering the table", checked)
	}
}
