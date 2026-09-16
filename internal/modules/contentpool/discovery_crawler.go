package contentpool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// CrawlerRequest is the Cloud-owned input to a channel adapter. Credentials
// are deliberately absent: adapters read them only from the server runtime.
type CrawlerRequest struct {
	Platform  string
	Operation string
	Config    map[string]any
}

type CrawlerResult struct {
	Items   []map[string]any
	Scanned int
	Failed  int
}

type Crawler interface {
	Discover(context.Context, CrawlerRequest) (CrawlerResult, error)
}

var ErrCrawlerUnavailable = errors.New("content crawler is not configured")

type DouyinCrawler struct {
	base           string
	apiKey         string
	cookie         string
	authorEndpoint string
	client         *http.Client
}

func NewDouyinCrawlerFromEnv() *DouyinCrawler {
	return &DouyinCrawler{
		base:           strings.TrimRight(envOr("WT_MEDIA_DOUYIN_API_BASE", "https://api.itfaba.com"), "/"),
		apiKey:         os.Getenv("WT_MEDIA_DOUYIN_API_KEY"),
		cookie:         os.Getenv("WT_MEDIA_DOUYIN_COOKIE"),
		authorEndpoint: envOr("WT_MEDIA_DOUYIN_AUTHOR_ENDPOINT", "/dyUser/detail"),
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *DouyinCrawler) Discover(ctx context.Context, request CrawlerRequest) (CrawlerResult, error) {
	if c == nil || strings.TrimSpace(c.apiKey) == "" {
		return CrawlerResult{}, ErrCrawlerUnavailable
	}
	if strings.TrimSpace(request.Platform) != "douyin" {
		return CrawlerResult{}, fmt.Errorf("unsupported discovery platform: %s", request.Platform)
	}
	config := request.Config
	if config == nil {
		config = map[string]any{}
	}
	switch request.Operation {
	case "url":
		urls := stringValues(config["urls"])
		if len(urls) == 0 {
			urls = []string{strings.TrimSpace(fmt.Sprint(config["url"]))}
		}
		result := CrawlerResult{}
		for _, source := range urls {
			if source == "" {
				continue
			}
			result.Scanned++
			item, err := c.fetchByURL(ctx, source)
			if err != nil {
				result.Failed++
				continue
			}
			if item != nil {
				result.Items = append(result.Items, item)
			}
		}
		if len(result.Items) == 0 && result.Failed > 0 {
			return result, errors.New("douyin url discovery failed")
		}
		return result, nil
	case "keyword":
		keywords := stringValues(config["keywords"])
		if len(keywords) == 0 {
			keywords = []string{strings.TrimSpace(fmt.Sprint(config["keyword"]))}
		}
		result := CrawlerResult{}
		for _, keyword := range keywords {
			if keyword == "" {
				continue
			}
			items, err := c.search(ctx, keyword, intValue(config["limit"], 20), intValue(config["offset"], 0))
			if err != nil {
				result.Failed++
				continue
			}
			result.Scanned += len(items)
			result.Items = append(result.Items, items...)
		}
		if len(result.Items) == 0 && result.Failed > 0 {
			return result, errors.New("douyin keyword discovery failed")
		}
		return result, nil
	case "author":
		author := strings.TrimSpace(fmt.Sprint(config["author"]))
		if author == "" {
			author = strings.TrimSpace(fmt.Sprint(config["author_id"]))
		}
		items, err := c.authorPosts(ctx, author, intValue(config["limit"], 20), intValue(config["offset"], 0))
		if err != nil {
			return CrawlerResult{Failed: 1}, err
		}
		return CrawlerResult{Items: items, Scanned: len(items)}, nil
	default:
		return CrawlerResult{}, fmt.Errorf("unsupported discovery operation: %s", request.Operation)
	}
}

func (c *DouyinCrawler) fetchByURL(ctx context.Context, source string) (map[string]any, error) {
	fields := map[string]string{}
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("source URL is required")
	}
	if _, err := strconv.ParseUint(strings.TrimSpace(source), 10, 64); err == nil {
		fields["id"] = strings.TrimSpace(source)
	} else {
		fields["shorturl"] = strings.TrimSpace(source)
	}
	var payload map[string]any
	if err := c.post(ctx, "/dyVideo/detail", fields, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if len(data) == 0 {
		return nil, nil
	}
	return normalizeDouyinItem(data), nil
}

func (c *DouyinCrawler) search(ctx context.Context, keyword string, limit, offset int) ([]map[string]any, error) {
	fields := map[string]string{"keywords": keyword, "limit": strconv.Itoa(clamp(limit, 1, 30)), "offset": strconv.Itoa(max(offset, 0)), "sort_type": "0", "content_type": "1", "publish_time": "0", "filter_duration": "0"}
	var payload map[string]any
	if err := c.post(ctx, "/dyRank", fields, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	return normalizeDouyinList(data), nil
}

func (c *DouyinCrawler) authorPosts(ctx context.Context, author string, limit, offset int) ([]map[string]any, error) {
	if author == "" {
		return nil, errors.New("author is required")
	}
	fields := map[string]string{"uid": author, "nickname": author, "limit": strconv.Itoa(clamp(limit, 1, 30)), "offset": strconv.Itoa(max(offset, 0))}
	var payload map[string]any
	if err := c.post(ctx, c.authorEndpoint, fields, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if detail, ok := data["aweme_detail"].(map[string]any); ok {
		if item := normalizeDouyinItem(detail); item != nil {
			return []map[string]any{item}, nil
		}
	}
	return normalizeDouyinList(data), nil
}

func (c *DouyinCrawler) post(ctx context.Context, path string, fields map[string]string, target *map[string]any) error {
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	endpoint := c.base + path + "?apiKey=" + url.QueryEscape(c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "WT-Media-Cloud/1")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("douyin http status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	if !successValue((*target)["result"]) {
		return errors.New("douyin API rejected request")
	}
	return nil
}

func normalizeDouyinList(data map[string]any) []map[string]any {
	values := []any{}
	if raw, ok := data["data"].([]any); ok {
		values = raw
	} else if raw, ok := data["aweme_list"].([]any); ok {
		values = raw
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			if nested, ok := item["aweme_info"].(map[string]any); ok {
				item = nested
			}
			if normalized := normalizeDouyinItem(item); normalized != nil {
				out = append(out, normalized)
			}
		}
	}
	return out
}

func normalizeDouyinItem(item map[string]any) map[string]any {
	id := strings.TrimSpace(fmt.Sprint(item["aweme_id"]))
	if id == "" || id == "<nil>" {
		id = strings.TrimSpace(fmt.Sprint(item["id"]))
	}
	if id == "" || id == "<nil>" {
		return nil
	}
	description := fmt.Sprint(item["desc"])
	if description == "<nil>" || description == "" {
		description = fmt.Sprint(item["preview_title"])
	}
	author, _ := item["author"].(map[string]any)
	video, _ := item["video"].(map[string]any)
	cover := firstNestedURL(video, "origin_cover")
	if cover == "" {
		cover = firstNestedURL(video, "cover")
	}
	created := int64Value(item["create_time"])
	published := ""
	if created > 0 {
		published = time.Unix(created, 0).UTC().Format(time.RFC3339)
	}
	return map[string]any{"platform_content_id": id, "title": truncate(description, 500), "description": description, "cover_url": cover, "source_url": "https://www.douyin.com/video/" + id, "author_id": fmt.Sprint(author["uid"]), "author_name": fmt.Sprint(author["nickname"]), "published_at": published, "raw": item}
}

func firstNestedURL(parent map[string]any, key string) string {
	child, _ := parent[key].(map[string]any)
	values, _ := child["url_list"].([]any)
	if len(values) > 0 {
		return fmt.Sprint(values[0])
	}
	return ""
}
func successValue(value any) bool {
	switch v := value.(type) {
	case float64:
		return v == 1
	case int:
		return v == 1
	case string:
		return v == "1"
	}
	return false
}
func stringValues(value any) []string {
	out := []string{}
	switch values := value.(type) {
	case []any:
		for _, value := range values {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	case []string:
		for _, value := range values {
			if text := strings.TrimSpace(value); text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}
func intValue(value any, fallback int) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
func int64Value(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}
func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
func max(value, low int) int {
	if value < low {
		return low
	}
	return value
}
func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
