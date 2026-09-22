# Content Pool Discovery Data Completeness Plan

## Status

- [ ] In progress

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
- Use `20260922130225` as the reference timestamp in focused tests.

## Task 1: Data contract and migration

- [ ] Add view, comment, share, author `uid`, author `sec_uid`, and author homepage to the source model and input DTO.
- [ ] Extend the search result contract with interaction counts, author identifiers, homepage, and raw payload.
- [ ] Add the additive `source_contents` migration.
- [ ] Update focused model/DTO/migration expectations.

## Task 2: Discovery normalization and import traceability

- [ ] Normalize Douyin interaction counts and author identity fields.
- [ ] Generate the approved author homepage URL from `sec_uid`.
- [ ] Preserve the nested raw platform object through keyword search and import.
- [ ] Persist normalized fields on both synchronous import and strategy-discovery paths.
- [ ] Update focused normalization, import, and discovery tests.

## Task 3: Repository and task-name projection

- [ ] Extend explicit source insert and view SQL with new columns.
- [ ] Scan new nullable and count fields.
- [ ] Change crawl-task display names to `策略名_YYYYMMDDHHmmss`.
- [ ] Update focused repository SQL tests.

## Task 4: Content-pool interaction and visual workspace

- [ ] Make list, detail, and search titles open `source_url`.
- [ ] Apply the primary link style to all actionable columns.
- [ ] Add view/comment/share columns and detail metrics.
- [ ] Add author homepage and identity details.
- [ ] Add image enlargement for list, detail, and search covers.
- [ ] Add consolidated `获取数据` entry with ID search and keyword search.
- [ ] Validate and submit numeric IDs through the existing task path.
- [ ] Rename and visually distinguish active/inactive review mode.
- [ ] Add explicit review exit/cancel behavior.

## Task 5: Verification and handoff

- [ ] Run `go test ./internal/modules/contentpool/...`.
- [ ] Run `cd web && npm test`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `cd web && npm run build`.
- [ ] Run architecture-boundary checks.
- [ ] Review the final diff and preserve unrelated workspace changes.
- [ ] Commit focused implementation changes.
- [ ] Restart only Cloud API and Cloud Web for manual walkthrough.
- [ ] Report completed behavior, verification, and remaining gaps.
