package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type searchClientStub struct {
	searchCalls     int
	authorCalls     int
	fetchByURLCalls int
	fetchByIDsCalls int
	searchReq       douyinclient.SearchRequest
	authorReq       douyinclient.FindAuthorRequest
	fetchByURLReq   douyinclient.FetchByURLRequest
	fetchByIDsReqs  []douyinclient.FetchByIDsRequest
}

func (*searchClientStub) Configured() bool { return true }
func (s *searchClientStub) Search(_ context.Context, request douyinclient.SearchRequest) (map[string]any, error) {
	s.searchCalls++
	s.searchReq = request
	return map[string]any{"data": map[string]any{
		"datalist": []any{map[string]any{"aweme_info": map[string]any{
			"aweme_id": "keyword-1", "desc": "关键词结果",
			"author": map[string]any{"uid": "author-uid", "sec_uid": "author-sec-uid", "nickname": "关键词作者"},
			"statistics": map[string]any{
				"digg_count": 101, "collect_count": 102, "play_count": 103,
				"comment_count": 104, "share_count": 105,
			},
		}}},
		"cursor": float64(30), "has_more": float64(1),
	}}, nil
}
func (s *searchClientStub) FetchByURL(_ context.Context, request douyinclient.FetchByURLRequest) (map[string]any, error) {
	s.fetchByURLCalls++
	s.fetchByURLReq = request
	return map[string]any{"data": map[string]any{"aweme_detail": map[string]any{
		"aweme_id": "7681569320895352104", "desc": "链接结果",
		"author": map[string]any{"uid": "author-uid", "sec_uid": "author-sec-uid", "nickname": "链接作者"},
		"statistics": map[string]any{
			"digg_count": 201, "collect_count": 202, "play_count": 203,
			"comment_count": 204, "share_count": 205,
		},
	}}}, nil
}
func (s *searchClientStub) FetchByIDs(_ context.Context, request douyinclient.FetchByIDsRequest) (map[string]any, error) {
	s.fetchByIDsCalls++
	s.fetchByIDsReqs = append(s.fetchByIDsReqs, request)
	values := make([]any, 0, len(request.IDs))
	for _, id := range request.IDs {
		values = append(values, map[string]any{"aweme_info": map[string]any{
			"aweme_id": id, "desc": "视频 " + id,
			"author": map[string]any{"uid": "author-uid", "sec_uid": "author-sec-uid", "nickname": "视频作者"},
			"statistics": map[string]any{
				"digg_count": 101, "collect_count": 102, "play_count": 103,
				"comment_count": 104, "share_count": 105,
			},
		}})
	}
	return map[string]any{"data": values}, nil
}
func (s *searchClientStub) FindAuthor(_ context.Context, request douyinclient.FindAuthorRequest) (map[string]any, error) {
	s.authorCalls++
	s.authorReq = request
	return map[string]any{"data": map[string]any{
		"aweme_list": []any{map[string]any{"aweme_id": "author-1", "desc": "博主结果"}},
		"max_cursor": float64(456), "has_more": true,
	}}, nil
}

func TestKeywordSearchReturnsItemsWithoutCreatingTask(t *testing.T) {
	client := &searchClientStub{}
	result, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Keyword: "游戏", Limit: 15, Offset: 10}, client)
	if err != nil || client.searchCalls != 1 || len(result.Items) != 1 || result.Items[0].PlatformContentID != "keyword-1" {
		t.Fatalf("result=%+v calls=%d err=%v", result, client.searchCalls, err)
	}
	if client.searchReq.Keyword != "游戏" || client.searchReq.Count != 15 || client.searchReq.Offset != 10 {
		t.Fatalf("search request=%+v", client.searchReq)
	}
	item := result.Items[0]
	if item.AuthorID != "author-uid" || item.AuthorUID != "author-uid" || item.AuthorSecUID != "author-sec-uid" || item.AuthorName != "关键词作者" {
		t.Fatalf("author identity missing: %+v", item)
	}
	if item.AuthorHomeURL != "https://www.douyin.com/user/author-sec-uid?showSubTab=video&showTab=post" {
		t.Fatalf("author homepage missing: %+v", item)
	}
	if item.LikeCount != 101 || item.FavoriteCount != 102 || item.ViewCount != 103 || item.CommentCount != 104 || item.ShareCount != 105 {
		t.Fatalf("interaction metrics missing: %+v", item)
	}
	var raw map[string]any
	if err := json.Unmarshal(item.Raw, &raw); err != nil || raw["aweme_id"] != "keyword-1" {
		t.Fatalf("raw platform payload missing: raw=%s err=%v", item.Raw, err)
	}
	if result.NextOffset != 30 || !result.HasMore {
		t.Fatalf("search pagination=%+v", result)
	}
}

