package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
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
		_, _ = w.Write([]byte(`{"result":1,"data":{"datalist":[{"aweme_info":{"aweme_id":"a1","desc":"热点","author":{"uid":"u1","nickname":"作者"},"statistics":{"digg_count":12000,"collect_count":700}}}]}}`))
	}))
	defer server.Close()

	crawler := NewDouyinCrawlerWithClient(testDouyinClient(t, server))
	result, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "keyword", Config: map[string]any{"keyword": "王者荣耀"}})
	if err != nil || len(result.Items) != 1 || result.Items[0]["platform_content_id"] != "a1" || result.Items[0]["author_name"] != "作者" || result.Items[0]["like_count"] != int64(12000) || result.Items[0]["favorite_count"] != int64(700) {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
}

func TestDouyinCrawlerRejectsMissingCredentials(t *testing.T) {
	_, err := (&DouyinCrawler{}).Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "keyword", Config: map[string]any{"keyword": "demo"}})
	if err != ErrCrawlerUnavailable {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestDouyinCrawlerAuthorIsTemporarilyUnavailable(t *testing.T) {
	crawler := NewDouyinCrawlerWithClient(testDouyinClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected provider request %s", r.URL.Path)
	}))))
	_, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "author", Config: map[string]any{"author": "sec-author"}})
	if !errors.Is(err, ErrDouyinAuthorUnavailable) {
		t.Fatalf("err=%v", err)
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

	crawler := NewDouyinCrawlerWithClient(testDouyinClient(t, server))
	result, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "url", Config: map[string]any{"urls": []any{"bad", "good"}}})
	if err != nil || result.Failed != 1 || len(result.Items) != 1 || result.Items[0]["platform_content_id"] != "a1" {
		t.Fatalf("unexpected batch result=%+v err=%v", result, err)
	}
}

func testDouyinClient(t *testing.T, server *httptest.Server) *douyinclient.Client {
	transport, closer, err := httpclient.New(httpclient.Config{
		Name:       "douyin-test",
		Endpoint:   httpclient.EndpointConfig{Scheme: "http", Host: "127.0.0.1", Port: 18080},
		Timeout:    httpclient.Duration{Duration: 5 * time.Second},
		Connection: httpclient.ConnectionConfig{DialTimeout: httpclient.Duration{Duration: time.Second}},
		Retry:      httpclient.RetryConfig{Attempts: 1, Policy: "fixed"},
	})
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		if err := closer(); err != nil {
			t.Errorf("close HTTP client: %v", err)
		}
	})
	return douyinclient.NewWithClient(
		server.URL,
		config.DouyinCredentialConfig{APIKey: "secret-key", Cookie: "server-cookie"},
		transport,
	)
}

func TestDouyinCrawlerBatchURLChunksNumericIDsAndKeepsShortURLFallback(t *testing.T) {
	var mu sync.Mutex
	batches := make([][]string, 0, 2)
	shortCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		switch r.URL.Path {
		case "/batchDyVideo":
			ids := strings.Split(r.PostForm.Get("ids"), ",")
			mu.Lock()
			batches = append(batches, ids)
			mu.Unlock()
			items := make([]string, 0, len(ids))
			for _, id := range ids {
				items = append(items, `{"aweme_id":"`+id+`","desc":"batch"}`)
			}
			_, _ = w.Write([]byte(`{"result":1,"data":[` + strings.Join(items, ",") + `]}`))
		case "/dyVideo/detail":
			mu.Lock()
			shortCalls++
			mu.Unlock()
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
	crawler := NewDouyinCrawlerWithClient(testDouyinClient(t, server))
	result, err := crawler.Discover(context.Background(), CrawlerRequest{
		Platform:  "douyin",
		Operation: "url",
		Config:    map[string]any{"urls": append(ids, "https://v.douyin.test/demo")},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(batches) != 2 || len(batches[0]) != 10 || len(batches[1]) != 1 || batches[1][0] != "11" {
		t.Fatalf("batches = %#v", batches)
	}
	if shortCalls != 1 || len(result.Items) != 12 || result.Failed != 0 {
		t.Fatalf("result=%+v shortCalls=%d", result, shortCalls)
	}
}
