# Content Mining UX Optimization Plan

- Date: 2026-09-22
- Design: `docs/superpowers/specs/2026-09-22-content-mining-ux-optimization-design.md`

## Status

- [x] Complete

## Steps

1. Backend: add `DeleteStrategy` to the discovery store interface, repository, service, handler, and route; add focused test.
2. Frontend API: add `deleteStrategy`.
3. Rewrite `DiscoveryStrategiesPage.vue`: stats cards, search/filter, new columns, keyword/author modals, business rule labels, five-action row with state control, sectioned editor, copy.
4. Rewrite `CrawlTasksPage.vue`: task naming, trigger type, business stats, clickable source strategy, detail drawer with four tabs, `重新执行`.
5. Add 120-character title truncation with tooltip.
6. Update `DiscoveryPages.test.js` and `ContentPoolPage.test.js`.
7. Run `go test ./...`, `go vet ./...`, `npm test`, `npm run build`, `git diff --check`.
8. Commit and restart Cloud API + Web.
