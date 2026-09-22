# Content Pool Visual and Product Closure Design

- Date: 2026-09-22
- Status: Approved
- Approved approach: Option B — content-pool experience closure
- Related implementation plan: to be created after this specification is reviewed

## Background

The content-pool module already has the core persistence and service loop:

```text
discovery strategy
→ crawl task
→ source content
→ material conversion
→ material link
```

The current page, however, still presents this loop as a CRUD table. Its detail panel is a narrow, one-column field list, source relationships are displayed as raw IDs, and review mode advances even when an operation fails. The result is technically functional but not yet an operator-facing work surface.

This design upgrades the content pool into a readable decision workspace without redesigning the domain model or introducing a second content object.

## Current-State Findings

### Existing capabilities

The current implementation already provides:

- Content-pool list, detail, status update, materialization, and batch operations.
- Manual URL import, synchronous keyword search, and selected-result import.
- Strategy and crawl-task pages.
- `source_contents.strategy_id` and `source_contents.crawl_task_id`.
- `source_contents.material_id`.
- Automatic materialization thresholds and task snapshots.
- Failed-item retry through a new child crawl task.
- A material-library projection that reuses the content-pool page and filters by `material_created`.

### Product gaps

1. The detail drawer is only `520px` wide and renders a one-column descriptions table.
2. Cover, title, author, metrics, status, and source context have no visual hierarchy.
3. Strategy and task are displayed as `#ID`, not as business names.
4. Review mode does not expose progress such as `1 / 10`.
5. Review actions swallow failures and advance to the next item even after an error.
6. Converted content has `material_id`, but the page does not provide a truthful material navigation path.
7. The current model has no stable game or author-home fields, so those attributes must not be fabricated.

## Approved Product Direction

The content pool is the operator's content-decision center. It must answer:

- What content did the system discover?
- Which items need action?
- Why did this content enter the system?
- What happened after processing?
- Where did it come from and where did it go?

It must not become a second strategy manager, task executor, or material system.

## Scope

### 1. Content list

Upgrade the current table into an operator work list.

Required changes:

- Show a compact cover thumbnail in the title cell when `cover_url` exists.
- Keep title as the primary text and platform content ID as secondary text.
- Display strategy name instead of `strategy #ID`.
- Display task name instead of `task #ID`.
- Keep like count, favorite count, status, source type, and timestamps.
- Keep operations pinned to the right.
- Preserve current filters and existing API response envelope.
- Keep the read-only material-library projection on the same page.

The list remains a table. This design does not introduce a card wall or a second data object.

### 2. Source read model

Add a read-model projection for content-pool list and detail responses. The projection must include the existing source fields plus:

- `strategy_name`
- `crawl_task_name`

The task name should use the historical task snapshot when available, for example:

```text
三角洲热点 · 2026-09-21 14:30
```

The historical task snapshot is authoritative. If the strategy is renamed later, old tasks and their source records must continue to display the name captured at execution time.

The repository must use explicit SQL and may join the existing tables:

- `source_contents`
- `discovery_strategies`
- `crawl_tasks`

No new table or column is required.

### 3. Detail workspace

Replace the current narrow field-list drawer with a large detail workspace.

Layout:

```text
┌───────────────────────────────────────────────┐
│ Title / status / platform / author / time      │
├──────────────────────┬────────────────────────┤
│ Content preview      │ Source trace            │
│ - cover              │ - source type           │
│ - open original      │ - strategy name         │
│ - description        │ - task name             │
│                      │ - discovery time        │
│                      ├────────────────────────┤
│                      │ Processing result       │
│                      │ - status                │
│                      │ - material link         │
│                      │ - audit note            │
│                      │ - failure reason        │
├──────────────────────┴────────────────────────┤
│ Open original / view material / convert / ignore │
└───────────────────────────────────────────────┘
```

Drawer sizing:

- Desktop: approximately `72vw`, capped around `1200px`.
- Small desktop: no less than about `960px` where possible.
- Mobile or narrow viewport: full width.
- The action area remains visible at the bottom.

Information priority:

1. Title, cover, status, author, source URL, and core metrics.
2. Strategy, task, source type, published time, and discovery time.
3. Platform content ID, audit note, failure reason, and material ID.

The page must not embed the original platform in an iframe. External sites commonly block embedding, and this project must not add a proxy service for it. The truthful preview is the stored cover plus an explicit external link.

### 4. Source navigation

Strategy link:

```text
/crawl-tasks?strategy_id={strategy_id}
```

Task link:

```text
/crawl-tasks?task_id={crawl_task_id}
```

The task page already supports opening a task from `task_id` and filtering tasks by `strategy_id`.

Original content link:

- Use `source_url`.
- Open in a new tab.
- Do not fabricate an author-home URL.

### 5. Material navigation

For content with `material_id`, expose a `查看素材` action.

Use the existing material-library projection:

