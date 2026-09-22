# Content Pool Discovery Hardening Plan

## Status

- [x] Complete

## Spec

`docs/superpowers/specs/2026-09-21-content-pool-discovery-hardening-design.md`

## Execution Rules

- Preserve the user's local `config/database/primary.toml` change.
- Keep changes focused on P0 semantics.
- Do not add compatibility behavior; the product is pre-release.
- Run only focused core tests.
- Do not modify build artifacts.

## Task 1: Data contract and migration

- [x] Add `20260921_033_crawl_task_retry_lineage.sql`.
- [x] Add `crawl_tasks.parent_task_id`.
- [x] Add parent-task index and foreign key.
- [x] Extend the status check constraint with `partial_success`.
- [x] Extend `model.CrawlTask`.
- [x] Extend repository insert/select/update SQL.

## Task 2: Retry service semantics

- [x] Add `CrawlPartialSuccess`.
- [x] Retry only tasks containing failed result items.
- [x] Extract `failed` and `material_failed` items.
- [x] Create a pending `retry_failed_task`.
- [x] Copy the original immutable snapshot and store retry inputs.
- [x] Set `parent_task_id` on the retry task.
- [x] Leave the original task unchanged.
- [x] Update focused service tests.

## Task 3: Worker execution and status

- [x] Detect `retry_failed_task` in worker execution.
- [x] Process only retry inputs.
- [x] Reuse the unified materialize service.
- [x] Classify success, partial success, and failure.
- [x] Preserve AND/OR threshold behavior.
- [x] Update focused worker/service tests.

## Task 4: Frontend P0

- [x] Display `partial_success`.
- [x] Display retry-task parent relationship.
- [x] Make retry action link to or open the new task.
- [x] Keep AND/OR controls unchanged.
- [x] Update focused page test.

## Task 5: Verification and handoff

- [x] Run focused Go tests.
- [x] Run focused frontend test.
- [x] Apply migration to the stable local database.
- [x] Restart backend and worker.
- [x] Execute a real failed-item retry.
- [x] Read back original task, retry task, source contents, and materials.
- [x] Record results in a handoff document.
- [x] Commit final state.

## Acceptance

- Original task history is immutable.
- Retry is represented by a new `crawl_tasks` row.
- Only failed items are retried.
- Successful source contents and materials are not duplicated.
- `partial_success` is represented end to end.
- AND/OR behavior remains available and tested.
