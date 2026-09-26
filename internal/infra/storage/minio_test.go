package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func testStorageConfig(endpoint string) config.ObjectStorageConfig {
	return config.ObjectStorageConfig{
		Endpoint:   endpoint,
		Bucket:     "wt-media",
		Region:     "us-east-1",
		Prefix:     "dev/",
		UseSSL:     false,
		PresignTTL: config.Duration{Duration: 15 * time.Minute},
	}
}

func testCredential() config.ObjectStorageCredentialConfig {
	return config.ObjectStorageCredentialConfig{AccessKey: "test-access-key", SecretKey: "test-secret-key"}
}

// A request counter, so that "presigning does not touch the network" is asserted
// as an absence of requests rather than as an absence of errors.
type countingServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

func (s *countingServer) note(path string) {
	s.mu.Lock()
	s.requests = append(s.requests, path)
	s.mu.Unlock()
}

func (s *countingServer) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func newCountingServer(t *testing.T) *countingServer {
	t.Helper()
	server := &countingServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.note(r.URL.Path)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint>us-east-1</LocationConstraint>`))
	}))
	t.Cleanup(server.Close)
	return server
}

// Presigning is local signing, so the endpoint must never be contacted for it.
// This is the property the executor depends on: it asks Cloud for a URL and uses
// it against the bucket, and Cloud must be able to mint that URL while its own
// outbound path to the bucket is the thing that is slow.
func TestPresignGetDoesNotTouchTheNetwork(t *testing.T) {
	server := newCountingServer(t)
	store, err := newMinioStore(testStorageConfig(server.Listener.Addr().String()), testCredential())
	if err != nil {
		t.Fatalf("newMinioStore() error = %v", err)
	}
	fixed := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return fixed }

	grant, err := store.PresignGet(context.Background(), "materials/42/"+testDigest+".mp4", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
	if paths := server.paths(); len(paths) != 0 {
		t.Fatalf("presigning made %d request(s): %v", len(paths), paths)
	}
	if want := fixed.Add(15 * time.Minute); !grant.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %s, want %s", grant.ExpiresAt, want)
	}
	if grant.URL == "" {
		t.Fatal("PresignGet returned an empty URL")
	}
}

// An empty region is the configuration this repository ships, and minio-go
// answers it by asking the endpoint where the bucket lives before it can sign.
// That is a live request in the middle of an otherwise local operation, so it is
// pinned here rather than left as a surprise: the arm with a region set proves
// the configured value is passed through, and the arm without one proves what
// depends on it.
func TestPresignGetAsksTheEndpointForTheRegionOnlyWhenNoneIsConfigured(t *testing.T) {
	for _, testCase := range []struct {
		region       string
		wantRequests int
	}{
		{"us-east-1", 0},
		{"", 1},
	} {
		server := newCountingServer(t)
		cfg := testStorageConfig(server.Listener.Addr().String())
		cfg.Region = testCase.region
		store, err := newMinioStore(cfg, testCredential())
		if err != nil {
			t.Fatalf("region %q: newMinioStore() error = %v", testCase.region, err)
		}

		if _, err := store.PresignGet(context.Background(), "materials/42/x.mp4", time.Minute); err != nil {
			t.Fatalf("region %q: PresignGet() error = %v", testCase.region, err)
		}
		if got := len(server.paths()); got != testCase.wantRequests {
			t.Fatalf("region %q: presigning made %d request(s), want %d", testCase.region, got, testCase.wantRequests)
		}
	}
}

// The signed URL must carry the configured prefix, or a grant would point at an
// object in the release namespace while the acceptance run wrote to the dev one.
func TestPresignGetSignsThePrefixedObjectAndNeverTheCredential(t *testing.T) {
	server := newCountingServer(t)
	store, err := newMinioStore(testStorageConfig(server.Listener.Addr().String()), testCredential())
	if err != nil {
		t.Fatalf("newMinioStore() error = %v", err)
	}

	grant, err := store.PresignGet(context.Background(), "materials/42/source.mp4", time.Minute)
	if err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
	if !strings.Contains(grant.URL, "/wt-media/dev/materials/42/source.mp4") {
		t.Fatalf("presigned URL does not address the prefixed object: %s", grant.URL)
	}
	if strings.Contains(grant.URL, "test-secret-key") {
		t.Fatal("the signed URL contains the secret key")
	}
	if !strings.Contains(grant.URL, "X-Amz-Expires=60") {
		t.Fatalf("presigned URL does not carry the requested lifetime: %s", grant.URL)
	}
}

// A key that could leave the prefix is refused before any client call, so the
// guard cannot be defeated by a bucket policy that happens to allow it.
func TestEveryMethodRefusesAKeyThatCouldLeaveThePrefix(t *testing.T) {
	server := newCountingServer(t)
	store, err := newMinioStore(testStorageConfig(server.Listener.Addr().String()), testCredential())
	if err != nil {
		t.Fatalf("newMinioStore() error = %v", err)
	}
	ctx := context.Background()
	calls := map[string]func() error{
		"PresignGet": func() error { _, err := store.PresignGet(ctx, "../escape", time.Minute); return err },
		"Stat":       func() error { _, err := store.Stat(ctx, "../escape"); return err },
		"Put":        func() error { return store.Put(ctx, "../escape", strings.NewReader("x"), 1, "video/mp4") },
		"Copy":       func() error { return store.Copy(ctx, "../escape", "materials/1/x.mp4") },
		"Remove":     func() error { return store.Remove(ctx, "../escape") },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s accepted a key that leaves the prefix", name)
		}
	}
	if paths := server.paths(); len(paths) != 0 {
		t.Fatalf("a rejected key still reached the endpoint: %v", paths)
	}
}

func TestPutRefusesAnUnknownSize(t *testing.T) {
	server := newCountingServer(t)
	store, err := newMinioStore(testStorageConfig(server.Listener.Addr().String()), testCredential())
	if err != nil {
		t.Fatalf("newMinioStore() error = %v", err)
	}
	if err := store.Put(context.Background(), "materials/42/x.mp4", strings.NewReader("x"), -1, "video/mp4"); err == nil {
		t.Fatal("Put accepted an unknown size")
	}
	if paths := server.paths(); len(paths) != 0 {
		t.Fatalf("the refusal still reached the endpoint: %v", paths)
	}
}

func TestPresignGetRefusesANonPositiveLifetime(t *testing.T) {
	server := newCountingServer(t)
	store, err := newMinioStore(testStorageConfig(server.Listener.Addr().String()), testCredential())
	if err != nil {
		t.Fatalf("newMinioStore() error = %v", err)
	}
	for _, ttl := range []time.Duration{0, -time.Minute} {
		if _, err := store.PresignGet(context.Background(), "materials/42/x.mp4", ttl); err == nil {
			t.Errorf("PresignGet accepted a lifetime of %s", ttl)
		}
	}
}

// The registry is what makes an absent secret a startable state, so both of its
// branches are asserted: no credential publishes the refusing store, and a whole
// pair publishes one that can sign.
func TestTheRegistryPublishesARefusingStoreWithoutACredential(t *testing.T) {
	t.Cleanup(func() { _ = Close() })
	_ = Close()
	if err := Initialize(testStorageConfig("127.0.0.1:9000"), config.ObjectStorageCredentialConfig{}); err != nil {
		t.Fatalf("Initialize() error = %v, want an unconfigured store to be a valid state", err)
	}
	if Configured() {
		t.Fatal("Configured() = true without a credential")
	}
	if _, err := Get().PresignGet(context.Background(), "materials/42/x.mp4", time.Minute); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("PresignGet() error = %v, want ErrNotConfigured", err)
	}
}

func TestTheRegistryPublishesASigningStoreWithAWholeCredentialPair(t *testing.T) {
	t.Cleanup(func() { _ = Close() })
	_ = Close()
	if err := Initialize(testStorageConfig("127.0.0.1:9000"), testCredential()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if !Configured() {
		t.Fatal("Configured() = false with a credential pair")
	}
	if _, err := Get().PresignGet(context.Background(), "materials/42/x.mp4", time.Minute); err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
}

// A half pair cannot reach the store from configuration — `config.Validate`
// refuses it first — so this asserts the second line of defence, the one that
// holds if the store is ever built from something other than a loaded config.
func TestNewMinioStoreRefusesAHalfCredentialPair(t *testing.T) {
	for _, credential := range []config.ObjectStorageCredentialConfig{
		{AccessKey: "only-access"},
		{SecretKey: "only-secret"},
	} {
		if _, err := newMinioStore(testStorageConfig("127.0.0.1:9000"), credential); err == nil {
			t.Fatalf("newMinioStore accepted %+v", credential)
		}
	}
}

func TestInitializeRefusesASecondCall(t *testing.T) {
	t.Cleanup(func() { _ = Close() })
	_ = Close()
	if err := Initialize(testStorageConfig("127.0.0.1:9000"), testCredential()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := Initialize(testStorageConfig("127.0.0.1:9000"), testCredential()); !errors.Is(err, errAlreadyInitialized) {
		t.Fatalf("second Initialize() error = %v, want errAlreadyInitialized", err)
	}
}

// The package-level `PresignGet` exists so that no caller can choose how long an
// address stays valid, which means the configured value has to be the one that
// reaches the signer. A lifetime of zero would not be caught anywhere else: it
// makes the call fail rather than mint a grant with the wrong expiry, so a
// forgotten assignment shows up as a claim that cannot be served at all.
func TestPackagePresignGetUsesTheConfiguredLifetime(t *testing.T) {
	t.Cleanup(func() { _ = Close() })
	_ = Close()
	cfg := testStorageConfig("127.0.0.1:9000")
	cfg.PresignTTL = config.Duration{Duration: 7 * time.Minute}
	if err := Initialize(cfg, testCredential()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	before := time.Now()
	grant, err := PresignGet(context.Background(), "materials/42/x.mp4")
	if err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
	if !strings.Contains(grant.URL, "X-Amz-Expires=420") {
		t.Fatalf("the signed lifetime is not the configured seven minutes: %s", grant.URL)
	}
	if grant.ExpiresAt.Before(before.Add(7*time.Minute)) || grant.ExpiresAt.After(time.Now().Add(7*time.Minute)) {
		t.Fatalf("ExpiresAt = %s, want about seven minutes from now", grant.ExpiresAt)
	}
}
