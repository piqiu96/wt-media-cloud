package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
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

	closeHTTPClient(t, initializeHTTPClient(t, "agent", 1, server.URL))
	_ = Close()
	if _, err := Initialize("agent", config.CredentialsConfig{Agent: config.AgentCredentialConfig{AuthToken: "initialize-secret"}}); err != nil {
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

func newTransport(attempts int) *httpclient.Client {
	instance, closer, err := httpclient.New(httpclient.Config{
		Name:       "agent-test",
		Endpoint:   endpointForTest("http://127.0.0.1:8188"),
		Timeout:    httpclient.Duration{Duration: 5 * time.Second},
		Connection: httpclient.ConnectionConfig{DialTimeout: httpclient.Duration{Duration: time.Second}},
		Retry:      httpclient.RetryConfig{Attempts: attempts, Delay: httpclient.Duration{Duration: 0}, Policy: "fixed"},
	})
	if err != nil {
		panic(err)
	}
	_ = closer
	return instance
}

func initializeHTTPClient(t *testing.T, name string, attempts int, rawURL string) func() error {
	t.Helper()
	closer, err := httpclient.Initialize([]httpclient.Config{{
		Name:       name,
		Timeout:    httpclient.Duration{Duration: 5 * time.Second},
		Endpoint:   endpointForTest(rawURL),
		Connection: httpclient.ConnectionConfig{DialTimeout: httpclient.Duration{Duration: time.Second}},
		Retry:      httpclient.RetryConfig{Attempts: attempts, Delay: httpclient.Duration{Duration: 0}, Policy: "fixed"},
	}}, nil)
	if err != nil {
		t.Fatalf("initialize http client: %v", err)
	}
	return closer
}

func endpointForTest(rawURL string) httpclient.EndpointConfig {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	host := parsed.Hostname()
	port := 80
	if parsed.Port() != "" {
		port, _ = strconv.Atoi(parsed.Port())
	} else if parsed.Scheme == "https" {
		port = 443
	}
	return httpclient.EndpointConfig{Scheme: parsed.Scheme, Host: host, Port: port}
}
func closeHTTPClient(t *testing.T, closer func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := closer(); err != nil {
			t.Fatalf("close http client: %v", err)
		}
	})
}

func TestApplyProxyNeverRetriesWhenResponseIsLost(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path != "/api/v1/proxy-assignment" {
			t.Errorf("path = %s", r.URL.Path)
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}))
	defer server.Close()
	client := NewWithClient(server.URL, config.AgentCredentialConfig{}, newTransport(3))
	_, err := client.ApplyProxy(context.Background(), ProxyMutationRequest{ProfileID: "p1", ProxyProtocol: "socks5", Host: "127.0.0.1", Port: 1080})
	if err == nil {
		t.Fatal("response loss must be uncertain")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestApplyProxyAcceptsAcknowledgementWithoutReadback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/proxy-assignment" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"profile_id":"p1","accepted":true}}`))
	}))
	defer server.Close()
	client := NewWithClient(server.URL, config.AgentCredentialConfig{}, newTransport(2))
	result, err := client.ApplyProxy(context.Background(), ProxyMutationRequest{ProfileID: "p1", ProxyProtocol: "socks5", Host: "127.0.0.1", Port: 1080})
	if err != nil || !result.Accepted || result.ProfileID != "p1" {
		t.Fatalf("ack = %+v, err = %v", result, err)
	}
}
