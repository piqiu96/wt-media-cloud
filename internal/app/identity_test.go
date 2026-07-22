package app

import (
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func TestBootstrapIdentityRequiresCredentialPair(t *testing.T) {
	service := identity.NewService(identity.NewMemoryStore())
	if err := bootstrapIdentity(service, "", ""); err != nil {
		t.Fatalf("empty bootstrap configuration error = %v", err)
	}
	if err := bootstrapIdentity(service, "tech", ""); err == nil {
		t.Fatalf("partial bootstrap configuration error = nil")
	}
}

func TestBootstrapIdentityDoesNotResetExistingAdmin(t *testing.T) {
	service := identity.NewService(identity.NewMemoryStore())
	if err := bootstrapIdentity(service, "tech", "a-long-initial-password"); err != nil {
		t.Fatalf("first bootstrap error = %v", err)
	}
	if err := bootstrapIdentity(service, "replacement", "a-long-replacement-password"); err != nil {
		t.Fatalf("second bootstrap error = %v", err)
	}
	if _, err := service.Login("tech", "a-long-initial-password"); err != nil {
		t.Fatalf("original admin login error = %v", err)
	}
}
