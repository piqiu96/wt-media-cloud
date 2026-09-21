# Content Pool Discovery Closure Design

## Status

Approved for implementation.

## Date

2026-09-21

## Background

M3 already provides three operational pages:

- 内容池 / Content Pool
- 挖掘策略 / Discovery Strategies
- 挖掘任务 / Crawl Tasks

The current implementation can search by keyword, import a Douyin link or video ID, create strategies, queue crawl tasks, and insert discovered results into the content pool. The remaining gap is the end-to-end operating loop:

```text
挖掘策略
→ 挖掘任务
→ 内容池
→ 转素材
→ material
```

This design completes that loop without introducing a new content object, a new task system, a workflow engine, or a combinable rule engine.

## Product Goals

### Content Pool

Content Pool is the operational result center. It answers:

- Which external contents have been discovered?
- Which contents still need processing?
- Why did each content enter the system?
- Was a content converted to material or ignored?

### Discovery Strategies

Discovery Strategies is the rule center. It answers:

- What should the system discover?
- How often should discovery run?
- Which contents should be automatically converted to material?
- What was the latest execution result?

### Crawl Tasks

Crawl Tasks is the execution record. It answers:

- When was a strategy executed?
- What strategy snapshot was used?
- How many contents were found?
- Which contents were automatically materialized?
- Which items failed and can be retried?

## Confirmed Decisions

1. Automatic material thresholds support exactly two fixed modes:
   - `AND`: all enabled threshold conditions must pass.
   - `OR`: any enabled threshold condition passes.
2. No combinable rule DSL and no rule engine.
3. This phase supports:
   - keyword search,
   - link or video ID import,
   - strategy-driven keyword discovery.
4. Author/blogger discovery remains disabled until the external API is available again.
5. Failed-item retry is P0.
6. Failed retry must not duplicate successful source contents or materials.
7. Historical task snapshots must remain immutable when a strategy is later edited.
8. Existing code is pre-release; implementation may move directly to the ideal state without compatibility shims.
9. Testing is limited to core business links; no exhaustive or broad integration testing is required.
10. Douyin API key and cookie configuration is intentionally retained and is not part of this cleanup.

## Scope

### In Scope

- Minimal schema extension for source-content traceability.
- Strategy configuration for automatic materialization.
- Task execution snapshots and per-item result records.
- Automatic material conversion through the existing materialize service.
- Batch materialize, ignore, and restore operations.
- Content detail and review mode.
- Failed-item retry.
- Three-page frontend interactions and cross-page links.

### Out of Scope

- Author/blogger monitoring.
- Arbitrary AND/OR combinations.
- AI scoring or popularity scoring.
- Approval workflow.
- A second content object.
- A new task platform.
- New platform abstraction.
- Full historical raw JSON duplication.
- Broad architecture refactoring.

## Data Design

### `source_contents`

Extend the existing table with the minimum traceability and processing fields:

- `strategy_id`: originating discovery strategy, nullable.
- `crawl_task_id`: originating crawl task, nullable.
- `like_count`: likes at discovery time.
- `favorite_count`: favorites at discovery time.
- `audit_note`: operator review note.
- `failure_reason`: material conversion failure reason.
- `material_id`: generated material ID, nullable.

Existing fields remain canonical:

- `platform`
- `platform_content_id`
- `source_url`
- `author_id`
- `author_name`
- `published_at`
- `source_type`
- `status`
- `raw_json`

The existing statuses remain:

- `pending`
- `material_created`
- `ignored`

A separate `material_failed` status is not introduced. Failed material conversion preserves the source row in `pending` and records `failure_reason`; the operator can retry.

### `discovery_strategies`

No new rule table is introduced. The existing `config_json` receives fixed fields:

```json
{
  "keyword": "三角洲热点",
  "auto_material": true,
  "material_rule": "AND",
  "like_threshold": 10000,
  "favorite_threshold": 500
}
```

Rules:

- `material_rule` is `AND` or `OR`.
- A threshold greater than zero participates in the rule.
- A threshold less than or equal to zero is ignored.
- At least one positive threshold is required when `auto_material` is true.
- `AND`: all positive thresholds must pass.
- `OR`: at least one positive threshold must pass.

### `crawl_tasks`

The existing `snapshot_json` stores the execution-time strategy snapshot:

- strategy ID,
- strategy name,
- strategy type,
- platform,
- discovery config,
- schedule,
- automatic material config and thresholds.

The existing `result_json` becomes the per-item result list. Each item records:

- platform content ID,
- title,
- author,
- like count,
- favorite count,
- source content ID,
- material ID,
- processing status,
- failure reason,
- whether it was automatically materialized.

