package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wt-media/wt-media-cloud/internal/config"
)

// minioStore is the S3-compatible implementation. It satisfies the interface for
// MinIO, Aliyun OSS and anything else that speaks the same protocol, because the
// difference between them is configuration rather than behaviour.
type minioStore struct {
	client *minio.Client
	bucket string
	// prefix is already normalised to either "" or a value with a trailing
	// slash, so that joining it to a key is concatenation and cannot produce an
	// empty segment.
	prefix string
	// now is injectable so that a grant's expiry is asserted against a fixed
	// clock rather than against whatever the test's own sleep produced.
	now func() time.Time
}

func newMinioStore(cfg config.ObjectStorageConfig, credential config.ObjectStorageCredentialConfig) (*minioStore, error) {
	if err := validateSettings(cfg, credential); err != nil {
		return nil, err
	}
	client, err := minio.New(strings.TrimSpace(cfg.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(credential.AccessKey), strings.TrimSpace(credential.SecretKey), ""),
		Secure: cfg.UseSSL,
		// The region is passed through and never left for the client to look up.
		// minio-go fetches a bucket's location over the network when the region is
		// empty, which would make minting a grant — an operation that is otherwise
		// pure local signing — depend on a live request, and would fail in exactly
		// the situation the grant exists for: an executor asking for a URL it can
		// use when it can reach the bucket but Cloud's own control plane is slow.
		Region: strings.TrimSpace(cfg.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("object storage client: %w", err)
	}
	return &minioStore{
		client: client,
		bucket: strings.TrimSpace(cfg.Bucket),
		prefix: normalisePrefix(cfg.Prefix),
		now:    time.Now,
	}, nil
}

// validateSettings refuses a configuration that would fail later and less
// legibly. The credentials being absent is the one case that is not an error —
// it is the state every tree in this repository is committed in — so it is
// handled by the caller, which returns a store that says so.
func validateSettings(cfg config.ObjectStorageConfig, credential config.ObjectStorageCredentialConfig) error {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return errors.New("object storage endpoint is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return errors.New("object storage bucket is required")
	}
	if strings.TrimSpace(credential.AccessKey) == "" || strings.TrimSpace(credential.SecretKey) == "" {
		// Reached only when one of the two is set: config.Validate rejects a half
		// pair, and absence is turned into `notConfiguredStore` before this call.
		return ErrNotConfigured
	}
	return nil
}

func normalisePrefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

// object maps a logical key onto the bucket's namespace.
func (s *minioStore) object(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	return s.prefix + key, nil
}

func (s *minioStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (Grant, error) {
	object, err := s.object(key)
	if err != nil {
		return Grant{}, err
	}
	if ttl <= 0 {
		return Grant{}, fmt.Errorf("grant lifetime must be positive, got %s", ttl)
	}
	signed, err := s.client.PresignedGetObject(ctx, s.bucket, object, ttl, url.Values{})
	if err != nil {
		return Grant{}, fmt.Errorf("presign %s: %w", key, err)
	}
	return Grant{URL: signed.String(), ExpiresAt: s.now().Add(ttl)}, nil
}

func (s *minioStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	object, err := s.object(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := s.client.StatObject(ctx, s.bucket, object, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat %s: %w", key, err)
	}
	return ObjectInfo{
		Key:         key,
		Size:        info.Size,
		ETag:        info.ETag,
		ContentType: info.ContentType,
	}, nil
}

func (s *minioStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	object, err := s.object(key)
	if err != nil {
		return err
	}
	if size < 0 {
		return fmt.Errorf("put %s: size must be known and non-negative, got %d", key, size)
	}
	if _, err := s.client.PutObject(ctx, s.bucket, object, body, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

// Copy is how a staged object becomes a source object: the bytes are already in
// the bucket, so the promotion moves a reference rather than re-uploading
// something that may be gigabytes.
func (s *minioStore) Copy(ctx context.Context, sourceKey, destinationKey string) error {
	source, err := s.object(sourceKey)
	if err != nil {
		return err
	}
	destination, err := s.object(destinationKey)
	if err != nil {
		return err
	}
	if _, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: destination},
		minio.CopySrcOptions{Bucket: s.bucket, Object: source},
	); err != nil {
		return fmt.Errorf("copy %s to %s: %w", sourceKey, destinationKey, err)
	}
	return nil
}

func (s *minioStore) Remove(ctx context.Context, key string) error {
	object, err := s.object(key)
	if err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, object, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove %s: %w", key, err)
	}
	return nil
}
