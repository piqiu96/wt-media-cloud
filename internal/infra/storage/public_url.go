package storage

import (
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

// publicBaseOf composes the address prefix every object lives under. The
// bucket, endpoint and prefix are named only here; a stable public address is
// a fact about the same values a write goes through. Path-style, unsigned, no
// expiry — whether the bucket serves it anonymously is a policy question this
// package does not answer.
func publicBaseOf(cfg config.ObjectStorageConfig) string {
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	return scheme + "://" + strings.TrimSpace(cfg.Endpoint) + "/" + strings.TrimSpace(cfg.Bucket) + "/" + normalisePrefix(cfg.Prefix)
}

// PublicURL composes the stable address of one logical object key. The prefix
// is applied here as everywhere else, and a key that could leave it is
// refused. Works with no credential: composing the address touches no network.
func PublicURL(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	resources.RLock()
	base, ready := resources.publicBase, resources.store != nil
	resources.RUnlock()
	if !ready {
		panic("storage resources called before Initialize")
	}
	return base + key, nil
}
