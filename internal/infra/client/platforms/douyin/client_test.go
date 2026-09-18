package douyin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func TestDouyinSearchBuildsTypedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertDouyinRequest(t, r, "/dyRank", "search-secret", "search-cookie")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		want := map[string]string{
			"keywords": "王者荣耀", "limit": "20", "offset": "10",
			"sort_type": "0", "content_type": "1", "publish_time": "0",
			"filter_duration": "0", "ck": "search-cookie",
		}
		for key, value := range want {
			if got := r.PostForm.Get(key); got != value {
				t.Errorf("form %s = %q, want %q", key, got, value)
			}
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"data":[{"aweme_id":"a1"}]}}`))
	}))
	defer server.Close()

	client := NewWithHTTPClient(
		clientConfigForTest(server.URL),
		config.DouyinCredentialConfig{APIKey: "search-secret", Cookie: "search-cookie", Headers: map[string]string{"X-Client": "wt-media-cloud"}},
		server.Client(),
	)
	payload, err := client.Search(context.Background(), SearchRequest{Keyword: "王者荣耀", Limit: 20, Offset: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	data, _ := payload["data"].(map[string]any)
	list, _ := data["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("search payload = %#v, want one item", payload)
	}
}

func TestDouyinFindAuthorBuildsTypedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertDouyinRequest(t, r, "/dyUser/detail", "author-secret", "author-cookie")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		for key, value := range map[string]string{"uid": "author-1", "nickname": "author-1", "limit": "10", "offset": "5"} {
			if got := r.PostForm.Get(key); got != value {
				t.Errorf("form %s = %q, want %q", key, got, value)
			}
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"aweme_list":[{"id":"a1"}]}}`))
	}))
	defer server.Close()

	client := NewWithHTTPClient(clientConfigForTest(server.URL), config.DouyinCredentialConfig{APIKey: "author-secret", Cookie: "author-cookie"}, server.Client())
	payload, err := client.FindAuthor(context.Background(), FindAuthorRequest{Author: "author-1", Limit: 10, Offset: 5})
	if err != nil {
		t.Fatalf("FindAuthor() error = %v", err)
	}
	data, _ := payload["data"].(map[string]any)
	if _, exists := data["aweme_list"]; !exists {
		t.Fatalf("author payload = %#v, want aweme_list", payload)
	}
}

func TestDouyinFetchByURLBuildsTypedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertDouyinRequest(t, r, "/dyVideo/detail", "video-secret", "video-cookie")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("shorturl"); got != "https://v.douyin.test/demo" {
			t.Errorf("shorturl = %q", got)
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"aweme_id":"a1","desc":"video"}}`))
	}))
	defer server.Close()

	client := NewWithHTTPClient(clientConfigForTest(server.URL), config.DouyinCredentialConfig{APIKey: "video-secret", Cookie: "video-cookie"}, server.Client())
	payload, err := client.FetchByURL(context.Background(), FetchByURLRequest{URL: "https://v.douyin.test/demo"})
	if err != nil {
		t.Fatalf("FetchByURL() error = %v", err)
	}
	data, _ := payload["data"].(map[string]any)
	if data["aweme_id"] != "a1" {
		t.Fatalf("video payload = %#v", payload)
	}
}

func TestDouyinInitializePublishesConfiguredClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("apiKey"); got != "initialize-secret" {
			t.Errorf("apiKey = %q, want initialize-secret", got)
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{}}`))
	}))
	defer server.Close()

	_ = Close()
	if err := Initialize(clientConfigForTest(server.URL), config.DouyinCredentialConfig{APIKey: "initialize-secret"}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	if _, err := Get().Search(context.Background(), SearchRequest{Keyword: "demo"}); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
}

func TestClientMapsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"douyin unavailable"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewWithHTTPClient(clientConfigForTest(server.URL), config.DouyinCredentialConfig{APIKey: "secret"}, server.Client())
	_, err := client.Search(context.Background(), SearchRequest{Keyword: "demo"})
	if err == nil {
		t.Fatal("Search() with HTTP 502 succeeded")
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "douyin unavailable") {
		t.Fatalf("error = %v, want status and response body", err)
	}
}

func assertDouyinRequest(t *testing.T, r *http.Request, path, apiKey, cookie string) {
	t.Helper()
	if r.URL.Path != path {
		t.Errorf("path = %q, want %q", r.URL.Path, path)
	}
	if got := r.URL.Query().Get("apiKey"); got != apiKey {
		t.Errorf("apiKey = %q, want %q", got, apiKey)
	}
	if got, want := r.Header.Get("Cookie"), cookie; got != want {
		t.Errorf("cookie header = %q, want %q", got, want)
	}
	if got, want := r.Header.Get("X-Client"), "wt-media-cloud"; got != want && path == "/dyRank" {
		t.Errorf("x-client header = %q, want %q", got, want)
	}
	if got, want := r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"; got != want {
		t.Errorf("content type = %q, want %q", got, want)
	}
}

func clientConfigForTest(rawURL string) config.ClientConfig {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	port := 80
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			panic(err)
		}
	} else if parsed.Scheme == "https" {
		port = 443
	}
	return config.ClientConfig{
		Name:    "douyin",
		Scheme:  parsed.Scheme,
		Host:    parsed.Hostname(),
		Port:    port,
		Timeout: config.Duration{Duration: 5 * time.Second},
		Retry:   config.RetryConfig{Attempts: 1, Interval: config.Duration{}},
	}
}
