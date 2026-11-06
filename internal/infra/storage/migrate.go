package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type MigrationResult struct {
	Location string `json:"location"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

// MigrateVerified never changes a database reference or deletes an old object.
// The owner performs its compare-and-swap only after this returns successfully.
func MigrateVerified(ctx context.Context, store ImmutableStore, source, destination string, expectedSize int64, expectedDigest string, apply bool) (MigrationResult, error) {
	result := MigrationResult{}
	ref, err := store.Reference(destination)
	if err != nil {
		return result, err
	}
	info, err := store.Stat(ctx, source)
	if err != nil {
		return result, err
	}
	if info.Size <= 0 || (expectedSize > 0 && info.Size != expectedSize) {
		return result, errors.New("source size does not match recorded facts")
	}
	body, err := store.Open(ctx, source)
	if err != nil {
		return result, err
	}
	defer body.Close()
	dir, err := os.MkdirTemp("", "wt-storage-migrate-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(dir)
	file, err := os.CreateTemp(dir, "object-*")
	if err != nil {
		return result, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, hash), body)
	if err != nil {
		return result, err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if size != info.Size || (expectedDigest != "" && digest != strings.ToLower(expectedDigest)) {
		return result, errors.New("source checksum does not match recorded facts")
	}
	result = MigrationResult{Location: ref, Size: size, SHA256: digest}
	if !apply {
		return result, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	if err := store.PutImmutable(ctx, ref, file, size, info.ContentType, digest); err != nil {
		return result, fmt.Errorf("destination was not verified: %w", err)
	}
	return result, nil
}
