package httpclient_test

import (
	"testing"
	"time"

	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

func TestInitializePublishesConfiguredClientsAndCloseRevokesThem(t *testing.T) {
	closer, err := httpclient.Initialize([]httpclient.Config{
		clientConfig("agent", "1s", 1),
		clientConfig("douyin", "2s", 1),
	}, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if httpclient.Get("agent") == nil || httpclient.Get("douyin") == nil {
		t.Fatal("Initialize did not publish both clients")
	}
	if httpclient.Get("agent") == httpclient.Get("douyin") {
		t.Fatal("Initialize published the same client twice")
	}
	if err := closer(); err != nil {
		t.Fatalf("closer() error = %v", err)
	}
	assertGetPanics(t, "agent")
}

func TestInitializeRejectsDuplicateAndInvalidConfigs(t *testing.T) {
	invalid := clientConfig("invalid", "1s", 0)
	tests := [][]httpclient.Config{
		{clientConfig("same", "1s", 1), clientConfig("same", "1s", 1)},
		{invalid},
	}
	for index, configs := range tests {
		closer, err := httpclient.Initialize(configs, nil)
		if err == nil {
			t.Fatalf("tests[%d] Initialize() error = nil", index)
		}
		if closer != nil {
			_ = closer()
		}
		assertGetPanics(t, "test")
	}
}

func TestInitializeRollsBackEarlierClientsOnLaterFailure(t *testing.T) {
	closer, err := httpclient.Initialize([]httpclient.Config{
		clientConfig("valid", "1s", 1),
		clientConfig("invalid", "1s", 0),
	}, nil)
	if err == nil || closer != nil {
		t.Fatalf("Initialize() closer=%t error=%v", closer != nil, err)
	}
	assertGetPanics(t, "valid")
}

func clientConfig(name, timeout string, attempts int) httpclient.Config {
	return httpclient.Config{
		Name:       name,
		Timeout:    httpclient.Duration{Duration: parseDurationForTest(timeout)},
		Endpoint:   testEndpoint(),
		Connection: httpclient.ConnectionConfig{DialTimeout: httpclient.Duration{Duration: 100000000}},
		Retry:      httpclient.RetryConfig{Attempts: attempts},
	}
}

func testEndpoint() httpclient.EndpointConfig {
	return httpclient.EndpointConfig{Scheme: "http", Host: "127.0.0.1", Port: 8188}
}

func parseDurationForTest(value string) time.Duration {
	switch value {
	case "1s":
		return time.Second
	case "2s":
		return 2 * time.Second
	default:
		return 0
	}
}

func assertGetPanics(t *testing.T, name string) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("Get(%q) did not panic after close or failed initialization", name)
		}
	}()
	_ = httpclient.Get(name)
}
