package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
)

var ErrObjectConflict = errors.New("an immutable object already contains different bytes")

// ContentClaims permanently binds a storage location to one expected payload.
// Same-content retries remain valid even if an earlier upload was interrupted.
type ContentClaims interface {
	Claim(context.Context, string, string, int64) (bool, error)
}

// ImmutableStore is the capability required by formal uploads and migration.
// Put remains the replacement operation used for the slice library index.
type ImmutableStore interface {
	Store
	Reference(string) (string, error)
	Open(context.Context, string) (io.ReadCloser, error)
	PutImmutable(context.Context, string, io.Reader, int64, string, string) error
}

func (s *minioStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	bucket, object, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return s.client.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
}

func (s *minioStore) matches(ctx context.Context, key string, size int64, digest string) error {
	info, err := s.Stat(ctx, key)
	if err != nil {
		return err
	}
	if info.Size != size {
		return ErrObjectConflict
	}
	body, err := s.Open(ctx, key)
	if err != nil {
		return err
	}
	defer body.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, body)
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(hash.Sum(nil)) != digest {
		return ErrObjectConflict
	}
	return nil
}

func (s *minioStore) PutImmutable(ctx context.Context, key string, body io.Reader, size int64, contentType, digest string) error {
	if err := validateDigest(digest); err != nil {
		return err
	}
	if size <= 0 {
		return errors.New("immutable object must not be empty")
	}
	if s.claims == nil {
		return errors.New("immutable uploads require shared content claims")
	}
	digest = strings.ToLower(strings.TrimSpace(digest))
	bucket, object, err := s.resolve(key)
	if err != nil {
		return err
	}
	// Durable S3 locations identify the configured storage by bucket, not by
	// an endpoint alias; HTTP/HTTPS or hostname changes must not bypass claims.
	identity := DigestOf([]byte(bucket + "\n" + object))
	claimed, err := s.claims.Claim(ctx, identity, digest, size)
	if err != nil {
		return fmt.Errorf("claim immutable object: %w", err)
	}
	if !claimed {
		return ErrObjectConflict
	}
	err = s.matches(ctx, key, size, digest)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrObjectNotFound) {
		return err
	}
	// Shared claims reject different payloads before touching the bucket. Some
	// S3-compatible backends ignore conditional PUT, so the header is additional
	// protection only. Concurrent writers admitted here have identical bytes.
	options := minio.PutObjectOptions{ContentType: contentType, SendContentMd5: true, DisableMultipart: true, UserMetadata: map[string]string{"sha256": digest}}
	options.SetMatchETagExcept("*")
	_, err = s.client.PutObject(ctx, bucket, object, body, size, options)
	if err != nil {
		response := minio.ToErrorResponse(err)
		if response.StatusCode == 412 || response.Code == "PreconditionFailed" {
			return s.matches(ctx, key, size, digest)
		}
		return fmt.Errorf("immutable upload: %w", err)
	}
	// Read-back also catches a backend that accepted a checksum without verifying
	// it. A metadata value supplied by the writer is not an integrity proof.
	return s.matches(ctx, key, size, digest)
}

func PutImmutable(ctx context.Context, store Store, key string, body io.Reader, size int64, contentType, digest string) error {
	writer, ok := store.(interface {
		PutImmutable(context.Context, string, io.Reader, int64, string, string) error
	})
	if !ok {
		return errors.New("store does not support immutable uploads")
	}
	return writer.PutImmutable(ctx, key, body, size, contentType, digest)
}
