# Content Pool Discovery Closure Plan

## Status

In progress.

## Spec

`docs/superpowers/specs/2026-09-21-content-pool-discovery-closure-design.md`

## Implementation Baseline

- Date: 2026-09-21
- Repository: `wt-media-cloud`
- Module: `internal/modules/contentpool`
- Frontend module: `web/src/modules/contentpool`
- Existing branch: `main`
- Preserve the user's uncommitted local change in `config/database/primary.toml`.

## Execution Rules

- Work in small commits.
- Each task starts with a focused failing test where practical.
- Run only the tests named in this plan.
- Do not scan ignored, `dist`, or `node_modules` directories.
- Do not introduce compatibility layers for pre-release behavior.
- Do not add author discovery, a rule engine, or a second task system.
- Do not modify `web/dist-*` artifacts.

---

## Task 1: Extend data contracts and migration

**Files:**

- Create: `migrations/20260921_032_content_pool_discovery_link.sql`
- Modify: `internal/modules/contentpool/model/model.go`
- Modify: `internal/modules/contentpool/dto/dto.go`
- Modify: `internal/modules/contentpool/repository/store_mysql.go`
- Modify: `internal/modules/contentpool/repository/discovery_store_mysql.go`
- Modify: `internal/modules/contentpool/service/store_adapter.go`
- Test: `internal/modules/contentpool/repository/discovery_store_mysql_test.go`

**Steps:**

- [x] Add a migration for source-content strategy/task IDs, counts, audit note, failure reason, and material ID.
- [x] Extend `SourceContent` and `SourceInput`.
- [x] Extend source-content insert, select, update, and materialize SQL.
- [x] Extend task snapshot/result scan and persistence as needed.
- [x] Add focused repository assertions for new source fields.
- [x] Run `go test ./internal/modules/contentpool/repository`.
- [x] Commit with reference to this plan.

**Acceptance:**

- New source rows can retain strategy/task and metric fields.
- Materialization writes `material_id` and clears failure reason.
- Existing status model remains unchanged.

---

## Task 2: Implement strategy thresholds and immutable snapshots

**Files:**

- Modify: `internal/modules/contentpool/service/discovery.go`
- Modify: `internal/modules/contentpool/handler.go`
- Test: `internal/modules/contentpool/service/discovery_test.go`

**Steps:**

- [x] Validate `auto_material`, `material_rule`, `like_threshold`, and `favorite_threshold`.
- [x] Support only `AND` and `OR`.
- [x] Ignore thresholds less than or equal to zero.
- [x] Require at least one positive threshold when auto materialization is enabled.
- [x] Copy strategy ID, name, type, platform, schedule, and config into task snapshot.
- [x] Add tests for AND, OR, disabled thresholds, and invalid configuration.
- [x] Run `go test ./internal/modules/contentpool/service -run 'TestStrategy|TestCreateRun'`.
- [x] Commit.

**Acceptance:**

- A task snapshot is independent of later strategy edits.
- Invalid threshold configuration is rejected.
- Existing keyword strategy behavior remains available.

---

## Task 3: Project task results into content pool

**Files:**

- Modify: `internal/modules/contentpool/service/discovery.go`
- Modify: `internal/modules/contentpool/service/service.go`
- Modify: `internal/modules/contentpool/service/store_adapter.go`
- Test: `internal/modules/contentpool/service/discovery_test.go`
- Test: `internal/modules/contentpool/service/discovery_cloud_test.go`

**Steps:**

- [x] Extend crawler result normalization to retain like and favorite counts.
- [x] Deduplicate results by platform content ID.
- [x] Create or link source contents with strategy/task IDs.
- [x] Record per-item source ID, material ID, status, and failure reason in `result_json`.
- [x] Process items independently.
- [x] Extend task stats with auto-materialized and pending counts.
- [x] Add tests for result projection, duplicates, and partial failure.
- [x] Run `go test ./internal/modules/contentpool/service -run 'TestDiscovery|TestCreateRun'`.
- [x] Commit.

**Acceptance:**

- Every successfully discovered item has a stable source-content link or duplicate result.
- One failed item does not prevent other items from entering the pool.
- Task detail contains a per-item result list.

---

## Task 4: Automatic material conversion

**Files:**

- Modify: `internal/modules/contentpool/service/discovery.go`
- Modify: `internal/modules/contentpool/service/service.go`
- Test: `internal/modules/contentpool/service/discovery_test.go`

**Steps:**

- [ ] Evaluate thresholds using the task snapshot.
- [ ] Call the existing unified materialize service for qualifying items.
- [ ] Mark qualifying results as automatically materialized.
- [ ] Keep non-qualifying contents pending.
- [ ] Preserve source content when material conversion fails and record failure reason.
- [ ] Add tests for AND pass, OR pass, and threshold failure.
- [ ] Run `go test ./internal/modules/contentpool/service -run 'TestAutomaticMaterial|TestDiscovery'`.
- [ ] Commit.

**Acceptance:**

