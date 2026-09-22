package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

const (
	maxImportResults  = 100
	maxDirectSearch   = 100
	directIDBatchSize = 10
)

type directSearchTargets struct {
	ids  []string
	urls []string
}

type manualSearchClient interface {
	Configured() bool
	Search(context.Context, douyinclient.SearchRequest) (map[string]any, error)
	FindAuthor(context.Context, douyinclient.FindAuthorRequest) (map[string]any, error)
	FetchByURL(context.Context, douyinclient.FetchByURLRequest) (map[string]any, error)
	FetchByIDs(context.Context, douyinclient.FetchByIDsRequest) (map[string]any, error)
}

func douyinClient() manualSearchClient { return douyinclient.Get() }

func searchWithClient(ctx context.Context, input dto.SearchInput, client manualSearchClient) (dto.SearchResponse, error) {
	input.Platform = strings.TrimSpace(input.Platform)
	input.Keyword = strings.TrimSpace(input.Keyword)
	input.Query = strings.TrimSpace(input.Query)
	if input.Platform != "douyin" || (input.Query == "" && input.Keyword == "") || input.Offset < 0 {
		return dto.SearchResponse{}, ErrDiscoveryInvalid
	}
	if client == nil || !client.Configured() {
		return dto.SearchResponse{}, ErrCrawlerUnavailable
	}
	if input.Query != "" {
		return searchDirectWithClient(ctx, input.Query, client)
	}
	input.Limit = normalizedSearchLimit(input.Limit)
	payload, err := client.Search(ctx, douyinclient.SearchRequest{Keyword: input.Keyword, Count: input.Limit, Offset: input.Offset})
	if err != nil {
		return dto.SearchResponse{}, err
	}
	data := payloadData(payload)
	return dto.SearchResponse{
		Items:      typedSearchResults(normalizeDouyinList(data)),
		NextOffset: intValue64(data["cursor"], data["next_page"]),
		HasMore:    boolValue(data["has_more"]),
	}, nil
}

func searchDirectWithClient(ctx context.Context, query string, client manualSearchClient) (dto.SearchResponse, error) {
	targets, err := parseDirectSearchTargets(query)
	if err != nil {
		return dto.SearchResponse{}, err
	}
	items := make([]map[string]any, 0, len(targets.ids)+len(targets.urls))
	for start := 0; start < len(targets.ids); start += directIDBatchSize {
		end := start + directIDBatchSize
		if end > len(targets.ids) {
			end = len(targets.ids)
		}
		payload, err := client.FetchByIDs(ctx, douyinclient.FetchByIDsRequest{IDs: targets.ids[start:end]})
		if err != nil {
			return dto.SearchResponse{}, err
		}
		items = append(items, normalizeDouyinPayload(payload)...)
	}
	for _, source := range targets.urls {
		payload, err := client.FetchByURL(ctx, douyinclient.FetchByURLRequest{URL: source})
		if err != nil {
			return dto.SearchResponse{}, err
		}
		data, _ := payload["data"].(map[string]any)
		if detail, ok := data["aweme_detail"].(map[string]any); ok {
			if item := normalizeDouyinItem(detail); item != nil {
				items = append(items, item)
				continue
			}
		}
		items = append(items, normalizeDouyinPayload(payload)...)
	}
	return dto.SearchResponse{Items: typedSearchResults(items)}, nil
}

func parseDirectSearchTargets(query string) (directSearchTargets, error) {
	targets := directSearchTargets{}
	seenIDs := make(map[string]struct{})
	seenURLs := make(map[string]struct{})
	for _, token := range strings.FieldsFunc(query, func(r rune) bool {
		return r == ',' || r == '，' || unicode.IsSpace(r)
	}) {
		if isDirectID(token) {
			if _, exists := seenIDs[token]; exists {
				continue
			}
			seenIDs[token] = struct{}{}
			targets.ids = append(targets.ids, token)
			continue
		}
		id, source, valid := directDouyinTarget(token)
		if !valid {
			return directSearchTargets{}, ErrDiscoveryInvalid
		}
		if id != "" {
			if _, exists := seenIDs[id]; exists {
				continue
			}
			seenIDs[id] = struct{}{}
			targets.ids = append(targets.ids, id)
			continue
		}
		if _, exists := seenURLs[source]; exists {
			continue
		}
		seenURLs[source] = struct{}{}
		targets.urls = append(targets.urls, source)
	}
	if len(targets.ids)+len(targets.urls) == 0 || len(targets.ids)+len(targets.urls) > maxDirectSearch {
		return directSearchTargets{}, ErrDiscoveryInvalid
	}
	return targets, nil
}

func isDirectID(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func directDouyinTarget(value string) (id string, source string, valid bool) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if host == "douyin.com" || host == "www.douyin.com" {
		if len(segments) == 2 && segments[0] == "video" {
			if segment, err := url.PathUnescape(segments[1]); err == nil && isDirectID(segment) {
				return segment, "", true
			}
		}
		return "", "", false
	}
	if host == "v.douyin.com" && strings.Trim(parsed.EscapedPath(), "/") != "" {
		return "", value, true
	}
	return "", "", false
}

func findAuthorWithClient(ctx context.Context, input dto.AuthorSearchInput, _ manualSearchClient) (dto.SearchResponse, error) {
	return dto.SearchResponse{}, ErrDouyinAuthorUnavailable
}

func normalizedSearchLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 30 {
		return 30
	}
	return value
}

func normalizedAuthorLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 20 {
		return 20
	}
	return value
}

func payloadData(payload map[string]any) map[string]any {
	if data, ok := payload["data"].(map[string]any); ok {
		return data
	}
	return payload
}

