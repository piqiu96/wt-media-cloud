# Content Pool Video Search and Team Read Access Design

- Date: 2026-09-22 16:55:04 CST
- Status: Approved
- Approved approach: Option A — synchronous ID/link search through the existing search API
- Related implementation plan: `docs/superpowers/plans/2026-09-22-content-pool-video-search.md`

## Background

The content pool already supports synchronous keyword search and asynchronous ID import tasks. Operators now need ID or share-link input to fetch actual Douyin video details immediately and preview them before importing. In addition, non-admin operators currently see their operation team as a numeric ID because team listing is restricted to administrators.

## Product Decision

Adopt synchronous direct fetch:

1. Replace the `获取数据` primary button with `视频搜索`.
2. Offer `ID/链接搜索` and `关键词搜索`.
3. `ID/链接搜索` supports:
   - Numeric Douyin video IDs, for example `7681569320895352104`.
   - Full video URLs, for example `https://www.douyin.com/video/7681569320895352104`.
   - Douyin short/share links, for example `https://v.douyin.com/xxxxx`.
   - Multiple entries separated by newline, English comma, Chinese comma, or whitespace.
4. Direct search shows results in the existing preview table and imports selected results through the existing import API.
5. Keyword search keeps its current pagination and import behavior.
6. Non-admin users can read operation teams for display purposes, but team management remains admin-only and non-admin users still cannot write outside their assigned team.

## API and Service Design

Reuse:

```http
POST /api/v1/content-pool/search
```

Add the optional `query` field to the existing request contract. When both `query` and `keyword` are supplied, `query` takes precedence.

The direct path:

1. Splits and deduplicates inputs.
2. Treats pure numeric tokens as IDs.
3. Extracts IDs from valid Douyin `/video/{id}` URLs and calls the batch `FetchByIDs` client method.
4. Calls `FetchByURL` for `v.douyin.com` short/share links.
5. Rejects arbitrary text and non-Douyin URLs with the existing invalid-input error.
6. Chunks IDs into groups of ten, the current client limit.
7. Caps direct targets at one hundred.
8. Reuses the existing Douyin normalizer so titles, source URLs, author identities, interaction counts, and complete raw payloads remain traceable.
9. Returns the existing `SearchResponse` envelope. Direct search has no pagination.

The existing keyword path and error semantics remain unchanged. No new route, database migration, Agent dependency, MQ, or workflow is introduced.

## Team Read Access

`GET /api/v1/operation-teams` becomes readable by any valid authenticated user. This is necessary to resolve a user's assigned team ID to its display name.

The following remain unchanged:

- Team creation, renaming, and deletion are administrator-only.
- The operation-team management menu remains administrator-only.
- Non-admin users are locked to their assigned team in the content import dialog.
- Content import continues to enforce team scope in the backend.

## Frontend Interaction

- Primary dropdown label: `视频搜索`.
- Direct mode label: `ID/链接搜索`.
- Input label: `ID/链接`.
- Before search, the primary action reads `搜索`; after results exist, it reads `加入内容池`.
- Direct results reuse the current search-result table.
- Pagination controls are shown only for keyword search.
- Selected direct results are imported through the existing import-results flow.
- Non-admin users see their team's full name in a disabled selector; administrators can select any team.

## Acceptance Criteria

1. Numeric ID, full video URL, and short/share link inputs all return synchronous normalized video details.
2. Multiple mixed inputs are deduplicated and fetched; IDs are batched correctly.
3. Invalid text or non-Douyin URLs return the existing invalid-input error.
4. Direct results preserve author and interaction fields and can be imported into the content pool.
5. Keyword pagination remains unchanged.
6. Non-admin users see full team names but cannot choose another team or manage teams.
7. Team management APIs and menu visibility remain admin-only.
8. Existing API envelope, routes, and database schema remain unchanged.
