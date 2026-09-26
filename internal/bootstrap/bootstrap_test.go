package bootstrap

import (
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"

	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

func TestInitializeServerInitializesResourcesInOrder(t *testing.T) {
	var order []string
	steps := testResourceSteps(&order, "config", "logger", "metrics", "tracing", "database", "clients")

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
		"open:database", "open:clients",
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
	// `storage` was added to this list deliberately: the preparation worker writes
	// the prepared source object, so it needs the store. `clients` is still
	// pinned to douyin alone.
	want := []string{"config", "logger", "metrics", "tracing", "database", "storage", "clients"}
	assertOrder(t, names, want)
}

// The server plan had no assertion at all: the test above it builds its own
// synthetic steps, so the real plan could lose a step and nothing would say so.
// `storage` is the one that matters most — a claim mints the lease's download
// grant, so a server without the store answers every claim with an internal
// error, and the failure would look like the executor's.
func TestInitializeServerPlanCarriesStorage(t *testing.T) {
	names := resourceStepNames(serverResourcePlan())
	want := []string{"config", "logger", "metrics", "tracing", "database", "storage", "clients"}
	assertOrder(t, names, want)
}

// The two plans that must not need an object-storage credential. The migration
// runner has to be able to migrate a database in an environment where no bucket
// is reachable, and the scheduler does no object I/O — if either grew a storage
// step, the failure would appear as a deployment that cannot start, and the cause
// would be a resource nobody thought they depended on.
func TestTheMigrationAndSchedulerPlansCarryNoStorage(t *testing.T) {
	for name, plan := range map[string][]resourceStep{
		"migration": migrationResourcePlan(),
		"scheduler": schedulerResourcePlan(),
	} {
		for _, step := range plan {
			if step.name == "storage" {
				t.Errorf("the %s plan initializes object storage", name)
			}
		}
	}
}

func TestSelectHTTPClientConfigsDefaultsToAllAndSupportsNames(t *testing.T) {
	configs := []httpclient.Config{{Name: "douyin"}, {Name: "agent"}}

	all, err := selectHTTPClientConfigs(configs)
	if err != nil {
		t.Fatalf("selectHTTPClientConfigs() error = %v", err)
	}
	if len(all) != 2 || all[0].Name != "agent" || all[1].Name != "douyin" {
		t.Fatalf("all = %+v, want sorted agent and douyin", all)
	}

	onlyDouyin, err := selectHTTPClientConfigs(configs, "douyin")
	if err != nil {
		t.Fatalf("selectHTTPClientConfigs(douyin) error = %v", err)
	}
	if len(onlyDouyin) != 1 || onlyDouyin[0].Name != "douyin" {
		t.Fatalf("onlyDouyin = %+v", onlyDouyin)
	}

	if _, err := selectHTTPClientConfigs(configs, "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("selectHTTPClientConfigs(missing) error = %v, want missing name", err)
	}
}

// The scheduler plan assembles no clients, so nothing on its call path may
// reach douyinclient.Get()/agentclient.Get()/httpclient.Get(), which panic
// before Initialize and would kill the process from an unrecovered job
// goroutine. The scheduling service is kept crawler-free on purpose:
// contentpool's defaultSchedulerDiscoveryService (see its TestSchedulerDiscoveryServiceCarriesNoCrawler).
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
