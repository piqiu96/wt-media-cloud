package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
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

	client := NewWithHTTPClient(
		clientConfigForTest(server.URL),
		config.AgentCredentialConfig{AuthToken: "agent-secret"},
		server.Client(),
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

func TestClientRetriesOnlyConfiguredTransportFailures(t *testing.T) {
	attempts := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"connectivity":"ok"}}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})
	connection := clientConfigForTest("http://agent.example")
	connection.Retry = config.RetryConfig{Attempts: 3, Interval: config.Duration{}}
	client := NewWithHTTPClient(connection, config.AgentCredentialConfig{AuthToken: "secret"}, &http.Client{Transport: transport})

	if _, err := client.CheckProxy(context.Background(), ProxyCheckRequest{Host: "127.0.0.1"}); err != nil {
		t.Fatalf("CheckProxy() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("transport attempts = %d, want 3", attempts)
	}

	attempts = 0
	nonSuccess := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(`{"message":"agent unavailable"}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})
	client = NewWithHTTPClient(connection, config.AgentCredentialConfig{AuthToken: "secret"}, &http.Client{Transport: nonSuccess})
	if _, err := client.CheckProxy(context.Background(), ProxyCheckRequest{Host: "127.0.0.1"}); err == nil {
		t.Fatal("CheckProxy() with HTTP 502 succeeded")
	}
	if attempts != 1 {
		t.Fatalf("HTTP status attempts = %d, want 1", attempts)
	}
}

func clientConfigForTest(rawURL string) config.ClientConfig {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return config.ClientConfig{
		Name:    "agent",
		Scheme:  parsed.Scheme,
		Host:    parsed.Hostname(),
		Port:    portForTest(parsed),
		Timeout: config.Duration{Duration: 5 * time.Second},
		Retry:   config.RetryConfig{Attempts: 1, Interval: config.Duration{}},
	}
}

func portForTest(parsed *url.URL) int {
	if parsed.Port() == "" {
		if parsed.Scheme == "https" {
			return 443
		}
		return 80
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		panic(err)
	}
	return port
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
