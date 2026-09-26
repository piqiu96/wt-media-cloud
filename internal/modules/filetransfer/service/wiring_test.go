package service

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
)

// `newWiredService` cannot be exercised by a test: it reaches MySQL and the node
// table. What can be asserted is that it wires the object-storage issuer, because
// the failure mode is silent and expensive — a wired service that kept the
// refusing default answers every claim with an internal error on a deployment
// that has a bucket, and the error surfaces at the executor, where it reads as a
// Cloud-side fault with no cause. The service tests all build their own Service,
// so nothing else would notice.
//
// The assertion is on the source, in the shape `production` already uses for its
// otherwise-unreachable route helper: a substring that only the wired spelling
// produces.
func TestTheWiredServiceMintsGrantsFromTheObjectStore(t *testing.T) {
	source, err := os.ReadFile("operations.go")
	if err != nil {
		t.Fatalf("reading operations.go: %v", err)
	}
	if !strings.Contains(string(source), "WithGrantIssuer(objectStorageGrants{})") {
		t.Fatal("newWiredService no longer wires the object-storage grant issuer: every claim would fail with an internal error")
	}
}

// The adapter's job is the pair, not the URL. A grant that carries an address but
// no expiry would be accepted by the lease check — it only asks for a non-empty
// URL — and the executor would then have no way to know when its address dies,
// which is exactly the field the frozen `LocalLease` requires it to report.
func TestTheObjectStoreAdapterCarriesBothHalvesOfTheGrant(t *testing.T) {
	t.Cleanup(func() { _ = storage.Close() })
	_ = storage.Close()
	cfg := config.ObjectStorageConfig{
		Endpoint: "127.0.0.1:9000",
		Bucket:   "wt-media",
		// A region is set so that presigning stays local: minio-go asks the
		// endpoint where the bucket lives when this is empty, and there is no
		// endpoint here.
		Region:     "us-east-1",
		Prefix:     "dev/",
		PresignTTL: config.Duration{Duration: 15 * time.Minute},
	}
	credential := config.ObjectStorageCredentialConfig{AccessKey: "test-access-key", SecretKey: "test-secret-key"}
	if err := storage.Initialize(cfg, credential); err != nil {
		t.Fatalf("storage.Initialize() error = %v", err)
	}

	key := "materials/42/" + strings.Repeat("e3b0", 16) + ".mp4"
	before := time.Now()
	grant, err := objectStorageGrants{}.PresignGet(context.Background(), key)
	if err != nil {
		t.Fatalf("PresignGet() error = %v", err)
	}
	if !strings.Contains(grant.URL, "/wt-media/dev/"+key) {
		t.Fatalf("the grant does not address the object: %s", grant.URL)
	}
	if !grant.ExpiresAt.After(before) || grant.ExpiresAt.After(time.Now().Add(16*time.Minute)) {
		t.Fatalf("ExpiresAt = %s, want the configured fifteen minutes from now", grant.ExpiresAt)
	}
}
