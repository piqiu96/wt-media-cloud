package service

import (
	"context"
	"errors"
	"fmt"
	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
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
	client *douyinclient.Client
	lazy   bool
}

func NewDouyinCrawler() *DouyinCrawler {
	return &DouyinCrawler{lazy: true}
}
func NewDouyinCrawlerWithClient(client *douyinclient.Client) *DouyinCrawler {
	return &DouyinCrawler{client: client}
}

func (c *DouyinCrawler) Discover(ctx context.Context, request CrawlerRequest) (CrawlerResult, error) {
	client := c.activeClient()
	if client == nil || !client.Configured() {
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

func (c *DouyinCrawler) activeClient() *douyinclient.Client {
	if c == nil {
		return nil
	}
	if c.lazy {
		return douyinclient.Get()
	}
	return c.client
}

func (c *DouyinCrawler) fetchByURL(ctx context.Context, source string) (map[string]any, error) {
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("source URL is required")
	}
	payload, err := c.activeClient().FetchByURL(ctx, douyinclient.FetchByURLRequest{URL: source})
	if err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if len(data) == 0 {
		return nil, nil
	}
	return normalizeDouyinItem(data), nil
}

func (c *DouyinCrawler) search(ctx context.Context, keyword string, limit, offset int) ([]map[string]any, error) {
	payload, err := c.activeClient().Search(ctx, douyinclient.SearchRequest{
		Keyword: keyword,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	return normalizeDouyinList(data), nil
}

func (c *DouyinCrawler) authorPosts(ctx context.Context, author string, limit, offset int) ([]map[string]any, error) {
	if author == "" {
		return nil, errors.New("author is required")
	}
	payload, err := c.activeClient().FindAuthor(ctx, douyinclient.FindAuthorRequest{
		Author: author,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
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
