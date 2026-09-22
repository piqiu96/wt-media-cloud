# Content Pool Visual and Product Closure Plan

## Status

- [x] Complete

## Spec

`docs/superpowers/specs/2026-09-22-content-pool-visual-product-closure-design.md`

## Execution Rules

- Preserve the user's local `config/database/primary.toml` change.
- Do not add a database migration.
- Keep the existing API envelope, routes, and error codes.
- Keep repository code as the only SQL entry point.
- Do not modify `web/dist-*` build artifacts.
- Add focused tests before broad verification.

## Task 1: Read-model contract

- [x] Add source display fields to the response projection.
- [x] Add `material_id` to query filters.
- [x] Keep create/update persistence models separate from read display fields.
- [x] Add focused repository tests for joins and filtering.

## Task 2: Service and HTTP integration

- [x] Extend service list/get contracts with source display names.
- [x] Return historical strategy and task names.
- [x] Parse and authorize `material_id` list filtering.
- [x] Update focused service tests.

## Task 3: Content list experience

- [x] Add cover thumbnail to the title cell.
- [x] Replace raw strategy/task IDs with business names.
- [x] Keep status, metrics, source type, and timestamps readable.
- [x] Preserve pinned operations and current filters.

## Task 4: Detail workspace

- [x] Replace the 520px field-list drawer with a large responsive workspace.
- [x] Add cover, title, author, metrics, and external-link hierarchy.
- [x] Add source trace and processing-result panels.
- [x] Add material navigation for converted content.

## Task 5: Review workflow

- [x] Add pending total and current-index state.
- [x] Prevent advancing when materialize or ignore fails.
- [x] Show review errors inside the panel.
- [x] Support retry and skip without losing state.
- [x] Refresh list after completion.
- [x] Add focused frontend tests.

## Task 6: Verification and handoff

- [x] Run `go test ./internal/modules/contentpool/...`.
- [x] Run `cd web && npm test`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run architecture-boundary checks.
- [x] Review the final diff and preserve unrelated workspace changes.
- [x] Commit focused implementation changes.
- [x] Report completed behavior, verification, and remaining gaps.

## Acceptance

- Content list shows business-meaningful strategy and task names.
- Detail panel is a large operator workspace rather than a CRUD field table.
- Materialized content links to the material-library projection by `material_id`.
- Review mode displays progress and remains on the current item after failure.
- No migration, second content object, workflow engine, or iframe proxy is introduced.
