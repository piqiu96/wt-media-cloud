package douyin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

func TestDouyinSearchBuildsTypedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertDouyinRequest(t, r, "/v2dysearchvideo", "search-secret", "search-cookie")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		want := map[string]string{
			"keywords": "王者荣耀", "count": "20", "offset": "10", "ck": "search-cookie",
		}
		for key, value := range want {
			if got := r.PostForm.Get(key); got != value {
				t.Errorf("form %s = %q, want %q", key, got, value)
			}
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"data":[{"aweme_id":"a1"}]}}`))
	}))
	defer server.Close()

	client := NewWithClient(
		server.URL,
		config.DouyinCredentialConfig{APIKey: "search-secret", Cookie: "search-cookie", Headers: map[string]string{"X-Client": "wt-media-cloud"}},
		newTransport(1),
	)
	payload, err := client.Search(context.Background(), SearchRequest{Keyword: "王者荣耀", Count: 20, Offset: 10})
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
		assertDouyinRequest(t, r, "/v5/dyhome", "author-secret", "author-cookie")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		for key, value := range map[string]string{"sec_uid": "sec-author-1", "count": "10", "max_cursor": "123", "ck": "author-cookie"} {
			if got := r.PostForm.Get(key); got != value {
				t.Errorf("form %s = %q, want %q", key, got, value)
			}
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"aweme_list":[{"id":"a1"}]}}`))
	}))
	defer server.Close()

	client := NewWithClient(server.URL, config.DouyinCredentialConfig{APIKey: "author-secret", Cookie: "author-cookie"}, newTransport(1))
	payload, err := client.FindAuthor(context.Background(), FindAuthorRequest{SecUID: "sec-author-1", Count: 10, MaxCursor: 123})
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

	client := NewWithClient(server.URL, config.DouyinCredentialConfig{APIKey: "video-secret", Cookie: "video-cookie"}, newTransport(1))
	payload, err := client.FetchByURL(context.Background(), FetchByURLRequest{URL: "https://v.douyin.test/demo"})
	if err != nil {
		t.Fatalf("FetchByURL() error = %v", err)
	}
	data, _ := payload["data"].(map[string]any)
	if data["aweme_id"] != "a1" {
		t.Fatalf("video payload = %#v", payload)
	}
}

func TestDouyinFetchByIDsBuildsTypedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertDouyinRequest(t, r, "/batchDyVideo", "batch-secret", "batch-cookie")
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("ids"); got != "1,2,3" {
			t.Errorf("ids = %q, want 1,2,3", got)
		}
		_, _ = w.Write([]byte(`{"result":1,"data":[{"aweme_id":"a1"},{"aweme_id":"a2"}]}`))
	}))
	defer server.Close()

	client := NewWithClient(server.URL, config.DouyinCredentialConfig{APIKey: "batch-secret", Cookie: "batch-cookie"}, newTransport(1))
	payload, err := client.FetchByIDs(context.Background(), FetchByIDsRequest{IDs: []string{"1", "2", "2", " 3 "}})
	if err != nil {
		t.Fatalf("FetchByIDs() error = %v", err)
	}
	values, ok := payload["data"].([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("batch payload = %#v, want two-item data array", payload)
	}
}

func TestDouyinFetchByIDsValidatesCount(t *testing.T) {
	client := NewWithClient("http://douyin.invalid", config.DouyinCredentialConfig{APIKey: "secret"}, newTransport(1))
	for _, ids := range [][]string{nil, {""}, {"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}} {
		if _, err := client.FetchByIDs(context.Background(), FetchByIDsRequest{IDs: ids}); err == nil {
			t.Fatalf("FetchByIDs(%d IDs) succeeded, want error", len(ids))
		}
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

	closeHTTPClient(t, initializeHTTPClient(t, "douyin", 1))
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

	client := NewWithClient(server.URL, config.DouyinCredentialConfig{APIKey: "secret"}, newTransport(1))
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
	return config.ClientConfig{Name: "douyin", BaseURL: rawURL, Timeout: config.Duration{Duration: 5 * time.Second}}
}

func newTransport(attempts int) *httpclient.Client {
	instance, closer, err := httpclient.New(httpclient.Config{
		Timeout:    httpclient.Duration{5 * time.Second},
		Connection: httpclient.ConnectionConfig{DialTimeout: httpclient.Duration{time.Second}},
		Retry:      httpclient.RetryConfig{Attempts: attempts, Delay: httpclient.Duration{}, Policy: "fixed"},
	})
	if err != nil {
		panic(err)
	}
	_ = closer
	return instance
}

func initializeHTTPClient(t *testing.T, name string, attempts int) func() error {
	t.Helper()
	content := "timeout = \"5s\"\n[connection]\ndial_timeout = \"1s\"\n[retry]\nattempts = " + strconv.Itoa(attempts) + "\npolicy = \"fixed\"\n"
	closer, err := httpclient.Initialize([]pkgconfig.Document{{Name: name, Format: pkgconfig.FormatTOML, Raw: []byte(content)}}, nil)
	if err != nil {
		t.Fatalf("initialize http client: %v", err)
	}
	return closer
}

func closeHTTPClient(t *testing.T, closer func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := closer(); err != nil {
			t.Fatalf("close http client: %v", err)
		}
	})
}
