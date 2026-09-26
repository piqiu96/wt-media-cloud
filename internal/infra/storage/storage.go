// Package storage is the object-storage boundary: the only place in Cloud that
// holds a credential able to read or write a prepared source object.
//
// It exists as a narrow interface rather than as calls to a client library for
// two reasons that are both about what callers must not be able to do. Nothing
// above it can name a bucket or a durable credential — keys are logical, and the
// configured prefix is applied here — and a caller that needs to hand an address
// to an executor gets a `Grant`, which is a signed GET for one object that
// expires, rather than anything reusable.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ErrNotConfigured is returned by every method of the store when no credential
// is configured. It is a normal startup state, not a failure: the repository
// ships a tree without the key pair, so the process starts, its tests run, and
// only the calls that need object storage fail — with this error, which says why.
var ErrNotConfigured = errors.New("object storage is not configured")

// ObjectInfo describes an object Cloud has already written.
type ObjectInfo struct {
	// Key is the logical key the caller asked about, without the configured
	// prefix. Returning it means a caller never has to reconstruct it, and so
	// never has to know the prefix exists.
	Key         string
	Size        int64
	ETag        string
	ContentType string
}

// Grant is a short-lived, single-object GET grant.
//
// It is deliberately not a bare string: the expiry is part of what the caller
// must publish, and a caller that had to derive it from the TTL it passed in
// would be keeping a second copy of a decision made here.
type Grant struct {
	URL       string
	ExpiresAt time.Time
}

// Store is the whole of Cloud's object-storage surface.
type Store interface {
	// PresignGet mints a GET grant for one object. It is computed locally from
	// the credential and does not touch the network when the configuration names
	// a region; see the README note on the region.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (Grant, error)
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	// Put writes an object of exactly size bytes. The size is required rather
	// than discovered: it is known before the bytes are sent, it is part of what
	// the caller will later verify, and streaming an unknown length to a
	// content-addressed key would make retries ambiguous.
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Copy(ctx context.Context, sourceKey, destinationKey string) error
	Remove(ctx context.Context, key string) error
}

// SourceKey is the key a verified prepared source lives at, addressed by the
// sha256 of its own bytes.
//
// Content addressing is what makes a re-preparation of the same source
// idempotent: the second run writes the same key with the same bytes, so it
// neither duplicates an object nor invalidates a key an earlier task recorded.
func SourceKey(materialID int64, digest, extension string) (string, error) {
	if materialID <= 0 {
		return "", fmt.Errorf("source key needs a positive material id, got %d", materialID)
	}
	if err := validateDigest(digest); err != nil {
		return "", err
	}
	extension, err := validateExtension(extension)
	if err != nil {
		return "", err
	}
	return "materials/" + strconv.FormatInt(materialID, 10) + "/" + strings.ToLower(digest) + "." + extension, nil
}

// StagingKey is where an in-flight download writes, one per attempt.
//
// It is per task rather than per material so that two workers cannot interleave
// their bytes into one object, and it is inside the material's own prefix so that
// a staged file can be copied to its final key without crossing a bucket
// boundary or a permission.
func StagingKey(materialID int64, taskID, extension string) (string, error) {
	if materialID <= 0 {
		return "", fmt.Errorf("staging key needs a positive material id, got %d", materialID)
	}
	taskID = strings.TrimSpace(taskID)
	if err := validateTaskID(taskID); err != nil {
		return "", err
	}
	extension, err := validateExtension(extension)
	if err != nil {
		return "", err
	}
	return "materials/" + strconv.FormatInt(materialID, 10) + "/.staging/" + taskID + "." + extension, nil
}

// DigestOf is the sha256 a caller must have computed over the object's bytes
// before it can name a source key.
func DigestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func validateDigest(digest string) error {
	digest = strings.ToLower(strings.TrimSpace(digest))
	if len(digest) != 64 {
		return fmt.Errorf("source key needs a 64-character sha256 digest, got %d characters", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("source key digest is not hexadecimal: %w", err)
	}
	return nil
}

// validateExtension keeps the extension to what a media container can be named.
// It is not cosmetic: the value reaches a key, and a "." or a "/" in it would
// move the object to a different prefix than the one the caller asked for.
//
// Case and a leading dot are normalised rather than refused, because they are
// spellings of one extension and a caller resolving them from a URL path is not
// making a mistake. Whitespace is refused, but not by a rule of its own: a space
// is not a letter or a digit, so the character check below already rejects it.
// A separate guard for it was written and then deleted, because removing that
// guard changed no observable behaviour — no test could tell it apart from the
// check underneath it, and a rule nothing can falsify is not defence.
func validateExtension(extension string) (string, error) {
	extension = strings.ToLower(strings.TrimPrefix(extension, "."))
	if extension == "" || len(extension) > 8 {
		return "", fmt.Errorf("media extension must be 1 to 8 characters, got %q", extension)
	}
	for _, character := range extension {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return "", fmt.Errorf("media extension %q may only contain letters and digits", extension)
		}
	}
	return extension, nil
}

func validateTaskID(taskID string) error {
	if taskID == "" || len(taskID) > 64 {
		return fmt.Errorf("staging key needs a task id of 1 to 64 characters, got %q", taskID)
	}
	if strings.Contains(taskID, "..") {
		return fmt.Errorf("task id %q may not contain a relative path segment", taskID)
	}
	for _, character := range taskID {
		valid := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.'
		if !valid {
			return fmt.Errorf("task id %q may only contain letters, digits, dot, dash and underscore", taskID)
		}
	}
	return nil
}

// validateKey refuses a logical key that could leave the configured prefix.
//
// Every key this package hands to a client is built by `SourceKey` or
// `StagingKey` and already satisfies this, so the check is a guard on the seam
// rather than on the callers — which is exactly where it belongs, because the
// interface is exported and the next caller need not use those builders.
func validateKey(key string) error {
	if key == "" {
		return errors.New("object key is empty")
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "//") {
		return fmt.Errorf("object key %q must be relative and may not contain an empty segment", key)
	}
	if strings.Contains(key, "..") || strings.Contains(key, `\`) {
		return fmt.Errorf("object key %q must not contain a relative path segment or a backslash", key)
	}
	return nil
}

// notConfiguredStore answers every call with ErrNotConfigured.
type notConfiguredStore struct{}

func (notConfiguredStore) PresignGet(context.Context, string, time.Duration) (Grant, error) {
	return Grant{}, ErrNotConfigured
}

func (notConfiguredStore) Stat(context.Context, string) (ObjectInfo, error) {
	return ObjectInfo{}, ErrNotConfigured
}

func (notConfiguredStore) Put(context.Context, string, io.Reader, int64, string) error {
	return ErrNotConfigured
}

func (notConfiguredStore) Copy(context.Context, string, string) error { return ErrNotConfigured }

func (notConfiguredStore) Remove(context.Context, string) error { return ErrNotConfigured }
