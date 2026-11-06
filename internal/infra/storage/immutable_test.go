package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"sync"
	"testing"
)

func TestImmutableUploadIsIdempotentAndRejectsConflictingBytes(t *testing.T) {
	var mu sync.Mutex
	var stored []byte
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		switch r.Method {
		case "HEAD":
			if stored == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(stored)))
			w.Header().Set("ETag", `"test-etag"`)
		case "GET":
			if stored == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(stored)))
			_, _ = w.Write(stored)
		case "PUT":
			puts++
			if r.Header.Get("If-None-Match") != "*" {
				t.Error("missing non-overwrite condition")
			}
			if stored != nil {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(412)
				_, _ = w.Write([]byte(`<Error><Code>PreconditionFailed</Code></Error>`))
				return
			}
			var reader io.Reader = r.Body
			if strings.HasPrefix(r.Header.Get("X-Amz-Content-Sha256"), "STREAMING-") {
				reader = httputil.NewChunkedReader(r.Body)
			}
			stored, _ = io.ReadAll(reader)
			w.Header().Set("ETag", `"test-etag"`)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cfg := testStorageConfig(strings.TrimPrefix(server.URL, "http://"))
	cfg.Prefix = ""
	s, err := newMinioStore(cfg, testCredential())
	if err != nil {
		t.Fatal(err)
	}
	key, _ := s.Reference("media-ku/material/42.mp4")
	for _, body := range []string{"video-A", "video-A", "video-B"} {
		err = s.PutImmutable(context.Background(), key, strings.NewReader(body), int64(len(body)), "video/mp4", DigestOf([]byte(body)))
		if body == "video-B" {
			if err == nil {
				t.Fatal("overwrote immutable object")
			}
		} else if err != nil {
			t.Fatalf("upload %q: %v stored=%q", body, err, stored)
		}
	}
	if string(stored) != "video-A" || puts != 1 {
		t.Fatalf("stored=%q puts=%d", stored, puts)
	}
}