func TestDirectSearchFetchesNumericIDWithoutKeywordTask(t *testing.T) {
	client := &searchClientStub{}
	result, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Query: "7681569320895352104"}, client)
	if err != nil || client.searchCalls != 0 || client.fetchByIDsCalls != 1 || client.fetchByURLCalls != 0 {
		t.Fatalf("result=%+v err=%v search=%d ids=%d urls=%d", result, err, client.searchCalls, client.fetchByIDsCalls, client.fetchByURLCalls)
	}
	if len(client.fetchByIDsReqs) != 1 || len(client.fetchByIDsReqs[0].IDs) != 1 || client.fetchByIDsReqs[0].IDs[0] != "7681569320895352104" {
		t.Fatalf("fetch requests=%+v", client.fetchByIDsReqs)
	}
	if len(result.Items) != 1 || result.Items[0].PlatformContentID != "7681569320895352104" || result.Items[0].ViewCount != 103 {
		t.Fatalf("result=%+v", result)
	}
	if result.NextOffset != 0 || result.HasMore {
		t.Fatalf("direct search must not expose pagination: %+v", result)
	}
}

func TestDirectSearchExtractsIDFromDouyinVideoURL(t *testing.T) {
	client := &searchClientStub{}
	_, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Query: "https://www.douyin.com/video/7681569320895352104?from=share"}, client)
	if err != nil || client.fetchByURLCalls != 0 || client.fetchByIDsCalls != 1 || len(client.fetchByIDsReqs[0].IDs) != 1 || client.fetchByIDsReqs[0].IDs[0] != "7681569320895352104" {
		t.Fatalf("err=%v requests=%+v urlCalls=%d", err, client.fetchByIDsReqs, client.fetchByURLCalls)
	}
}

func TestDirectSearchUsesShortLinkFetch(t *testing.T) {
	client := &searchClientStub{}
	result, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Query: "https://v.douyin.com/demo/"}, client)
	if err != nil || client.fetchByURLCalls != 1 || client.fetchByIDsCalls != 0 {
		t.Fatalf("err=%v urlCalls=%d idCalls=%d", err, client.fetchByURLCalls, client.fetchByIDsCalls)
	}
	if client.fetchByURLReq.URL != "https://v.douyin.com/demo/" || len(result.Items) != 1 || result.Items[0].PlatformContentID != "7681569320895352104" {
		t.Fatalf("request=%+v result=%+v", client.fetchByURLReq, result)
	}
}

func TestDirectSearchDeduplicatesAndChunksIDs(t *testing.T) {
	client := &searchClientStub{}
	query := "7681569320895352100, 7681569320895352101\n7681569320895352102，7681569320895352103 7681569320895352104"
	for id := 7681569320895352105; id <= 7681569320895352114; id++ {
		query += " " + strconv.FormatInt(int64(id), 10)
	}
	query += " 7681569320895352100"
	result, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Query: query}, client)
	if err != nil || client.fetchByIDsCalls != 2 || len(result.Items) != 15 {
		t.Fatalf("err=%v calls=%d items=%d", err, client.fetchByIDsCalls, len(result.Items))
	}
	if len(client.fetchByIDsReqs[0].IDs) != 10 || len(client.fetchByIDsReqs[1].IDs) != 5 {
		t.Fatalf("batches=%+v", client.fetchByIDsReqs)
	}
}

func TestDirectSearchRejectsInvalidTargets(t *testing.T) {
	client := &searchClientStub{}
	for _, query := range []string{"hello", "https://example.com/video/7681569320895352104", "https://www.douyin.com/not-video/1"} {
		if _, err := searchWithClient(context.Background(), dto.SearchInput{Platform: "douyin", Query: query}, client); !errors.Is(err, ErrDiscoveryInvalid) {
			t.Fatalf("query=%q err=%v", query, err)
		}
	}
	if client.fetchByIDsCalls != 0 || client.fetchByURLCalls != 0 {
		t.Fatalf("invalid input must not call external API: ids=%d urls=%d", client.fetchByIDsCalls, client.fetchByURLCalls)
	}
}

func TestAuthorSearchIsTemporarilyUnavailable(t *testing.T) {
	client := &searchClientStub{}
	_, err := findAuthorWithClient(context.Background(), dto.AuthorSearchInput{Platform: "douyin", Author: "sec-author", Limit: 12, MaxCursor: 123}, client)
	if !errors.Is(err, ErrDouyinAuthorUnavailable) || client.authorCalls != 0 {
		t.Fatalf("err=%v authorCalls=%d", err, client.authorCalls)
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

func TestImportResultsPreservesRawPlatformPayload(t *testing.T) {
	team := identityservice.TeamID(7)
	actor := identityservice.PublicUser{ID: 2, Role: identityservice.RoleOperator, TeamID: &team}
	store := newMemoryStore()
	service := newContentService(store)
	raw := json.RawMessage(`{"aweme_id":"raw-1","desc":"raw result","statistics":{"play_count":999}}`)
	_, err := service.importResults(actor, dto.ImportResultsRequest{Platform: "douyin", Items: []dto.SearchResult{{
		PlatformContentID: "raw-1", Title: "raw result", SourceURL: "https://www.douyin.com/video/raw-1", Raw: raw,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := store.raws[1]
	if !ok || string(stored) != string(raw) {
		t.Fatalf("raw payload was not preserved: ok=%v raw=%s", ok, stored)
	}
}
