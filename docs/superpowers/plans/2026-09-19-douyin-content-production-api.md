# Douyin Content Production API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make keyword search, author `sec_uid` video listing, and batch video detail lookup use the three approved Douyin provider APIs end to end.

**Architecture:** All provider HTTP protocol and authentication remain in the Douyin infra client. Content-pool services normalize typed payloads and preserve synchronous manual search plus existing URL task execution. Vue calls the existing Cloud API contracts with cursor metadata and correct `sec_uid` copy.

**Tech Stack:** Go, CloudWeGo Hertz, existing `encoding/json` and `net/http`; Vue 3, TDesign, Vitest.

**Spec:** `docs/superpowers/specs/2026-09-19-douyin-content-production-api-design.md`

## Global Constraints

- Cloud directly calls public Douyin APIs; Desktop Agent is not introduced.
- Provider HTTP protocol stays only in `internal/infra/client/platforms/douyin`.
- Manual keyword/author searches remain synchronous and do not write content-pool rows on search.
- API Key and cookie stay separated in `config/credentials/douyin.yaml`; real secrets are never committed or printed.
- Preserve all unrelated workspace changes and do not modify `web/dist-*`.
- Do not add MQ, microservices, plugins, workflow engines, repositories, or background tasks.
- Keep HTTP response envelopes, routes, error-code semantics, database structure, and Cloud–Agent contracts unchanged.

---

### Task 1: Correct and extend the Douyin typed client

**Files:**
- Modify: `internal/infra/client/platforms/douyin/client.go`
- Test: `internal/infra/client/platforms/douyin/client_test.go`

**Interfaces:**
- Produces: `SearchRequest{Keyword string; Count int; Offset int}`
- Produces: `FindAuthorRequest{SecUID string; Count int; MaxCursor int64}`
- Produces: `FetchByIDsRequest{IDs []string}` and `FetchByIDs(context.Context, FetchByIDsRequest) (map[string]any, error)`

Steps:
- [ ] Replace old keyword request assertions with `/v2dysearchvideo`, `keywords/count/offset`, and `ck`.
- [ ] Replace old author assertions with `/v5/dyhome`, `sec_uid/count/max_cursor`, and `ck`.
- [ ] Add failing tests for batch `/batchDyVideo`, comma-joined IDs, deduplication, empty rejection, and ten-ID rejection.
- [ ] Run the client package and verify the new tests fail before implementation.
- [ ] Implement request models, provider parameter construction, endpoint changes, and `FetchByIDs`.
- [ ] Re-run `go test ./internal/infra/client/platforms/douyin` and confirm it passes.

### Task 2: Normalize provider payloads and cursors in content services

**Files:**
- Modify: `internal/modules/contentpool/dto/dto.go`
- Modify: `internal/modules/contentpool/service/discovery_crawler.go`
- Modify: `internal/modules/contentpool/service/manual_search.go`
- Test: `internal/modules/contentpool/service/discovery_crawler_test.go`
- Test: `internal/modules/contentpool/service/manual_search_test.go`

**Interfaces:**
- Consumes: Task 1 typed client requests.
- Produces: `SearchResponse{Items []SearchResult; NextOffset int64; MaxCursor int64; HasMore bool}` with JSON `next_offset`, `max_cursor`, and `has_more`.
- Produces: crawler helpers that normalize keyword data, author data, and array-valued batch data.

Steps:
- [ ] Add failing client-stub tests proving keyword and author requests use `count`, `offset`, and `max_cursor`.
- [ ] Add failing tests proving response cursors and `has_more` are surfaced.
- [ ] Add failing batch-response tests proving `data: []` normalizes like `aweme_list`.
- [ ] Implement DTO metadata and manual search mapping without changing routes or envelopes.
- [ ] Update crawler request construction and normalization helpers.
- [ ] Run `go test ./internal/modules/contentpool/service` and confirm it passes.

### Task 3: Use batch detail lookup for numeric ID URL discovery

**Files:**
- Modify: `internal/modules/contentpool/service/discovery_crawler.go`
- Test: `internal/modules/contentpool/service/discovery_crawler_test.go`

**Interfaces:**
- Consumes: `FetchByIDs`.
- Produces: URL discovery behavior that groups numeric IDs into batches of ten and leaves short URLs on `FetchByURL`.

Steps:
- [ ] Add a failing test with more than ten mixed numeric IDs and one short URL, asserting provider batch chunking and short-URL fallback.
- [ ] Implement numeric classification, trimming, de-duplication, ten-ID chunking, and partial-error accounting.
- [ ] Preserve successful items when another request fails.
- [ ] Run the contentpool service tests and confirm they pass.

### Task 4: Align handler contracts and content-pool frontend

**Files:**
- Modify: `internal/modules/contentpool/handler.go`
- Modify: `web/src/modules/contentpool/pages/ContentPoolPage.vue`
- Test: `web/src/modules/contentpool/pages/ContentPoolPage.test.js`

**Interfaces:**
- Consumes: Task 2 response metadata.
- Produces: author request JSON field `max_cursor`.
- Produces: frontend keyword next-page offset and author next-page `max_cursor`.

Steps:
- [ ] Add frontend source-contract tests for `sec_uid` copy, cursor state, and cursor-bearing requests.
- [ ] Extend the handler request with `max_cursor` and pass it to service input.
- [ ] Add result pagination controls while retaining selected-result import behavior.
- [ ] Run `npm --prefix web test -- ContentPoolPage` and confirm the updated test passes.

### Task 5: Configure local credentials and run real provider smoke checks

**Files:**
- Modify locally only: `config/credentials/douyin.yaml`

**Interfaces:**
- Consumes API values from `/Users/aqiuye/Develop/workspace/wt-media-flow/src/infra/http/douyin_api.py`.

Steps:
- [ ] Parse the existing Python constants without printing their values.
- [ ] Write them to local credential YAML.
- [ ] Run small guarded calls against keyword search, author videos, and batch details.
- [ ] Record only status/result-shape outcomes, never credentials or raw user data.

### Task 6: Full verification and architecture review

**Files:**
- Verify all modified runtime files and tests.

Steps:
- [ ] `gofmt` modified Go files.
- [ ] `go test ./internal/infra/client/platforms/douyin ./internal/modules/contentpool`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `npm --prefix web test`
- [ ] Run repository architecture-boundary scan for forbidden packages and direct business HTTP construction.
- [ ] Inspect `git status` and exact diffs; ensure no secret and no built `web/dist-*` changes belong to this feature.
