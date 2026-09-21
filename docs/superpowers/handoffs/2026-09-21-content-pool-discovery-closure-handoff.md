# Content Pool Discovery Closure Handoff

- Date: 2026-09-21
- Status: Complete
- Plan: `docs/superpowers/plans/2026-09-21-content-pool-discovery-closure.md`
- Spec: `docs/superpowers/specs/2026-09-21-content-pool-discovery-closure-design.md`
- Verified at: 2026-09-21 23:13 CST

## Scope

This closure links discovery strategies and crawl-task results to the content pool, enables threshold-based automatic material creation, adds batch review operations, and makes failed crawl-task items retryable. The implementation intentionally excludes author/blogger discovery and a combinable rule engine.

## Implementation Summary

### Backend

- Module: `internal/modules/contentpool`
- Areas changed:
  - DTO, model, router, handler
  - Service operations and discovery processing
  - Repository filters and MySQL storage
- Migration: `migrations/20260921_032_content_pool_discovery_link.sql`
- HTTP client infrastructure:
  - `pkg/clients/http/client.go`
  - `pkg/clients/http/endpoint.go`
  - `pkg/clients/http/client_test.go`

### Frontend

- `web/src/modules/contentpool/pages/ContentPoolPage.vue`
- `web/src/modules/contentpool/pages/DiscoveryStrategiesPage.vue`
- `web/src/modules/contentpool/pages/CrawlTasksPage.vue`
- `web/src/shared/api/contentPool.js`
- `web/src/shared/api/discovery.js`

### Tests

- `internal/modules/contentpool/**`
- `web/src/modules/contentpool/pages/ContentPoolPage.test.js`
- `web/src/modules/contentpool/pages/DiscoveryPages.test.js`
- `pkg/clients/http/client_test.go`

## Data Model

The `source_contents` table now carries the discovery lineage and review facts:

- `strategy_id`: originating discovery strategy
- `crawl_task_id`: originating crawl task
- `like_count`: source like count
- `favorite_count`: source favorite count
- `audit_note`: operator audit note
- `failure_reason`: item failure reason
- `material_id`: generated material ID

Relationships:

- One discovery strategy has many crawl tasks.
- One crawl task has many source contents.
- One source content has zero or one material.
- Crawl-task snapshots remain immutable and preserve historical strategy configuration.
- Crawl-task results preserve per-item processing status, source ID, material ID, and failure reason.

## Product Capabilities

### Content Pool

- List and filter by `strategy_id` and `crawl_task_id`.
- Review pending items.
- Materialize a single item.
- Batch update status.
- Batch materialize items; each item is processed independently.
- Open detail view and navigate back to the originating task or strategy.

### Discovery Strategy

- Configure `auto_material`.
- Choose `AND` or `OR` threshold rules.
- Configure like threshold and favorite threshold.
- View latest execution status and result summary.
- Navigate to task list and content-pool filter.
- Author strategy remains disabled while the external author API is unavailable.

### Crawl Task

- View immutable strategy snapshot.
- View automatic-material rule and thresholds.
- View per-item processing results.
- Link successful results to content-pool records.
- Retry failed items without reprocessing successful items.

### Automatic Material Rule

Only two rule modes are supported:

- `AND`: like threshold and favorite threshold must both pass.
- `OR`: either threshold passes.

Material conversion always uses the unified materialize service. No second material-conversion path was introduced.

## API Surface

- `POST /api/v1/content-pool/batch/status`
- `POST /api/v1/content-pool/batch/materialize`
- `POST /api/v1/crawl-tasks/:id/retry-failed`

Existing content-pool and crawl-task list/detail routes remain unchanged.

## HTTPS Client Fix

The first real keyword run failed with:

```text
douyin keyword discovery failed: not support tls
```

Root cause:

- Hertz used the default Netpoll dialer, which did not support TLS in this environment.
- The Douyin endpoint is HTTPS.

Fix:

- HTTPS Hertz clients now configure TLS with the endpoint host as `ServerName` and TLS 1.2 as the minimum version.
- The custom endpoint dialer now delegates to the Hertz standard dialer so direct and load-balanced HTTPS endpoints support TLS.

Commit: `fc10ab5 fix: enable TLS for Hertz HTTPS clients`

## Verification Results

### Focused Backend Tests

Passed:

```text
go test ./pkg/clients/http
go test ./internal/infra/client/platforms/douyin
go test ./internal/modules/contentpool/...
```

### Focused Frontend Tests

Passed:

```text
npx vitest run \
  src/modules/contentpool/pages/ContentPoolPage.test.js \
  src/modules/contentpool/pages/DiscoveryPages.test.js
```

Result: 2 test files, 8 tests passed.

### Frontend Build

Passed:

```text
npm run build
```

Known pre-existing warning: one Vite chunk exceeds 500 kB. This is not a blocker for this closure.

### Migration

Passed:

```text
migration ok: 1 applied, 33 total
applied 20260921_032_content_pool_discovery_link
```

### Real Data Loop

Verified strategy and task:

- Strategy ID: `3`
- Strategy name: `自动素材验证-0921`
- Keyword: `王者荣耀`
- Rule: `AND`
- Like threshold: `10000`
- Favorite threshold: `500`
- Crawl task ID: `23`

Task 23 result:

- Status: `success`
- Found: `20`
- Added: `19`
- Duplicate: `1`
- Failed: `0`
- Auto-materialized: `16`
- Pending review: `3`

Database readback from `wt_media_cloud`:

```text
crawl_tasks:
  id=23, status=success, strategy_id=3,
  material_rule=AND, like_threshold=10000, favorite_threshold=500

source_contents:
  total=19
  material_created=16
  pending=3
  linked material count=16
  distinct material count=16

materials:
  joined material count=16
```

Sample source rows:

```text
id=84, status=material_created, like_count=16620, favorite_count=1782, material_id=1
id=85, status=material_created, like_count=11273, favorite_count=557, material_id=2
id=86, status=material_created, like_count=77481, favorite_count=1708, material_id=3
```

The content-pool API filtered by `strategy_id=3` returned 19 records.

## Retry Semantics

- Retry only targets failed result items.
- Successful items are not reprocessed.
- Each item is processed independently.
- The verified retry created 19 unique source rows, detected one duplicate, created 16 materials, and left 3 items pending review.
- No fake success state was reported.

## Known Limitations

- Author/blogger discovery remains disabled until the external API is reopened.
- Only `AND` and `OR` threshold modes are supported; no combinable rule engine was added.
- Verification was focused on the core loop, not a full integration sweep.
- Real keyword discovery depends on the configured Douyin API and credentials.
- Douyin API key and cookie configuration was intentionally preserved per product decision.

## Runtime

At handoff, the local runtime was active:

- Backend: `http://127.0.0.1:18080`
- Web: `http://127.0.0.1:5176`
- Discovery worker: `go run ./cmd/discovery-worker`
