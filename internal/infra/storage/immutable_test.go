package storage

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestImmutableUploadSplitsLargeVideoAndVerifiesReadback(t *testing.T) {
	// Leave room for signed streaming framing around an 8 MiB payload part.
	for _, scenario := range []string{"success", "part-failure", "corrupt-readback"} {
		t.Run(scenario, func(t *testing.T) {
			const gatewayLimit = 10 * 1024 * 1024
			body := bytes.Repeat([]byte("v"), gatewayLimit+1024)
			digest := DigestOf(body)
			var mu sync.Mutex
			var stored []byte
			parts := map[int][]byte{}
			var completed, aborted bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
				w.Header().Set("ETag", `"video-etag"`)
				switch {
				case r.Method == "HEAD":
					if stored == nil {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.Header().Set("Content-Length", fmt.Sprint(len(stored)))
				case r.Method == "GET":
					if scenario == "corrupt-readback" {
						_, _ = w.Write(bytes.Repeat([]byte("x"), len(stored)))
						return
					}
					_, _ = w.Write(stored)
				case r.Method == "POST" && r.URL.Query().Has("uploads"):
					if r.Header.Get("X-Amz-Meta-Sha256") != digest {
						t.Error("multipart initiation lost content digest metadata")
					}
					w.Header().Set("Content-Type", "application/xml")
					_, _ = w.Write([]byte(`<InitiateMultipartUploadResult><Bucket>test-bucket</Bucket><Key>media-ku/material/188.mp4</Key><UploadId>video-upload</UploadId></InitiateMultipartUploadResult>`))
				case r.Method == "PUT":
					if scenario == "part-failure" {
						w.Header().Set("Content-Type", "application/xml")
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`<Error><Code>InvalidRequest</Code></Error>`))
						return
					}
					if r.ContentLength > gatewayLimit {
						w.WriteHeader(http.StatusRequestEntityTooLarge)
						return
					}
					if r.URL.Query().Get("uploadId") != "video-upload" {
						t.Error("large upload bypassed multipart protocol")
					}
					if r.Header.Get("Content-Md5") == "" {
						t.Error("uploaded part lost its checksum")
					}
					var part int
					_, _ = fmt.Sscan(r.URL.Query().Get("partNumber"), &part)
					reader := io.Reader(r.Body)
					if strings.HasPrefix(r.Header.Get("X-Amz-Content-Sha256"), "STREAMING-") {
						reader = httputil.NewChunkedReader(r.Body)
					}
					parts[part], _ = io.ReadAll(reader)
				case r.Method == "POST" && r.URL.Query().Get("uploadId") == "video-upload":
					if r.Header.Get("If-None-Match") != "*" {
						t.Error("multipart completion lost the non-overwrite condition")
					}
					var completion struct {
						Parts []struct {
							Number int `xml:"PartNumber"`
						} `xml:"Part"`
					}
					if err := xml.NewDecoder(r.Body).Decode(&completion); err != nil {
						t.Error(err)
					}
					if len(completion.Parts) != 2 {
						t.Errorf("completed %d parts, want 2", len(completion.Parts))
					}
					var numbers []int
					for number := range parts {
						numbers = append(numbers, number)
					}
					sort.Ints(numbers)
					for _, number := range numbers {
						stored = append(stored, parts[number]...)
					}
					completed = true
					w.Header().Set("Content-Type", "application/xml")
					_, _ = w.Write([]byte(`<CompleteMultipartUploadResult><Bucket>test-bucket</Bucket><Key>media-ku/material/188.mp4</Key><ETag>"video-etag"</ETag></CompleteMultipartUploadResult>`))
				case r.Method == "DELETE":
					aborted = true
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected upload request %s", r.Method)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			s, err := newMinioStore(testStorageConfig(strings.TrimPrefix(server.URL, "http://")), testCredential())
			if err != nil {
				t.Fatal(err)
			}
			s.claims = &testClaims{values: map[string]string{}}
			err = s.PutImmutable(context.Background(), "media-ku/material/188.mp4", bytes.NewReader(body), int64(len(body)), "video/mp4", digest)
			if scenario == "part-failure" {
				if err == nil || !aborted || completed {
					t.Fatalf("err=%v aborted=%v completed=%v", err, aborted, completed)
				}
				return
			}
			if scenario == "corrupt-readback" {
				if err != ErrObjectConflict || !completed {
					t.Fatalf("corrupt readback: err=%v completed=%v", err, completed)
				}
				return
			}
			if err != nil {
				t.Fatalf("upload video through a bounded gateway: %v", err)
			}
			if !completed || !bytes.Equal(stored, body) {
				t.Fatal("multipart upload did not preserve the verified video")
			}
			if err := s.PutImmutable(context.Background(), "media-ku/material/188.mp4", strings.NewReader("other"), 5, "video/mp4", DigestOf([]byte("other"))); err != ErrObjectConflict {
				t.Fatalf("conflicting video was not refused: %v", err)
			}
		})
	}
}

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
	s.claims = &testClaims{values: map[string]string{}}
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

type testClaims struct {
	sync.Mutex
	values map[string]string
}

func (c *testClaims) Claim(_ context.Context, key, digest string, size int64) (bool, error) {
	c.Lock()
	defer c.Unlock()
	identity := fmt.Sprintf("%s:%d", digest, size)
	if prior, ok := c.values[key]; ok {
		return prior == identity, nil
	}
	c.values[key] = identity
	return true, nil
}

func TestImmutableUploadFailsClosedWithoutSharedClaims(t *testing.T) {
	s, err := newMinioStore(testStorageConfig("127.0.0.1:1"), testCredential())
	if err != nil {
		t.Fatal(err)
	}
	err = s.PutImmutable(context.Background(), "media-ku/material/42.mp4", strings.NewReader("video-A"), 7, "video/mp4", DigestOf([]byte("video-A")))
	if err == nil || !strings.Contains(err.Error(), "claims") {
		t.Fatalf("must refuse an uncoordinated upload: %v", err)
	}
}
