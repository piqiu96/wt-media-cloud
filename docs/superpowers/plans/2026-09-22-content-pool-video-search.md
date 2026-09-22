# Content Pool Video Search Implementation Plan

- Date: 2026-09-22 16:55:04 CST
- Design: `docs/superpowers/specs/2026-09-22-content-pool-video-search-design.md`

## Steps

1. Extend focused backend tests for direct query search.
   - Numeric ID calls `FetchByIDs`.
   - Full Douyin URL extracts the ID and calls `FetchByIDs`.
   - Short link calls `FetchByURL`.
   - Multiple IDs are deduplicated and chunked into batches of ten.
   - Invalid input returns `ErrDiscoveryInvalid`.
2. Implement `dto.SearchInput.Query` and handler decoding.
3. Implement direct-query parsing, batching, normalization, and response assembly in `manual_search.go`.
4. Extend identity tests to prove operators can list teams but cannot create, rename, or delete teams.
5. Relax only `ListTeams` authentication to valid actors.
6. Update frontend static tests for the `视频搜索` entry, synchronous ID/link search, team locking, and full team-name display.
7. Update `ContentPoolPage.vue`:
   - Rename the acquisition entry.
   - Replace ID-task creation with direct search.
   - Show direct results and selected-result import.
   - Load teams for all roles and lock non-admin team selection.
8. Run focused backend tests, full backend tests and vet, frontend tests and build, then diff checks.
9. Review the diff, ensure unrelated files are excluded, and commit.
10. Restart only Cloud API and Cloud Web for manual verification.
