# Content Pool Discovery Data Completeness and Interaction Design

- Date: 2026-09-22
- Status: Approved
- Approved approach: Option B — normalized discovery fields plus complete raw-result traceability
- Related implementation plan: to be created after this specification is reviewed

## Background

The content-pool workspace now presents source content as a review surface rather than a CRUD table. The next gap is data completeness and interaction precision:

1. The title is readable but does not jump to the real platform landing page.
2. Clickable table columns do not consistently use the primary link color.
3. Keyword-search results only retain the title, cover, author, source URL, and publication time. Interaction counts and complete platform payloads are discarded before import.
4. Author identity fields and the author homepage are not persisted.
5. Review-mode entry does not communicate its active state clearly.
6. Crawl-task display names use a human-readable date rather than the approved compact timestamp.
7. Manual URL import is now replaced by numeric content-ID search.
8. Cover images cannot be enlarged for inspection.

This design closes those gaps without changing the module boundary, response envelope, or Cloud–Agent contract.

## Product Direction

The content pool remains the operator's content-decision center. It must preserve the platform's original facts at ingestion time, expose the fields needed for decisions, and make external navigation obvious and truthful.

The implementation must not:

- Introduce a second content object.
- Add an iframe or proxy for the original platform.
- Move public-platform discovery to Desktop Agent.
- Introduce MQ, workflow, microservices, or a dynamic plugin system.
- Change existing routes, error codes, or API envelope semantics.

## Scope

### 1. Original-content navigation

The following title areas become links to the real platform landing page:

- Content-pool list title.
- Content detail title.
- Manual search result title.
- ID-search result title, if synchronous result preview is added later.

Rules:

- Use the stored `source_url`.
- Open in a new tab.
- Include `rel="noopener noreferrer"`.
- Render a normal non-clickable title when `source_url` is empty.
- Never fabricate a URL from the title or content ID.

The existing explicit `打开原作品` action remains available in the detail workspace.

### 2. Consistent clickable-column styling

All actionable text uses the same primary link treatment:

- Title.
- Author name when an author homepage exists.
- Source strategy.
- Source crawl task.
- `查看素材`.
- Manual search result title.
- Author homepage link in the detail panel.

Requirements:

- Use the design-system primary color.
- Provide recognizable hover and focus states.
- Do not make non-actionable data look clickable.
- Preserve text truncation and table layout stability.

### 3. Discovery data projection

`source_contents` currently has `like_count` and `favorite_count`. Add the following normalized fields:

```sql
view_count     BIGINT UNSIGNED NOT NULL DEFAULT 0
comment_count  BIGINT UNSIGNED NOT NULL DEFAULT 0
share_count    BIGINT UNSIGNED NOT NULL DEFAULT 0
author_sec_uid VARCHAR(255) NULL
author_uid     VARCHAR(255) NULL
author_home_url TEXT NULL
```

Mapping from the Douyin payload:

| Business field | Source field |
| --- | --- |
| `like_count` | `statistics.digg_count` |
| `favorite_count` | `statistics.collect_count` |
| `view_count` | `statistics.play_count` |
| `comment_count` | `statistics.comment_count` |
| `share_count` | `statistics.share_count` |
| `author_sec_uid` | `author.sec_uid` |
| `author_uid` | `author.uid` |
| `author_home_url` | generated from `author.sec_uid` or a validated returned homepage |

The existing `author_id` field remains for compatibility and stores the author `uid` for new records.

### 4. Author homepage format

When `author_sec_uid` exists, generate:

```text
https://www.douyin.com/user/{sec_uid}?showSubTab=video&showTab=post
```

Example:

```text
https://www.douyin.com/user/MS4wLjABAAAAHVBR5Z9ew2-JSheoj8Fg3XDZPEWgmKH477owHtfTaqk?showSubTab=video&showTab=post
```

If the platform response already supplies a homepage URL, validate and prefer that URL. Otherwise use the generated URL above. If no `sec_uid` or valid homepage exists, do not fabricate a clickable author link.

The detail workspace displays:

- Author name as an external link when available.
- `author_sec_uid`.
- `author_uid`.

### 5. Raw-result traceability

Continue using the existing `source_contents.raw_json` JSON field. Do not add a second JSON column.

For synchronous keyword search:

1. The Douyin normalizer preserves the complete normalized item and its nested raw platform object.
2. The search API returns the display fields and the raw object.
3. The frontend keeps the raw object with each selected result.
4. `import-results` passes it back to Cloud.
5. Cloud writes the complete raw object to `source_contents.raw_json`.

