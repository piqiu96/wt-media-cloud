# Content Pool Discovery Data Completeness Plan

## Status

- [x] Completed

## Spec

`docs/superpowers/specs/2026-09-22-content-pool-discovery-data-completeness-design.md`

## Execution Rules

- Preserve the user's local `config/database/primary.toml` change.
- Add only the approved additive migration.
- Keep the existing API envelope, routes, and error codes.
- Keep repository code as the only SQL entry point.
- Keep Cloud as the business fact center and do not involve Desktop Agent.
- Do not modify `web/dist-*` build artifacts.
- Add focused tests before broad verification.
- Use Asia/Shanghai local time for the `策略名_YYYYMMDDHHmmss` task display name.
- Use `20260922140725` as the reference timestamp in focused tests.

## Task 1: Data contract and migration

- [x] Add view, comment, share, author `uid`, author `sec_uid`, and author homepage to the source model and input DTO.
- [x] Extend the search result contract with interaction counts, author identifiers, homepage, and raw payload.
- [x] Add the additive `source_contents` migration.
- [x] Update focused model/DTO/migration expectations.

## Task 2: Discovery normalization and import traceability

- [x] Normalize Douyin interaction counts and author identity fields.
- [x] Generate the approved author homepage URL from `sec_uid`.
- [x] Preserve the nested raw platform object through keyword search and import.
- [x] Persist normalized fields on both synchronous import and strategy-discovery paths.
- [x] Update focused normalization, import, and discovery tests.

## Task 3: Repository and task-name projection

- [x] Extend explicit source insert and view SQL with new columns.
- [x] Scan new nullable and count fields.
- [x] Change crawl-task display names to `策略名_YYYYMMDDHHmmss`.
- [x] Update focused repository SQL tests.

## Task 4: Content-pool interaction and visual workspace

- [x] Make list, detail, and search titles open `source_url`.
- [x] Apply the primary link style to all actionable columns.
- [x] Add view/comment/share columns and detail metrics.
- [x] Add author homepage and identity details.
- [x] Add image enlargement for list, detail, and search covers.
- [x] Add consolidated `获取数据` entry with ID search and keyword search.
- [x] Validate and submit numeric IDs through the existing task path.
- [x] Rename and visually distinguish active/inactive review mode.
- [x] Add explicit review exit/cancel behavior.

## Task 5: Verification and handoff

- [x] Run `go test ./internal/modules/contentpool/...`.
- [x] Run `cd web && npm test`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `cd web && npm run build`.
- [x] Run architecture-boundary checks.
- [x] Review the final diff and preserve unrelated workspace changes.
- [x] Commit focused implementation changes.
- [x] Restart only Cloud API and Cloud Web for manual walkthrough.
- [x] Report completed behavior, verification, and remaining gaps.
