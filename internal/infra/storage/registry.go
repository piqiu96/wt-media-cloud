package storage

import (
	"errors"
	"strings"
	"sync"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

var errAlreadyInitialized = errors.New("object storage already initialized")

var resources struct {
	sync.RWMutex
	store Store
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
		return nil
	}
	store, err := newMinioStore(cfg, credential)
	if err != nil {
		return err
	}
	resources.store = store
	return nil
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
	resources.Unlock()
	return nil
}
