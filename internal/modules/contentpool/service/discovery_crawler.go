package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
)

var (
	ErrCrawlerUnavailable      = errors.New("content crawler is not configured")
	ErrDouyinAuthorUnavailable = errors.New("douyin author discovery is temporarily unavailable")
)

type douyinCrawler struct{ client *douyinclient.Client }

func newDouyinCrawler() *douyinCrawler { return &douyinCrawler{client: douyinclient.Get()} }
func newDouyinCrawlerWithClient(client *douyinclient.Client) *douyinCrawler {
	return &douyinCrawler{client: client}
}

func (c *douyinCrawler) Discover(ctx context.Context, request dto.CrawlerRequest) (dto.CrawlerResult, error) {
	if c == nil || c.client == nil || !c.client.Configured() {
		return dto.CrawlerResult{}, ErrCrawlerUnavailable
	}
	if strings.TrimSpace(request.Platform) != "douyin" {
		return dto.CrawlerResult{}, fmt.Errorf("unsupported discovery platform: %s", request.Platform)
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
		return c.discoverURLs(ctx, urls)
	case "keyword":
		keywords := stringValues(config["keywords"])
		if len(keywords) == 0 {
			keywords = []string{strings.TrimSpace(fmt.Sprint(config["keyword"]))}
		}
		result := dto.CrawlerResult{}
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
		return dto.CrawlerResult{}, ErrDouyinAuthorUnavailable
	default:
		return dto.CrawlerResult{}, fmt.Errorf("unsupported discovery operation: %s", request.Operation)
	}
}

func (c *douyinCrawler) discoverURLs(ctx context.Context, urls []string) (dto.CrawlerResult, error) {
	result := dto.CrawlerResult{}
	ids := make([]string, 0, len(urls))
	shortURLs := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, source := range urls {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if _, exists := seen[source]; exists {
			continue
		}
		seen[source] = struct{}{}
		if _, err := strconv.ParseUint(source, 10, 64); err == nil {
			ids = append(ids, source)
		} else {
			shortURLs = append(shortURLs, source)
		}
	}
	result.Scanned = len(ids) + len(shortURLs)
	for start := 0; start < len(ids); start += 10 {
		end := min(start+10, len(ids))
		items, err := c.fetchByIDs(ctx, ids[start:end])
		if err != nil {
			result.Failed += end - start
			continue
		}
		result.Items = append(result.Items, items...)
	}
	for _, source := range shortURLs {
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
}

func (c *douyinCrawler) fetchByURL(ctx context.Context, source string) (map[string]any, error) {
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("source URL is required")
	}
	payload, err := c.client.FetchByURL(ctx, douyinclient.FetchByURLRequest{URL: source})
	if err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	return normalizeDouyinItem(data), nil
}
func (c *douyinCrawler) fetchByIDs(ctx context.Context, ids []string) ([]map[string]any, error) {
	payload, err := c.client.FetchByIDs(ctx, douyinclient.FetchByIDsRequest{IDs: ids})
	if err != nil {
		return nil, err
	}
	return normalizeDouyinPayload(payload), nil
}

func (c *douyinCrawler) search(ctx context.Context, keyword string, limit, offset int) ([]map[string]any, error) {
	payload, err := c.client.Search(ctx, douyinclient.SearchRequest{Keyword: keyword, Count: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	return normalizeDouyinPayload(payload), nil
}
func (c *douyinCrawler) authorPosts(ctx context.Context, author string, limit int, maxCursor int64) ([]map[string]any, error) {
	if author == "" {
		return nil, errors.New("author is required")
	}
	payload, err := c.client.FindAuthor(ctx, douyinclient.FindAuthorRequest{SecUID: author, Count: limit, MaxCursor: maxCursor})
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
func normalizeDouyinPayload(payload map[string]any) []map[string]any {
	if values, ok := payload["data"].([]any); ok {
		return normalizeDouyinValues(values)
	}
	return normalizeDouyinList(payloadData(payload))
}

func normalizeDouyinList(data map[string]any) []map[string]any {
	values := []any{}
	if raw, ok := data["datalist"].([]any); ok {
		values = raw
	} else if raw, ok := data["data"].([]any); ok {
		values = raw
	} else if raw, ok := data["aweme_list"].([]any); ok {
		values = raw
	}
	return normalizeDouyinValues(values)
}

func normalizeDouyinValues(values []any) []map[string]any {
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

func intValue64(values ...any) int64 {
	for _, value := range values {
		if parsed := int64Value(value); parsed != 0 {
			return parsed
		}
	}
	return 0
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}
func normalizeDouyinItem(item map[string]any) map[string]any {
	if item == nil {
		return nil
	}
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
	published := ""
	if created := int64Value(item["create_time"]); created > 0 {
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
	switch value := value.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
func int64Value(value any) int64 {
	switch value := value.(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	}
	return 0
}
func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
