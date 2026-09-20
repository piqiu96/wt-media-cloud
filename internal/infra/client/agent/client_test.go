package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

func TestAgentClientAddsCredentialWithoutBusinessPassingHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer agent-secret"; got != want {
			t.Errorf("authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
			t.Errorf("content type = %q, want %q", got, want)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if _, exists := body["auth_token"]; exists {
			t.Error("credential leaked into business request body")
		}
		if body["host"] != "127.0.0.1" {
			t.Errorf("host = %#v, want 127.0.0.1", body["host"])
		}
		_, _ = w.Write([]byte(`{"data":{"connectivity":"ok"}}`))
	}))
	defer server.Close()

	client := NewWithClient(
		server.URL,
		config.AgentCredentialConfig{AuthToken: "agent-secret"},
		newTransport(1),
	)
	result, err := client.CheckProxy(context.Background(), ProxyCheckRequest{Host: "127.0.0.1", Port: 1080})
	if err != nil {
		t.Fatalf("CheckProxy() error = %v", err)
	}
	if result.Connectivity != "ok" {
		t.Fatalf("connectivity = %q, want ok", result.Connectivity)
	}
}

func TestInitializePublishesConfiguredAgentClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer initialize-secret"; got != want {
			t.Errorf("authorization = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"data":{"connectivity":"ok"}}`))
	}))
	defer server.Close()

	closeHTTPClient(t, initializeHTTPClient(t, "agent", 1))
	_ = Close()
	if err := Initialize(clientConfigForTest(server.URL), config.AgentCredentialConfig{AuthToken: "initialize-secret"}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	result, err := Get().CheckProxy(context.Background(), ProxyCheckRequest{Host: "127.0.0.1", Port: 1080})
	if err != nil {
		t.Fatalf("CheckProxy() error = %v", err)
	}
	if result.Connectivity != "ok" {
		t.Fatalf("connectivity = %q, want ok", result.Connectivity)
	}
}

func TestClientDoesNotRetryNonSuccessResponses(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(w, `{"message":"agent unavailable"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewWithClient(server.URL, config.AgentCredentialConfig{AuthToken: "secret"}, newTransport(3))
	_, err := client.CheckProxy(context.Background(), ProxyCheckRequest{Host: "127.0.0.1"})
	if err == nil {
		t.Fatal("CheckProxy() with HTTP 502 succeeded")
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "agent unavailable") {
		t.Fatalf("error = %v, want status and response body", err)
	}
	if attempts != 1 {
		t.Fatalf("HTTP status attempts = %d, want 1", attempts)
	}
}

func clientConfigForTest(rawURL string) config.ClientConfig {
	return config.ClientConfig{Name: "agent", BaseURL: rawURL, Timeout: config.Duration{Duration: 5 * time.Second}}
}

func newTransport(attempts int) *httpclient.Client {
	instance, closer, err := httpclient.New(httpclient.Config{
		Timeout:    httpclient.Duration{5 * time.Second},
		Connection: httpclient.ConnectionConfig{DialTimeout: httpclient.Duration{time.Second}},
		Retry:      httpclient.RetryConfig{Attempts: attempts, Delay: httpclient.Duration{}, Policy: "fixed"},
	})
	if err != nil {
		panic(err)
	}
	_ = closer
	return instance
}

func initializeHTTPClient(t *testing.T, name string, attempts int) func() error {
	t.Helper()
	content := "timeout = \"5s\"\n[connection]\ndial_timeout = \"1s\"\n[retry]\nattempts = " + strconv.Itoa(attempts) + "\npolicy = \"fixed\"\n"
	closer, err := httpclient.Initialize([]pkgconfig.Document{{Name: name, Format: pkgconfig.FormatTOML, Raw: []byte(content)}}, nil)
	if err != nil {
		t.Fatalf("initialize http client: %v", err)
	}
	return closer
}

func closeHTTPClient(t *testing.T, closer func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := closer(); err != nil {
			t.Fatalf("close http client: %v", err)
		}
	})
}
