# Content Pool Discovery Hardening Plan

## Status

- [ ] In progress

## Spec

`docs/superpowers/specs/2026-09-21-content-pool-discovery-hardening-design.md`

## Execution Rules

- Preserve the user's local `config/database/primary.toml` change.
- Keep changes focused on P0 semantics.
- Do not add compatibility behavior; the product is pre-release.
- Run only focused core tests.
- Do not modify build artifacts.

## Task 1: Data contract and migration

- [ ] Add `20260921_033_crawl_task_retry_lineage.sql`.
- [ ] Add `crawl_tasks.parent_task_id`.
- [ ] Add parent-task index and foreign key.
- [ ] Extend the status check constraint with `partial_success`.
- [ ] Extend `model.CrawlTask`.
- [ ] Extend repository insert/select/update SQL.

## Task 2: Retry service semantics

- [ ] Add `CrawlPartialSuccess`.
- [ ] Retry only tasks containing failed result items.
- [ ] Extract `failed` and `material_failed` items.
- [ ] Create a pending `retry_failed_task`.
- [ ] Copy the original immutable snapshot and store retry inputs.
- [ ] Set `parent_task_id` on the retry task.
- [ ] Leave the original task unchanged.
- [ ] Update focused service tests.

## Task 3: Worker execution and status

- [ ] Detect `retry_failed_task` in worker execution.
- [ ] Process only retry inputs.
- [ ] Reuse the unified materialize service.
- [ ] Classify success, partial success, and failure.
- [ ] Preserve AND/OR threshold behavior.
- [ ] Update focused worker/service tests.

## Task 4: Frontend P0

- [ ] Display `partial_success`.
- [ ] Display retry-task parent relationship.
- [ ] Make retry action link to or open the new task.
- [ ] Keep AND/OR controls unchanged.
- [ ] Update focused page test.

## Task 5: Verification and handoff

- [ ] Run focused Go tests.
- [ ] Run focused frontend test.
- [ ] Apply migration to the stable local database.
- [ ] Restart backend and worker.
- [ ] Execute a real failed-item retry.
- [ ] Read back original task, retry task, source contents, and materials.
- [ ] Record results in a handoff document.
- [ ] Commit final state.

## Acceptance

- Original task history is immutable.
- Retry is represented by a new `crawl_tasks` row.
- Only failed items are retried.
- Successful source contents and materials are not duplicated.
- `partial_success` is represented end to end.
- AND/OR behavior remains available and tested.