Task stats are extended with:

- `auto_materialized`
- `pending`
- material/input failures where applicable.

Existing stats remain:

- `scanned`
- `found`
- `added`
- `duplicate`
- `failed`

### `materials`

No schema change is required. The material table remains the single material object and keeps its unique source-content relation.

## Service Rules

### Unified Material Conversion

All conversion paths must call the same content-pool materialize service:

- manual single conversion,
- batch conversion,
- strategy automatic conversion,
- retry conversion.

No page and no crawler may create a material row directly.

### Strategy Execution

1. Load the enabled strategy.
2. Copy the strategy name and config into a task snapshot.
3. Claim the pending task.
4. Call the existing Douyin crawler.
5. Deduplicate results by platform content ID.
6. Create or link each source-content row.
7. Record the source and task IDs in each result item.
8. Evaluate automatic materialization using the task snapshot.
9. Materialize qualifying contents through the unified service.
10. Process each item independently; one failure must not roll back other items.
11. Persist task status, stats, and per-item results.

### Failed-Item Retry

Retry only result items whose processing status is failed. Successful and duplicate items are not reprocessed. Already materialized items are not converted again. On retry:

1. Load the historical task.
2. Select only failed result items.
3. Re-run only the corresponding keyword/link/ID request.
4. Reuse the historical task snapshot, not the current strategy config.
5. Create source content only if it does not already exist.
6. Materialize only when the historical snapshot says it qualifies and the source is not already materialized.
7. Update the original task result and stats.

### Batch Operations

Batch materialize, ignore, and restore process each item independently. One failed item must not prevent other items from being processed. The response reports per-item outcomes.

## API Design

All APIs continue using the standard response envelope:

```json
{
  "errcode": 0,
  "message": "",
  "data": {},
  "logid": ""
}
```

New or extended routes:

- `POST /api/v1/content-pool/batch/materialize`
- `POST /api/v1/crawl-tasks/:id/retry-failed`

Existing routes remain:

- `GET /api/v1/content-pool`
- `GET /api/v1/content-pool/:id`
- `POST /api/v1/content-pool/:id/status`
- `POST /api/v1/content-pool/batch/status`
- `POST /api/v1/content-pool/:id/materialize`
- strategy CRUD and run routes,
- crawl task list/detail/confirm routes.

## Frontend Design

### Content Pool

- Header actions: import link, keyword search, enter review mode, refresh.
- Stat cards: all, pending, materialized, ignored.
- Filters: search, platform, source type, processing status.
- Table: cover, title/content ID, author, likes, favorites, source type, strategy, task, status, discovered time, actions.
- Batch operations: materialize, ignore, restore.
- Detail drawer: content metadata, source trace, processing state, audit note.
- Review mode: sequential pending review with “materialize and next” and “ignore and next”.

### Discovery Strategies

- Strategy form includes automatic material switch, AND/OR rule, like threshold, and favorite threshold.
- Table shows latest execution status and result summary.
- Actions include run, edit, enable/disable, task list, and content-pool filtering.
- Author strategy UI remains visible but disabled until the external API returns.

### Crawl Tasks

- Table shows task, strategy, platform, status, stats, and creation time.
- Detail drawer shows the immutable strategy snapshot, thresholds, stats, and result list.
- Result list shows each discovered content and its processing state.
- Failed tasks expose “retry failed items”.
- Result rows can link to the content-pool detail.

## Architecture Boundaries

- Module: `internal/modules/contentpool`.
- Handler stays in the module root and only parses HTTP requests.
- DTO stays in `internal/modules/contentpool/dto`.
- Business rules stay in `internal/modules/contentpool/service`.
- SQL stays in `internal/modules/contentpool/repository`.
- External Douyin protocol stays in `internal/infra/client/platforms/douyin`.
- Handlers and services must not execute SQL.
- Repositories must not import module DTOs.
- No business code may construct an HTTP client directly.
- Bootstrap remains the only production initializer for infrastructure resources.

## Verification

Core tests only:

1. `AND` threshold automatic materialization.
2. `OR` threshold automatic materialization.
3. Failed threshold remains pending.
4. Duplicate result does not create a second source or material.
5. Failed-item retry does not duplicate successful source/material rows.
6. Historical snapshot is not changed by strategy edits.
7. Focused content-pool service tests.
8. Focused frontend page/build validation.

Real-data validation should confirm at least the primary loop:

1. Create a strategy with automatic materialization.
2. Run it.
3. Verify source-content and material rows.
4. Open content pool and task detail.
5. Retry a failed item if the external API produces one.