- Only one material per source content is created.
- Failed conversion does not delete the source content.
- Threshold decisions use the historical task snapshot.

---

## Task 5: Batch operations and failed retry API

**Files:**

- Modify: `internal/modules/contentpool/service/service.go`
- Modify: `internal/modules/contentpool/service/discovery.go`
- Modify: `internal/modules/contentpool/service/operations.go`
- Modify: `internal/modules/contentpool/handler.go`
- Modify: `internal/modules/contentpool/router.go`
- Test: `internal/modules/contentpool/service/service_test.go`
- Test: `internal/modules/contentpool/service/discovery_test.go`

**Steps:**

- [ ] Implement independent batch materialize results.
- [ ] Implement independent batch ignore/restore behavior where not already complete.
- [ ] Add `POST /crawl-tasks/:id/retry-failed`.
- [ ] Retry only failed result items.
- [ ] Prevent successful and already-materialized items from being reprocessed.
- [ ] Add tests for retry deduplication and partial retry failure.
- [ ] Run `go test ./internal/modules/contentpool/service -run 'TestBatch|TestRetry'`.
- [ ] Commit.

**Acceptance:**

- One failed batch item does not roll back successful items.
- Retry does not create duplicate source or material rows.
- Original task results and stats are updated in place.

---

## Task 6: Content Pool frontend closure

**Files:**

- Modify: `web/src/shared/api/contentPool.js`
- Modify: `web/src/modules/contentpool/pages/ContentPoolPage.vue`
- Test: `web/src/modules/contentpool/pages/ContentPoolPage.test.js`

**Steps:**

- [ ] Add batch materialize API client.
- [ ] Add metric and source columns.
- [ ] Add strategy/task links.
- [ ] Add processing status filter and stat-card filtering.
- [ ] Add batch materialize, ignore, and restore actions.
- [ ] Add detail drawer fields and audit note display.
- [ ] Add review mode with next-item behavior.
- [ ] Update focused page test.
- [ ] Run the focused frontend test or a single build check.
- [ ] Commit.

**Acceptance:**

- Operators can process pending contents without leaving the page.
- Source strategy and task are traceable.
- Review mode advances after materialize or ignore.

---

## Task 7: Discovery Strategy frontend

**Files:**

- Modify: `web/src/modules/contentpool/pages/DiscoveryStrategiesPage.vue`
- Modify: `web/src/shared/api/discovery.js`
- Test: `web/src/modules/contentpool/pages/DiscoveryPages.test.js`

**Steps:**

- [ ] Add automatic material switch.
- [ ] Add AND/OR selector.
- [ ] Add like and favorite thresholds.
- [ ] Show latest execution status and result summary.
- [ ] Add links to task list and content-pool filter.
- [ ] Keep author strategy disabled.
- [ ] Update focused page test.
- [ ] Run the focused frontend test.
- [ ] Commit.

**Acceptance:**

- Strategy form can express the approved threshold rule.
- Operators can see the latest execution outcome.
- Strategy changes do not rewrite historical task snapshots.

---

## Task 8: Crawl Task frontend

**Files:**

- Modify: `web/src/shared/api/discovery.js`
- Modify: `web/src/modules/contentpool/pages/CrawlTasksPage.vue`
- Test: `web/src/modules/contentpool/pages/DiscoveryPages.test.js`

**Steps:**

- [ ] Show strategy and immutable snapshot in task detail.
- [ ] Show automatic-material rule and thresholds.
- [ ] Show per-item result table with processing state.
- [ ] Add failed-item retry action.
- [ ] Link result rows to content-pool detail.
- [ ] Update focused page test.
- [ ] Run the focused frontend test.
- [ ] Commit.

**Acceptance:**

- Task detail explains what was found and how each item was processed.
- Failed items can be retried without duplicating successful rows.
- Task detail uses snapshot data rather than current strategy data.

---

## Task 9: Focused verification and handoff

**Files:**

- Modify: this plan and the linked spec if implementation decisions change.
- Create: `docs/superpowers/handoffs/2026-09-21-content-pool-discovery-closure-handoff.md`

**Steps:**

- [ ] Run `gofmt` on modified Go files.
- [ ] Run `go test ./internal/modules/contentpool/...`.
- [ ] Run the focused frontend tests.
- [ ] Start or restart the existing local Cloud backend and Web process.
- [ ] Validate the primary real-data loop:
  - create/update strategy,
  - run strategy,
  - verify task result,
  - verify source content,
  - verify automatic material,
  - open content pool.
- [ ] Record exact verification results and remaining risks in the handoff.
- [ ] Commit final state.

**Acceptance:**

- Core backend tests pass.
- Core frontend tests or build validation pass.
- Real data can be read back from `crawl_tasks`, `source_contents`, and `materials`.
- No fake success state is reported.

## Explicit Non-Tasks

- No author/blogger discovery implementation.
- No arbitrary rule combinations.
- No new task platform.
- No second content object.
- No broad architecture cleanup.
- No full integration test sweep.
