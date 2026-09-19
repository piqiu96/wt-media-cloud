package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

const maxImportResults = 100

type manualSearchClient interface {
	Configured() bool
	Search(context.Context, douyinclient.SearchRequest) (map[string]any, error)
	FindAuthor(context.Context, douyinclient.FindAuthorRequest) (map[string]any, error)
}

func douyinClient() manualSearchClient { return douyinclient.Get() }

func searchWithClient(ctx context.Context, input dto.SearchInput, client manualSearchClient) (dto.SearchResponse, error) {
	input.Platform = strings.TrimSpace(input.Platform)
	input.Keyword = strings.TrimSpace(input.Keyword)
	if input.Platform != "douyin" || input.Keyword == "" || input.Offset < 0 {
		return dto.SearchResponse{}, ErrDiscoveryInvalid
	}
	if client == nil || !client.Configured() {
		return dto.SearchResponse{}, ErrCrawlerUnavailable
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
			AuthorName:        cleanString(item["author_name"]),
			PublishedAt:       published,
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
		raw, _ := json.Marshal(item)
		created, createErr := s.createSource(actor, dto.SourceInput{
			TeamID:            team,
			Platform:          input.Platform,
			PlatformContentID: item.PlatformContentID,
			Title:             item.Title,
			Description:       item.Description,
			CoverURL:          item.CoverURL,
			SourceURL:         item.SourceURL,
			AuthorID:          item.AuthorID,
			AuthorName:        item.AuthorName,
			SourceType:        input.SourceType,
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
