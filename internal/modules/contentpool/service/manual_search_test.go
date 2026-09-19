package service

import (
	"context"
	"errors"
	"testing"

	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type searchClientStub struct {
	searchCalls int
	authorCalls int
}

func (*searchClientStub) Configured() bool { return true }
func (s *searchClientStub) Search(_ context.Context, _ douyinclient.SearchRequest) (map[string]any, error) {
	s.searchCalls++
	return map[string]any{"data": map[string]any{"aweme_list": []any{map[string]any{"aweme_id": "keyword-1", "desc": "关键词结果"}}}}, nil
}
func (s *searchClientStub) FindAuthor(_ context.Context, _ douyinclient.FindAuthorRequest) (map[string]any, error) {
	s.authorCalls++
	return map[string]any{"data": map[string]any{"aweme_detail": map[string]any{"aweme_id": "author-1", "desc": "博主结果"}}}, nil
}

func TestKeywordSearchReturnsItemsWithoutCreatingTask(t *testing.T) {
	client := &searchClientStub{}
	result, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Keyword: "游戏"}, client)
	if err != nil || client.searchCalls != 1 || len(result.Items) != 1 || result.Items[0].PlatformContentID != "keyword-1" {
		t.Fatalf("result=%+v calls=%d err=%v", result, client.searchCalls, err)
	}
}

func TestAuthorSearchReturnsItemsWithoutCreatingTask(t *testing.T) {
	client := &searchClientStub{}
	result, err := findAuthorWithClient(context.Background(), dto.AuthorSearchInput{Platform: "douyin", Author: "作者"}, client)
	if err != nil || client.authorCalls != 1 || len(result.Items) != 1 || result.Items[0].PlatformContentID != "author-1" {
		t.Fatalf("result=%+v calls=%d err=%v", result, client.authorCalls, err)
	}
}

func TestImportResultsRejectsInvalidPlatformOrSource(t *testing.T) {
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	service := newContentService(newMemoryStore())
	requests := []dto.ImportResultsRequest{
		{Platform: "bilibili", Items: []dto.SearchResult{{PlatformContentID: "a1", SourceURL: "https://www.douyin.com/video/a1"}}},
		{Platform: "douyin", Items: []dto.SearchResult{{PlatformContentID: "a1", SourceURL: "https://example.com/video/a1"}}},
		{Platform: "douyin"},
	}
	for _, request := range requests {
		if _, err := service.importResults(actor, request); !errors.Is(err, ErrDiscoveryInvalid) {
			t.Fatalf("request=%+v err=%v", request, err)
		}
	}
}

func TestImportResultsUsesExistingContentPoolDeduplication(t *testing.T) {
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	store := newMemoryStore()
	service := newContentService(store)
	request := dto.ImportResultsRequest{Platform: "douyin", Items: []dto.SearchResult{
		{PlatformContentID: "a1", Title: "first", SourceURL: "https://www.douyin.com/video/a1"},
		{PlatformContentID: "a1", Title: "duplicate", SourceURL: "https://www.douyin.com/video/a1"},
	}}
	result, err := service.importResults(actor, request)
	if err != nil || result.Imported != 1 || result.Duplicate != 1 || len(store.items) != 1 {
		t.Fatalf("result=%+v stored=%d err=%v", result, len(store.items), err)
	}
}
