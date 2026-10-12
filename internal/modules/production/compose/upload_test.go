package compose

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
)

type uploadStore struct {
	url     string
	bytes   []byte
	putErr  error
	removed bool
}

func (s *uploadStore) Put(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.bytes, _ = io.ReadAll(body)
	return nil
}
func (s *uploadStore) Stat(_ context.Context, key string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Key: key, Size: int64(len(s.bytes))}, nil
}
func (s *uploadStore) PresignGet(_ context.Context, _ string, _ time.Duration) (storage.Grant, error) {
	return storage.Grant{URL: s.url}, nil
}
func (s *uploadStore) Remove(context.Context, string) error { s.removed = true; return nil }
func (s *uploadStore) Copy(context.Context, string, string) error {
	return errors.New("unexpected copy")
}

func TestVerifyUploadReadsBackAndCleans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.mp4")
	if err := os.WriteFile(path, []byte("video-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &uploadStore{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(store.bytes) }))
	defer server.Close()
	store.url = server.URL
	if err := VerifyUpload(context.Background(), store, time.Minute, path, "verify-abc"); err != nil {
		t.Fatal(err)
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
	for _, tc := range []struct {
		name     string
		putErr   error
		readback []byte
		want     string
	}{
		{"unconfigured", storage.ErrNotConfigured, nil, "not configured"},
		{"upload", errors.New("upload failed"), nil, "upload failed"},
		{"mismatch", nil, []byte("wrong-bytes"), "sha256"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &uploadStore{putErr: tc.putErr}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.readback != nil {
					_, _ = w.Write(tc.readback)
				} else {
					_, _ = io.Copy(w, bytes.NewReader(store.bytes))
				}
			}))
			defer server.Close()
			store.url = server.URL
			err := VerifyUpload(context.Background(), store, time.Minute, path, "verify-abc")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %s", err, tc.want)
			}
			if !store.removed {
				t.Fatal("temporary object was not removed")
			}
		})
	}
}