func typedSearchResults(items []map[string]any) []dto.SearchResult {
	results := make([]dto.SearchResult, 0, len(items))
	for _, item := range items {
		published := parseSearchTime(cleanString(item["published_at"]))
		result := dto.SearchResult{
			PlatformContentID: cleanString(item["platform_content_id"]),
			Title:             cleanString(item["title"]),
			Description:       cleanString(item["description"]),
			CoverURL:          cleanString(item["cover_url"]),
			SourceURL:         cleanString(item["source_url"]),
			AuthorID:          cleanString(item["author_id"]),
			AuthorSecUID:      cleanString(item["author_sec_uid"]),
			AuthorUID:         cleanString(item["author_uid"]),
			AuthorHomeURL:     cleanString(item["author_home_url"]),
			AuthorName:        cleanString(item["author_name"]),
			LikeCount:         int64Value(item["like_count"]),
			FavoriteCount:     int64Value(item["favorite_count"]),
			ViewCount:         int64Value(item["view_count"]),
			CommentCount:      int64Value(item["comment_count"]),
			ShareCount:        int64Value(item["share_count"]),
			PublishedAt:       published,
		}
		if raw := item["raw"]; raw != nil {
			result.Raw = mustJSON(raw)
		}
		if result.PlatformContentID != "" && result.SourceURL != "" {
			results = append(results, result)
		}
	}
	return results
}

func cleanString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func parseSearchTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func (s *contentService) importResults(actor identityservice.PublicUser, input dto.ImportResultsRequest) (dto.ImportResultsResponse, error) {
	team, err := s.scope(actor, input.TeamID)
	if err != nil {
		return dto.ImportResultsResponse{}, err
	}
	input.Platform = strings.TrimSpace(input.Platform)
	input.SourceType = strings.TrimSpace(input.SourceType)
	if input.SourceType == "" {
		input.SourceType = "search"
	}
	if team == nil || input.Platform != "douyin" || (input.SourceType != "search" && input.SourceType != "author") || len(input.Items) == 0 || len(input.Items) > maxImportResults {
		return dto.ImportResultsResponse{}, ErrDiscoveryInvalid
	}
	normalized := make([]dto.SearchResult, len(input.Items))
	for index, item := range input.Items {
		value, valid := normalizeImportedResult(item)
		if !valid {
			return dto.ImportResultsResponse{}, ErrDiscoveryInvalid
		}
		normalized[index] = value
	}

	response := dto.ImportResultsResponse{Items: make([]dto.ImportItemResult, 0, len(normalized))}
	for _, item := range normalized {
		raw := item.Raw
		if len(raw) == 0 || !json.Valid(raw) {
			raw, _ = json.Marshal(item)
		}
		created, createErr := s.createSource(actor, dto.SourceInput{
			TeamID:            team,
			Platform:          input.Platform,
			PlatformContentID: item.PlatformContentID,
			Title:             item.Title,
			Description:       item.Description,
			CoverURL:          item.CoverURL,
			SourceURL:         item.SourceURL,
			AuthorID:          item.AuthorID,
			AuthorSecUID:      item.AuthorSecUID,
			AuthorUID:         item.AuthorUID,
			AuthorHomeURL:     item.AuthorHomeURL,
			AuthorName:        item.AuthorName,
			SourceType:        input.SourceType,
			LikeCount:         item.LikeCount,
			FavoriteCount:     item.FavoriteCount,
			ViewCount:         item.ViewCount,
			CommentCount:      item.CommentCount,
			ShareCount:        item.ShareCount,
			PublishedAt:       item.PublishedAt,
			RawJSON:           raw,
		})
		result := dto.ImportItemResult{PlatformContentID: item.PlatformContentID}
		switch {
		case createErr == nil:
			response.Imported++
			result.Status = "imported"
			result.SourceID = created.ID
		case errors.Is(createErr, ErrDuplicate):
			response.Duplicate++
			result.Status = "duplicate"
		case errors.Is(createErr, ErrForbidden), errors.Is(createErr, ErrInvalidInput):
			return dto.ImportResultsResponse{}, createErr
		default:
			response.Failed++
			result.Status = "failed"
			result.Message = "content import failed"
		}
		response.Items = append(response.Items, result)
	}
	return response, nil
}

func normalizeImportedResult(item dto.SearchResult) (dto.SearchResult, bool) {
	item.PlatformContentID = strings.TrimSpace(item.PlatformContentID)
	item.SourceURL = strings.TrimSpace(item.SourceURL)
	if utf8.RuneCountInString(item.PlatformContentID) > 191 || utf8.RuneCountInString(item.SourceURL) > 2048 {
		return dto.SearchResult{}, false
	}
	item.Title = limitedString(item.Title, 500)
	item.Description = limitedString(item.Description, 5000)
	item.CoverURL = limitedString(item.CoverURL, 2048)
	item.AuthorID = limitedString(item.AuthorID, 191)
	item.AuthorSecUID = limitedString(item.AuthorSecUID, 255)
	item.AuthorUID = limitedString(item.AuthorUID, 255)
	item.AuthorHomeURL = limitedString(item.AuthorHomeURL, 2048)
	item.AuthorName = limitedString(item.AuthorName, 255)
	if item.PublishedAt != nil {
		value := item.PublishedAt.UTC()
		item.PublishedAt = &value
	}
	return item, validDouyinSource(item.PlatformContentID, item.SourceURL)
}

func limitedString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func validDouyinSource(contentID, source string) bool {
	if contentID == "" || source == "" {
		return false
	}
	parsed, err := url.ParseRequestURI(source)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "douyin.com" && !strings.HasSuffix(host, ".douyin.com") {
		return false
	}
	for _, segment := range strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/") {
		value, err := url.PathUnescape(segment)
		if err == nil && value == contentID {
			return true
		}
	}
	return false
}
