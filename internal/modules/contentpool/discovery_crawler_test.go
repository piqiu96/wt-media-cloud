package contentpool

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDouyinCrawlerSearchUsesServerCredentialsAndNormalizesItems(t *testing.T) {
	seen := ""
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.URL.Path + "?" + r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/dyRank" || r.URL.Query().Get("apiKey") != "secret-key" || !strings.Contains(string(body), "keywords=%E7%8E%8B%E8%80%85%E8%8D%A3%E8%80%80") {
			t.Fatalf("unexpected request %s body=%s", r.URL.String(), body)
		}
		return jsonResponse(`{"result":1,"data":{"data":[{"aweme_info":{"aweme_id":"a1","desc":"热点","author":{"uid":"u1","nickname":"作者"}}}]}}`), nil
	})}
	crawler := &DouyinCrawler{base: "http://crawler.test", apiKey: "secret-key", client: client}
	result, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "keyword", Config: map[string]any{"keyword": "王者荣耀"}})
	if err != nil || len(result.Items) != 1 || result.Items[0]["platform_content_id"] != "a1" || result.Items[0]["author_name"] != "作者" {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
	if !strings.HasPrefix(seen, "/dyRank?apiKey=secret-key") {
		t.Fatalf("unexpected path %s", seen)
	}
}

func TestDouyinCrawlerRejectsMissingCredentials(t *testing.T) {
	_, err := (&DouyinCrawler{}).Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "keyword", Config: map[string]any{"keyword": "demo"}})
	if err != ErrCrawlerUnavailable {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestDouyinCrawlerBatchURLKeepsSuccessfulItemsWhenOneFails(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "shorturl=bad") {
			return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: r}, nil
		}
		return jsonResponse(`{"result":1,"data":{"aweme_id":"a1","desc":"ok"}}`), nil
	})}
	crawler := &DouyinCrawler{base: "http://crawler.test", apiKey: "secret-key", client: client}
	result, err := crawler.Discover(context.Background(), CrawlerRequest{Platform: "douyin", Operation: "url", Config: map[string]any{"urls": []any{"bad", "good"}}})
	if err != nil || result.Failed != 1 || len(result.Items) != 1 || result.Items[0]["platform_content_id"] != "a1" {
		t.Fatalf("unexpected batch result=%+v err=%v", result, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(value string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(value)), Header: make(http.Header)}
}
