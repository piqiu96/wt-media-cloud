# Content Pool Discovery Hardening Handoff

- Date: 2026-09-22
- Status: Complete
- Spec: `docs/superpowers/specs/2026-09-21-content-pool-discovery-hardening-design.md`
- Plan: `docs/superpowers/plans/2026-09-21-content-pool-discovery-hardening.md`
- Verified at: 2026-09-22 10:39 CST

## Scope

This hardening corrects the two approved P0 semantics:

1. Keep the existing `AND` / `OR` automatic-material threshold rule.
2. Make failed-item retry a new `crawl_task` execution record while preserving the original task as immutable history.

It also adds the `partial_success` task status.

## Implementation

### Data Model

Migration:

- `migrations/20260921_033_crawl_task_retry_lineage.sql`

Changes:

- Added `crawl_tasks.parent_task_id`.
- Added `idx_crawl_tasks_parent`.
- Added a self foreign key to the original task.
- Extended the task status check with `partial_success`.

`model.CrawlTask` now exposes:

- `parent_task_id`

Task statuses now include:

- `pending`
- `running`
- `success`
- `partial_success`
- `failed`

### Repository

Updated:

- `internal/modules/contentpool/repository/discovery_store_mysql.go`
- `internal/modules/contentpool/repository/discovery_store_mysql_test.go`

The repository now reads and writes `parent_task_id` for crawl tasks.

### Retry Semantics

Updated:

- `internal/modules/contentpool/service/discovery.go`
- `internal/modules/contentpool/service/discovery_cloud_test.go`
- `internal/modules/contentpool/service/discovery_test.go`

Behavior:

- Retry is available for `failed` and `partial_success` tasks with failed result items.
- Only `failed` and `material_failed` items are copied into the retry snapshot.
- The original task is not modified.
- A new pending task is created with:
  - `task_type = retry_failed_task`
  - `parent_task_id = original task ID`
  - the original immutable strategy snapshot
  - only the failed retry inputs
- The worker claims and executes the retry task separately.
- Existing source contents and materials are not reprocessed.
- Material conversion still uses the unified materialize service.
- `AND` / `OR` threshold behavior is unchanged.

### Status Semantics

A completed task is classified as:

- `success`: no failed items.
- `partial_success`: at least one failed item and at least one processed item.
- `failed`: no processed items.

A crawler error with processed items also produces `partial_success`.

### Frontend

Updated:

- `web/src/modules/contentpool/pages/CrawlTasksPage.vue`
- `web/src/modules/contentpool/pages/DiscoveryPages.test.js`

Behavior:

- Displays `partial_success` as `部分成功`.
- Displays retry task type as `失败重试`.
- Shows and links the originating task through `parent_task_id`.
- Retry action now reports that a retry task has been created.
- Keeps the approved `AND` / `OR` strategy controls.

## Verification

### Migration

Passed:

```text
migration ok: 1 applied, 34 total
applied 20260921_033_crawl_task_retry_lineage crawl_task_retry_lineage
```

### Go Tests

Passed:

```text
go test ./internal/modules/contentpool/...
```

Focused retry and status tests also passed:

```text
go test ./internal/modules/contentpool/service -run 'TestRetry|TestDiscovery|TestWorker|TestAutomaticMaterialization|TestCrawlStatus'
go test ./internal/modules/contentpool/repository
```

### Frontend Test

Passed:

```text
npm --prefix web run test -- src/modules/contentpool/pages/DiscoveryPages.test.js
```

Result:

- 1 test file passed
- 2 tests passed

## Real-Data Verification

### Failed Task and New Retry Task

Original task:

```text
id=24
status=failed
task_type=manual_discovery_task
parent_task_id=NULL
scanned=2
found=0
duplicate=0
failed=2
result_count=2
```

Retry task:

```text
id=25
status=failed
task_type=retry_failed_task
parent_task_id=24
scanned=2
found=2
duplicate=0
failed=2
result_count=2
```

The external URLs remained invalid, so the retry correctly failed again. The original task 24 remained `failed` and retained its original statistics and results.

### Partial Success and Failed-Only Retry

Original task:

```text
id=27
status=partial_success
task_type=manual_discovery_task
parent_task_id=NULL
scanned=2
found=1
duplicate=1
failed=1
result_count=2
```

Results:

- One valid Douyin content was detected as duplicate.
- One invalid input failed.

Retry task:

```text
id=28
status=failed
task_type=retry_failed_task
parent_task_id=27
scanned=1
found=1
duplicate=0
failed=1
result_count=1
```

The retry task contained only the one failed input. The duplicate item was not retried. Original task 27 remained `partial_success` with unchanged statistics and results.

Database readback also confirmed:

- No new `source_contents` rows were created for tasks 24, 25, 27, or 28.
- No new materials were created for those tasks.
- This is expected because the valid input was already a duplicate and the invalid input failed.
- It verifies that retry did not duplicate the already-existing successful content.

## Runtime

At handoff, the local runtime was active:

- Backend: `http://127.0.0.1:18080`
- Web: `http://127.0.0.1:5176`
- Discovery worker: `go run ./cmd/discovery-worker`

## Known Limitations

- The real retry samples intentionally used an invalid external input, so the retry tasks correctly ended as failed.
- A successful material-failure retry is covered by focused service tests using the unified materialize service.
- Author discovery remains disabled until the external API is reopened.
- No combinable rule engine was added.
