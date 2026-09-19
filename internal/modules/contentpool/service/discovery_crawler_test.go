package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
)

func TestDouyinCrawlerSearchUsesServerCredentialsAndNormalizesItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2dysearchvideo" || r.URL.Query().Get("apiKey") != "secret-key" {
			t.Fatalf("unexpected request %s", r.URL.String())
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.PostForm.Get("keywords"); got != "王者荣耀" || r.PostForm.Get("count") != "20" || r.PostForm.Get("ck") != "server-cookie" {
			t.Fatalf("unexpected form %v", r.PostForm)
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"datalist":[{"aweme_info":{"aweme_id":"a1","desc":"热点","author":{"uid":"u1","nickname":"作者"}}}]}}`))
	}))
	defer server.Close()

	crawler := NewDouyinCrawlerWithClient(testDouyinClient(server))
	result, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "keyword", Config: map[string]any{"keyword": "王者荣耀"}})
	if err != nil || len(result.Items) != 1 || result.Items[0]["platform_content_id"] != "a1" || result.Items[0]["author_name"] != "作者" {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
}

func TestDouyinCrawlerRejectsMissingCredentials(t *testing.T) {
	_, err := (&DouyinCrawler{}).Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "keyword", Config: map[string]any{"keyword": "demo"}})
	if err != ErrCrawlerUnavailable {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestDouyinCrawlerBatchURLKeepsSuccessfulItemsWhenOneFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.PostForm.Get("shorturl") == "bad" {
			http.Error(w, "", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"result":1,"data":{"aweme_id":"a1","desc":"ok"}}`))
	}))
	defer server.Close()

	crawler := NewDouyinCrawlerWithClient(testDouyinClient(server))
	result, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "url", Config: map[string]any{"urls": []any{"bad", "good"}}})
	if err != nil || result.Failed != 1 || len(result.Items) != 1 || result.Items[0]["platform_content_id"] != "a1" {
		t.Fatalf("unexpected batch result=%+v err=%v", result, err)
	}
}

func testDouyinClient(server *httptest.Server) *douyinclient.Client {
	parsed, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(parsed.Port())
	return douyinclient.NewWithHTTPClient(
		config.ClientConfig{Name: "douyin", Scheme: parsed.Scheme, Host: parsed.Hostname(), Port: port, Retry: config.RetryConfig{Attempts: 1}},
		config.DouyinCredentialConfig{APIKey: "secret-key", Cookie: "server-cookie"},
		server.Client(),
	)
}

func TestDouyinCrawlerBatchURLChunksNumericIDsAndKeepsShortURLFallback(t *testing.T) {
	batches := make([][]string, 0, 2)
	shortCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		switch r.URL.Path {
		case "/batchDyVideo":
			ids := strings.Split(r.PostForm.Get("ids"), ",")
			batches = append(batches, ids)
			items := make([]string, 0, len(ids))
			for _, id := range ids {
				items = append(items, `{"aweme_id":"`+id+`","desc":"batch"}`)
			}
			_, _ = w.Write([]byte(`{"result":1,"data":[` + strings.Join(items, ",") + `]}`))
		case "/dyVideo/detail":
			shortCalls++
			if got := r.PostForm.Get("shorturl"); got != "https://v.douyin.test/demo" {
				t.Errorf("shorturl = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":1,"data":{"aweme_id":"short-1","desc":"short"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	ids := make([]string, 11)
	for index := range ids {
		ids[index] = strconv.Itoa(index + 1)
	}
	crawler := NewDouyinCrawlerWithClient(testDouyinClient(server))
	result, err := crawler.Discover(context.Background(), CrawlerRequest{
		Platform:  "douyin",
		Operation: "url",
		Config:    map[string]any{"urls": append(ids, "https://v.douyin.test/demo")},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(batches) != 2 || len(batches[0]) != 10 || len(batches[1]) != 1 || batches[1][0] != "11" {
		t.Fatalf("batches = %#v", batches)
	}
	if shortCalls != 1 || len(result.Items) != 12 || result.Failed != 0 {
		t.Fatalf("result=%+v shortCalls=%d", result, shortCalls)
	}
}
