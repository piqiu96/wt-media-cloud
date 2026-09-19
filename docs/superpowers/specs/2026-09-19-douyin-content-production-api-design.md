# Douyin Content Production API Convergence Design

## Background

The content pool already has synchronous keyword and author search flows, a URL import flow, and a shared frontend page. The current Douyin client does not call the three target provider APIs required by content production:

- Author videos must use `POST /v5/dyhome`.
- Keyword videos must use `POST /v2dysearchvideo`.
- Batch video details must use `POST /batchDyVideo`.

The approved direction is Option B: align the full production chain while preserving the modular-monolith and synchronous-search boundaries.

## Goals

- Call the three target provider endpoints with their documented request fields.
- Keep all provider HTTP protocol code in `internal/infra/client/platforms/douyin`.
- Normalize keyword, author, single-video, and batch-video payloads into the existing content-pool search/import contracts.
- Update the frontend author input and pagination behavior to match the provider parameters.
- Support local real-API smoke testing with credentials read from `/Users/aqiuye/Develop/workspace/wt-media-flow/src/infra/http/douyin_api.py`.

## Non-Goals

- Do not introduce Agent execution for public Douyin data.
- Do not add a message queue, microservice, dynamic plugin, or workflow engine.
- Do not convert manual keyword or author search back into asynchronous crawl tasks.
- Do not redesign the content-pool data model or response envelope.
- Do not commit real credentials.

## API Contracts

### Keyword Search

`Search(ctx, SearchRequest)` calls `POST /v2dysearchvideo` with:

- Required `keywords`.
- Optional `count`, normalized to a positive bounded value.
- Optional `offset` for the provider cursor.
- Optional `ck` when a cookie is configured.

The client returns the provider JSON payload as-is. The service extracts and normalizes nested video items.

### Author Videos

`FindAuthor(ctx, FindAuthorRequest)` calls `POST /v5/dyhome` with:

- Required `sec_uid`.
- Optional `count`, normalized to 1–20 because the provider maximum is 20.
- Optional `max_cursor`; the next request cursor is taken from the prior response.
- Optional `ck` when a cookie is configured.

The request model and frontend copy must distinguish `sec_uid` from numeric UID and nickname.

### Batch Video Details

A typed batch method calls `POST /batchDyVideo` with:

- Required `ids`: trimmed, de-duplicated numeric or provider video IDs joined by commas.
- A maximum of 10 IDs per provider request.

The method accepts the array response and returns normalized service items. Short URLs continue through the existing single-item detail path; IDs and short URLs are not mixed in one provider request.

## Service Behavior

- `normalizeDouyinList` supports provider list shapes containing `data`, `aweme_list`, and nested `aweme_info`.
- Batch responses with `data` as an array are normalized without treating the array as a map.
- Keyword and author searches remain synchronous and do not write content-pool rows until the user submits selected results.
- URL import groups numeric IDs and uses batch requests, while short URLs retain per-URL detail requests.
- Search responses expose pagination metadata needed by the page without changing the existing HTTP response envelope.
- Existing import validation and content-pool deduplication remain authoritative.

## Frontend Behavior

- Keyword search displays results and preserves provider pagination metadata.
- Author search asks for a Douyin `sec_uid`, not an account name or numeric UID.
- Author pagination sends `max_cursor` from the last provider response.
- Selected-result import continues to call `/api/v1/content-pool/import-results`.
- URL import continues to accept multiple lines and reports the normal import outcome.

## Configuration

Provider transport remains in `config/clients/platforms/douyin.yaml` and credentials remain in `config/credentials/douyin.yaml`:

```yaml
api_key: "<local secret>"
cookie: "<local cookie>"
```

For implementation and smoke testing, values are copied from the existing `wt-media-flow` Python source named above. Real values are not printed in logs, specs, tests, or commits.

## Testing

- Client tests assert exact target paths, form fields, API-key query authentication, cookie form fields, limits, and error handling.
- Service tests cover provider payload shapes, normalization, cursor metadata, batch ID handling, and mixed numeric/short URL imports.
- Frontend tests cover `sec_uid` copy and cursor-based next-page requests.
- Real smoke checks call each provider API with local credentials before broader integration testing.
- Final verification runs focused Go and web tests, then `go test ./...`, `go vet ./...`, formatting, and architecture-boundary scans.

## Architecture Review

Cloud remains the business fact center and directly calls public Douyin APIs. Desktop Agent remains responsible only for operations requiring the local machine. No repository boundary, global-resource ownership, scheduler, or worker responsibility changes.