If a caller submits a result without the nested raw object, Cloud persists the complete normalized result instead. The field is never empty.

The strategy-discovery path already normalizes each result before creating source content and continues to persist the complete normalized item, including the nested raw object.

### 6. Manual ID search

Replace the existing `导入链接` entry with `ID搜索`.

Input:

- One or more Douyin content IDs.
- IDs may be separated by newline, comma, or whitespace.
- Only numeric IDs are accepted.
- Duplicate IDs are removed before submission.
- Empty submissions are rejected in the UI.

Execution:

- Reuse the existing manual URL-import task path because the crawler already treats numeric values as content IDs.
- Create a discovery task and allow the worker to fetch the details.
- Do not add a new route or task type.
- Continue to use the existing response envelope and error handling.

### 7. Consolidated data-acquisition entry

Collapse the separate acquisition buttons into one `获取数据` dropdown:

```text
获取数据
├── ID搜索
└── 关键词搜索
```

Rules:

- `获取数据` is the primary entry.
- `审核模式` and `刷新` remain separate actions.
- The manual dialog provides one clear primary action for the selected mode.
- Keyword search keeps its existing pagination and selected-result import flow.

### 8. Image enlargement

Covers become inspectable:

- Content-pool list cover.
- Content detail cover.
- Manual search result cover.

Use an image-viewer component or equivalent existing UI capability:

- Click opens the full image.
- Support zoom in and zoom out.
- Support rotation when the component provides it.
- Close returns to the previous workspace state.
- Show a `zoom-in` cursor for clickable images.
- Keep non-image fallback text non-clickable.

The viewer must not alter source data or create a stored asset.

### 9. Review-mode state

Rename `进入审核模式` to `审核模式`.

Inactive state:

- Outline button.
- Disabled when there is no pending item.

Active state:

- Primary filled button.
- Visible active treatment on the page or content card.
- Review progress remains visible inside the detail workspace.

Exit behavior:

- Provide a clear cancel/exit action while review mode is active.
- Closing the drawer or canceling review exits the queue without changing item status.
- Show feedback after exiting.
- Preserve the existing error-safe advancement behavior: failed operations do not advance; successful operations advance; skip does not change status.

### 10. Crawl-task display name

Generate the task display name as:

```text
策略名_YYYYMMDDHHmmss
```

Use Asia/Shanghai local time. For example, a task created at `2026-09-22 12:57:15 CST` displays as:

```text
王者荣耀新英雄_20260922125715
```

The strategy name is taken from the historical task snapshot when available. Historical tasks without a snapshot strategy name use the strategy table or `人工任务`. Renaming a strategy later must not rewrite the names of historical tasks.

## Architecture Boundaries

- Cloud remains the business fact center and owns discovery persistence.
- Public-platform search remains a Cloud-side capability.
- Desktop Agent is not involved in this feature.
- Repository code remains the only SQL access layer and continues to use explicit SQL.
- Service code normalizes platform payloads and enforces business rules.
- Handler code decodes requests and maps errors; it does not execute SQL.
- The database change is one additive migration on `source_contents`.
- No table is renamed or replaced.
- No Cloud–Agent contract changes.

## Verification

Backend:

- Douyin normalization tests for like, favorite, view, comment, share, author `uid`, author `sec_uid`, and generated homepage.
- Search tests ensure the raw platform object survives the response and import path.
- Import tests ensure `raw_json` stores the raw object and normalized results remain valid.
- Repository SQL tests cover new insert, list, and detail columns.
- Task-name tests cover the `YYYYMMDDHHmmss` format and historical snapshot behavior.
- ID-search tests confirm numeric IDs reuse the existing task path.
- Run `go test ./...` and `go vet ./...`.

Frontend:

- Verify title, author, strategy, task, and material links are blue and actionable.
- Verify external links open in new tabs with safe rel attributes.
- Verify author homepage uses the approved format.
- Verify list and detail show view, comment, and share counts.
- Verify image viewer opens and closes without breaking table or drawer state.
- Verify consolidated `获取数据` entry and ID-search validation.
- Verify review-mode active and inactive states, cancellation, and error-safe progression.
- Run `npm test` and `npm run build`.

Architecture checks:

- No prohibited package or dependency is introduced.
- No `web/dist-*` build artifact is modified.
- Only the approved additive database migration is added.
