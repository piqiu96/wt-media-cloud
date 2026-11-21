package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"net/http"
	"net/http/httptest"
	"os"

	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc, key string) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewWithClient(&http.Client{Timeout: time.Second}, server.URL, key, "deepseek-flash")
}
func TestCompleteJSONUsesConfiguredModelAndReturnsOnlyCompleteContent(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("wrong request identity")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "deepseek-flash" || body["stream"] != false || body["thinking"].(map[string]any)["type"] != "disabled" || body["response_format"].(map[string]any)["type"] != "json_object" {
			t.Errorf("wrong bounded JSON operation: %+v", body)
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"description\":\"真实建议\"}"}}]}`))
	}, "test-secret")
	got, err := client.CompleteJSON(context.Background(), "只返回JSON", "素材标题")
	if err != nil || got != `{"description":"真实建议"}` {
		t.Fatalf("content=%q err=%v", got, err)
	}
}
func TestCompleteJSONRefusesPartialEmptyRefusedAndFailedResponses(t *testing.T) {
	for _, body := range []string{
		`{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"{\"description\":\"半成品\"}"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"   "}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{}","refusal":"rejected"}}]}`,
		`{"choices":[]}`, `not JSON`,
	} {
		t.Run(body, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }, "test-secret")
			if _, err := c.CompleteJSON(context.Background(), "json", "x"); err == nil {
				t.Fatal("invalid completion accepted")
			}
		})
	}
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
		_, _ = w.Write([]byte("test-secret-private-response"))
	}, "test-secret")
	_, err := c.CompleteJSON(context.Background(), "json", "x")
	if err == nil || strings.Contains(err.Error(), "test-secret") || calls != 1 {
		t.Fatalf("error leaked or operation replayed: %v calls=%d", err, calls)
	}
}
func TestMissingKeyAndCancelledContextNeverCallProvider(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls++ }, "")
	if _, err := c.CompleteJSON(context.Background(), "json", "x"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing key=%v", err)
	}
	c.key = "test-secret"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.CompleteJSON(ctx, "json", "x"); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancel=%v calls=%d", err, calls)
	}
}

func TestInFlightCancellationReturnsBeforeProviderCompletes(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { close(started); <-release }, "test-secret")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := c.CompleteJSON(ctx, "JSON", "test"); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel=%v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("in-flight model request ignores cancellation")
	}
}

// This explicitly opted-in check uses only the ignored local runtime credential.
func TestLiveDeepSeekCompleteJSON(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_LIVE_AI") != "1" {
		t.Skip("requires explicitly enabled real provider")
	}
	cfg, err := config.LoadFromDir("../../../../config")
	if err != nil {
		t.Fatal(err)
	}
	close, err := Initialize(cfg.App.PublicationGeneration, cfg.Credentials.DeepSeek)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	ctx, cancel := context.WithTimeout(context.Background(), 175*time.Second)
	defer cancel()
	content, err := Get().CompleteJSON(ctx, "只返回形如{\"description\":\"作品描述字符串\",\"topics\":\"话题字符串\"}的JSON对象；两个值必须是字符串，不使用数组或嵌套对象。根据素材标题提供简洁作品描述和话题，不编造情节。", `{"material_title":"三角洲行动实战思路：学会判断与掌控全局"}`)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if json.Unmarshal([]byte(content), &got) != nil || strings.TrimSpace(got["description"]) == "" {
		keys := []string{}
		for key := range got {
			keys = append(keys, key)
		}
		t.Fatalf("invalid live JSON fields=%v, bytes=%d", keys, len(content))
	}
	t.Logf("live model=%s complete_fields=%d", cfg.App.PublicationGeneration.Model, len(got))
}
