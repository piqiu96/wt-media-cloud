package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hertzclient "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

func TestNewConvertsConnectionAndRetryOptions(t *testing.T) {
	cfg := decode(t, `
timeout = "4s"

[connection]
dial_timeout = "1s"
read_timeout = "2s"
write_timeout = "3s"
max_conns_per_host = 21
max_idle_conn_duration = "30s"
max_conn_duration = "2m"
max_conn_wait_timeout = "700ms"
keep_alive = true

[retry]
attempts = 3
delay = "100ms"
max_delay = "1s"
policy = "backoff"
`)

	instance, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer closer()

	options := instance.hertz.GetOptions()
	if options.DialTimeout != time.Second || options.ReadTimeout != 2*time.Second || options.WriteTimeout != 3*time.Second {
		t.Fatalf("connection timeouts = dial:%s read:%s write:%s", options.DialTimeout, options.ReadTimeout, options.WriteTimeout)
	}
	if options.MaxConnsPerHost != 21 || options.MaxIdleConnDuration != 30*time.Second || options.MaxConnDuration != 2*time.Minute || options.MaxConnWaitTimeout != 700*time.Millisecond || !options.KeepAlive {
		t.Fatalf("connection options = %+v", options)
	}
	if options.RetryConfig == nil || options.RetryConfig.MaxAttemptTimes != 3 || options.RetryConfig.Delay != 100*time.Millisecond || options.RetryConfig.MaxDelay != time.Second {
		t.Fatalf("retry options = %+v", options.RetryConfig)
	}
}

func TestNewAppliesNamedMiddleware(t *testing.T) {
	calls := make([]string, 0, 2)
	middleware := func(next hertzclient.Endpoint) hertzclient.Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
			calls = append(calls, "before")
			err := next(ctx, req, resp)
			calls = append(calls, "after")
			return err
		}
	}
	instance, closer, err := New(Config{Timeout: Duration{time.Second}, Retry: RetryConfig{Attempts: 1}}, middleware)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer closer()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()

	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	defer protocol.ReleaseRequest(req)
	defer protocol.ReleaseResponse(resp)
	req.SetMethod("GET")
	req.SetRequestURI(server.URL)
	if err := instance.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if len(calls) != 2 || calls[0] != "before" || calls[1] != "after" {
		t.Fatalf("middleware calls = %v", calls)
	}
}

func TestNewRejectsInvalidSettings(t *testing.T) {
	tests := []Config{
		{Timeout: Duration{}, Connection: ConnectionConfig{DialTimeout: Duration{time.Second}}, Retry: RetryConfig{Attempts: 1}},
		{Timeout: Duration{time.Second}, Connection: ConnectionConfig{DialTimeout: Duration{time.Second}}, Retry: RetryConfig{Attempts: 0}},
		{Timeout: Duration{time.Second}, Connection: ConnectionConfig{DialTimeout: Duration{time.Second}}, Retry: RetryConfig{Attempts: 1, Delay: Duration{-time.Second}}},
		{Timeout: Duration{time.Second}, Connection: ConnectionConfig{DialTimeout: Duration{time.Second}}, Retry: RetryConfig{Attempts: 1, Policy: "random"}},
		{Timeout: Duration{time.Second}, Connection: ConnectionConfig{DialTimeout: Duration{-time.Second}}, Retry: RetryConfig{Attempts: 1}},
	}
	for index, cfg := range tests {
		if _, _, err := New(cfg); err == nil {
			t.Fatalf("tests[%d] New() error = nil", index)
		}
	}
}

func decode(t *testing.T, content string) Config {
	t.Helper()
	document := pkgconfig.Document{Name: "test", Format: pkgconfig.FormatTOML, Raw: []byte(content)}
	var cfg Config
	if err := document.Decode(&cfg); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return cfg
}

func TestDoAppliesWholeRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	instance, closer, err := New(Config{
		Timeout:    Duration{5 * time.Millisecond},
		Connection: ConnectionConfig{DialTimeout: Duration{time.Second}},
		Retry:      RetryConfig{Attempts: 1},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer closer()

	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	defer protocol.ReleaseRequest(req)
	defer protocol.ReleaseResponse(resp)
	req.SetMethod("GET")
	req.SetRequestURI(server.URL)
	if err := instance.Do(context.Background(), req, resp); err == nil {
		t.Fatal("Do() with an expired whole-request timeout succeeded")
	}
}
