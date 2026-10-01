package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
)

// The address this module publishes stopped being the bucket's stable address
// and became a signed grant, because the bucket refuses anonymous reads: the
// unsigned spelling was answered 403 by the store itself, so a link built from
// it is one Cloud has already proven a browser cannot open. What the route
// serves is only ever an `ObjectLinker` answer, so the adapter is the one place
// that decision lives, and the failure mode of getting it wrong is silent —
// every other test in this package answers the linker from a stub, and the
// contract body is a single `url` string either way.
//
// The assertion is on the signature, not on the shape of the call: reverting the
// adapter to the stable unsigned address produces a URL that addresses the right
// object and turns this red anyway.
func TestTheObjectStoreAdapterSignsTheAddressItPublishes(t *testing.T) {
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

	const key = "materials/42/x.mp4"
	url, err := productionObjectLinker{}.PresignObjectURL(context.Background(), key)
	if err != nil {
		t.Fatalf("PresignObjectURL() error = %v", err)
	}
	if !strings.Contains(url, "/wt-media/dev/"+key) {
		t.Fatalf("the published address does not address the object: %s", url)
	}
	if !strings.Contains(url, "X-Amz-Signature=") {
		t.Fatal("the published address carries no signature, so the bucket will answer it 403")
	}
	if !strings.Contains(url, "X-Amz-Expires=900") {
		t.Fatalf("the published address does not carry the configured fifteen minutes: %s", url)
	}
	if strings.Contains(url, "test-secret-key") {
		t.Fatal("the published address contains the secret key")
	}
}
