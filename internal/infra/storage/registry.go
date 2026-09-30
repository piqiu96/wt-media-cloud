package storage

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

var errAlreadyInitialized = errors.New("object storage already initialized")

var resources struct {
	sync.RWMutex
	store Store
	// grantTTL is the configured lifetime of a minted grant. It is kept here
	// rather than passed by each caller so that how long an address stays valid
	// is one operational decision, not one per call site.
	grantTTL time.Duration
	// publicBase is the address prefix `PublicURL` composes from, published at
	// Initialize in both the configured and the unconfigured branch: the stable
	// address is not a credential and must not depend on one being present.
	publicBase string
}

// Initialize publishes the process-wide object-storage store.
//
// A missing credential is not an error here. The store is published anyway, as
// one that refuses every call with `ErrNotConfigured`: the alternative is a
// process that will not start without a secret, which would make the release
// tree and this repository's own tests unable to run at all. What must not happen
// is a call that silently does nothing, and this store cannot do that.
func Initialize(cfg config.ObjectStorageConfig, credential config.ObjectStorageCredentialConfig) error {
	resources.Lock()
	defer resources.Unlock()
	if resources.store != nil {
		return errAlreadyInitialized
	}
	if strings.TrimSpace(credential.AccessKey) == "" && strings.TrimSpace(credential.SecretKey) == "" {
		resources.store = notConfiguredStore{}
		resources.grantTTL = cfg.PresignTTL.Duration
		resources.publicBase = publicBaseOf(cfg)
		return nil
	}
	store, err := newMinioStore(cfg, credential)
	if err != nil {
		return err
	}
	resources.store = store
	resources.grantTTL = cfg.PresignTTL.Duration
	resources.publicBase = publicBaseOf(cfg)
	return nil
}

// PresignGet mints a grant with the configured lifetime.
//
// The lifetime is not a parameter here, unlike on the interface. A caller that
// chose one would be writing a second copy of a decision configuration already
// makes, and the two would drift with nothing to catch it.
func PresignGet(ctx context.Context, key string) (Grant, error) {
	resources.RLock()
	store, ttl := resources.store, resources.grantTTL
	resources.RUnlock()
	if store == nil {
		panic("storage resources called before Initialize")
	}
	return store.PresignGet(ctx, key, ttl)
}

// Get returns the initialized store and fails fast before Bootstrap, in the same
// shape as the HTTP client registry: a caller that reaches this before the
// resource exists has a wiring bug, not a runtime condition.
func Get() Store {
	resources.RLock()
	defer resources.RUnlock()
	if resources.store == nil {
		panic("storage resources called before Initialize")
	}
	return resources.store
}

// Configured reports whether a credential was published, so that a caller which
// can do something useful without one — a worker that has no work to do, a
// health endpoint — can find out without provoking an error.
func Configured() bool {
	resources.RLock()
	defer resources.RUnlock()
	_, notConfigured := resources.store.(notConfiguredStore)
	return resources.store != nil && !notConfigured
}

// Close releases the store and is safe to call repeatedly.
func Close() error {
	resources.Lock()
	resources.store = nil
	resources.grantTTL = 0
	resources.publicBase = ""
	resources.Unlock()
	return nil
}
