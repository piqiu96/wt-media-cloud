# Content Pool Discovery Hardening Design

- Date: 2026-09-21
- Status: Approved for P0 implementation
- Related plan: `docs/superpowers/plans/2026-09-21-content-pool-discovery-hardening.md`

## Background

The content-pool discovery closure completed the core strategy, task, source-content, and material loop. A product review identified two semantic gaps that must be corrected before further UI expansion.

## Approved Decisions

1. Keep the existing `AND` / `OR` automatic-material threshold rule. No rule engine or arbitrary expression support will be added.
2. Failed-item retry must create a new `crawl_task`. The original task remains immutable history.

## P0 Scope

### Retry Task Semantics

A retry is a new execution record, not an update of historical execution facts.

Add `crawl_tasks.parent_task_id`:

- `NULL` for normal strategy and manual tasks.
- Original task ID for a failed-item retry task.

When retrying:

1. Read the original task.
2. Authorize the actor against the original task team.
3. Allow retry only when the task has failed result items.
4. Extract only `failed` and `material_failed` items.
5. Create a new pending `retry_failed_task`.
6. Copy the original immutable snapshot.
7. Store only the failed items as retry inputs in the snapshot.
8. Do not modify the original task.
9. Worker claims and executes the new task.
10. Successful original items are not reprocessed.

The retry task records its own results and statistics. A source content keeps the task that originally discovered it. Material conversion can still be performed by a retry task and is recorded in that retry task's result.

### Task Status Semantics

Add `partial_success`:

- `success`: no failed items.
- `partial_success`: at least one failed item and at least one successfully processed item.
- `failed`: no successfully processed items.

Existing `pending` and `running` semantics remain unchanged.

### Frontend P0

- Display `partial_success` with a clear Chinese label.
- Show parent/child task relationship when present.
- Retry action creates a new task and navigates or links to that new task.
- Preserve AND/OR strategy controls.

## Non-Goals

- No combinable rule engine.
- No second content object.
- No second task platform.
- No author discovery implementation.
- No broad UI redesign.
- No full integration-test sweep.

## Verification

Focused tests must cover:

1. Retry creates a new task and leaves the original unchanged.
2. Only failed items are retried.
3. Successful items are not duplicated.
4. Partial success is classified correctly.
5. Repository reads and writes `parent_task_id`.
6. Existing AND/OR materialization behavior remains correct.

Real verification must read back:

- original `crawl_tasks`
- retry `crawl_tasks`
- `source_contents`
- `materials`
