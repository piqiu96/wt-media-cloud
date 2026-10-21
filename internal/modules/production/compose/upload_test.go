package compose

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
)

type uploadStore struct {
	bytes    []byte
	statSize *int64
	putErr   error
	removed  bool
}

func (s *uploadStore) Put(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.bytes, _ = io.ReadAll(body)
	return nil
}
func (s *uploadStore) Stat(_ context.Context, key string) (storage.ObjectInfo, error) {
	size := int64(len(s.bytes))
	if s.statSize != nil {
		size = *s.statSize
	}
	return storage.ObjectInfo{Key: key, Size: size}, nil
}
func (s *uploadStore) PresignGet(_ context.Context, _ string, _ time.Duration) (storage.Grant, error) {
	return storage.Grant{}, errors.New("unexpected presign")
}
func (s *uploadStore) Remove(context.Context, string) error { s.removed = true; return nil }
func (s *uploadStore) Copy(context.Context, string, string) error {
	return errors.New("unexpected copy")
}

func TestVerifyUploadStoresTheBytesAndCleans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.mp4")
	if err := os.WriteFile(path, []byte("video-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &uploadStore{}
	if err := VerifyUpload(context.Background(), store, path, "verify-abc"); err != nil {
		t.Fatal(err)
	}
	if string(store.bytes) != "video-bytes" {
		t.Fatalf("uploaded %q, want the local file", store.bytes)
	}
	if !store.removed {
		t.Fatal("temporary object was not removed")
	}
}

func TestVerifyUploadRejectsFailuresAndCleans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.mp4")
	if err := os.WriteFile(path, []byte("video-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	wrongSize := int64(len("video-bytes") + 1)
	for _, tc := range []struct {
		name     string
		putErr   error
		statSize *int64
		want     string
	}{
		{"unconfigured", storage.ErrNotConfigured, nil, "not configured"},
		{"upload", errors.New("upload failed"), nil, "upload failed"},
		{"size", nil, &wrongSize, "size mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &uploadStore{putErr: tc.putErr, statSize: tc.statSize}
			err := VerifyUpload(context.Background(), store, path, "verify-abc")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %s", err, tc.want)
			}
			if !store.removed {
				t.Fatal("temporary object was not removed")
			}
		})
	}
}