```text
/material-library?material_id={material_id}
```

Extend the existing content-pool list filter with `material_id`.

This is a minimal, truthful navigation path. It does not claim to be a full material-detail page and does not create a second material object.

### 6. Review mode

Review mode must become a continuous, error-safe workflow.

State:

- `pendingTotal`
- `currentIndex`
- `reviewNote`
- `reviewError`
- `reviewProcessing`

Display:

```text
待审核 10
当前 1 / 10
```

Actions:

- `转素材并下一条`
- `忽略并下一条`
- `跳过` after a failure
- `重新转素材` after a materialization failure

Required behavior:

1. Enter review mode only from pending content.
2. Disable the entry action when there is no pending content.
3. Show the current item and total pending count.
4. Perform only the selected action.
5. Update the current row locally from the API response.
6. Advance only after the action succeeds.
7. On failure, stay on the current item.
8. On failure, show the API error inside the review panel.
9. Do not silently continue to the next item.
10. Allow retry without losing the current note.
11. Allow skip without changing the item's status.
12. After the final item, close review mode and refresh the list.

The action helpers used by normal list operations may keep their existing user-facing behavior, but review mode must use an error-safe path that can distinguish success from failure.

### 7. Batch feedback

Preserve independent per-item batch semantics.

After a batch operation:

- Show succeeded and failed counts.
- Keep the existing response envelope.
- Do not roll back successful items when another item fails.
- Do not introduce a new task center or workflow.

## API Contract

Keep the existing routes and response envelope.

Extend the content-pool list query with:

```text
material_id={id}
```

Extend list and detail responses with source-read fields:

```json
{
  "strategy_name": "三角洲热点",
  "crawl_task_name": "三角洲热点 · 2026-09-21 14:30"
}
```

These are read-model fields. They do not change persistence ownership or introduce a migration.

## Architecture

This design stays inside the existing modular-monolith boundary.

- Cloud remains the business fact center.
- No microservice, MQ, workflow engine, dynamic plugin, or second task platform is introduced.
- Handler code does not execute SQL.
- Repository code remains the only SQL entry point.
- Service code owns business rules and authorization.
- Existing API error codes and envelope remain unchanged.
- No production background task is started from the page.
- The page continues to use the shared resource layout and TDesign components.
- No `web/dist-*` artifact is modified.

## Non-Goals

This round explicitly does not build:

- A full material-detail page.
- A game taxonomy or game field.
- An author-home field or inferred author URL.
- AI scoring or hot-content scoring.
- Multi-level review, assignment, or approval.
- A comment system.
- A content version system.
- A second content object.
- A generic workflow engine.
- An iframe proxy.
- Backend pagination or aggregate statistics.
- A broad visual redesign outside the content-pool page.

Game and author-home capabilities remain documented gaps until a real data source and product decision exist.

## Testing

### Backend

Focused tests must cover:

1. Source list returns strategy and task names.
2. Source detail returns strategy and task names.
3. Historical task snapshot takes precedence over the current strategy name.
4. `material_id` filtering works.
5. Existing source filters remain correct.
6. Authorization and team scoping remain correct.
7. Materialization and status transitions remain unchanged.

Repository tests must continue using `go-sqlmock` and must not depend on real MySQL.

### Frontend

Focused tests must cover:

1. The detail drawer uses the large workspace layout.
2. Cover and external link are rendered when available.
3. Strategy and task names are displayed instead of raw IDs.
4. Material link uses `/material-library?material_id=...`.
5. Review mode exposes pending total and current index.
6. Review mode does not advance on failure.
7. Review mode advances after success.
8. Skip does not change status.
9. The material-library projection remains read-only.

### Verification commands

Run focused tests first:

```bash
go test ./internal/modules/contentpool/...
cd web && npm test
```

Then run the full checks:

```bash
go test ./...
go vet ./...
```

Also run the repository architecture-boundary scan and review the final diff.

## Real-Data Acceptance

Use real records to verify:

1. A pending item can be opened in the large detail panel.
2. Cover, title, author, metrics, and source URL are visible.
3. A strategy-discovered item shows the strategy name.
4. The source task shows its historical name and time.
5. Clicking strategy opens the filtered task list.
6. Clicking task opens the task detail.
7. Materialized content exposes `查看素材`.
8. The material link filters the material library to the expected record.
9. Review mode shows `current / total`.
10. A successful conversion advances to the next item.
11. A failed conversion stays on the current item and shows the error.
12. Ignore succeeds and advances.
13. The final action exits review mode and refreshes the list.

## Implementation Slices

1. Add the source read-model projection and repository query.
2. Extend list/detail service and handler responses.
3. Add `material_id` filtering.
4. Upgrade the content list.
5. Upgrade the detail drawer.
6. Repair review-mode state and error handling.
7. Add navigation actions.
8. Add focused backend and frontend tests.
9. Run full verification and real-data walkthrough.
