package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTLSVariablesServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRunnerVersion(t *testing.T) {
	var stdout, stderr strings.Builder
	runner := Runner{Stdout: &stdout, Stderr: &stderr}
	if code := runner.Run(context.Background(), []string{"version"}); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wtmctl") {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestDeploymentPlanJSONShape(t *testing.T) {
	raw, err := json.Marshal(map[string]bool{"server": true})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"server":true}` {
		t.Fatalf("json=%s", raw)
	}
}
