package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestInitializeServerInitializesResourcesInOrder(t *testing.T) {
	var order []string
	steps := testResourceSteps(&order, "config", "logger", "metrics", "tracing", "database", "http-clients", "agent-client", "douyin-client")

	engine, closer, err := initializeServer(
		steps,
		"test-addr",
		func(addr string) *server.Hertz {
			order = append(order, "engine:"+addr)
			return server.New()
		},
		func(*server.Hertz) error {
			order = append(order, "routes")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("initializeServer() error = %v", err)
	}
	if engine == nil {
		t.Fatal("initializeServer() engine = nil")
	}
	if err := closer(); err != nil {
		t.Fatalf("closer() error = %v", err)
	}

	want := []string{
		"open:config", "open:logger", "open:metrics", "open:tracing",
		"open:database", "open:http-clients", "open:agent-client", "open:douyin-client",
		"engine:test-addr", "routes",
	}
	assertOrder(t, order[:len(want)], want)
}

func TestInitializeServerRollsBackInReverseOrder(t *testing.T) {
	var order []string
	steps := []resourceStep{
		{name: "first", open: func() (func() error, error) {
			order = append(order, "open:first")
			return func() error { order = append(order, "close:first"); return nil }, nil
		}},
		{name: "second", open: func() (func() error, error) {
			order = append(order, "open:second")
			return func() error { order = append(order, "close:second"); return nil }, nil
		}},
		{name: "third", open: func() (func() error, error) {
			order = append(order, "open:third")
			return nil, errForcedFailure
		}},
	}

	engine, closer, err := initializeServer(steps, "test-addr", func(string) *server.Hertz { return server.New() }, func(*server.Hertz) error { return nil })
	if err == nil || !errors.Is(err, errForcedFailure) {
		t.Fatalf("initializeServer() error = %v, want forced failure", err)
	}
	if engine != nil || closer != nil {
		t.Fatal("initializeServer() returned engine or closer, want nil")
	}
	assertOrder(t, order, []string{"open:first", "open:second", "open:third", "close:second", "close:first"})
}

func TestInitializeWorkerInitializesOnlyDouyinHTTPClient(t *testing.T) {
	names := resourceStepNames(workerResourcePlan())
	want := []string{"config", "logger", "metrics", "tracing", "database", "http-clients", "douyin-client"}
	assertOrder(t, names, want)
}

func TestLoadHTTPClientDocumentsSelectsOnlyRequestedNames(t *testing.T) {
	dir := t.TempDir()
	writeHTTPClientDocument(t, dir, "agent")
	writeHTTPClientDocument(t, dir, "douyin")

	documents, err := loadHTTPClientDocuments(dir, "douyin")
	if err != nil {
		t.Fatalf("loadHTTPClientDocuments() error = %v", err)
	}
	if len(documents) != 1 || documents[0].Name != "douyin" {
		t.Fatalf("documents = %#v, want only douyin", documents)
	}
}

func TestLoadHTTPClientDocumentsRejectsMissingName(t *testing.T) {
	dir := t.TempDir()
	writeHTTPClientDocument(t, dir, "agent")

	if _, err := loadHTTPClientDocuments(dir, "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("loadHTTPClientDocuments() error = %v, want missing name", err)
	}
}

func writeHTTPClientDocument(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name+".toml")
	content := "timeout = \"1s\"\n[retry]\nattempts = 1\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write HTTP client config: %v", err)
	}
}

func TestInitializeSchedulerDoesNotInitializeDouyinClient(t *testing.T) {
	names := resourceStepNames(schedulerResourcePlan())
	want := []string{"config", "logger", "metrics", "tracing", "database"}
	assertOrder(t, names, want)
}

func TestCloseStackIsIdempotent(t *testing.T) {
	calls := 0
	closer, err := initializeResources([]resourceStep{{
		name: "resource",
		open: func() (func() error, error) {
			return func() error { calls++; return nil }, nil
		},
	}})
	if err != nil {
		t.Fatalf("initializeResources() error = %v", err)
	}
	if err := closer(); err != nil {
		t.Fatalf("first closer() error = %v", err)
	}
	if err := closer(); err != nil {
		t.Fatalf("second closer() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("close calls = %d, want 1", calls)
	}
}

func testResourceSteps(order *[]string, names ...string) []resourceStep {
	steps := make([]resourceStep, 0, len(names))
	for _, name := range names {
		steps = append(steps, resourceStep{
			name: name,
			open: func() (func() error, error) {
				*order = append(*order, "open:"+name)
				return func() error {
					*order = append(*order, "close:"+name)
					return nil
				}, nil
			},
		})
	}
	return steps
}

func resourceStepNames(steps []resourceStep) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.name)
	}
	return names
}

func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order length = %d, want %d; got %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("order[%d] = %q, want %q; got %v", index, got[index], want[index], got)
		}
	}
}

var errForcedFailure = errors.New("forced failure")
