package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestCloudWebServesEntryAssetAndHistoryRoute(t *testing.T) {
	root := t.TempDir()
	writeCloudWebFile(t, root, "index.cloud.html", "<html>cloud-entry</html>")
	writeCloudWebFile(t, root, "assets/app.js", "console.log('asset')")
	writeCloudWebFile(t, root, "desktop-downloads.json", `{"schema_version":1,"version":"v0.1.0"}`)

	engine := server.New()
	if err := registerCloudWeb(engine, root); err != nil {
		t.Fatalf("registerCloudWeb() error = %v", err)
	}

	for _, testCase := range []struct {
		path string
		want string
	}{
		{"/", "cloud-entry"},
		{"/assets/app.js", "console.log('asset')"},
		{"/login", "cloud-entry"},
		{"/home", "cloud-entry"},
		{"/desktop-downloads.json", `"version":"v0.1.0"`},
	} {
		response := ut.PerformRequest(engine.Engine, consts.MethodGet, testCase.path, nil)
		if response.Result().StatusCode() != consts.StatusOK {
			t.Fatalf("GET %s status = %d body=%s", testCase.path, response.Result().StatusCode(), response.Result().Body())
		}
		if !strings.Contains(string(response.Result().Body()), testCase.want) {
			t.Fatalf("GET %s body = %q, want substring %q", testCase.path, response.Result().Body(), testCase.want)
		}
	}
}

func TestCloudWebKeepsAPINotFoundAndRejectsNonReadMethods(t *testing.T) {
	root := t.TempDir()
	writeCloudWebFile(t, root, "index.cloud.html", "<html>cloud-entry</html>")

	engine := server.New()
	if err := registerCloudWeb(engine, root); err != nil {
		t.Fatalf("registerCloudWeb() error = %v", err)
	}

	for _, request := range []struct {
		method string
		path   string
	}{
		{consts.MethodGet, "/api/v1/not-registered"},
		{consts.MethodPost, "/login"},
	} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusNotFound {
			t.Fatalf("%s %s status = %d body=%s, want 404", request.method, request.path, response.Result().StatusCode(), response.Result().Body())
		}
		if strings.Contains(string(response.Result().Body()), "cloud-entry") {
			t.Fatalf("%s %s returned the Cloud entry page", request.method, request.path)
		}
	}
}

func TestCloudWebDoesNotServeOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "web")
	writeCloudWebFile(t, root, "index.cloud.html", "<html>cloud-entry</html>")
	writeCloudWebFile(t, parent, "secret.txt", "outside-secret")

	engine := server.New()
	if err := registerCloudWeb(engine, root); err != nil {
		t.Fatalf("registerCloudWeb() error = %v", err)
	}

	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/%2e%2e/secret.txt", nil)
	if strings.Contains(string(response.Result().Body()), "outside-secret") {
		t.Fatal("directory traversal returned a file outside the web root")
	}

	link := filepath.Join(root, "linked-secret.txt")
	if err := os.Symlink(filepath.Join(parent, "secret.txt"), link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	response = ut.PerformRequest(engine.Engine, consts.MethodGet, "/linked-secret.txt", nil)
	if strings.Contains(string(response.Result().Body()), "outside-secret") {
		t.Fatal("symlink returned a file outside the web root")
	}
}

func TestCloudWebRequiresEntryFile(t *testing.T) {
	if err := registerCloudWeb(server.New(), t.TempDir()); err == nil {
		t.Fatal("registerCloudWeb() error = nil, want missing entry error")
	}
}

func writeCloudWebFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
